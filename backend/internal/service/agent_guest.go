package service

// #854 anonymous agent surface — service layer: guest conversation
// ownership/expiry, the public read-only tool whitelist (declaration AND
// execution side), and the shared ChatStream integration points. The guest
// surface's budget and identity live in middleware/guest_agent.go.

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

// ErrAgentConversationExpired marks a guest conversation past its retention
// window: reads and continuations are refused immediately and the expiry is
// never extended by activity.
var ErrAgentConversationExpired = errors.New("agent conversation expired")

// AgentErrorCodeConversationExpired is the SSE terminal code for continuing
// (or replaying) an expired guest conversation. The reservation still counts
// as consumed when it fired post-reserve.
const AgentErrorCodeConversationExpired = "AGENT_CONVERSATION_EXPIRED"

// guestToolAllowlist is the complete guest tool surface: public, read-only,
// in-site. Everything else — publish assistance (a write-path aid), image
// generation (independent spend), every mcp_* bridged tool (identity-bound) —
// stays outside the anonymous budget. Both the declared schema surface and
// the executor are filtered through this one set, so a forged model tool
// call cannot reach a privileged handler.
var guestToolAllowlist = map[string]bool{
	ToolSearchContent:    true,
	ToolSearchIPs:        true,
	ToolGetContentDetail: true,
	ToolGetUsageGuide:    true,
}

func guestToolAllowed(name string) bool { return guestToolAllowlist[name] }

// guestToolRuntime wraps the production dispatch runtime for guest turns.
type guestToolRuntime struct{ inner ToolRuntime }

func (r *guestToolRuntime) ToolDefinitions(ctx context.Context, viewerID int64) []llm.ToolDefinition {
	var out []llm.ToolDefinition
	for _, def := range r.inner.ToolDefinitions(ctx, viewerID) {
		if guestToolAllowed(def.Name) {
			out = append(out, def)
		}
	}
	return out
}

func (r *guestToolRuntime) ExecuteTool(ctx context.Context, name string, rawArgs json.RawMessage, scope ToolScope) (*AgentToolOutcome, error) {
	if !guestToolAllowed(name) {
		// Same safe code the executor returns for unknown tools: the model
		// sees an ordinary failure, never a privilege hint.
		return nil, ErrAgentToolUnknown
	}
	// Guest tools always run under the anonymous public viewer: citations,
	// search and detail reads resolve through the visibility scope that
	// hides private/unpublished/banned content from viewer 0.
	scope.ViewerID = 0
	return r.inner.ExecuteTool(ctx, name, rawArgs, scope)
}

// turnToolRuntime resolves the per-turn tool seam: logged-in turns keep the
// production dispatch runtime untouched, guest turns ride the whitelist
// wrapper.
func (s *AgentService) turnToolRuntime(turn ChatTurnInput) ToolRuntime {
	rt := s.toolRuntimeOrFallback()
	if turn.GuestDeviceKey != "" {
		return &guestToolRuntime{inner: rt}
	}
	return rt
}

// guestConversationTTL is the retention window for guest conversations.
func (s *AgentService) guestConversationTTL() time.Duration {
	if s.cfg == nil || s.cfg.Agent.Guest.ConversationTTLDays <= 0 {
		return 7 * 24 * time.Hour
	}
	return time.Duration(s.cfg.Agent.Guest.ConversationTTLDays) * 24 * time.Hour
}

// guestConversationExpired reports whether a guest conversation is past its
// retention window at the given instant (computed from creation time; reads
// and continuations never extend it).
func (s *AgentService) guestConversationExpired(conv *model.AgentConversation, now time.Time) bool {
	return conv.CreatedAt.Add(s.guestConversationTTL()).Before(now)
}

// EnsureGuestConversationUsable is the pre-quota ownership + expiry gate for
// guest continuations: foreign, missing and expired conversations are all
// refused before any reservation, so none of them consumes a turn.
func (s *AgentService) EnsureGuestConversationUsable(ctx context.Context, deviceKey string, conversationID int64) error {
	if s.db == nil || conversationID <= 0 {
		return ErrAgentConversationNotFound
	}
	var conv model.AgentConversation
	if err := s.db.WithContext(ctx).
		Where("id = ? AND is_guest = ? AND guest_device_key = ?", conversationID, true, deviceKey).
		First(&conv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAgentConversationNotFound
		}
		return err
	}
	if s.guestConversationExpired(&conv, time.Now()) {
		return ErrAgentConversationExpired
	}
	return nil
}

