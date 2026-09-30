package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service/promptregistry"
)

const (
	// generationLeaseTTL bounds a crashed generator's dedup lease; healthy
	// generation releases explicitly.
	generationLeaseTTL = 10 * time.Minute
	// waiterPollInterval/waiterTimeout bound the merged-waiter path: a
	// waiter polls the shared cache row instead of double-generating.
	waiterPollInterval = 250 * time.Millisecond
	defaultWaiterWait  = 30 * time.Second
)

// UsageGuideCacheService (#728): the single generation-and-cache entry shared
// by publish-time preheat and first-visitor lazy generation. Auto results
// live in content_usage_guide_cache only — author rows in
// content_usage_guides are read (preheat skip decision) but never written.
type UsageGuideCacheService struct {
	/* waiterWait 可注入（测试压缩等待者超时路径；<=0 走默认 30s）。 */
	waiterWait  time.Duration
	repo        *repository.UsageGuideCacheRepository
	sf          RedisSingleflight
	prompts     *promptregistry.PromptResolver
	guideRepo   *repository.UsageGuideRepository
	contentRepo *repository.ContentRepository
	cfg         *config.Config
}

func NewUsageGuideCacheService(
	repo *repository.UsageGuideCacheRepository,
	sf RedisSingleflight,
	prompts *promptregistry.PromptResolver,
	guideRepo *repository.UsageGuideRepository,
	contentRepo *repository.ContentRepository,
	cfg *config.Config,
) *UsageGuideCacheService {
	return &UsageGuideCacheService{repo: repo, sf: sf, prompts: prompts, guideRepo: guideRepo, contentRepo: contentRepo, cfg: cfg}
}

// UsageGuideInputFingerprint hashes the generation inputs whose edits
// invalidate the cache (title / description / content type).
func UsageGuideInputFingerprint(title, description, contentType string) string {
	sum := sha256.Sum256([]byte(title + "\x1f" + description + "\x1f" + contentType))
	return hex.EncodeToString(sum[:])
}

// PromptVersion reports the production version of usage_guide_prompt — a
// slot bump invalidates rows generated under older prompts.
func (s *UsageGuideCacheService) PromptVersion(ctx context.Context) int {
	if s.prompts == nil {
		return 0 // 与 promptregistry 无注册表态的版本空间一致（builtin=0）
	}
	_, version := s.prompts.Resolve(ctx, promptregistry.SlotUsageGuide)
	return version
}

// FindValid returns the cached guide when the row matches the content's
// current input fingerprint AND the current prompt version.
func (s *UsageGuideCacheService) FindValid(ctx context.Context, content *model.ContentItem, locale string) (string, bool) {
	if s.repo == nil || content == nil {
		return "", false
	}
	row, err := s.repo.Find(ctx, content.ID, locale)
	if err != nil || row == nil {
		return "", false
	}
	if row.InputFingerprint != UsageGuideInputFingerprint(content.Title, content.Description, content.ContentType) {
		return "", false
	}
	if row.PromptVersion != s.PromptVersion(ctx) {
		return "", false
	}
	return row.GuideMarkdown, true
}

