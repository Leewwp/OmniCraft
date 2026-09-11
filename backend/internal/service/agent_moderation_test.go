package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/llm"
)

// fakeAgentGreenScanner stands in for the Green text scan seam: it records
// every scanned text and returns a canned result or error.
type fakeAgentGreenScanner struct {
	result       string
	scanErr      error
	scannedTexts []string
}

func (f *fakeAgentGreenScanner) TextModeration(_ context.Context, text string) (*aliyun.GreenScanResult, error) {
	f.scannedTexts = append(f.scannedTexts, text)
	if f.scanErr != nil {
		return nil, f.scanErr
	}
	return &aliyun.GreenScanResult{Result: f.result}, nil
}

func releaseModerationConfig() *config.Config {
	cfg := continuationTestConfig(100000)
	cfg.Server.Mode = "release"
	return cfg
}

// A-05 input gate: an explicit "block" verdict rejects the chat input before
// the turn starts.
func TestModerateChatInputBlocksFlaggedInput(t *testing.T) {
	svc, _ := newStreamTestService(t, nil, continuationTestConfig(100000))
	svc.SetGreenScanner(&fakeAgentGreenScanner{result: "block"})

	err := svc.ModerateChatInput(context.Background(), "违规输入")
	require.ErrorIs(t, err, ErrAgentInputBlocked)
}

// A-05 input gate: the provider vocabulary is normalized like every other
// text gate — a raw "violation" verdict is a block.
func TestModerateChatInputNormalizesViolationToBlock(t *testing.T) {
	svc, _ := newStreamTestService(t, nil, continuationTestConfig(100000))
	svc.SetGreenScanner(&fakeAgentGreenScanner{result: "violation"})

	err := svc.ModerateChatInput(context.Background(), "违规输入")
	require.ErrorIs(t, err, ErrAgentInputBlocked)
}

// A-05 input gate: clean input passes without error.
func TestModerateChatInputPassesCleanInput(t *testing.T) {
	svc, _ := newStreamTestService(t, nil, continuationTestConfig(100000))
	svc.SetGreenScanner(&fakeAgentGreenScanner{result: "pass"})

	require.NoError(t, svc.ModerateChatInput(context.Background(), "正常问题"))
}

// A-05 / A4: without a configured Green client the local/test environment
// fails open — chat stays usable and the skip is observable in logs.
func TestModerateChatInputFailsOpenWhenGreenNotConfigured(t *testing.T) {
	svc, _ := newStreamTestService(t, nil, continuationTestConfig(100000))

	require.NoError(t, svc.ModerateChatInput(context.Background(), "任何输入"))
}

// A-05 / A4: outside release mode the explicit not-configured signal is the
// only fail-open path (chat stays usable without local credentials).
func TestModerateChatInputFailsOpenWhenGreenNotConfiguredSignal(t *testing.T) {
	svc, _ := newStreamTestService(t, nil, continuationTestConfig(100000))
	svc.SetGreenScanner(&fakeAgentGreenScanner{scanErr: aliyun.ErrGreenNotConfigured})

	require.NoError(t, svc.ModerateChatInput(context.Background(), "任何输入"))
}

// A-05 / A4: any other scan error fails closed in every environment, mirroring
// the other text gates — a half-available scanner must not silently admit
// input.
func TestModerateChatInputFailsClosedOnScanErrorOutsideRelease(t *testing.T) {
	svc, _ := newStreamTestService(t, nil, continuationTestConfig(100000))
	svc.SetGreenScanner(&fakeAgentGreenScanner{scanErr: errors.New("green down")})

	err := svc.ModerateChatInput(context.Background(), "任何输入")
	require.ErrorIs(t, err, ErrAgentModerationUnavailable)
}

// A-05 / A4: in release mode any moderation failure fails closed — the chat
// turn is rejected with the unavailable error instead of skipping the gate.
func TestModerateChatInputFailsClosedInReleaseOnScanError(t *testing.T) {
	svc, _ := newStreamTestService(t, nil, releaseModerationConfig())
	svc.SetGreenScanner(&fakeAgentGreenScanner{scanErr: errors.New("green down")})

	err := svc.ModerateChatInput(context.Background(), "任何输入")
	require.ErrorIs(t, err, ErrAgentModerationUnavailable)
}

// A-05 input gate: blank text never triggers an external scan.
func TestModerateChatInputSkipsBlankText(t *testing.T) {
	svc, _ := newStreamTestService(t, nil, continuationTestConfig(100000))
	green := &fakeAgentGreenScanner{result: "block"}
	svc.SetGreenScanner(green)

	require.NoError(t, svc.ModerateChatInput(context.Background(), "   "))
	require.Empty(t, green.scannedTexts, "blank input must not be scanned")
}

// A-05 output audit: after a completed turn the persisted assistant answer is
// audited asynchronously; a blocked verdict flags the stored row (audit copy
// of the text stays stored) without affecting the stream outcome.
func TestChatStreamFlagsBlockedAnswerAsynchronously(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("get_content_detail", `{"content_id":88}`)},
		{{Content: "Grounded answer about Published Test Content"}, {Done: true}},
	}}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(100000))
	svc.SetGreenScanner(&fakeAgentGreenScanner{result: "block"})

	require.NoError(t, svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "what is this"},
		resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error { return nil }))

	require.Eventually(t, func() bool {
		var rows []model.AgentMessage
		if err := db.Where("role = ?", "assistant").Find(&rows).Error; err != nil {
			return false
		}
		for _, row := range rows {
			if row.ToolCalls != nil && row.ToolCalls["moderation"] == "blocked" {
				return true
			}
		}
		return false
	}, 5*time.Second, 20*time.Millisecond, "blocked answer must be flagged asynchronously")

	var flagged model.AgentMessage
	require.NoError(t, db.Where("role = ? AND tool_calls IS NOT NULL", "assistant").First(&flagged).Error)
	require.NotNil(t, flagged.Content, "raw answer stays stored for audit")
	require.Contains(t, *flagged.Content, "Grounded answer")
}

// A-05 output audit: a clean answer is scanned but never flagged.
func TestChatStreamLeavesCleanAnswerUnflagged(t *testing.T) {
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta("get_content_detail", `{"content_id":88}`)},
		{{Content: "Grounded answer about Published Test Content"}, {Done: true}},
	}}
	svc, db := newStreamTestService(t, provider, continuationTestConfig(100000))
	green := &fakeAgentGreenScanner{result: "pass"}
	svc.SetGreenScanner(green)

	require.NoError(t, svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "what is this"},
		resolveGlobalChatContext(t, svc, 7), func(ev AgentStreamEvent) error { return nil }))

	require.Eventually(t, func() bool { return len(green.scannedTexts) > 0 }, 5*time.Second, 20*time.Millisecond,
		"output audit must run after the turn")
	var rows []model.AgentMessage
	require.NoError(t, db.Where("role = ?", "assistant").Find(&rows).Error)
	for _, row := range rows {
		if row.ToolCalls != nil {
			require.NotEqual(t, "blocked", row.ToolCalls["moderation"], "clean answer must not be flagged")
		}
	}
}
