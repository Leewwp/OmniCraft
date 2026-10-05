package service

import (
	"context"
	"errors"
	"fmt"
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
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service/promptregistry"
)

/* #792 使用指导流式生命周期收口（TDD）：
 * 1. 生成中 / 渲染→取租约之间的 prompt 版本漂移：起始 template/version
 *    贯穿租约键、等待与落库（fake Store 升 label + 显式 Invalidate，不 sleep TTL）；
 * 2. 单入口 GetOrGenerateStream 拥有命中/等待/持有三分支与守卫写、释放；
 * 3. 生成/转发回调可返回 error，delta/done 语义保留；
 * 4. 失败/取消/半截/空结果不缓存；等待者不释放他人租约。
 */

// bumpablePromptStore is a fake promptregistry.Store whose production label
// version can move mid-flight — the realistic cross-cut is admin SetLabel +
// resolver.Invalidate (admin_prompt.go), not the 30s TTL expiring.
type bumpablePromptStore struct {
	mu       sync.Mutex
	version  int
	contents map[int]string
}

func (s *bumpablePromptStore) GetByLabel(_ context.Context, name, _ string) (*model.PromptRegistry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &model.PromptRegistry{Name: name, Version: s.version, Content: s.contents[s.version]}, nil
}

func (s *bumpablePromptStore) CreateVersion(context.Context, *model.PromptRegistry) error { return nil }
func (s *bumpablePromptStore) EnsureLabel(context.Context, string, string, int) error     { return nil }

func (s *bumpablePromptStore) SetLabel(_ context.Context, _ string, _ string, version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version = version
	return nil
}

// bump moves the production label and drops the resolver's TTL cache — the
// admin SetLabel handler's exact effect while a stream is in flight.
func (s *bumpablePromptStore) bump(version int, resolver *promptregistry.PromptResolver) {
	_ = s.SetLabel(context.Background(), promptregistry.SlotUsageGuide.Name, promptregistry.ProductionLabel, version)
	resolver.Invalidate()
}

func (s *bumpablePromptStore) contentsFor(version int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contents[version]
}

// driftBumpProvider records streamed requests and lets the test mutate the
// prompt registry from inside the live stream (the drift window).
type driftBumpProvider struct {
	mu       sync.Mutex
	streams  []llm.ChatRequest
	onStream func()
}

func (p *driftBumpProvider) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: "unused"}, nil
}

func (p *driftBumpProvider) ChatStream(_ context.Context, req llm.ChatRequest, cb func(llm.ChatDelta) error) error {
	p.mu.Lock()
	p.streams = append(p.streams, req)
	p.mu.Unlock()
	if p.onStream != nil {
		p.onStream()
	}
	if err := cb(llm.ChatDelta{Content: "drift-proof body"}); err != nil {
		return err
	}
	return cb(llm.ChatDelta{Done: true})
}

func (p *driftBumpProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, nil
}