// GetOrGenerate is the unified entry (#728):
//   - force (studio draft) neither reads nor writes the cache;
//   - cache hit returns with zero LLM calls;
//   - cross-process dedup: the lease holder generates and guarded-upserts;
//     concurrent first visitors poll the shared row instead of re-running
//     (server and worker are separate processes — dedup must be shared);
//   - failures, empty results and stale-writer races never poison the cache;
//     a failed generation releases the lease so the next request retries.
func (s *UsageGuideCacheService) GetOrGenerate(
	ctx context.Context,
	content *model.ContentItem,
	locale string,
	force bool,
	generate func(context.Context) (string, error),
) (string, error) {
	if force || s.repo == nil || s.sf == nil || content == nil {
		return generate(ctx)
	}
	fingerprint := UsageGuideInputFingerprint(content.Title, content.Description, content.ContentType)
	version := s.PromptVersion(ctx)

	if cached, ok := s.findValidRow(ctx, content.ID, locale, fingerprint, version); ok {
		return cached, nil
	}

	key := fmt.Sprintf("guide:%d:%s:%s:%d", content.ID, locale, fingerprint, version)
	acquired, err := s.sf.Acquire(ctx, key, generationLeaseTTL)
	if err != nil {
		// dedup 基础设施故障：不阻塞读取路径，退化为本地生成（缓存仍受守卫保护）。
		slog.WarnContext(ctx, "usage guide cache: dedup acquire failed, generating locally", "error", err)
		return s.generateAndCache(ctx, content, locale, fingerprint, version, generate)
	}
	if !acquired {
		// 与预热任务/其他进程的生成合并：等待共享结果。
		if cached, ok := s.waitSharedResult(ctx, content.ID, locale, fingerprint, version); ok {
			return cached, nil
		}
		// 等待超时（生成方崩溃/过慢）——本地兜底生成；守卫防旧写。
		return s.generateAndCache(ctx, content, locale, fingerprint, version, generate)
	}
	defer func() { _ = s.sf.Release(ctx, key) }()
	return s.generateAndCache(ctx, content, locale, fingerprint, version, generate)
}

func (s *UsageGuideCacheService) findValidRow(ctx context.Context, contentID int64, locale, fingerprint string, version int) (string, bool) {
	row, err := s.repo.Find(ctx, contentID, locale)
	if err != nil || row == nil {
		return "", false
	}
	if row.InputFingerprint != fingerprint || row.PromptVersion != version {
		return "", false
	}
	return row.GuideMarkdown, true
}

// generateAndCache runs the generator and persists only COMPLETE successful
// results; a stale-writer refusal is not an error (the caller still gets the
// generated text, only the newer cache row stays).
func (s *UsageGuideCacheService) generateAndCache(
	ctx context.Context,
	content *model.ContentItem,
	locale, fingerprint string,
	version int,
	generate func(context.Context) (string, error),
) (string, error) {
	result, err := generate(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result) == "" {
		return result, nil // 半截/空结果不缓存
	}
	row := &model.ContentUsageGuideCache{
		ContentItemID:    content.ID,
		Locale:           locale,
		PromptVersion:    version,
		InputFingerprint: fingerprint,
		ContentUpdatedAt: content.UpdatedAt,
		GuideMarkdown:    result,
		Source:           model.UsageGuideCacheSourceAuto,
	}
	if _, err := s.repo.UpsertGuarded(ctx, row); err != nil {
		slog.WarnContext(ctx, "usage guide cache: guarded upsert failed", "error", err)
	}
	return result, nil
}

// waitSharedResult polls the shared cache row until it appears, the caller
// cancels, or the bounded timeout elapses. A single waiter disconnecting
// only cancels ITS wait — the shared generation keeps running.
func (s *UsageGuideCacheService) waitSharedResult(ctx context.Context, contentID int64, locale, fingerprint string, version int) (string, bool) {
	wait := s.waiterWait
	if wait <= 0 {
		wait = defaultWaiterWait
	}
	ticker := time.NewTicker(waiterPollInterval)
	defer ticker.Stop()
	deadline := time.After(wait)
	for {
		select {
		case <-ctx.Done():
			return "", false
		case <-deadline:
			return "", false
		case <-ticker.C:
			if cached, ok := s.findValidRow(ctx, contentID, locale, fingerprint, version); ok {
				return cached, true
			}
		}
	}
}

// PreheatContent is the publish-time hook (#728): for zh/en, a language with
// an author-confirmed row is already satisfied; only the missing languages
// generate into the cache. Failures are logged and left to lazy generation
// — publishing is never blocked by this path. Honors the web-agent feature
// gate (auto generation follows the existing switch).
func (s *UsageGuideCacheService) PreheatContent(ctx context.Context, contentID int64, generate func(context.Context, *model.ContentItem, string, bool) (string, error)) {
	if s.cfg == nil || !s.cfg.Agent.WebAgentEnabled {
		return
	}
	content, err := s.contentRepo.FindByID(contentID)
	if err != nil || content == nil || content.Status != "published" {
		return
	}
	for _, locale := range []string{"zh", "en"} {
		if row, err := s.guideRepo.Find(ctx, contentID, locale); err == nil && row != nil {
			continue // 作者确认行（含 llm_assisted）= 该语言已就绪
		}
		if _, err := s.GetOrGenerate(ctx, content, locale, false, func(gctx context.Context) (string, error) {
			return generate(gctx, content, locale, false)
		}); err != nil {
			slog.WarnContext(ctx, "usage guide preheat failed (lazy generation will retry)", "content_id", contentID, "locale", locale, "error", err)
		}
	}
}

