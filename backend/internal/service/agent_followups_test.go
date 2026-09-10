package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"omnicraft/backend/internal/pkg/llm"
)

// SP-15 B #435 follow-up suite: the four contract states — only grounded turns
// carry follow_ups / join success / timeout abandons / parse failure stays
// silent. Generation is a speculative side call and must never change the
// turn outcome itself.

// followUpTestProvider scripts the streaming rounds and the non-streaming
// follow-up call independently of the async auto-title path.
type followUpTestProvider struct {
	rounds      [][]llm.ChatDelta
	chatReply   string
	chatErr     error
	chatDelay   time.Duration
	chatCalls   int
	streamCalls int
}

func (p *followUpTestProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	p.chatCalls++
	if p.chatDelay > 0 {
		time.Sleep(p.chatDelay)
	}
	if p.chatErr != nil {
		return nil, p.chatErr
	}
	return &llm.ChatResponse{Content: p.chatReply}, nil
}

func (p *followUpTestProvider) ChatStream(_ context.Context, _ llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	p.streamCalls++
	if p.streamCalls <= len(p.rounds) {
		for _, d := range p.rounds[p.streamCalls-1] {
			if err := handler(d); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *followUpTestProvider) GetEmbedding(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1}, nil
}

func followUpGroundedRounds() [][]llm.ChatDelta {
	return [][]llm.ChatDelta{
		{toolCallDelta("search_content", `{"query":"配色练习 原创"}`)},
		{{Content: "这是带引用的回答 [1]，介绍配色练习相关内容。"}, {Done: true}},
	}
}

func runFollowUpTurn(t *testing.T, svc *AgentService, provider *followUpTestProvider) *AgentStreamEvent {
	t.Helper()
	var done *AgentStreamEvent
	err := svc.ChatStream(context.Background(), 7, ChatTurnInput{Message: "站内有哪些配色练习内容？"},
		resolveGlobalChatContext(t, svc, 7),
		func(ev AgentStreamEvent) error {
			if ev.Type == AgentEventDone {
				done = &ev
			}
			return nil
		})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if done == nil {
		t.Fatal("done event missing")
	}
	return done
}

func followUpTestService(t *testing.T, provider *followUpTestProvider) *AgentService {
	t.Helper()
	cfg := conversationalLaneTestConfig()
	cfg.Features.RAGHybridEnabled = true
	svc, _ := newStreamTestService(t, provider, cfg)
	// 复用 #434 套件的检索桩：chunk key 匹配种子 RagChunk，引用复验通过 → grounded。
	svc.hybridRetriever = &queryUnderstandingRetriever{}
	return svc
}

// TestFollowUpsJoinSuccessOnGroundedTurn locks state ②: a grounded turn whose
// speculative call returns valid lines carries them on done (over-cap lines
// dropped, order kept).
func TestFollowUpsJoinSuccessOnGroundedTurn(t *testing.T) {
	reply := "还有类似的配色练习吗\n水彩和彩铅哪个适合新手\n" + strings.Repeat("超", 30) + "\n这篇的作者还有别的作品吗"
	provider := &followUpTestProvider{rounds: followUpGroundedRounds(), chatReply: reply}
	svc := followUpTestService(t, provider)

	done := runFollowUpTurn(t, svc, provider)

	if done.AnswerKind != AgentAnswerGroundedContent {
		t.Fatalf("answer_kind = %q, want grounded_content (fixture drift)", done.AnswerKind)
	}
	want := []string{"还有类似的配色练习吗", "水彩和彩铅哪个适合新手", "这篇的作者还有别的作品吗"}
	if len(done.FollowUps) != 3 {
		t.Fatalf("follow_ups = %v, want the 3 valid lines (30-rune line dropped)", done.FollowUps)
	}
	for i := range want {
		if done.FollowUps[i] != want[i] {
			t.Fatalf("follow_ups[%d] = %q, want %q", i, done.FollowUps[i], want[i])
		}
	}
}

// TestFollowUpsNotAttachedOnConversationalTurn locks state ①: a zero-tool
// conversational turn never starts generation and its done carries no
// follow_ups.
func TestFollowUpsNotAttachedOnConversationalTurn(t *testing.T) {
	reply := "你好，想找站内的作品还是了解用法？可以告诉我题材或关键词。"
	provider := &followUpTestProvider{
		rounds:    [][]llm.ChatDelta{{{Content: reply}, {Done: true}}},
		chatReply: "不该出现的追问",
	}
	svc := followUpTestService(t, provider)

	done := runFollowUpTurn(t, svc, provider)

	if done.AnswerKind != AgentAnswerConversational {
		t.Fatalf("answer_kind = %q, want conversational (fixture drift)", done.AnswerKind)
	}
	if len(done.FollowUps) != 0 {
		t.Fatalf("conversational done must carry no follow_ups: %v", done.FollowUps)
	}
}

// TestFollowUpsTimeoutAbandonedSilently locks state ③: when the speculative
// call exceeds the remaining budget, done proceeds without follow_ups and the
// turn outcome is unchanged.
func TestFollowUpsTimeoutAbandonedSilently(t *testing.T) {
	original := followUpBudget
	followUpBudget = 60 * time.Millisecond
	t.Cleanup(func() { followUpBudget = original })

	provider := &followUpTestProvider{rounds: followUpGroundedRounds(), chatReply: "合法的问题", chatDelay: 500 * time.Millisecond}
	svc := followUpTestService(t, provider)

	started := time.Now()
	done := runFollowUpTurn(t, svc, provider)
	elapsed := time.Since(started)

	if done.AnswerKind != AgentAnswerGroundedContent {
		t.Fatalf("answer_kind = %q, want grounded_content", done.AnswerKind)
	}
	if len(done.FollowUps) != 0 {
		t.Fatalf("timed-out follow-ups must be abandoned, got %v", done.FollowUps)
	}
	if elapsed > 400*time.Millisecond {
		t.Fatalf("join blocked %v, want bounded by the shortened budget", elapsed)
	}
	if done.Answer == "" {
		t.Fatal("grounded answer must stay visible despite the follow-up miss")
	}
}

// TestFollowUpsParseFailureStaysSilent locks state ④: garbage output (empty
// or over-long lines) yields no follow_ups while the rest of the done contract
// is intact.
func TestFollowUpsParseFailureStaysSilent(t *testing.T) {
	provider := &followUpTestProvider{rounds: followUpGroundedRounds(), chatReply: "这是一段超过二十个字符长度的失败解析输出没有一行合法"}
	svc := followUpTestService(t, provider)

	done := runFollowUpTurn(t, svc, provider)

	if done.AnswerKind != AgentAnswerGroundedContent {
		t.Fatalf("answer_kind = %q, want grounded_content", done.AnswerKind)
	}
	if len(done.FollowUps) != 0 {
		t.Fatalf("unparseable output must yield no follow_ups: %v", done.FollowUps)
	}
	if done.Answer == "" || len(done.Citations) == 0 {
		t.Fatalf("done contract must stay intact: answer=%.20q citations=%d", done.Answer, len(done.Citations))
	}
}

// TestParseFollowUpItems unit-locks the line normalizer: markers stripped,
// empty/dup/over-cap dropped, cap 3.
func TestParseFollowUpItems(t *testing.T) {
	got := parseFollowUpItems("- 还有没有别的？\n*水彩教程在哪\n\n重复行\n重复行\n第四行会不会进\n第五行超了不会进吗不会")
	want := []string{"还有没有别的？", "水彩教程在哪", "重复行"}
	if len(got) != len(want) {
		t.Fatalf("items = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("items[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if parseFollowUpItems("") != nil || parseFollowUpItems("\n\n  \n") != nil {
		t.Fatal("empty output must parse to nil")
	}
	if got := parseFollowUpItems(strings.Repeat("长", 21)); got != nil {
		t.Fatalf("21-rune line over the cap must drop, got %v", got)
	}
}
