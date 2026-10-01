package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"omnicraft/backend/config"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

/* #729 匿名公开端点细档限流（heavy，TDD）——测试先行：本文件在实现落地前
 * 编写并确认失败。算法契约 = Redis 原子令牌桶（Lua，判定/扣量/过期一次
 * EVAL 完成），时钟由调用方注入（FakeClock 时间旅行），禁止单键 INCR+
 * 续 TTL 伪滑窗。 */

/* ---- 测试基建 ---- */

type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time          { return f.now }
func (f *fakeClock) Advance(d time.Duration) { f.now = f.now.Add(d) }

func newGuestTestStack(t *testing.T, mutate func(cfg *config.Config)) (*gin.Engine, *miniredis.Miniredis, *fakeClock, *config.Config) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	cfg := &config.Config{RateLimit: config.RateLimitConfig{Enabled: true}, Features: config.FeaturesConfig{GuestRateLimitEnabled: true}}
	if mutate != nil {
		mutate(cfg)
	}
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	limiter := NewGuestRateLimiter(rdb, &cfg.RateLimit, &cfg.Features, clock.Now)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	// 镜像真实装配（main.go SetTrustedProxies(cfg.Security.TrustedProxies)，
	// 出厂 = loopback）：gin 默认全信代理，不设此则 XFF 断言无意义。
	if err := engine.SetTrustedProxies([]string{"127.0.0.1"}); err != nil {
		t.Fatalf("trusted proxies: %v", err)
	}
	engine.GET("/thing", limiter.Tier("contents_detail"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return engine, mr, clock, cfg
}

func guestGet(engine *gin.Engine, ip string, headers map[string]string) *httptest.ResponseRecorder {
	return guestGetPath(engine, "/thing", ip, headers)
}

func guestGetPath(engine *gin.Engine, urlPath, ip string, headers map[string]string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, urlPath, nil)
	req.RemoteAddr = ip + ":12345"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	engine.ServeHTTP(w, req)
	return w
}

func guestSpecOf(t *testing.T, cfg *config.Config, tier string) GuestBucketSpec {
	t.Helper()
	spec, ok := cfg.RateLimit.GuestBucketFor(tier)
	if !ok {
		t.Fatalf("default tier spec missing for %s", tier)
	}
	return spec
}

/* ---- 三态（票面验收第 1 条）---- */

func TestGuestRateLimitThreeStates(t *testing.T) {
	engine, _, clock, cfg := newGuestTestStack(t, nil)
	spec := guestSpecOf(t, cfg, "contents_detail")

	// 状态 1：未达限放行（容量内）
	for i := 0; i < spec.Capacity; i++ {
		w := guestGet(engine, "198.51.100.7", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d within capacity: got %d", i+1, w.Code)
		}
	}
	// 状态 2：达限拒绝 = 429 + 结构化错误码 + Retry-After
	w := guestGet(engine, "198.51.100.7", nil)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("over capacity: got %d", w.Code)
	}
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("429 body not the Error envelope: %v (%s)", err, w.Body.String())
	}
	if body.Code != "GUEST_RATE_LIMIT_EXCEEDED" {
		t.Fatalf("code = %q", body.Code)
	}
	retryAfter, err := strconv.Atoi(w.Header().Get("Retry-After"))
	if err != nil || retryAfter < 1 {
		t.Fatalf("Retry-After = %q (%v)", w.Header().Get("Retry-After"), err)
	}
	// 状态 3：令牌回补后恢复放行
	clock.Advance(time.Duration(float64(time.Minute) / spec.RefillPerMinute * 1.2))
	if w := guestGet(engine, "198.51.100.7", nil); w.Code != http.StatusOK {
		t.Fatalf("after refill window: got %d", w.Code)
	}
}

/* ---- 算法正确性组（票面验收第 2 条）---- */

/*
captureDefaultSlog 把默认 logger 换成 buffer 并在测试结束还原；返回

	指针供事后断言（fail-open 告警出现在默认 logger 上）。
*/
func captureDefaultSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func requireNoFailOpenWarn(t *testing.T, buf *bytes.Buffer) {
	t.Helper()
	if strings.Contains(buf.String(), "fail-open") {
		t.Fatalf("unexpected fail-open during test (redis timeout/error leaked into allowed count): %s", buf.String())
	}
}

