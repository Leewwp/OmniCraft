package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/pkg/recovery"
)

// ErrAgentConversationNotFound is the owner-scoped miss for chat
// continuation: a foreign or missing conversation id is indistinguishable so
// existence cannot be probed.
var ErrAgentConversationNotFound = errors.New("agent conversation not found")

// AgentErrorCodeConversationNotFound marks a continuation whose conversation
// disappeared between the pre-quota ownership gate and the stream start (for
// example the user deleted it in another tab). The reservation still counts
// as consumed, matching every other outcome after reservation.
const AgentErrorCodeConversationNotFound = "AGENT_CONVERSATION_NOT_FOUND"

// ChatTurnInput is the per-turn chat request. The client submits only the new
// message; conversation history assembly is server-side (A-01).
type ChatTurnInput struct {
	// ConversationID continues an existing conversation; zero starts a new
	// one whose id is returned in the start event.
	ConversationID int64
	Message        string
}

const (
	// ConversationTitleMaxRunes bounds conversation titles for both the
	// auto-generated store path and the PATCH rename validation. It stays
	// below the VARCHAR(200) column limit (which counts characters, not
	// bytes) to keep titles comfortably single-line.
	ConversationTitleMaxRunes = 50
	// conversationTitlePromptCap bounds the excerpt handed to the title call.
	conversationTitlePromptCap = 500
)

var thinkBlockPattern = regexp.MustCompile(`(?s)<think>.*?</think>`)

// persistPartialTurn stores the already-streamed partial answer (if any)
// after a failed or cancelled turn and refreshes the conversation activity
// timestamp. It runs on a detached bounded context because the request
// context is usually canceled by the disconnect that caused the partial turn.
func (s *AgentService) persistPartialTurn(conversationID int64, partial string) {
	if s.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if partial != "" {
		content := partial
		if err := s.db.WithContext(ctx).Create(&model.AgentMessage{
			ConversationID: conversationID,
			Role:           "assistant",
			Content:        &content,
			CreatedAt:      time.Now(),
		}).Error; err != nil {
			slog.Error("failed to persist partial agent answer", "conversation_id", conversationID, "error", err)
		}
	}
	if err := s.db.WithContext(ctx).Model(&model.AgentConversation{}).
		Where("id = ?", conversationID).
		Update("updated_at", time.Now()).Error; err != nil {
		slog.Error("failed to update agent conversation timestamp after partial turn", "conversation_id", conversationID, "error", err)
	}
}

// EnsureConversationOwned is the pre-quota ownership gate for chat
// continuation: foreign or missing conversations are rejected here, before
// any quota reservation or Provider work, so they cannot be probed and never
// consume a request.
func (s *AgentService) EnsureConversationOwned(ctx context.Context, userID, conversationID int64) error {
	if s.db == nil || conversationID <= 0 {
		return ErrAgentConversationNotFound
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.AgentConversation{}).
		Where("id = ? AND user_id = ?", conversationID, userID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrAgentConversationNotFound
	}
	return nil
}

// resolveChatConversation loads (or creates) the owner-scoped conversation,
// appends the turn's user message, and returns the full stored history in
// chronological order. hadAssistantBefore reports whether the conversation
// already carried an assistant answer before this turn — the auto-title
// trigger (legacy rows always have one, because the pre-A-01 code deleted
// failed conversations, so legacy rows are never re-titled).
func (s *AgentService) resolveChatConversation(ctx context.Context, userID int64, turn ChatTurnInput, contextType string, contextID *int64) (*model.AgentConversation, []model.AgentMessage, bool, error) {
	userContent := turn.Message

	// The provider seam allows a DB-less AgentService (e.g. the streaming-only
	// adapter test): the turn runs ephemeral, without persistence.
	if s.db == nil {
		if turn.ConversationID > 0 {
			return nil, nil, false, ErrAgentConversationNotFound
		}
		conv := &model.AgentConversation{UserID: userID, ContextType: contextType, ContextID: contextID}
		history := []model.AgentMessage{{ConversationID: 0, Role: "user", Content: &userContent, CreatedAt: time.Now()}}
		return conv, history, false, nil
	}

	conv := &model.AgentConversation{}
	var history []model.AgentMessage

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if turn.ConversationID > 0 {
			if err := tx.Where("id = ? AND user_id = ?", turn.ConversationID, userID).First(conv).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrAgentConversationNotFound
				}
				return err
			}
		} else {
			conv.UserID = userID
			conv.ContextType = contextType
			conv.ContextID = contextID
			conv.CreatedAt = time.Now()
			conv.UpdatedAt = time.Now()
			if err := tx.Create(conv).Error; err != nil {
				return err
			}
		}

		if err := tx.Create(&model.AgentMessage{
			ConversationID: conv.ID,
			Role:           "user",
			Content:        &userContent,
			CreatedAt:      time.Now(),
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.AgentConversation{}).Where("id = ?", conv.ID).Update("updated_at", time.Now()).Error; err != nil {
			return err
		}
		return tx.Where("conversation_id = ?", conv.ID).Order("created_at ASC, id ASC").Find(&history).Error
	})
	if err != nil {
		return nil, nil, false, err
	}

	hadAssistantBefore := false
	for _, msg := range history {
		if msg.Role == "assistant" {
			hadAssistantBefore = true
			break
		}
	}
	return conv, history, hadAssistantBefore, nil
}