// setupAgentWithCacheResolver mirrors setupAgentWithCache with an injectable
// prompt resolver (drift scenarios need a real store behind it).
func setupAgentWithCacheResolver(t *testing.T, provider llm.LLMProvider, resolver *promptregistry.PromptResolver) (*AgentService, *UsageGuideCacheService, *gorm.DB) {
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

// TestUsageGuideStreamVersionDriftSavesUnderStartIdentity is the #792 red
// test: a prompt label bump mid-generation must not leak into the persisted
// row — the row keeps the version the generation STARTED under, and a request
// resolving the NEW version must not mis-hit it.
func TestUsageGuideStreamVersionDriftSavesUnderStartIdentity(t *testing.T) {
	ctx := context.Background()
	store := &bumpablePromptStore{version: 1, contents: map[int]string{
		1: "DRIFT-V1 guide for {{title}} focus {{guide_focus}} in {{language}}",
		2: "DRIFT-V2 guide for {{title}} focus {{guide_focus}} in {{language}}",
	}}
	resolver := promptregistry.NewPromptResolver(store)
	provider := &driftBumpProvider{onStream: func() {
		store.bump(2, resolver)
	}}
	svc, cacheSvc, db := setupAgentWithCacheResolver(t, provider, resolver)
	content := seedCacheContent(t, db, 1030, "drift-desc")

	if err := svc.UsageGuideStream(ctx, 910, content.ID, false, "zh", func(string, bool) error { return nil }); err != nil {
		t.Fatal(err)
	}

	// 固定 template：流式请求必须用起始（v1）模板渲染。
	provider.mu.Lock()
	streams := append([]llm.ChatRequest(nil), provider.streams...)
	provider.mu.Unlock()
	if len(streams) != 1 || !strings.Contains(streams[0].Messages[len(streams[0].Messages)-1].Content, "DRIFT-V1") {
		t.Fatalf("stream prompt must render from the start-of-call template: %d streams", len(streams))
	}

	// 落库身份：行携带起始版本，而不是保存时刻解析到的新版本。
	var row model.ContentUsageGuideCache
	if err := db.Where("content_id = ? AND locale = ?", content.ID, "zh").First(&row).Error; err != nil {
		t.Fatalf("complete stream must persist the cache row: %v", err)
	}
	if row.PromptVersion != 1 {
		t.Fatalf("saved row must carry the start version 1, got %d", row.PromptVersion)
	}

	// 新版本（v2）请求不得把该行误当命中（内容是 v1 prompt 产物）。
	if _, ok := cacheSvc.FindValid(ctx, content, "zh"); ok {
		t.Fatal("a v2 request must not mis-hit the row generated under the v1 prompt")
	}
}

/* ---------- 单入口 GetOrGenerateStream 覆盖矩阵（#792） ---------- */

// hookSingleflight records acquire keys and lets a test mutate the world from
// inside the acquire call (the resolve→acquire drift window).
type hookSingleflight struct {
	memSingleflight
	mu        sync.Mutex
	acquired  []string
	onAcquire func(key string)
}

func (h *hookSingleflight) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	h.mu.Lock()
	h.acquired = append(h.acquired, key)
	hook := h.onAcquire
	h.mu.Unlock()
	if hook != nil {
		hook(key)
	}
	return h.memSingleflight.Acquire(ctx, key, ttl)
}

func (h *hookSingleflight) acquireLog() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.acquired...)
}

// recordedForward captures the forwarded stream; failOn >= 0 makes the
// failOn-th forwarded event fail with err (forward-failure propagation).
type recordedForward struct {
	mu     sync.Mutex
	events int
	deltas []string
	done   int
	failOn int
	err    error
}

func (f *recordedForward) forward(delta string, done bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn >= 0 && f.events == f.failOn {
		return f.err
	}
	f.events++
	if done {
		f.done++
		return nil
	}
	f.deltas = append(f.deltas, delta)
	return nil
}

func (f *recordedForward) snapshot() ([]string, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.deltas...), f.done
}

func setupStreamStack(t *testing.T, resolver *promptregistry.PromptResolver, sf RedisSingleflight) (*UsageGuideCacheService, *gorm.DB) {
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
	svc := NewUsageGuideCacheService(
		repository.NewUsageGuideCacheRepository(db),
		sf,
		resolver,
		repository.NewUsageGuideRepository(db),
		repository.NewContentRepository(db),
		cfg,
	)
	return svc, db
}

// streamLeaseKey recomputes the identity lease key the entry must use.
func streamLeaseKey(content *model.ContentItem, locale string, version int) string {
	return fmt.Sprintf("guide:%d:%s:%s:%d",
		content.ID, locale,
		UsageGuideInputFingerprint(content.Title, content.Description, content.ContentType), version)
}

func cacheRowCount(t *testing.T, db *gorm.DB, contentID int64) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&model.ContentUsageGuideCache{}).Where("content_id = ?", contentID).Count(&n).Error; err != nil {
		t.Fatalf("count cache rows: %v", err)
	}
	return n
}

// emitDone emits a completed body: one full delta then done.
func emitDone(emit func(string, bool) error, body string) error {
	if err := emit(body, false); err != nil {
		return err
	}
	return emit("", true)
}

func TestGetOrGenerateStreamCacheHitForwardsSingleDeltaAndDone(t *testing.T) {
	ctx := context.Background()
	svc, db := setupStreamStack(t, promptregistry.NewPromptResolver(nil), &memSingleflight{})
	content := seedCacheContent(t, db, 1031, "d")
	row := &model.ContentUsageGuideCache{
		ContentItemID: content.ID, Locale: "zh", PromptVersion: 0,
		InputFingerprint: UsageGuideInputFingerprint(content.Title, content.Description, content.ContentType),
		ContentUpdatedAt: content.UpdatedAt, GuideMarkdown: "cached-body", Source: model.UsageGuideCacheSourceAuto,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}

	forward := &recordedForward{failOn: -1}
	generateCalls := int32(0)
	if err := svc.GetOrGenerateStream(ctx, content, "zh",
		func(context.Context, string, func(string, bool) error) error {
			atomic.AddInt32(&generateCalls, 1)
			return nil
		},
		forward.forward,
	); err != nil {
		t.Fatal(err)
	}
	deltas, done := forward.snapshot()
	if generateCalls != 0 {
		t.Fatalf("cache hit must not generate (calls=%d)", generateCalls)
	}
	if len(deltas) != 1 || deltas[0] != "cached-body" || done != 1 {
		t.Fatalf("cache hit must emit ONE full delta + one done: deltas=%v done=%d", deltas, done)
	}
}