func TestGuestRateLimitConcurrentNoOverissue(t *testing.T) {
	// fail-open 感知（审查轮 -race 真实复现的 flake）：并发调度压力下
	// 100ms 固定超时偶发误判 Redis 故障 → fail-open 放行被计入 allowed。
	// 测试内放宽超时根除误判源，并捕获默认 logger——任何 fail-open 告警
	// 直接失败，绝不混入放行计数。
	origTimeout := guestRedisCallTimeout
	guestRedisCallTimeout = 5 * time.Second
	t.Cleanup(func() { guestRedisCallTimeout = origTimeout })
	logBuf := captureDefaultSlog(t)

	engine, _, _, cfg := newGuestTestStack(t, nil)
	spec := guestSpecOf(t, cfg, "contents_detail")
	if spec.Capacity < 2 {
		t.Fatalf("test needs capacity >= 2")
	}

	var allowed atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < spec.Capacity*5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := guestGet(engine, "203.0.113.5", nil)
			if w.Code == http.StatusOK {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	requireNoFailOpenWarn(t, logBuf)
	if got := allowed.Load(); got != int64(spec.Capacity) {
		t.Fatalf("concurrent burst: allowed %d, want exactly capacity %d (no over-issue)", got, spec.Capacity)
	}
}

func TestGuestRateLimitSustainedLowRateNeverAccumulatesBan(t *testing.T) {
	engine, _, clock, cfg := newGuestTestStack(t, nil)
	spec := guestSpecOf(t, cfg, "contents_detail")
	// 恰按回补速率消费：100 个请求永不限（伪滑窗写法在低速累计下会永久超限）
	interval := time.Duration(float64(time.Minute) / spec.RefillPerMinute)
	for i := 0; i < 100; i++ {
		clock.Advance(interval)
		if w := guestGet(engine, "198.51.100.9", nil); w.Code != http.StatusOK {
			t.Fatalf("sustained low-rate request %d rejected: %d", i, w.Code)
		}
	}
}

func TestGuestRateLimitRetryStormDoesNotDelayRecovery(t *testing.T) {
	engine, _, clock, cfg := newGuestTestStack(t, nil)
	spec := guestSpecOf(t, cfg, "contents_detail")
	for i := 0; i < spec.Capacity; i++ {
		guestGet(engine, "198.51.100.11", nil)
	}
	// 拒绝重试风暴（不推进时钟 → 全部 429）
	for i := 0; i < 20; i++ {
		if w := guestGet(engine, "198.51.100.11", nil); w.Code != http.StatusTooManyRequests {
			t.Fatalf("storm request %d unexpectedly allowed", i)
		}
	}
	// 恢复时间不被风暴推迟：恰回补 1 令牌即放行
	clock.Advance(time.Duration(float64(time.Minute) / spec.RefillPerMinute * 1.05))
	if w := guestGet(engine, "198.51.100.11", nil); w.Code != http.StatusOK {
		t.Fatalf("recovery delayed by rejected storm: got %d", w.Code)
	}
}

func TestGuestRateLimitTTLExpiryRecyclesBucket(t *testing.T) {
	engine, mr, clock, cfg := newGuestTestStack(t, nil)
	spec := guestSpecOf(t, cfg, "contents_detail")
	for i := 0; i < spec.Capacity; i++ {
		guestGet(engine, "198.51.100.13", nil)
	}
	if w := guestGet(engine, "198.51.100.13", nil); w.Code != http.StatusTooManyRequests {
		t.Fatal("expected depletion first")
	}
	// TTL = 桶满时长 + 余量（时间旅行快进过 TTL → key 回收，满桶重启）
	ttl := time.Duration(float64(spec.Capacity)/spec.RefillPerMinute*float64(time.Minute)) + 2*time.Minute
	mr.FastForward(ttl)
	clock.Advance(ttl)
	if w := guestGet(engine, "198.51.100.13", nil); w.Code != http.StatusOK {
		t.Fatalf("bucket not recycled after TTL: got %d", w.Code)
	}
}

func TestGuestRateLimitIsolationAcrossIPAndTier(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cfg := &config.Config{RateLimit: config.RateLimitConfig{Enabled: true}, Features: config.FeaturesConfig{GuestRateLimitEnabled: true}}
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	limiter := NewGuestRateLimiter(rdb, &cfg.RateLimit, &cfg.Features, clock.Now)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/detail", limiter.Tier("contents_detail"), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	engine.GET("/list", limiter.Tier("contents_list"), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	detailSpec := guestSpecOf(t, cfg, "contents_detail")
	listSpec := guestSpecOf(t, cfg, "contents_list")

	// 跨 IP：A 打满，B 不受影响
	for i := 0; i < detailSpec.Capacity; i++ {
		guestGetPath(engine, "/detail", "198.51.100.31", nil)
	}
	if w := guestGetPath(engine, "/detail", "198.51.100.31", nil); w.Code != http.StatusTooManyRequests {
		t.Fatal("IP A should be limited")
	}
	if w := guestGetPath(engine, "/detail", "198.51.100.32", nil); w.Code != http.StatusOK {
		t.Fatalf("IP B must be isolated, got %d", w.Code)
	}
	// 跨档位：同 IP 的另一档位不受 contents_detail 打满影响
	for i := 0; i < listSpec.Capacity; i++ {
		if w := guestGetPath(engine, "/list", "198.51.100.31", nil); w.Code != http.StatusOK {
			t.Fatalf("tier isolation broken at %d: %d", i, w.Code)
		}
	}
	// 键 = 稳定档位标识 + IP，不含资源 ID/query（键形态断言：档位白名单 +
	// 纯 IP 尾段，无路径参数或 query 成分）
	keys := mr.Keys()
	if len(keys) == 0 {
		t.Fatal("no bucket keys")
	}
	keyShape := regexp.MustCompile(`^ratelimit:guest:(contents_detail|contents_list):[0-9.]+$`)
	for _, k := range keys {
		if !keyShape.MatchString(k) {
			t.Fatalf("unexpected key shape: %q (template key must carry no resource ID/query)", k)
		}
	}
}

/* ---- 豁免名单 ---- */

func TestGuestRateLimitExemptIPBypassesNewLayer(t *testing.T) {
	engine, _, _, cfg := newGuestTestStack(t, func(c *config.Config) {
		c.RateLimit.GuestExemptIPs = []string{"198.51.100.99", "192.0.2.0/24"}
	})
	for i := 0; i < 50; i++ {
		if w := guestGet(engine, "198.51.100.99", nil); w.Code != http.StatusOK {
			t.Fatalf("exact exempt IP rejected at %d: %d", i, w.Code)
		}
		if w := guestGet(engine, "192.0.2.77", nil); w.Code != http.StatusOK {
			t.Fatalf("CIDR exempt IP rejected at %d: %d", i, w.Code)
		}
	}
	spec := guestSpecOf(t, cfg, "contents_detail")
	for i := 0; i < spec.Capacity; i++ {
		guestGet(engine, "203.0.113.99", nil)
	}
	if w := guestGet(engine, "203.0.113.99", nil); w.Code != http.StatusTooManyRequests {
		t.Fatal("non-exempt IP should still be limited")
	}
}

/* ---- Redis 故障 fail-open + 恢复（票面验收第 3 条）---- */

func TestGuestRateLimitFailOpenAndRecover(t *testing.T) {
	engine, mr, _, cfg := newGuestTestStack(t, nil)
	spec := guestSpecOf(t, cfg, "contents_detail")
	// 故障注入：所有命令报错 → 全部放行（fail-open，可用性优先）
	mr.SetError("redis unavailable")
	for i := 0; i < 30; i++ {
		if w := guestGet(engine, "198.51.100.17", nil); w.Code != http.StatusOK {
			t.Fatalf("fail-open request %d rejected: %d", i, w.Code)
		}
	}
	// 恢复后限流自动生效（故障态打空的请求不得换得无限通行）
	mr.SetError("")
	for i := 0; i < spec.Capacity; i++ {
		if w := guestGet(engine, "198.51.100.17", nil); w.Code != http.StatusOK {
			t.Fatalf("post-recovery request %d: %d", i, w.Code)
		}
	}
	if w := guestGet(engine, "198.51.100.17", nil); w.Code != http.StatusTooManyRequests {
		t.Fatal("limiting must resume after redis recovery")
	}
}

/* ---- 关闭态零 Redis 调用 / 零响应差异（票面验收第 4 条）---- */

func TestGuestRateLimitDisabledAddsNoRedisCallOrResponseDiff(t *testing.T) {
	// 关闭态零调用须有真检出力（审查轮指正永真版）：用活 miniredis——若
	// 实现漏掉 enabled 门，任何一次桶 EVAL 都会 HSET 落 key，键集非空即
	// 败；fail-open 告警（默认 logger）同样不得出现。
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	logBuf := captureDefaultSlog(t)

	cfg := &config.Config{RateLimit: config.RateLimitConfig{Enabled: true}, Features: config.FeaturesConfig{GuestRateLimitEnabled: false}}
	clock := &fakeClock{now: time.Now()}
	limiter := NewGuestRateLimiter(rdb, &cfg.RateLimit, &cfg.Features, clock.Now)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/thing", limiter.Tier("contents_detail"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	for i := 0; i < 5; i++ {
		if w := guestGet(engine, "198.51.100.19", nil); w.Code != http.StatusOK {
			t.Fatalf("disabled layer must be a no-op, request %d got %d", i, w.Code)
		}
	}
	// 总开关关（rate_limit.enabled=false）同态：连坐关闭新层。
	cfg.RateLimit.Enabled = false
	cfg.Features.GuestRateLimitEnabled = true
	for i := 0; i < 5; i++ {
		if w := guestGet(engine, "198.51.100.19", nil); w.Code != http.StatusOK {
			t.Fatalf("master-off must disable guest layer too, request %d got %d", i, w.Code)
		}
	}
	if keys := mr.Keys(); len(keys) != 0 {
		t.Fatalf("disabled layer touched redis (bucket keys created): %v", keys)
	}
	requireNoFailOpenWarn(t, logBuf)
}

