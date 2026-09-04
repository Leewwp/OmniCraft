package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/events"
	"omnicraft/backend/internal/pkg/rediskeys"
	"omnicraft/backend/internal/repository"
)

var (
	ErrReviewTargetNotFound = errors.New("review target not found")
	// ErrCoverNotPlatformOSSObject is returned when a cover URL is not a
	// platform-verified OSS object (cfg.OSS.Domain prefix): such a cover is
	// never handed to Green for scanning.
	ErrCoverNotPlatformOSSObject = errors.New("cover image is not a platform OSS object")
)

// greenScanner is the scan seam for ReviewService. *aliyun.GreenClient is the
// production implementation; tests inject a fake to assert scan parameters and
// rejection semantics without real Aliyun credentials.
type greenScanner interface {
	TextModeration(ctx context.Context, text string) (*aliyun.GreenScanResult, error)
	ImageModeration(ctx context.Context, imageURL string) (*aliyun.GreenScanResult, error)
	VideoAsyncScan(ctx context.Context, params aliyun.VideoScanParams) (*aliyun.GreenScanResult, error)
}

type SubmitReviewInput struct {
	TargetType  string
	TargetID    int64
	ContentType string
	Title       string
	Description string
	AuthorID    int64
	// CoverImageURL is the cover image of the reviewed target (content
	// CoverImageURL / IP CoverURL). Only platform-verified OSS object URLs are
	// scanned; anything else fails the submission with
	// ErrCoverNotPlatformOSSObject.
	CoverImageURL string
	Attachments   []AttachmentInput
	// SyncKey is an opaque stable idempotency key for at-least-once
	// redelivery (the worker path stamps it with "sync:" + message id). A
	// non-empty SyncKey short-circuits a duplicate submission before any Green
	// scan and lands in provider_task_id so the 068 unique index dedups
	// concurrent redeliveries. Empty keeps the legacy HTTP-direct behavior:
	// every submission scans and records with a NULL key.
	SyncKey string
}

// SetGreenScanner injects a Green scan provider. Tests substitute a fake to
// assert scan parameters and rejection semantics without real credentials.
func (s *ReviewService) SetGreenScanner(sc greenScanner) {
	s.green = sc
}

type AICallbackInput struct {
	TargetType     string                 `json:"target_type" binding:"required"`
	TargetID       int64                  `json:"target_id" binding:"required"`
	Result         string                 `json:"result" binding:"required"`
	RawResponse    map[string]interface{} `json:"raw_response"`
	ProviderTaskID string                 `json:"provider_task_id"`
}

type ReviewService struct {
	db          *gorm.DB
	rdb         *redis.Client
	cfg         *config.Config
	reputSvc    *ReputationService
	green       greenScanner
	oss         *aliyun.OSSClient
	outbox      repository.OutboxWriter
	archiveGate *ArchiveScanGate
	notifSvc    *NotificationService
}

func NewReviewService(db *gorm.DB, rdb *redis.Client, cfg *config.Config, reputSvc *ReputationService) *ReviewService {
	var greenClient *aliyun.GreenClient
	var ossClient *aliyun.OSSClient
	if cfg != nil {
		greenClient = aliyun.NewGreenClient(cfg.Green.AccessKeyID, cfg.Green.AccessKeySecret, cfg.Green.Region)
		// Signing client for scan URLs: moderation objects live in the
		// private bucket, and Aliyun cannot fetch unsigned platform URLs
		// (probed 2026-09-01: Green received OSS 403 for them). nil when
		// OSS is not configured, preserving local fail-open behavior.
		ossClient, _ = aliyun.NewOSSClient(cfg.OSS.Endpoint, cfg.OSS.AccessKeyID, cfg.OSS.AccessKeySecret, cfg.OSS.BucketName)
	}
	return &ReviewService{
		db:       db,
		rdb:      rdb,
		cfg:      cfg,
		reputSvc: reputSvc,
		green:    greenClient,
		oss:      ossClient,
	}
}

// SetOutboxRepository attaches the transactional outbox. Content terminal
// transitions (published/banned) write one outbox event per transition inside
// the same database transaction as the status update.
func (s *ReviewService) SetOutboxRepository(outbox repository.OutboxWriter) {
	s.outbox = outbox
}