// ---------------------------------------------------------------------------
// 流式路径原语：租约持有者边生成边转发；等待者合并到共享结果。

// UsageGuideLease is a generation lease handed to the streaming entry.
type UsageGuideLease struct {
	sf    RedisSingleflight
	key   string
	owner bool
}

// AcquireForStream: cache hit → (nil, true, cached); lease owner → (lease,
// false, ""); otherwise a WAITER lease (owner=false) whose WaitShared merges
// into the running generation.
func (s *UsageGuideCacheService) AcquireForStream(ctx context.Context, content *model.ContentItem, locale string) (*UsageGuideLease, bool, string) {
	if s.repo == nil || s.sf == nil || content == nil {
		return nil, false, ""
	}
	fingerprint := UsageGuideInputFingerprint(content.Title, content.Description, content.ContentType)
	version := s.PromptVersion(ctx)
	if cached, ok := s.findValidRow(ctx, content.ID, locale, fingerprint, version); ok {
		return nil, true, cached
	}
	key := fmt.Sprintf("guide:%d:%s:%s:%d", content.ID, locale, fingerprint, version)
	acquired, err := s.sf.Acquire(ctx, key, generationLeaseTTL)
	if err != nil {
		slog.WarnContext(ctx, "usage guide cache: stream dedup acquire failed, generating locally", "error", err)
		return &UsageGuideLease{key: "", owner: true}, false, "" // 本地直生成（无租约守卫仍受 UpsertGuarded 保护）
	}
	if acquired {
		return &UsageGuideLease{sf: s.sf, key: key, owner: true}, false, ""
	}
	return &UsageGuideLease{key: key, owner: false}, false, ""
}

// Owner reports whether the lease holder must run the generation.
func (l *UsageGuideLease) Owner() bool { return l != nil && l.owner }

// WaitShared: waiter path — poll the shared row (single waiter cancel only
// cancels its own wait).
func (s *UsageGuideCacheService) WaitShared(ctx context.Context, content *model.ContentItem, locale string) (string, bool) {
	if s.repo == nil || content == nil {
		return "", false
	}
	fingerprint := UsageGuideInputFingerprint(content.Title, content.Description, content.ContentType)
	return s.waitSharedResult(ctx, content.ID, locale, fingerprint, s.PromptVersion(ctx))
}

// SaveComplete persists a fully-successful stream result (owner lease only;
// guarded against stale writers).
func (s *UsageGuideCacheService) SaveComplete(ctx context.Context, content *model.ContentItem, locale, result string) {
	if s.repo == nil || content == nil || strings.TrimSpace(result) == "" {
		return
	}
	row := &model.ContentUsageGuideCache{
		ContentItemID:    content.ID,
		Locale:           locale,
		PromptVersion:    s.PromptVersion(ctx),
		InputFingerprint: UsageGuideInputFingerprint(content.Title, content.Description, content.ContentType),
		ContentUpdatedAt: content.UpdatedAt,
		GuideMarkdown:    result,
		Source:           model.UsageGuideCacheSourceAuto,
	}
	if _, err := s.repo.UpsertGuarded(ctx, row); err != nil {
		slog.WarnContext(ctx, "usage guide cache: stream guarded upsert failed", "error", err)
	}
}

// Release drops the generation lease (owner path). Uses a fresh context —
// the streaming request context may already be cancelled at this point.
func (l *UsageGuideLease) Release() {
	if l == nil || !l.owner || l.key == "" || l.sf == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = l.sf.Release(ctx, l.key)
}