func TestGetOrGenerateStreamConcurrentVisitorsMergeIntoOwner(t *testing.T) {
	ctx := context.Background()
	svc, db := setupStreamStack(t, promptregistry.NewPromptResolver(nil), &memSingleflight{})
	content := seedCacheContent(t, db, 1032, "d")

	generateCalls := int32(0)
	generate := func(_ context.Context, _ string, emit func(string, bool) error) error {
		atomic.AddInt32(&generateCalls, 1)
		time.Sleep(150 * time.Millisecond) // 慢生成，让并发访客进入等待路径
		return emitDone(emit, "shared-body")
	}

	const visitors = 4
	forwards := make([]*recordedForward, visitors)
	errs := make([]error, visitors)
	var wg sync.WaitGroup
	for i := 0; i < visitors; i++ {
		forwards[i] = &recordedForward{failOn: -1}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = svc.GetOrGenerateStream(ctx, content, "zh", generate, forwards[idx].forward)
		}(i)
	}
	wg.Wait()

	if generateCalls != 1 {
		t.Fatalf("concurrent visitors must merge to ONE generation (calls=%d)", generateCalls)
	}
	for i := 0; i < visitors; i++ {
		if errs[i] != nil {
			t.Fatalf("visitor %d: %v", i, errs[i])
		}
		deltas, done := forwards[i].snapshot()
		if len(deltas) != 1 || deltas[0] != "shared-body" || done != 1 {
			t.Fatalf("visitor %d must receive one full delta + one done: deltas=%v done=%d", i, deltas, done)
		}
	}
}

func TestGetOrGenerateStreamSharesLeaseWithPreheatGetOrGenerate(t *testing.T) {
	ctx := context.Background()
	svc, db := setupStreamStack(t, promptregistry.NewPromptResolver(nil), &memSingleflight{})
	content := seedCacheContent(t, db, 1033, "d")

	// 流式持有者慢生成；ownerStarted 信号发出时租约已取得。
	ownerStarted := make(chan struct{})
	streamDone := make(chan error, 1)
	go func() {
		streamDone <- svc.GetOrGenerateStream(ctx, content, "zh",
			func(_ context.Context, _ string, emit func(string, bool) error) error {
				close(ownerStarted)
				time.Sleep(200 * time.Millisecond)
				return emitDone(emit, "preheat-shared-body")
			},
			func(string, bool) error { return nil })
	}()
	<-ownerStarted

	// 并发的非流式（预热）请求：同一身份键 → 合并等待共享行，零额外生成。
	preheatCalls := int32(0)
	preheatText, err := svc.GetOrGenerate(ctx, content, "zh", false, func(context.Context) (string, error) {
		atomic.AddInt32(&preheatCalls, 1)
		return "preheat-local-fallback", nil
	})
	if err != nil {
		t.Fatalf("preheat get-or-generate: %v", err)
	}
	if err := <-streamDone; err != nil {
		t.Fatalf("stream owner: %v", err)
	}
	if preheatCalls != 0 {
		t.Fatalf("preheat must merge into the stream owner's lease (preheat generations=%d)", preheatCalls)
	}
	if preheatText != "preheat-shared-body" {
		t.Fatalf("preheat must share the stream result, got %q", preheatText)
	}
}