// SetArchiveScanGate makes the content review transition consume the same
// clean-only policy as downloads. A blocked or pending archive leaves content
// unpublished and returns an error so the review message is retried.
func (s *ReviewService) SetArchiveScanGate(gate *ArchiveScanGate) {
	s.archiveGate = gate
}

// ReviewText runs Aliyun Green text moderation over text and returns the
// normalized result ("pass", "review" or "block"). When the Green client is
// not configured it returns aliyun.ErrGreenNotConfigured so callers can apply
// their environment-specific availability policy.
func (s *ReviewService) ReviewText(ctx context.Context, text string) (string, error) {
	if s == nil || s.green == nil {
		return "", aliyun.ErrGreenNotConfigured
	}
	res, err := s.green.TextModeration(ctx, text)
	if err != nil {
		return "", err
	}
	return NormalizeReviewResult(res.Result), nil
}

// ReviewImageURL runs a synchronous Aliyun Green image scan over a single
// platform OSS object URL and returns the normalized result ("pass", "review"
// or "block"). When the Green client is not configured it returns
// aliyun.ErrGreenNotConfigured so callers can apply their environment-specific
// availability policy. Unlike SubmitForAIReview this seam performs no record
// writes and no reputation side effects: callers own their persistence rules.
func (s *ReviewService) ReviewImageURL(ctx context.Context, imageURL string) (string, error) {
	if s == nil || s.green == nil {
		return "", aliyun.ErrGreenNotConfigured
	}
	// Aliyun fetches the URL server-side, so platform objects must be handed
	// over as signed read URLs (private bucket); external URLs pass through.
	res, err := s.green.ImageModeration(ctx, s.resolveScanURL(imageURL))
	if err != nil {
		return "", err
	}
	return NormalizeReviewResult(res.Result), nil
}

func (s *ReviewService) SubmitForAIReview(ctx context.Context, in SubmitReviewInput) error {
	if s == nil || s.db == nil {
		return errors.New("review service not initialized")
	}
	if s.green == nil {
		return aliyun.ErrGreenNotConfigured
	}

	syncKey := strings.TrimSpace(in.SyncKey)
	// #195: at-least-once redelivery of the same submit_ai_review message is
	// deduplicated by the stable synthetic key ("sync:" + message id). A
	// record that already carries the key means the first delivery completed:
	// return success without re-scanning (a duplicate scan would burn Green
	// quota) and without re-recording.
	if syncKey != "" {
		var count int64
		if err := s.db.WithContext(ctx).Model(&model.AIReviewRecord{}).
			Where("provider = ? AND provider_task_id = ?", "aliyun", syncKey).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
	}

	result := "pass"
	raw := map[string]interface{}{"source": "green"}

	textResult, err := s.green.TextModeration(ctx, strings.TrimSpace(in.Title+"\n"+in.Description))
	if err != nil {
		return err
	}
	result = MergeReviewResult(result, textResult.Result)
	for k, v := range textResult.RawResponse {
		raw[k] = v
	}

	// Cover images enter the image review as a first-class input. The cover
	// must be a platform-verified OSS object: arbitrary URLs are never handed
	// to Green, and a non-platform cover fails the submission explicitly
	// (decision 6).
	if in.CoverImageURL != "" {
		coverURL, coverErr := s.resolveCoverScanURL(in.CoverImageURL)
		if coverErr != nil {
			return coverErr
		}
		coverRes, scanErr := s.green.ImageModeration(ctx, s.resolveScanURL(coverURL))
		if scanErr != nil {
			return scanErr
		}
		result = MergeReviewResult(result, coverRes.Result)
	}

	for _, a := range in.Attachments {
		mime := strings.ToLower(strings.TrimSpace(a.MimeType))
		ossKey := strings.TrimSpace(a.OSSKey)
		if ossKey == "" {
			continue
		}

		scanURL := s.resolveScanURL(s.resolveScanObjectURL(ossKey))
		if strings.HasPrefix(mime, "image/") {
			imgRes, scanErr := s.green.ImageModeration(ctx, scanURL)
			if scanErr != nil {
				return scanErr
			}
			result = MergeReviewResult(result, imgRes.Result)
		}
		if strings.HasPrefix(mime, "video/") || strings.EqualFold(a.FileType, "video") {
			videoRes, scanErr := s.green.VideoAsyncScan(ctx, aliyun.VideoScanParams{
				VideoURL:    scanURL,
				CallbackURL: s.cfg.Green.CallbackURL,
				Seed:        s.cfg.Green.Seed,
				DataID:      fmt.Sprintf("%s:%d", in.TargetType, in.TargetID),
			})
			if scanErr != nil {
				return scanErr
			}
			result = MergeReviewResult(result, videoRes.Result)
			if videoRes.TaskID != "" {
				raw["video_task_id"] = videoRes.TaskID
			}
		}
	}

	callback := AICallbackInput{
		TargetType:  in.TargetType,
		TargetID:    in.TargetID,
		Result:      result,
		RawResponse: raw,
	}
	// The synthetic sync key lands in provider_task_id so recordAIReview lets
	// the 068 unique index dedup a concurrent redelivery racing the count
	// check above. The "sync:" prefix never collides with Aliyun async task
	// ids, keeping the sync/async idempotency keys distinct (068 comment).
	if syncKey != "" {
		callback.ProviderTaskID = syncKey
	}
	if err := s.ProcessAICallback(ctx, callback); err != nil {
		return err
	}
	return nil
}

