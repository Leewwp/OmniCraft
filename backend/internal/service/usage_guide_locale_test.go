package service

import (
	"context"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm/logger"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/repository"
)

/* #723 使用指导 locale 化：
 * 1. usage_guide_prompt v2 槽位（输出语言条款 + language 必需占位符）；
 * 2. stream 与非 stream 共用同一槽位渲染（双链漂移回归）；
 * 3. RenderUsageGuideMarkdown zh/en 标题；
 * 4. 非法 locale 报错、缺省 zh 兼容；
 * 5. 仅有 zh 作者行而请求 en 时不误用 zh 行（走生成路径）。
 */

type usageGuideRecordingProvider struct {
	mu       sync.Mutex
	chats    []llm.ChatRequest
	streams  []llm.ChatRequest
	response string
}

func (p *usageGuideRecordingProvider) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.mu.Lock()
	p.chats = append(p.chats, req)
	p.mu.Unlock()
	return &llm.ChatResponse{Content: p.response}, nil
}

func (p *usageGuideRecordingProvider) ChatStream(_ context.Context, req llm.ChatRequest, cb func(llm.ChatDelta) error) error {
	p.mu.Lock()
	p.streams = append(p.streams, req)
	p.mu.Unlock()
	if err := cb(llm.ChatDelta{Content: p.response}); err != nil {
		return err
	}
	return cb(llm.ChatDelta{Done: true})
}

func (p *usageGuideRecordingProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, nil
}

func setupUsageGuideLocaleStack(t *testing.T) (*AgentService, *usageGuideRecordingProvider, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentUsageGuide{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := &config.Config{}
	cfg.Agent.WebAgentEnabled = true
	contentRepo := repository.NewContentRepository(db)
	provider := &usageGuideRecordingProvider{response: "guide body"}
	svc := NewAgentService(provider, nil, contentRepo, nil, db, cfg)
	svc.SetUsageGuideService(NewUsageGuideService(repository.NewUsageGuideRepository(db), contentRepo))
	seed := model.ContentItem{ID: 901, Title: "locale target", AuthorID: 910, Zone: "original", ContentType: "mod", Status: "published", IsPublic: true}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	return svc, provider, db
}

func TestUsageGuideStreamReusesSlotPrompt(t *testing.T) {
	ctx := context.Background()
	svc, provider, _ := setupUsageGuideLocaleStack(t)

	if _, err := svc.UsageGuide(ctx, 910, 901, true, "en"); err != nil {
		t.Fatalf("non-stream: %v", err)
	}
	if err := svc.UsageGuideStream(ctx, 910, 901, true, "en", func(string, bool) error { return nil }); err != nil {
		t.Fatalf("stream: %v", err)
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if len(provider.chats) != 1 || len(provider.streams) != 1 {
		t.Fatalf("calls: chats=%d streams=%d", len(provider.chats), len(provider.streams))
	}
	nonStream := provider.chats[0]
	streamReq := provider.streams[0]
	// 双链漂移回归：stream 与非 stream 的 user prompt 完全一致（同一槽位、
	// 同一 values；历史缺陷 = stream 用代码内硬编码英文 prompt）。
	if nonStream.Messages[len(nonStream.Messages)-1].Content != streamReq.Messages[len(streamReq.Messages)-1].Content {
		t.Fatalf("stream prompt drifted from slot render:\nnon-stream=%q\nstream=%q",
			nonStream.Messages[len(nonStream.Messages)-1].Content, streamReq.Messages[len(streamReq.Messages)-1].Content)
	}
	if !strings.Contains(streamReq.Messages[len(streamReq.Messages)-1].Content, "Focus on:") {
		t.Fatalf("slot render must carry guide_focus: %q", streamReq.Messages[len(streamReq.Messages)-1].Content)
	}
}

func TestRenderUsageGuideMarkdownLocaleHeadings(t *testing.T) {
	view := &UsageGuideView{Requirements: []string{"r1"}, Steps: []string{"s1"}, Notes: "n", Safety: []string{"a1"}}
	zh := RenderUsageGuideMarkdown(view, "zh")
	en := RenderUsageGuideMarkdown(view, "en")
	for _, want := range []string{"前置要求", "使用步骤", "说明", "安全提示"} {
		if !strings.Contains(zh, want) {
			t.Fatalf("zh headings missing %q", want)
		}
	}
	for _, want := range []string{"Requirements", "How to Use", "Notes", "Safety"} {
		if !strings.Contains(en, want) {
			t.Fatalf("en headings missing %q", want)
		}
	}
	if strings.Contains(zh, "Requirements") || strings.Contains(en, "前置要求") {
		t.Fatal("locale headings crossed")
	}
}

func TestUsageGuideInvalidLocaleRejected(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := setupUsageGuideLocaleStack(t)
	if _, err := svc.UsageGuide(ctx, 910, 901, true, "fr"); err == nil {
		t.Fatal("invalid locale must error on the non-stream path")
	}
	if err := svc.UsageGuideStream(ctx, 910, 901, true, "fr", func(string, bool) error { return nil }); err == nil {
		t.Fatal("invalid locale must error on the stream path")
	}
}

func TestUsageGuideEnRequestDoesNotReuseZhRow(t *testing.T) {
	ctx := context.Background()
	svc, provider, db := setupUsageGuideLocaleStack(t)

	// zh 作者行存在；en 请求不得误用。
	zhRow := model.ContentUsageGuide{
		ContentItemID: 901, Locale: "zh",
		Requirements: `["zh-req"]`, Steps: `["zh-step"]`, Notes: "zh-notes",
		Source: model.UsageGuideSourceAuthor,
	}
	if err := db.Create(&zhRow).Error; err != nil {
		t.Fatalf("seed zh row: %v", err)
	}
	if !svc.HasStructuredGuide(ctx, 901, "zh") {
		t.Fatal("zh request must hit the persisted zh row (quota-free)")
	}
	if svc.HasStructuredGuide(ctx, 901, "en") {
		t.Fatal("en request must NOT be served by the zh row (quota must engage)")
	}
	result, err := svc.UsageGuide(ctx, 910, 901, false, "en")
	if err != nil {
		t.Fatalf("en generation: %v", err)
	}
	if result.Structured {
		t.Fatalf("en answer must be generated, not rendered from zh specifics")
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if len(provider.chats) != 1 {
		t.Fatalf("provider calls = %d, want 1 (en generation)", len(provider.chats))
	}
}