func TestGetOrGenerateStreamWaiterTimeoutFallsBackToLocal(t *testing.T) {
	ctx := context.Background()
	sf := &memSingleflight{}
	svc, db := setupStreamStack(t, promptregistry.NewPromptResolver(nil), sf)
	svc.waiterWait = 200 * time.Millisecond
	content := seedCacheContent(t, db, 1034, "d")

	// 外部进程持有真实身份键的租约且永不产出 → 等待者必然超时。
	key := streamLeaseKey(content, "zh", 0)
	if held, err := sf.Acquire(ctx, key, time.Minute); err != nil || !held {
		t.Fatalf("seed foreign lease: held=%v err=%v", held, err)
	}

	forward := &recordedForward{failOn: -1}
	generateCalls := int32(0)
	if err := svc.GetOrGenerateStream(ctx, content, "zh",
		func(_ context.Context, _ string, emit func(string, bool) error) error {
			atomic.AddInt32(&generateCalls, 1)
			return emitDone(emit, "fallback-body")
		},
		forward.forward,
	); err != nil {
		t.Fatal(err)
	}
	deltas, done := forward.snapshot()
	if generateCalls != 1 || len(deltas) != 1 || deltas[0] != "fallback-body" || done != 1 {
		t.Fatalf("waiter timeout must fall back to local generation: calls=%d deltas=%v done=%d", generateCalls, deltas, done)
	}
	// 本地兜底结果守卫落库（供后续命中）。
	if cached, ok := svc.FindValid(ctx, content, "zh"); !ok || cached != "fallback-body" {
		t.Fatalf("local fallback must be guarded-cached: %q %v", cached, ok)
	}
	// 等待者不得释放他人租约。
	if reacquired, _ := sf.Acquire(ctx, key, time.Minute); reacquired {
		t.Fatal("waiter must not release the foreign lease")
	}
}

func TestGetOrGenerateStreamWaiterCancelEndsWithoutGenerating(t *testing.T) {
	ctx := context.Background()
	sf := &memSingleflight{}
	svc, db := setupStreamStack(t, promptregistry.NewPromptResolver(nil), sf)
	svc.waiterWait = 5 * time.Second // 足够长，保证取消先于超时
	content := seedCacheContent(t, db, 1035, "d")
	key := streamLeaseKey(content, "zh", 0)
	if held, err := sf.Acquire(ctx, key, time.Minute); err != nil || !held {
		t.Fatalf("seed foreign lease: held=%v err=%v", held, err)
	}

	wctx, cancel := context.WithCancel(ctx)
	go func() {
		time.Sleep(80 * time.Millisecond)
		cancel()
	}()
	generateCalls := int32(0)
	err := svc.GetOrGenerateStream(wctx, content, "zh",
		func(context.Context, string, func(string, bool) error) error {
			atomic.AddInt32(&generateCalls, 1)
			return nil
		},
		func(string, bool) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("wait cancellation must end the call with the ctx error, got %v", err)
	}
	if generateCalls != 0 {
		t.Fatal("wait cancellation must NOT start another generation")
	}
	if n := cacheRowCount(t, db, content.ID); n != 0 {
		t.Fatalf("cancelled wait must not cache (rows=%d)", n)
	}
	if reacquired, _ := sf.Acquire(ctx, key, time.Minute); reacquired {
		t.Fatal("waiter must not release the foreign lease")
	}
}

func TestGetOrGenerateStreamProviderFailurePropagatesAndReleases(t *testing.T) {
	ctx := context.Background()
	sf := &memSingleflight{}
	svc, db := setupStreamStack(t, promptregistry.NewPromptResolver(nil), sf)
	content := seedCacheContent(t, db, 1036, "d")

	providerErr := errors.New("provider exploded mid-stream")
	forward := &recordedForward{failOn: -1}
	err := svc.GetOrGenerateStream(ctx, content, "zh",
		func(_ context.Context, _ string, emit func(string, bool) error) error {
			if err := emit("partial-", false); err != nil {
				return err
			}
			return providerErr
		},
		forward.forward)
	if !errors.Is(err, providerErr) {
		t.Fatalf("provider failure must propagate, got %v", err)
	}
	if n := cacheRowCount(t, db, content.ID); n != 0 {
		t.Fatalf("failed stream must NOT cache (rows=%d)", n)
	}
	// 持有者所有返回路径都释放。
	if reacquired, _ := sf.Acquire(ctx, streamLeaseKey(content, "zh", 0), time.Minute); !reacquired {
		t.Fatal("owner must release the lease on failure")
	}
}