func (s *ReviewService) ProcessAICallback(ctx context.Context, in AICallbackInput) error {
	if s == nil || s.db == nil {
		return errors.New("review service not initialized")
	}

	result := NormalizeReviewResult(in.Result)
	providerTaskID := strings.TrimSpace(in.ProviderTaskID)

	// Idempotent short-circuit: a callback for an already-recorded provider
	// task id returns success (HTTP 200 semantics) without re-recording,
	// re-penalizing or re-freezing. Alibaba Cloud retries callback delivery,
	// so duplicate callbacks are the normal case, not an error.
	if providerTaskID != "" {
		var count int64
		if err := s.db.WithContext(ctx).Model(&model.AIReviewRecord{}).
			Where("provider = ? AND provider_task_id = ?", "aliyun", providerTaskID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
	}

	// The dedup re-check and the record insert share one transaction with the
	// status transition, so two concurrent duplicate callbacks cannot both
	// pass the check; the unique index converts the losing insert into a
	// no-op. The record is written before the repeat-violation count so the
	// current violation is visible inside the window.
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		recorded, err := s.recordAIReview(ctx, tx, in.TargetType, in.TargetID, result, in.RawResponse, providerTaskID)
		if err != nil {
			return err
		}
		if !recorded {
			return nil
		}

		if strings.EqualFold(in.TargetType, "ip") {
			return s.processIPReviewResult(tx, in.TargetID, result)
		}
		if !strings.EqualFold(in.TargetType, "content") {
			return nil
		}
		return s.applyContentReviewResult(ctx, tx, in.TargetID, result)
	})
}

// applyContentReviewResult transitions a content item according to the
// normalized AI result. banned is the terminal state of the AI channel:
// pass and review results use a conditional update that can never overwrite
// a banned row, so a late async result cannot resurrect blocked content.
// The block branch stays idempotent: an already-banned row is not
// re-penalized.
func (s *ReviewService) applyContentReviewResult(ctx context.Context, tx *gorm.DB, contentID int64, result string) error {
	var content model.ContentItem
	if err := tx.First(&content, contentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReviewTargetNotFound
		}
		return err
	}

	switch result {
	case "block":
		if content.Status == "banned" {
			return nil
		}
		if err := tx.Model(&model.ContentItem{}).Where("id = ?", content.ID).Update("status", "banned").Error; err != nil {
			return err
		}
		if err := s.emitContentEvent(ctx, tx, events.TopicContentBanned, content, "banned"); err != nil {
			return err
		}
		relatedID := content.ID
		if s.reputSvc != nil {
			if err := addReputationTx(tx, content.AuthorID, -3, "ai_violation", &relatedID); err != nil {
				return err
			}
		}
		s.notifyContentStatus(&content, "banned", "AI 审核判定违规")
		return s.applyRepeatViolationPenalty(ctx, tx, content.AuthorID)
	case "review":
		res := tx.Model(&model.ContentItem{}).
			Where("id = ? AND status <> ?", content.ID, "banned").
			Update("status", "under_review")
		if err := res.Error; err != nil {
			return err
		}
		if res.RowsAffected == 0 {
			return nil
		}
		s.notifyContentStatus(&content, "under_review", "AI 审核转人工复核")
		return s.ensureJudgeCase(tx, content)
	default:
		if s.archiveGate != nil {
			if err := s.archiveGate.RequireContentCleanTx(ctx, tx, content.ID); err != nil {
				// The archive worker will notify this service after a clean
				// result. Keep the AI pass record committed so that notification
				// can publish the content without re-running Green moderation.
				if errors.Is(err, ErrArchiveNotClean) {
					return nil
				}
				return err
			}
		}
		res := tx.Model(&model.ContentItem{}).
			Where("id = ? AND status <> ?", content.ID, "banned").
			Update("status", "published")
		if err := res.Error; err != nil {
			return err
		}
		if res.RowsAffected == 0 {
			return nil
		}
		// Postgres reports rows affected even for value-identical updates, so
		// the pre-update status is the real transition guard: only an actual
		// pending/under_review -> published transition emits an event.
		if content.Status == "published" {
			return nil
		}
		s.notifyContentStatus(&content, "published", "")
		if err := s.emitContentEvent(ctx, tx, events.TopicContentPublished, content, "published"); err != nil {
			return err
		}
	}
	return nil
}

