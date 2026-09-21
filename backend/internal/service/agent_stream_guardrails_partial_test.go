package service

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

// FR-07（中-1 落库面）：persistPartialTurn 落库前必须过图片白名单消毒与
// 孤儿角标剥离——失败/取消路径不得把未消毒的模型原文写进历史。
func TestPersistPartialTurnSanitizesBeforePersist(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.AgentMessage{}, &model.AgentConversation{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	conv := model.AgentConversation{UserID: 7, ContextType: "general", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&conv).Error; err != nil {
		t.Fatal(err)
	}

	s := &AgentService{cfg: &config.Config{}, db: db}
	s.cfg.Agent.Guardrails.ImageURLAllowHosts = []string{"cdn.omnicraft.local"}

	partial := "看这张 ![cat](https://evil.example.org/cat.png) 和自家 ![ok](https://cdn.omnicraft.local/a.png) 图[3]。"
	s.persistPartialTurn(conv.ID, partial, nil, 0, "zh")

	var row model.AgentMessage
	if err := db.Where("conversation_id = ? AND role = ?", conv.ID, "assistant").First(&row).Error; err != nil {
		t.Fatalf("load partial row: %v", err)
	}
	content := *row.Content
	if strings.Contains(content, "evil.example.org") {
		t.Fatalf("partial row must not persist external image URLs, got %q", content)
	}
	if !strings.Contains(content, "cdn.omnicraft.local/a.png") {
		t.Fatalf("allowed platform image must survive, got %q", content)
	}
	if strings.Contains(content, "[3]") {
		t.Fatalf("orphan citation markers must be stripped from partial rows, got %q", content)
	}
	if !strings.Contains(content, "[图片链接已移除：非平台图源]") {
		t.Fatalf("zh placeholder expected, got %q", content)
	}
}

// FR-07（低-1）：图片移除占位文案必须跟随会话语言（chitchat zh/en 同款约定）。
func TestSanitizeImageURLsLocalizedPlaceholder(t *testing.T) {
	s := &AgentService{cfg: &config.Config{}}

	en := s.sanitizeAnswerImages("![a](https://evil.example.org/x.png)", "en")
	if !strings.Contains(en, "[image link removed: non-platform image source]") {
		t.Fatalf("en placeholder expected, got %q", en)
	}
	zh := s.sanitizeAnswerImages("![a](https://evil.example.org/x.png)", "zh")
	if !strings.Contains(zh, "[图片链接已移除：非平台图源]") {
		t.Fatalf("zh placeholder expected, got %q", zh)
	}
}

// FR-07（中-2）：预算基线加载失败必须显性返回错误（供调用方保守拦截），
// 不得吞错返回零用量让护栏在 DB 故障期静默失效。
func TestConversationBudgetBaselinePropagatesDBError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.AgentMessage{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable("agent_messages"); err != nil {
		t.Fatal(err)
	}

	s := &AgentService{cfg: &config.Config{}, db: db}
	if _, _, err := s.conversationBudgetBaseline(context.Background(), 42); err == nil {
		t.Fatal("budget baseline must surface DB load errors instead of swallowing them")
	}
}

// FR-07（低-6）：会话删除后异步 best-effort 清理 agent-images/<convID>/ 前缀对象。
type cleanupRecordingStore struct {
	mu      sync.Mutex
	prefix  string
	deleted []string
	signal  chan struct{}
}

func (s *cleanupRecordingStore) PutAgentImage(ctx context.Context, key string, r io.Reader) error {
	return nil
}
func (s *cleanupRecordingStore) SignedAgentImageURL(ctx context.Context, key string) (string, error) {
	return "https://cdn.omnicraft.local/signed/" + key, nil
}
func (s *cleanupRecordingStore) DeleteAgentImagePrefix(ctx context.Context, prefix string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prefix = prefix
	if s.signal != nil {
		close(s.signal)
	}
	return nil
}

func TestCleanupConversationImagesBestEffort(t *testing.T) {
	store := &cleanupRecordingStore{signal: make(chan struct{})}
	cfg := &config.Config{}
	cfg.Agent.Image = config.AgentImageConfig{Enabled: true, APIKey: "k"}
	s := &AgentService{cfg: cfg}
	s.SetImageTool(cfg.Agent.Image, llm.AgentImageGenerator(nil), store)

	s.CleanupConversationImages(42)

	select {
	case <-store.signal:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup must run asynchronously")
	}
	if store.prefix != "agent-images/42/" {
		t.Fatalf("cleanup prefix = %q, want agent-images/42/", store.prefix)
	}
}