func TestGetOrGenerateStreamForwardFailurePropagatesAndReleases(t *testing.T) {
	ctx := context.Background()
	sf := &memSingleflight{}
	svc, db := setupStreamStack(t, promptregistry.NewPromptResolver(nil), sf)
	content := seedCacheContent(t, db, 1037, "d")

	forwardErr := errors.New("client disconnected")
	forward := &recordedForward{failOn: 0, err: forwardErr}
	err := svc.GetOrGenerateStream(ctx, content, "zh",
		func(_ context.Context, _ string, emit func(string, bool) error) error {
			// 转发失败必须中断生成（emit 不返回 error 会吞掉客户端断开）。
			if err := emit("chunk", false); err != nil {
				return err
			}
			return emit("", true)
		},
		forward.forward)
	if !errors.Is(err, forwardErr) {
		t.Fatalf("forward failure must propagate, got %v", err)
	}
	if n := cacheRowCount(t, db, content.ID); n != 0 {
		t.Fatalf("forward-failed stream must NOT cache (rows=%d)", n)
	}
	if reacquired, _ := sf.Acquire(ctx, streamLeaseKey(content, "zh", 0), time.Minute); !reacquired {
		t.Fatal("owner must release the lease on forward failure")
	}
}

func TestGetOrGenerateStreamEmptyResultNotCached(t *testing.T) {
	ctx := context.Background()
	sf := &memSingleflight{}
	svc, db := setupStreamStack(t, promptregistry.NewPromptResolver(nil), sf)
	content := seedCacheContent(t, db, 1038, "d")

	forward := &recordedForward{failOn: -1}
	if err := svc.GetOrGenerateStream(ctx, content, "zh",
		func(_ context.Context, _ string, emit func(string, bool) error) error {
			return emit("", true) // 完整结束但内容为空
		},
		forward.forward); err != nil {
		t.Fatal(err)
	}
	deltas, done := forward.snapshot()
	if done != 1 {
		t.Fatalf("empty result must still forward done exactly once (done=%d)", done)
	}
	if len(deltas) != 0 {
		t.Fatalf("empty result must not forward body deltas: %v", deltas)
	}
	if n := cacheRowCount(t, db, content.ID); n != 0 {
		t.Fatalf("empty result must NOT cache (rows=%d)", n)
	}
	if reacquired, _ := sf.Acquire(ctx, streamLeaseKey(content, "zh", 0), time.Minute); !reacquired {
		t.Fatal("owner must release the lease after an empty result")
	}
}

func TestGetOrGenerateStreamVersionDriftMidGenerationKeepsStartIdentity(t *testing.T) {
	ctx := context.Background()
	store := &bumpablePromptStore{version: 1, contents: map[int]string{
		1: "DRIFT-V1 guide for {{title}} focus {{guide_focus}} in {{language}}",
		2: "DRIFT-V2 guide for {{title}} focus {{guide_focus}} in {{language}}",
	}}
	resolver := promptregistry.NewPromptResolver(store)
	svc, db := setupStreamStack(t, resolver, &memSingleflight{})
	content := seedCacheContent(t, db, 1039, "d")

	var gotTemplate string
	if err := svc.GetOrGenerateStream(ctx, content, "zh",
		func(_ context.Context, template string, emit func(string, bool) error) error {
			gotTemplate = template
			store.bump(2, resolver) // 生成中：admin 升 label + Invalidate
			return emitDone(emit, "v1-identity-body")
		},
		func(string, bool) error { return nil },
	); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(gotTemplate, "DRIFT-V1") {
		t.Fatalf("generator must receive the START template, got %q", gotTemplate)
	}
	var row model.ContentUsageGuideCache
	if err := db.Where("content_id = ? AND locale = ?", content.ID, "zh").First(&row).Error; err != nil {
		t.Fatalf("complete stream must persist: %v", err)
	}
	if row.PromptVersion != 1 || row.GuideMarkdown != "v1-identity-body" {
		t.Fatalf("row must carry the start identity (v1), got v%d %q", row.PromptVersion, row.GuideMarkdown)
	}
	if svc.PromptVersion(ctx) != 2 {
		t.Fatalf("current version must have moved to 2, got %d", svc.PromptVersion(ctx))
	}
	if _, ok := svc.FindValid(ctx, content, "zh"); ok {
		t.Fatal("a v2 request must not mis-hit the row generated under the v1 prompt")
	}
}