// notifyContentStatus informs the author about a content status transition
// decided by the AI review channel. Best-effort by design (FIX-17): the
// notification goes through the queue/safe-goroutine seam and never blocks the
// status transition itself.
func (s *ReviewService) notifyContentStatus(content *model.ContentItem, status, reason string) {
	if s.notifSvc == nil || content == nil {
		return
	}
	s.notifSvc.NotifyContentStatus(content.AuthorID, content.ID, content.Title, status, reason, 0)
}

// ArchiveScanClean resumes a content pass that was recorded before its mod
// archive became clean. It is idempotent: if no pass is waiting, or content is
// already terminal, the notification is acknowledged without changing state.
// The archive worker retries this seam until it succeeds, so a clean scan
// cannot strand a previously approved content review.
func (s *ReviewService) ArchiveScanClean(ctx context.Context, attachmentID int64) error {
	if s == nil || s.db == nil {
		return errors.New("review service not initialized")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var attachment model.ContentAttachment
		if err := tx.Where("id = ?", attachmentID).First(&attachment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var content model.ContentItem
		if err := tx.Where("id = ?", attachment.ContentItemID).First(&content).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if content.Status == "published" || content.Status == "banned" {
			return nil
		}
		var latest model.AIReviewRecord
		if err := tx.Where("target_type = ? AND target_id = ?", "content", content.ID).
			Order("scanned_at DESC, id DESC").First(&latest).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if latest.Result != "pass" {
			return nil
		}
		return s.applyContentReviewResult(ctx, tx, content.ID, "pass")
	})
}

// emitContentEvent writes one outbox row inside the caller's transaction, so
// the terminal status transition and its event commit atomically. nil outbox
// (unwired service) is a no-op for backwards-compatible callers; the container
// always wires one.
func (s *ReviewService) emitContentEvent(ctx context.Context, tx *gorm.DB, topic string, content model.ContentItem, newStatus string) error {
	return EmitContentStatusEventTx(ctx, tx, s.outbox, topic, &content, newStatus)
}

func (s *ReviewService) processIPReviewResult(tx *gorm.DB, ipID int64, result string) error {
	var ip model.IP
	if err := tx.First(&ip, ipID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReviewTargetNotFound
		}
		return err
	}
	if result != "block" && result != "violation" {
		return nil
	}
	// Snapshot the not-yet-banned contents before the cascade so the author
	// notification can dedupe per author and mention the affected count.
	var affected []model.ContentItem
	if err := tx.Where("ip_id = ? AND status <> ?", ipID, "banned").Find(&affected).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.IP{}).Where("id = ?", ipID).Update("status", "banned").Error; err != nil {
		return err
	}
	if err := tx.Model(&model.ContentItem{}).Where("ip_id = ?", ipID).Update("status", "banned").Error; err != nil {
		return err
	}
	s.notifyIPCascadeDowned(&ip, affected)
	return nil
}

