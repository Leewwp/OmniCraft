package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

// ToolGenerateImage creates one illustration for the user's request and
// returns a platform-owned OSS URL (SP-23 M1, #566). The external provider
// URL never leaves this file: bytes are re-uploaded to the platform bucket
// first. The tool only enters the model's tool list when
// agent.image.ImageConfigured() holds (switch on + key present).
const ToolGenerateImage = "generate_image"

// AgentImageResult is the tool outcome handed back to the model and the SSE
// tool-status surface: only platform-owned coordinates, never the provider
// URL.
type AgentImageResult struct {
	URL        string  `json:"url"`
	OSSKey     string  `json:"oss_key"`
	Size       string  `json:"size"`
	DurationMs int64   `json:"duration_ms"`
	CostCNY    float64 `json:"cost_cny"`
}

// AgentImageStore is the storage seam: production wires the aliyun OSS
// adapter (agent-images/<conversation>/<id>.png, signed GET URL); tests run
// an in-memory stub. A nil store keeps the tool unregistered.
type AgentImageStore interface {
	PutAgentImage(ctx context.Context, key string, r io.Reader) error
	SignedAgentImageURL(ctx context.Context, key string) (string, error)
}

// ErrAgentImageQuotaExceeded is the budget-cap signal: the conversation
// already consumed its per-session image allowance. The tool result carries
// a stable degradation code so the model relays it instead of retrying.
var ErrAgentImageQuotaExceeded = errors.New("agent image quota exceeded")

type generateImageToolArgs struct {
	Prompt string `json:"prompt"`
	Size   string `json:"size,omitempty"`
}

func (s *AgentService) imageToolAvailable() bool {
	return s.agentImageCfg.ImageConfigured() && s.agentImageGen != nil && s.agentImageStore != nil
}

// imageSizesAllowed folds the configured size whitelist into a lookup set.
func (s *AgentService) imageSizesAllowed() map[string]bool {
	out := make(map[string]bool, len(s.agentImageCfg.SizeOptions))
	for _, opt := range s.agentImageCfg.SizeOptions {
		out[strings.TrimSpace(opt)] = true
	}
	return out
}

func (s *AgentService) toolGenerateImage(ctx context.Context, rawArgs json.RawMessage, scope agentToolScope) (*AgentToolOutcome, error) {
	var args generateImageToolArgs
	if err := decodeToolArgs(rawArgs, &args); err != nil {
		return nil, err
	}
	prompt := strings.TrimSpace(args.Prompt)
	if prompt == "" || len([]rune(prompt)) > defaultMaxToolQueryLength {
		return nil, ErrAgentToolInvalidArgs
	}
	size := strings.TrimSpace(args.Size)
	if size == "" {
		size = s.agentImageCfg.SizeDefault
	}
	if !s.imageSizesAllowed()[size] {
		size = s.agentImageCfg.SizeDefault
	}

	// Per-conversation budget = persisted generate_image steps from earlier
	// turns (#538 tool phase rows are the durable record) + images already
	// generated in the live turn (those rows only land at end of turn).
	used, err := s.conversationImageCount(ctx, scope.ConversationID)
	if err != nil {
		return nil, err
	}
	if used+scope.TurnImages >= s.agentImageCfg.SessionImageLimit {
		// Stable degradation code, not a raw error string: the model relays
		// the cap to the user instead of retrying the call.
		return &AgentToolOutcome{
			Image: &AgentImageResult{},
			Execution: AgentToolExecution{
				Name: ToolGenerateImage, Status: AgentToolStatusError,
				ArgsSummary: quotaSummary(prompt),
			},
		}, ErrAgentImageQuotaExceeded
	}

	timeout := time.Duration(s.agentImageCfg.TimeoutSec) * time.Second
	genCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	imageBytes, err := s.agentImageGen.GenerateImage(genCtx, prompt, size, s.agentImageCfg.MaxImageBytes)
	if err != nil {
		return nil, fmt.Errorf("image generation: %w", err)
	}
	duration := time.Since(start).Milliseconds()

	key := agentImageObjectKey(scope.ConversationID)
	if err := s.agentImageStore.PutAgentImage(ctx, key, bytes.NewReader(imageBytes)); err != nil {
		return nil, fmt.Errorf("image upload: %w", err)
	}
	url, err := s.agentImageStore.SignedAgentImageURL(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("image url sign: %w", err)
	}
	result := &AgentImageResult{
		URL: url, OSSKey: key, Size: size, DurationMs: duration,
		CostCNY: s.agentImageCfg.PricePerImageCNY,
	}
	return &AgentToolOutcome{
		Image: result,
		Execution: AgentToolExecution{
			Name: ToolGenerateImage, Status: AgentToolStatusSuccess, Hits: 1,
			ArgsSummary: truncateRunes(prompt, 80) + " [" + size + "]",
			DurationMs:  duration,
		},
	}, nil
}

// conversationImageCount counts prior generate_image executions persisted in
// the conversation's tool phase rows. Zero when the DB seam is absent
// (unit tests) — the caller still enforces the cap from the live turn.
func (s *AgentService) conversationImageCount(ctx context.Context, conversationID int64) (int, error) {
	if s.db == nil || conversationID <= 0 {
		return 0, nil
	}
	var rows []model.AgentMessage
	if err := s.db.WithContext(ctx).
		Where("conversation_id = ? AND role = ?", conversationID, "assistant").
		Find(&rows).Error; err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		phase, _ := row.ToolCalls["phase"]
		if phase != "tools" {
			continue
		}
		steps, _ := row.ToolCalls["steps"].([]any)
		for _, step := range steps {
			m, ok := step.(map[string]any)
			if !ok {
				continue
			}
			if name, _ := m["name"].(string); name == ToolGenerateImage {
				count++
			}
		}
	}
	return count, nil
}

// agentImageObjectKey mints agent-images/<conversation>/<random>.png. The
// random component is server-side (never model- or user-controlled) so a
// generated image can never overwrite another object.
func agentImageObjectKey(conversationID int64) string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failure is catastrophic; fall back to a time-based id
		// rather than silently disabling the tool.
		return fmt.Sprintf("agent-images/%d/%d.png", conversationID, time.Now().UnixNano())
	}
	return fmt.Sprintf("agent-images/%d/%s.png", conversationID, hex.EncodeToString(buf))
}

func quotaSummary(prompt string) string {
	return truncateRunes(prompt, 80) + " [quota]"
}

// agentImageProviderFromConfig builds the production generator; nil when the
// config is off (container uses this to keep the tool unregistered).
func agentImageProviderFromConfig(cfg config.AgentImageConfig) llm.AgentImageGenerator {
	if !cfg.ImageConfigured() {
		return nil
	}
	return llm.NewCogViewClient(cfg.APIBase, cfg.APIKey, cfg.Model)
}