func TestGetOrGenerateStreamDriftBetweenResolveAndAcquireKeepsKeyIdentity(t *testing.T) {
	ctx := context.Background()
	store := &bumpablePromptStore{version: 1, contents: map[int]string{
		1: "DRIFT-V1 guide for {{title}} focus {{guide_focus}} in {{language}}",
		2: "DRIFT-V2 guide for {{title}} focus {{guide_focus}} in {{language}}",
	}}
	resolver := promptregistry.NewPromptResolver(store)
	sf := &hookSingleflight{onAcquire: func(string) {
		store.bump(2, resolver) // 渲染→取租约之间：label 升级 + Invalidate
	}}
	svc, db := setupStreamStack(t, resolver, sf)
	content := seedCacheContent(t, db, 1040, "d")

	var gotTemplate string
	if err := svc.GetOrGenerateStream(ctx, content, "zh",
		func(_ context.Context, template string, emit func(string, bool) error) error {
			gotTemplate = template
			return emitDone(emit, "key-identity-body")
		},
		func(string, bool) error { return nil },
	); err != nil {
		t.Fatal(err)
	}

	keys := sf.acquireLog()
	if len(keys) != 1 || !strings.HasSuffix(keys[0], ":1") {
		t.Fatalf("lease key must carry the START version: %v", keys)
	}
	if !strings.Contains(gotTemplate, "DRIFT-V1") {
		t.Fatalf("generator must receive the START template, got %q", gotTemplate)
	}
	var row model.ContentUsageGuideCache
	if err := db.Where("content_id = ? AND locale = ?", content.ID, "zh").First(&row).Error; err != nil {
		t.Fatalf("complete stream must persist: %v", err)
	}
	if row.PromptVersion != 1 {
		t.Fatalf("row must carry the start version 1, got %d", row.PromptVersion)
	}
	if svc.PromptVersion(ctx) != 2 {
		t.Fatalf("current version must have moved to 2, got %d", svc.PromptVersion(ctx))
	}
	if _, ok := svc.FindValid(ctx, content, "zh"); ok {
		t.Fatal("a v2 request must not mis-hit the row generated under the v1 prompt")
	}
}

func TestGetOrGenerateStreamOldWriterNeverOverwritesNewerVersionRow(t *testing.T) {
	ctx := context.Background()
	store := &bumpablePromptStore{version: 1, contents: map[int]string{
		1: "DRIFT-V1 guide for {{title}} focus {{guide_focus}} in {{language}}",
		2: "DRIFT-V2 guide for {{title}} focus {{guide_focus}} in {{language}}",
	}}
	resolver := promptregistry.NewPromptResolver(store)
	svc, db := setupStreamStack(t, resolver, &memSingleflight{})
	content := seedCacheContent(t, db, 1041, "d")

	// 老写者：v1 身份，生成中（等待期）。
	ownerAStarted := make(chan struct{})
	ownerADone := make(chan error, 1)
	var aDeltas []string
	go func() {
		ownerADone <- svc.GetOrGenerateStream(ctx, content, "zh",
			func(_ context.Context, _ string, emit func(string, bool) error) error {
				close(ownerAStarted) // 此刻 A 的 v1 身份（键/等待/落库）已固定
				time.Sleep(250 * time.Millisecond)
				return emitDone(emit, "old-version-body")
			},
			func(delta string, done bool) error {
				if !done {
					aDeltas = append(aDeltas, delta)
				}
				return nil
			})
	}()
	<-ownerAStarted

	// 等待期间版本升到 v2：新身份（v2）的写者先完成并落库。
	store.bump(2, resolver)
	if err := svc.GetOrGenerateStream(ctx, content, "zh",
		func(_ context.Context, template string, emit func(string, bool) error) error {
			if template != store.contentsFor(2) {
				t.Errorf("v2 writer must render the v2 template")
			}
			return emitDone(emit, "new-version-body")
		},
		func(string, bool) error { return nil }); err != nil {
		t.Fatal(err)
	}

	if err := <-ownerADone; err != nil {
		t.Fatalf("stale refusal is not an error for the old writer: %v", err)
	}
	if strings.Join(aDeltas, "") != "old-version-body" {
		t.Fatalf("old writer must still have streamed its own text: %q", strings.Join(aDeltas, ""))
	}
	var row model.ContentUsageGuideCache
	if err := db.Where("content_id = ? AND locale = ?", content.ID, "zh").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.PromptVersion != 2 || row.GuideMarkdown != "new-version-body" {
		t.Fatalf("old writer must not overwrite the newer-version row: v%d %q", row.PromptVersion, row.GuideMarkdown)
	}
	if cached, ok := svc.FindValid(ctx, content, "zh"); !ok || cached != "new-version-body" {
		t.Fatalf("current-version request must hit the v2 row: %q %v", cached, ok)
	}
}