// notifyIPCascadeDowned notifies each affected author exactly once about the
// IP cascade ban (FIX-17): one notification per author, never per content, so
// a large IP cannot flood an author's notification feed.
func (s *ReviewService) notifyIPCascadeDowned(ip *model.IP, affected []model.ContentItem) {
	if s.notifSvc == nil || len(affected) == 0 {
		return
	}
	byAuthor := make(map[int64][]model.ContentItem)
	for _, content := range affected {
		byAuthor[content.AuthorID] = append(byAuthor[content.AuthorID], content)
	}
	for authorID, contents := range byAuthor {
		reason := fmt.Sprintf("所属 IP《%s》审核未通过，该 IP 下你的 %d 篇内容已随之下架", ip.Name, len(contents))
		representative := contents[0]
		s.notifSvc.NotifyContentStatus(authorID, representative.ID, representative.Title, "banned", reason, 0)
	}
}

func (s *ReviewService) applyRepeatViolationPenalty(ctx context.Context, tx *gorm.DB, authorID int64) error {
	windowDays := s.cfg.Reputation.RepeatViolationWindowDays
	if windowDays <= 0 {
		windowDays = 7
	}
	threshold := s.cfg.Reputation.RepeatViolationThreshold
	if threshold <= 0 {
		threshold = 2
	}
	extraPenalty := s.cfg.Reputation.RepeatViolationExtraPenalty
	if extraPenalty == 0 {
		extraPenalty = -1
	}

	windowStart := time.Now().AddDate(0, 0, -windowDays)
	var count int64
	err := tx.WithContext(ctx).
		Model(&model.AIReviewRecord{}).
		Joins("JOIN content_items ON ai_review_records.target_id = content_items.id").
		Distinct("ai_review_records.target_id").
		Where("ai_review_records.target_type = ?", "content").
		Where("content_items.author_id = ?", authorID).
		Where("ai_review_records.result IN ?", []string{"block", "violation"}).
		Where("ai_review_records.scanned_at >= ?", windowStart).
		Count(&count).Error
	if err != nil {
		return err
	}

	// Apply extra penalty when the threshold is first reached.
	if count < int64(threshold) {
		return nil
	}

	if s.reputSvc != nil {
		if err := addReputationTx(tx, authorID, extraPenalty, "repeat_violation", nil); err != nil {
			return err
		}
	}

	if s.rdb != nil {
		freezeKey := rediskeys.PublishFreezeKey(authorID)
		freezeTTL := 7 * 24 * time.Hour
		if s.cfg != nil && s.cfg.Cache.PublishFreezeTTL > 0 {
			freezeTTL = time.Duration(s.cfg.Cache.PublishFreezeTTL) * time.Second
		}
		if err := s.rdb.Set(ctx, freezeKey, "1", freezeTTL).Err(); err != nil {
			return err
		}
	}

	return nil
}

// EnsureContentJudgeCase creates the crowd-judge case for a content that
// entered manual review, idempotently (an existing open case is reused).
// Shared by the AI review path and the report auto-hide path (FIX-11).
func (s *ReviewService) EnsureContentJudgeCase(tx *gorm.DB, content model.ContentItem) error {
	return s.ensureJudgeCase(tx, content)
}

func (s *ReviewService) ensureJudgeCase(tx *gorm.DB, content model.ContentItem) error {
	var count int64
	if err := tx.Model(&model.JudgeCase{}).
		Where("target_id = ? AND status = ?", content.ID, "open").
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	minVotes := s.cfg.Judge.MinVotesRequired
	if minVotes <= 0 {
		minVotes = 20
	}

	judgeCase := model.JudgeCase{
		TargetType: content.ContentType,
		TargetID:   content.ID,
		Status:     "open",
		MinVotes:   minVotes,
	}
	return tx.Create(&judgeCase).Error
}

// recordAIReview persists one ai_review_records row inside tx. It returns
// false when the row was skipped because the same (provider,
// provider_task_id) already exists (a concurrent duplicate callback). Rows
// without a provider task id are stored as NULL so they never conflict in the
// unique index: synchronous submissions always record, and the initial
// submission record and the async result keep separate idempotency keys.
func (s *ReviewService) recordAIReview(ctx context.Context, tx *gorm.DB, targetType string, targetID int64, result string, raw map[string]interface{}, providerTaskID string) (bool, error) {
	rawBytes, _ := json.Marshal(raw)
	record := model.AIReviewRecord{
		TargetType:  strings.TrimSpace(targetType),
		TargetID:    targetID,
		Provider:    "aliyun",
		Result:      result,
		RawResponse: rawBytes,
	}
	if providerTaskID != "" {
		record.ProviderTaskID = &providerTaskID
	}
	stmt := tx.WithContext(ctx).Model(&model.AIReviewRecord{}).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "provider"}, {Name: "provider_task_id"}},
			DoNothing: true,
		}).
		Create(&record)
	if err := stmt.Error; err != nil {
		return false, err
	}
	return stmt.RowsAffected > 0, nil
}

