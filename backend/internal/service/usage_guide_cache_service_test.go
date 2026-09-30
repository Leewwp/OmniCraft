package service

import (
	"context"
	"errors"
	"fmt"
	"omnicraft/backend/internal/pkg/llm"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service/promptregistry"
)

/* #728 使用指导生成缓存（heavy，TDD）：
 * 统一入口 + 独立缓存表 + 跨进程去重 + 失效守卫 + 作者行隔离 + 草稿隔离。
 */

type memSingleflight struct {
	mu   sync.Mutex
	held map[string]bool
}

func (m *memSingleflight) Acquire(_ context.Context, key string, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.held == nil {
		m.held = map[string]bool{}
	}
	if m.held[key] {
		return false, nil
	}
	m.held[key] = true
	return true, nil
}

func (m *memSingleflight) Release(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.held, key)
	return nil
}

func setupUsageGuideCacheStack(t *testing.T) (*UsageGuideCacheService, *gorm.DB, *memSingleflight) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentUsageGuide{}, &model.ContentUsageGuideCache{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sf := &memSingleflight{}
	cfg := &config.Config{}
	cfg.Agent.WebAgentEnabled = true
	svc := NewUsageGuideCacheService(
		repository.NewUsageGuideCacheRepository(db),
		sf,
		promptregistry.NewPromptResolver(nil),
		repository.NewUsageGuideRepository(db),
		repository.NewContentRepository(db),
		cfg,
	)
	return svc, db, sf
}

func seedCacheContent(t *testing.T, db *gorm.DB, id int64, description string) *model.ContentItem {
	t.Helper()
	content := &model.ContentItem{ID: id, Title: "cache 目标", AuthorID: 910, Zone: "original", ContentType: "3d_print", Status: "published", IsPublic: true, Description: description, UpdatedAt: time.Now()}
	if err := db.Save(content).Error; err != nil {
		t.Fatalf("seed content: %v", err)
	}
	return content
}

func TestUsageGuideInputFingerprintChangesWithInputs(t *testing.T) {
	a := UsageGuideInputFingerprint("t", "d", "mod")
	if a == "" {
		t.Fatal("fingerprint must not be empty")
	}
	if a != UsageGuideInputFingerprint("t", "d", "mod") {
		t.Fatal("fingerprint must be deterministic")
	}
	for _, changed := range []struct{ title, desc, ct string }{
		{"t2", "d", "mod"}, {"t", "d2", "mod"}, {"t", "d", "mod2"},
	} {
		if UsageGuideInputFingerprint(changed.title, changed.desc, changed.ct) == a {
			t.Fatalf("fingerprint must change with input %+v", changed)
		}
	}
}

func TestGetOrGenerateCachesCompleteResultsOnly(t *testing.T) {
	ctx := context.Background()
	svc, db, _ := setupUsageGuideCacheStack(t)
	content := seedCacheContent(t, db, 1001, "desc-v1")

	var calls int32
	generate := func(context.Context) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "## Requirements\n- r1", nil
	}

	got, err := svc.GetOrGenerate(ctx, content, "zh", false, generate)
	if err != nil || got != "## Requirements\n- r1" {
		t.Fatalf("first call: %q %v", got, err)
	}
	if calls != 1 {
		t.Fatalf("generate calls = %d", calls)
	}

	// 第二访客：缓存命中零生成。
	again, err := svc.GetOrGenerate(ctx, content, "zh", false, generate)
	if err != nil || again != got {
		t.Fatalf("cache hit: %q %v", again, err)
	}
	if calls != 1 {
		t.Fatalf("cache must absorb the second visitor (calls=%d)", calls)
	}

	// FindValid 直接命中。
	if cached, ok := svc.FindValid(ctx, content, "zh"); !ok || cached != got {
		t.Fatalf("FindValid after cache: %q %v", cached, ok)
	}
}

func TestCacheInvalidatedByInputEditAndPromptBump(t *testing.T) {
	ctx := context.Background()
	svc, db, _ := setupUsageGuideCacheStack(t)
	content := seedCacheContent(t, db, 1002, "desc-v1")
	var calls int32
	gen := func(context.Context) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "guide", nil
	}
	if _, err := svc.GetOrGenerate(ctx, content, "zh", false, gen); err != nil {
		t.Fatal(err)
	}

	// 描述编辑 → 指纹变化 → 缓存失效。
	edited := *content
	edited.Description = "desc-v2"
	if _, err := svc.GetOrGenerate(ctx, &edited, "zh", false, gen); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("input edit must invalidate (calls=%d)", calls)
	}

	// 提示词版本升版 → 旧缓存失效。
	if err := db.Model(&model.ContentUsageGuideCache{}).
		Where("content_id = ? AND locale = ?", content.ID, "zh").
		Update("prompt_version", svc.PromptVersion(ctx)-1).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetOrGenerate(ctx, &edited, "zh", false, gen); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("prompt bump must invalidate (calls=%d)", calls)
	}
}