// estimateChatTokens returns a conservative token estimate for CJK-heavy
// content: one rune ≈ one token. ASCII text is over-counted, which only makes
// the budget stricter — never looser.
func estimateChatTokens(s string) int { return len([]rune(s)) }

// assembleChatContext builds the provider message list: the server-owned
// system prompt plus the newest conversation history that fits the token
// budget. Exchanges are admitted newest-first; a single over-long message is
// tail-truncated (head kept) at half the budget so one huge turn cannot evict
// everything else. The newest message is always included even when it alone
// saturates the budget.
func assembleChatContext(system llm.ChatMessage, history []model.AgentMessage, tokenBudget, maxMessages int) []llm.ChatMessage {
	type assembledMessage struct {
		role    string
		content string
	}
	perMessageCap := tokenBudget / 2
	if perMessageCap < 1 {
		perMessageCap = 1
	}

	var kept []assembledMessage
	used := 0
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		content := ""
		if msg.Content != nil {
			content = *msg.Content
		}
		if estimateChatTokens(content) > perMessageCap {
			runes := []rune(content)
			content = string(runes[:perMessageCap-1]) + "…"
		}
		cost := estimateChatTokens(content)
		if len(kept) >= maxMessages {
			break
		}
		if len(kept) > 0 && used+cost > tokenBudget {
			break
		}
		kept = append(kept, assembledMessage{role: msg.Role, content: content})
		used += cost
	}

	out := make([]llm.ChatMessage, 0, len(kept)+1)
	out = append(out, system)
	for i := len(kept) - 1; i >= 0; i-- {
		out = append(out, llm.ChatMessage{Role: kept[i].role, Content: kept[i].content})
	}
	return out
}

// scheduleAutoTitle generates and stores the conversation title after the
// first completed turn. The write only lands while title IS NULL, so later
// turns and concurrent turns can never overwrite an established title.
func (s *AgentService) scheduleAutoTitle(traceID string, conversationID int64, firstUserText string) {
	if s.db == nil {
		return
	}
	recovery.GoSafe(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		title := s.generateConversationTitle(ctx, firstUserText)
		if title == "" {
			return
		}
		if err := s.db.WithContext(ctx).Model(&model.AgentConversation{}).
			Where("id = ? AND title IS NULL", conversationID).
			Update("title", title).Error; err != nil {
			slog.Error("failed to store agent conversation title", "conversation_id", conversationID, "error", err)
			traceAgentEvent(traceID, "conversation_title_store_failed", "conversation_id", conversationID)
		}
	})
}

// generateConversationTitle makes one short non-streaming call for a summary
// title; on any failure it falls back to a truncation of the first user
// message. Title generation is a side effect of a completed turn and never
// consumes chat quota.
func (s *AgentService) generateConversationTitle(ctx context.Context, firstUserMessage string) string {
	fallback := truncateChatRunes(strings.TrimSpace(firstUserMessage), ConversationTitleMaxRunes)
	if s.llmProvider == nil {
		return fallback
	}
	prompt := fmt.Sprintf(
		"为下面的对话生成一个不超过 16 个字的简短中文标题，只输出标题本身，不要引号、序号或句号：\n%s",
		truncateChatRunes(firstUserMessage, conversationTitlePromptCap),
	)
	resp, err := s.llmProvider.Chat(ctx, llm.ChatRequest{
		Messages:  []llm.ChatMessage{{Role: "user", Content: prompt}},
		MaxTokens: 64,
	})
	if err != nil || resp == nil {
		return fallback
	}
	title := sanitizeConversationTitle(resp.Content)
	if title == "" {
		return fallback
	}
	return title
}

// sanitizeConversationTitle strips any provider reasoning block, collapses
// whitespace and caps the length to the stored-title bound.
func sanitizeConversationTitle(raw string) string {
	title := thinkBlockPattern.ReplaceAllString(raw, "")
	title = strings.Join(strings.Fields(strings.TrimSpace(title)), " ")
	return truncateChatRunes(title, ConversationTitleMaxRunes)
}

func truncateChatRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// firstUserMessage returns the oldest stored user message of the turn's
// history. The auto-title summarises the conversation's actual first
// question, which after a failed first turn differs from the current turn's
// message.
func firstUserMessage(history []model.AgentMessage) string {
	for _, msg := range history {
		if msg.Role == "user" && msg.Content != nil {
			return *msg.Content
		}
	}
	return ""
}