// addReputationTx applies a reputation log entry and the score delta inside
// the caller's transaction, so the review record, the penalty and the status
// transition are atomic: a failed penalty rolls the record back too, and a
// duplicate callback can never charge the penalty twice.
func addReputationTx(tx *gorm.DB, userID int64, delta int, reason string, relatedID *int64) error {
	log := model.ReputationLog{
		UserID:    userID,
		Delta:     delta,
		Reason:    reason,
		RelatedID: relatedID,
	}
	if err := tx.Create(&log).Error; err != nil {
		return err
	}
	return tx.Model(&model.User{}).
		Where("id = ?", userID).
		UpdateColumn("reputation", gorm.Expr("reputation + ?", delta)).Error
}

func (s *ReviewService) resolveScanObjectURL(ossKey string) string {
	if s.cfg == nil {
		return ossKey
	}
	return aliyun.ObjectURL(s.cfg.OSS.Domain, ossKey)
}

// greenScanURLTTL bounds how long Aliyun has to fetch a signed scan URL after
// submission. Generous relative to the provider timeout so slow fetches do not
// hit an expired signature mid-scan.
const greenScanURLTTL = 15 * time.Minute

// resolveScanURL prepares a URL for Aliyun Green consumption. Moderation
// objects live in the private bucket: an unsigned platform URL gets OSS 403
// when Aliyun fetches it, so platform object URLs are re-signed as read URLs.
// External (non-platform) URLs are passed through unchanged — they are already
// publicly reachable, and resolveCoverScanURL gates cover inputs to platform
// objects before this layer.
func (s *ReviewService) resolveScanURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if s.oss == nil || s.cfg == nil || rawURL == "" {
		return rawURL
	}
	key, ok := aliyun.ObjectKeyFromURL(s.cfg.OSS.Domain, rawURL)
	if !ok {
		return rawURL
	}
	signed, err := s.oss.GetSignedURL(key, http.MethodGet, greenScanURLTTL)
	if err != nil {
		return rawURL
	}
	return signed
}

// resolveCoverScanURL verifies a cover URL is a platform OSS object before it
// is handed to Green. The cover is a client-visible URL (server-derived for
// media content, client-supplied for IPs and legacy text content), so it must
// match the configured delivery domain exactly; anything else fails closed
// with ErrCoverNotPlatformOSSObject instead of scanning an arbitrary URL.
func (s *ReviewService) resolveCoverScanURL(coverURL string) (string, error) {
	coverURL = strings.TrimSpace(coverURL)
	if s.cfg == nil || !aliyun.IsPlatformObjectURL(s.cfg.OSS.Domain, coverURL) {
		return "", ErrCoverNotPlatformOSSObject
	}
	return coverURL, nil
}

// MergeReviewResult keeps the strictest of two review results
// (pass < review < block). Inputs are normalized by NormalizeReviewResult
// before comparison, so callers may pass raw Aliyun values.
func MergeReviewResult(current, incoming string) string {
	priority := map[string]int{"pass": 1, "review": 2, "block": 3, "violation": 3}
	curr := NormalizeReviewResult(current)
	next := NormalizeReviewResult(incoming)
	if priority[next] > priority[curr] {
		return next
	}
	return curr
}

// NormalizeReviewResult maps a raw review result value onto the application
// review vocabulary ("pass", "review" or "block"). Unknown, empty or
// otherwise unexpected values fail closed to "pass".
func NormalizeReviewResult(result string) string {
	r := strings.ToLower(strings.TrimSpace(result))
	switch r {
	case "block", "violation":
		return "block"
	case "review", "pending":
		return "review"
	default:
		return "pass"
	}
}

// SetNotificationService attaches the notification service used to inform
// authors about content status transitions decided by the AI review channel
// and IP cascade bans.
func (s *ReviewService) SetNotificationService(ns *NotificationService) {
	s.notifSvc = ns
}