func TestFailedOrForcedGenerationNeverCaches(t *testing.T) {
	ctx := context.Background()
	svc, db, _ := setupUsageGuideCacheStack(t)
	content := seedCacheContent(t, db, 1003, "d")

	// 失败不落缓存。
	if _, err := svc.GetOrGenerate(ctx, content, "en", false, func(context.Context) (string, error) {
		return "", errors.New("provider down")
	}); err == nil {
		t.Fatal("failure must propagate")
	}
	if _, ok := svc.FindValid(ctx, content, "en"); ok {
		t.Fatal("failed generation must not cache")
	}
	// 失败后可再次请求。
	if _, err := svc.GetOrGenerate(ctx, content, "en", false, func(context.Context) (string, error) {
		return "ok", nil
	}); err != nil {
		t.Fatalf("retry after failure: %v", err)
	}

	// draft（force）不读不写缓存。
	var calls int32
	forced, err := svc.GetOrGenerate(ctx, content, "en", true, func(context.Context) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "draft result", nil
	})
	if err != nil || forced != "draft result" {
		t.Fatalf("force path: %q %v", forced, err)
	}
	if calls != 1 {
		t.Fatal("force must always generate")
	}
	// 草稿结果不得污染正式缓存（缓存仍是 ok）。
	if cached, ok := svc.FindValid(ctx, content, "en"); !ok || cached != "ok" {
		t.Fatalf("draft must not write the formal cache: %q %v", cached, ok)
	}
}

func TestStaleWriterNeverOverwritesNewerRow(t *testing.T) {
	ctx := context.Background()
	svc, db, _ := setupUsageGuideCacheStack(t)
	content := seedCacheContent(t, db, 1004, "d")

	// 新输入已写缓存（content_updated_at = 更晚）。
	newer := *content
	newer.Description = "newer input"
	newer.UpdatedAt = content.UpdatedAt.Add(time.Hour)
	if _, err := svc.GetOrGenerate(ctx, &newer, "zh", false, func(context.Context) (string, error) {
		return "newer result", nil
	}); err != nil {
		t.Fatal(err)
	}

	// 旧任务（旧输入快照）后完成：不得覆盖新行。
	if _, err := svc.GetOrGenerate(ctx, content, "zh", false, func(context.Context) (string, error) {
		return "stale result", nil
	}); err != nil {
		t.Fatal(err)
	}
	if cached, ok := svc.FindValid(ctx, &newer, "zh"); !ok || cached != "newer result" {
		t.Fatalf("stale writer overwrote the newer row: %q", cached)
	}
}

func TestConcurrentFirstVisitorsMergeIntoOneGeneration(t *testing.T) {
	ctx := context.Background()
	svc, db, _ := setupUsageGuideCacheStack(t)
	content := seedCacheContent(t, db, 1005, "d")

	var calls int32
	var mu sync.Mutex
	started := make(chan struct{})
	gen := func(context.Context) (string, error) {
		atomic.AddInt32(&calls, 1)
		mu.Lock()
		close(started)
		mu.Unlock()
		time.Sleep(120 * time.Millisecond) // 模拟慢生成，让并发等待者进入等待路径
		return fmt.Sprintf("guide-%d", content.ID), nil
	}

	const visitors = 4
	results := make([]string, visitors)
	errs := make([]error, visitors)
	var wg sync.WaitGroup
	for i := 0; i < visitors; i += 1 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = svc.GetOrGenerate(ctx, content, "zh", false, gen)
		}(i)
	}
	wg.Wait()

	if calls != 1 {
		t.Fatalf("concurrent first visitors must merge to ONE generation (calls=%d)", calls)
	}
	for i := 0; i < visitors; i += 1 {
		if errs[i] != nil || results[i] != fmt.Sprintf("guide-%d", content.ID) {
			t.Fatalf("visitor %d: %q %v", i, results[i], errs[i])
		}
	}
}