// ListGuestConversations returns the device's live guest conversations,
// newest activity first. Expired rows are hidden immediately (the cleanup
// worker deletes them later) and another device's rows are never visible.
func (s *AgentService) ListGuestConversations(ctx context.Context, deviceKey string, limit int) ([]model.AgentGuestConversation, error) {
	if s.db == nil || limit <= 0 {
		return nil, nil
	}
	var rows []model.AgentGuestConversation
	if err := s.db.WithContext(ctx).
		Where("is_guest = ? AND guest_device_key = ? AND created_at > ?", true, deviceKey, time.Now().Add(-s.guestConversationTTL())).
		Order("pinned_at DESC NULLS LAST").
		Order("updated_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetGuestConversationMessages loads one own, live guest conversation plus a
// page of its messages for replay. Expired rows are refused with the typed
// expiry error.
func (s *AgentService) GetGuestConversation(ctx context.Context, deviceKey string, conversationID int64) (*model.AgentConversation, error) {
	if s.db == nil || conversationID <= 0 {
		return nil, ErrAgentConversationNotFound
	}
	var conv model.AgentConversation
	if err := s.db.WithContext(ctx).
		Where("id = ? AND is_guest = ? AND guest_device_key = ?", conversationID, true, deviceKey).
		First(&conv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentConversationNotFound
		}
		return nil, err
	}
	if s.guestConversationExpired(&conv, time.Now()) {
		return nil, ErrAgentConversationExpired
	}
	return &conv, nil
}

// resolveGuestConversation is the guest twin of resolveChatConversation:
// owner-scoped load (or create), expiry gate, user-message append and full
// history assembly — all inside one transaction.
func (s *AgentService) resolveGuestConversation(ctx context.Context, deviceKey string, turn ChatTurnInput, contextType string, contextID *int64) (*model.AgentConversation, []model.AgentMessage, bool, error) {
	userContent := turn.Message
	now := time.Now()

	conv := &model.AgentConversation{}
	var history []model.AgentMessage

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if turn.ConversationID > 0 {
			if err := tx.Where("id = ? AND is_guest = ? AND guest_device_key = ?", turn.ConversationID, true, deviceKey).
				First(conv).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrAgentConversationNotFound
				}
				return err
			}
			if s.guestConversationExpired(conv, now) {
				return ErrAgentConversationExpired
			}
		} else {
			guest := &model.AgentGuestConversation{
				IsGuest:        true,
				GuestDeviceKey: deviceKey,
				ContextType:    contextType,
				ContextID:      contextID,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if err := tx.Create(guest).Error; err != nil {
				return err
			}
			*conv = model.AgentConversation{
				ID:          guest.ID,
				ContextType: contextType,
				ContextID:   contextID,
				CreatedAt:   guest.CreatedAt,
				UpdatedAt:   guest.UpdatedAt,
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

// GuestConversationCleaner deletes expired guest conversations (messages
// cascade through the FK). The cumulative device budget lives in Redis and
// is deliberately untouched: history retention never replenishes a budget.
// The cleaner runs on the worker lifecycle (container StartWorkers ticker);
// RunOnce is the testable unit.
type GuestConversationCleaner struct {
	db  *gorm.DB
	ttl time.Duration
}

// NewGuestConversationCleaner builds the cleaner. A non-positive ttl falls
// back to the same 7-day window the service read path assumes
// (guestConversationTTL), so a disabled/absent guest config must not turn the
// worker's hourly sweep into an error loop; the non-positive guard in RunOnce
// stays as a defensive invariant, not a reachable state.
func NewGuestConversationCleaner(db *gorm.DB, ttl time.Duration) *GuestConversationCleaner {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &GuestConversationCleaner{db: db, ttl: ttl}
}

// RunOnce deletes every guest conversation created before now-ttl and
// returns the affected row count.
func (w *GuestConversationCleaner) RunOnce(ctx context.Context, now time.Time) (int64, error) {
	if w.db == nil {
		return 0, nil
	}
	if w.ttl <= 0 {
		return 0, errors.New("guest conversation cleaner: non-positive retention window")
	}
	res := w.db.WithContext(ctx).
		Where("is_guest = ? AND created_at < ?", true, now.Add(-w.ttl)).
		Delete(&model.AgentGuestConversation{})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}