/* ---- 真实 IP 传递链（票面验收第 5 条，代理链身份）---- */

func TestGuestRateLimitUntrustedXFFKeepsSingleIdentity(t *testing.T) {
	engine, _, _, cfg := newGuestTestStack(t, nil)
	// 不信代理直连：同一 RemoteAddr 变造 XFF 不换桶
	spec := guestSpecOf(t, cfg, "contents_detail")
	xfValues := []string{"1.1.1.1", "2.2.2.2, 3.3.3.3", "203.0.113.1"}
	for i := 0; i < spec.Capacity; i++ {
		w := guestGet(engine, "198.51.100.21", map[string]string{"X-Forwarded-For": xfValues[i%len(xfValues)]})
		if w.Code != http.StatusOK {
			t.Fatalf("setup request %d: %d", i, w.Code)
		}
	}
	if w := guestGet(engine, "198.51.100.21", map[string]string{"X-Forwarded-For": "9.9.9.9"}); w.Code != http.StatusTooManyRequests {
		t.Fatal("forged XFF must not buy a fresh bucket")
	}
}

/*
可信代理后的不同实际访客隔离（票面预裁决点 3 的正向半边）：

	RemoteAddr 为可信代理（loopback）时按 XFF 里的真实访客分桶。
*/
func TestGuestRateLimitTrustedProxyVisitorsIsolated(t *testing.T) {
	engine, _, _, cfg := newGuestTestStack(t, nil)
	spec := guestSpecOf(t, cfg, "contents_detail")
	// 访客 A（经可信代理）打满
	for i := 0; i < spec.Capacity; i++ {
		w := guestGet(engine, "127.0.0.1", map[string]string{"X-Forwarded-For": "198.51.100.41"})
		if w.Code != http.StatusOK {
			t.Fatalf("visitor A setup %d: %d", i, w.Code)
		}
	}
	if w := guestGet(engine, "127.0.0.1", map[string]string{"X-Forwarded-For": "198.51.100.41"}); w.Code != http.StatusTooManyRequests {
		t.Fatal("visitor A should be limited")
	}
	// 访客 B 同经代理，独立桶
	if w := guestGet(engine, "127.0.0.1", map[string]string{"X-Forwarded-For": "198.51.100.42"}); w.Code != http.StatusOK {
		t.Fatalf("visitor B must be isolated, got %d", w.Code)
	}
}