func TestPreheatRespectsAuthorRowsAndFeatureGate(t *testing.T) {
	ctx := context.Background()
	svc, db, _ := setupUsageGuideCacheStack(t)
	content := seedCacheContent(t, db, 1006, "d")

	// zh 有作者确认行：预热只生成 en。
	zhRow := model.ContentUsageGuide{ContentItemID: content.ID, Locale: "zh", Requirements: `["a"]`, Steps: `["b"]`, Source: model.UsageGuideSourceAuthor}
	if err := db.Create(&zhRow).Error; err != nil {
		t.Fatal(err)
	}

	var generated []string
	var mu sync.Mutex
	svc.PreheatContent(ctx, content.ID, func(_ context.Context, c *model.ContentItem, locale string, _ bool) (string, error) {
		mu.Lock()
		generated = append(generated, locale)
		mu.Unlock()
		return "auto " + locale, nil
	})
	mu.Lock()
	defer mu.Unlock()
	if len(generated) != 1 || generated[0] != "en" {
		t.Fatalf("preheat must only generate the language lacking an author row: %v", generated)
	}
	// 作者行未被触碰。
	var authorRows int64
	db.Model(&model.ContentUsageGuide{}).Where("content_id = ?", content.ID).Count(&authorRows)
	if authorRows != 1 {
		t.Fatalf("author rows must stay untouched (got %d)", authorRows)
	}
	// en 缓存行就绪。
	if cached, ok := svc.FindValid(ctx, content, "en"); !ok || cached != "auto en" {
		t.Fatalf("en cache row: %q %v", cached, ok)
	}
}

/* ---------- 双轴审查修复的回归覆盖 ---------- */

func TestUpsertGuardedUpdatePathActuallyPersists(t *testing.T) {
	// P2-4 回归：UPDATE 分支（同快照重生成）必须真实落库——曾因
	// gorm.Expr("now()") 在 sqlite 下失败被 warn 吞掉而零覆盖。
	ctx := context.Background()
	svc, db, _ := setupUsageGuideCacheStack(t)
	content := seedCacheContent(t, db, 1010, "d-v1")
	if _, err := svc.GetOrGenerate(ctx, content, "zh", false, func(context.Context) (string, error) {
		return "first", nil
	}); err != nil {
		t.Fatal(err)
	}
	// 老化行版本（FindValid 失效）后重生成——指纹一致走 UPDATE 分支。
	if err := db.Model(&model.ContentUsageGuideCache{}).
		Where("content_id = ? AND locale = ?", content.ID, "zh").
		Update("prompt_version", svc.PromptVersion(ctx)-1).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetOrGenerate(ctx, content, "zh", false, func(context.Context) (string, error) {
		return "second", nil
	}); err != nil {
		t.Fatal(err)
	}
	var row model.ContentUsageGuideCache
	if err := db.Where("content_id = ? AND locale = ?", content.ID, "zh").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.GuideMarkdown != "second" {
		t.Fatalf("guarded UPDATE must persist: got %q", row.GuideMarkdown)
	}
}

func TestWaiterTimeoutFallsBackToLocalGeneration(t *testing.T) {
	// 等待者超时 → 本地兜底生成（waiterWait 注入压缩超时）。
	ctx := context.Background()
	svc, db, sf := setupUsageGuideCacheStack(t)
	svc.waiterWait = 300 * time.Millisecond
	content := seedCacheContent(t, db, 1011, "d")

	// 模拟另一进程持有租约但永不产出（等待者必然超时）。
	key := UsageGuideInputFingerprint(content.Title, content.Description, content.ContentType)
	_ = key
	held, err := sf.Acquire(ctx, "guide:1011:zh:x:0", time.Minute)
	if err != nil || !held {
		t.Fatalf("seed foreign lease: %v %v", held, err)
	}
	var calls int32
	got, err := svc.GetOrGenerate(ctx, content, "zh", false, func(context.Context) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "fallback", nil
	})
	if err != nil || got != "fallback" {
		t.Fatalf("waiter fallback: %q %v", got, err)
	}
	if calls != 1 {
		t.Fatalf("fallback generation calls = %d", calls)
	}
}

func TestPreheatFeatureGateOffGeneratesNothing(t *testing.T) {
	// P3-15：gate 关闭零生成。
	ctx := context.Background()
	svc, db, _ := setupUsageGuideCacheStack(t)
	svc.cfg.Agent.WebAgentEnabled = false
	content := seedCacheContent(t, db, 1012, "d")
	var calls int32
	svc.PreheatContent(ctx, content.ID, func(context.Context, *model.ContentItem, string, bool) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "x", nil
	})
	if calls != 0 {
		t.Fatalf("feature gate off must generate nothing (calls=%d)", calls)
	}
}

/* ---------- P2-6：流式缓存路径与免配额判定覆盖 ---------- */

func setupAgentWithCache(t *testing.T, provider llm.LLMProvider) (*AgentService, *UsageGuideCacheService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentUsageGuide{}, &model.ContentUsageGuideCache{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := &config.Config{}
	cfg.Agent.WebAgentEnabled = true
	contentRepo := repository.NewContentRepository(db)
	resolver := promptregistry.NewPromptResolver(nil)
	cacheSvc := NewUsageGuideCacheService(
		repository.NewUsageGuideCacheRepository(db),
		&memSingleflight{},
		resolver,
		repository.NewUsageGuideRepository(db),
		contentRepo,
		cfg,
	)
	svc := NewAgentService(provider, nil, contentRepo, nil, db, cfg)
	svc.SetUsageGuideService(NewUsageGuideService(repository.NewUsageGuideRepository(db), contentRepo))
	svc.SetUsageGuideCacheService(cacheSvc)
	svc.SetPromptResolver(resolver)
	return svc, cacheSvc, db
}

func TestUsageGuideStreamCacheHitSingleDeltaZeroLLM(t *testing.T) {
	ctx := context.Background()
	provider := &usageGuideRecordingProvider{response: "irrelevant"}
	svc, cacheSvc, db := setupAgentWithCache(t, provider)
	content := seedCacheContent(t, db, 1020, "d")
	author := content.AuthorID
	// 预置有效缓存行（指纹一致）。
	fp := UsageGuideInputFingerprint(content.Title, content.Description, content.ContentType)
	if err := db.Create(&model.ContentUsageGuideCache{ContentItemID: content.ID, Locale: "zh", PromptVersion: cacheSvc.PromptVersion(ctx), InputFingerprint: fp, ContentUpdatedAt: content.UpdatedAt, GuideMarkdown: "cached-stream", Source: model.UsageGuideCacheSourceAuto}).Error; err != nil {
		t.Fatal(err)
	}
	if !svc.HasCachedGuide(ctx, content.ID, "zh") {
		t.Fatal("HasCachedGuide must report the valid row (quota-free path)")
	}
	var deltas []string
	var doneSeen bool
	err := svc.UsageGuideStream(ctx, author, content.ID, false, "zh", func(delta string, done bool) error {
		if done {
			doneSeen = true
			return nil
		}
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if doneSeen && len(deltas) == 1 && deltas[0] == "cached-stream" {
		// 契约：命中 = 单 delta 全文 + done。
	} else {
		t.Fatalf("cache hit must emit one full delta + done: %v done=%v", deltas, doneSeen)
	}
	provider.mu.Lock()
	streams := len(provider.streams)
	chats := len(provider.chats)
	provider.mu.Unlock()
	if streams+chats != 0 {
		t.Fatalf("cache hit must perform ZERO LLM calls (streams=%d chats=%d)", streams, chats)
	}
}

func TestUsageGuideStreamOwnerPersistsCompleteResultOnly(t *testing.T) {
	ctx := context.Background()

	// 完整成功：累计全文、落缓存。
	provider := &usageGuideRecordingProvider{response: "stream full text"}
	svc, _, db := setupAgentWithCache(t, provider)
	content := seedCacheContent(t, db, 1021, "d-ok")
	var got []string
	if err := svc.UsageGuideStream(ctx, 910, content.ID, false, "zh", func(delta string, done bool) error {
		if !done {
			got = append(got, delta)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "") != "stream full text" {
		t.Fatalf("streamed text = %q", strings.Join(got, ""))
	}
	var row model.ContentUsageGuideCache
	if err := db.Where("content_id = ? AND locale = ?", content.ID, "zh").First(&row).Error; err != nil {
		t.Fatalf("complete stream must persist cache: %v", err)
	}
	if row.GuideMarkdown != "stream full text" {
		t.Fatalf("cached markdown = %q", row.GuideMarkdown)
	}
}

type streamFailingProvider struct{ usageGuideRecordingProvider }

func (p *streamFailingProvider) ChatStream(context.Context, llm.ChatRequest, func(llm.ChatDelta) error) error {
	return errors.New("stream broke mid-way")
}

func TestUsageGuideStreamHalfStreamNotCached(t *testing.T) {
	ctx := context.Background()
	provider := &streamFailingProvider{}
	svc, _, db := setupAgentWithCache(t, provider)
	content := seedCacheContent(t, db, 1022, "d-fail")
	if err := svc.UsageGuideStream(ctx, 910, content.ID, false, "zh", func(string, bool) error { return nil }); err == nil {
		t.Fatal("broken stream must surface the error")
	}
	var count int64
	db.Model(&model.ContentUsageGuideCache{}).Where("content_id = ?", content.ID).Count(&count)
	if count != 0 {
		t.Fatalf("half-broken stream must NOT cache (rows=%d)", count)
	}
}
