package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/captcha"
	"omnicraft/backend/internal/repository"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Feedback-specific sentinel errors for handler comparison via errors.Is().
var (
	ErrFeedbackInvalidCategory                 = errors.New("INVALID_CATEGORY")
	ErrFeedbackTitleAndDescriptionReq          = errors.New("TITLE_AND_DESCRIPTION_REQUIRED")
	ErrFeedbackTitleTooLong                    = errors.New("TITLE_TOO_LONG")
	ErrFeedbackContactEmailRequired            = errors.New("CONTACT_EMAIL_REQUIRED_FOR_ANONYMOUS")
	ErrFeedbackCaptchaRequired                 = errors.New("CAPTCHA_REQUIRED_FOR_ANONYMOUS")
	ErrFeedbackCaptchaFailed                   = errors.New("CAPTCHA_VERIFICATION_FAILED")
	ErrFeedbackInvalidMimeType                 = errors.New("INVALID_MIME_TYPE")
	ErrFeedbackFileTooLarge                    = errors.New("FILE_TOO_LARGE")
	ErrFeedbackTicketNotFound                  = errors.New("TICKET_NOT_FOUND")
	ErrFeedbackForbidden                       = errors.New("FORBIDDEN")
	ErrFeedbackInvalidStatus                   = errors.New("INVALID_STATUS")
	ErrFeedbackInvalidPriority                 = errors.New("INVALID_PRIORITY")
	ErrFeedbackBodyRequired                    = errors.New("BODY_REQUIRED")
	ErrFeedbackBodyTooLong                     = errors.New("BODY_TOO_LONG")
	ErrFeedbackDeliveryFailed                  = errors.New("FEEDBACK_DELIVERY_FAILED")
	ErrFeedbackUploadGrantInvalid              = errors.New("UPLOAD_GRANT_INVALID")
	ErrFeedbackAttachmentBlocked               = errors.New("ATTACHMENT_BLOCKED")
	ErrFeedbackAttachmentModerationUnavailable = errors.New("ATTACHMENT_MODERATION_UNAVAILABLE")
)

var allowedDiagnosticKeys = map[string]struct{}{
	"app_version": {},
	"platform":    {},
	"route":       {},
	"error_code":  {},
}

var validCategories = map[string]bool{
	"web_bug": true, "desktop_deploy": true, "content_or_community": true,
	"account_or_security": true, "agent_quality": true, "feature_request": true, "other": true,
}

// ImageReviewer is the minimal moderation dependency FeedbackService needs to
// gate screenshot attachments before they are persisted. *ReviewService is
// the production implementation; tests inject a fake.
type ImageReviewer interface {
	ReviewImageURL(ctx context.Context, imageURL string) (string, error)
}

type FeedbackService struct {
	repo            *repository.FeedbackRepository
	userRepo        *repository.UserRepository
	rdb             *redis.Client
	captchaVerifier captcha.CaptchaVerifier
	uploadGrantTTL  int
	ossSigner       feedbackOSSSigner
	notificationSvc *NotificationService
	mailSender      FeedbackMailSender
	cfg             *config.Config
	reviewSvc       ImageReviewer
}

type FeedbackMailSender interface {
	SendFeedbackUpdate(ctx context.Context, to, subject, body string) error
}

type feedbackOSSSigner interface {
	GeneratePresignUploadURL(ctx context.Context, req PresignUploadRequest, userID int64) (*PresignUploadResponse, error)
	GenerateFeedbackPresignUploadURL(ctx context.Context, req PresignUploadRequest, userID int64) (*PresignUploadResponse, error)
}

func NewFeedbackService(
	repo *repository.FeedbackRepository,
	userRepo *repository.UserRepository,
	rdb *redis.Client,
	captchaVerifier captcha.CaptchaVerifier,
	uploadGrantTTL int,
	ossSigner ...feedbackOSSSigner,
) *FeedbackService {
	var signer feedbackOSSSigner
	if len(ossSigner) > 0 {
		signer = ossSigner[0]
	}
	return &FeedbackService{
		repo:            repo,
		userRepo:        userRepo,
		rdb:             rdb,
		captchaVerifier: captchaVerifier,
		uploadGrantTTL:  uploadGrantTTL,
		ossSigner:       signer,
	}
}

func (s *FeedbackService) SetNotificationService(notificationSvc *NotificationService) {
	s.notificationSvc = notificationSvc
}

func (s *FeedbackService) SetFeedbackMailSender(mailSender FeedbackMailSender) {
	s.mailSender = mailSender
}

// SetReviewService wires the image review gate used to scan screenshot
// attachments before they are persisted.
func (s *FeedbackService) SetReviewService(reviewSvc ImageReviewer) {
	s.reviewSvc = reviewSvc
}

// SetConfig wires the environment configuration that resolves the A4
// availability policy for attachment image moderation.
func (s *FeedbackService) SetConfig(cfg *config.Config) {
	s.cfg = cfg
}

type SubmitTicketInput struct {
	UserID            *int64
	ContactEmail      string
	Category          string
	Title             string
	Description       string
	DiagnosticSummary map[string]interface{}
	CaptchaToken      string
	AttachmentOSSKeys []string
	AttachmentGrants  []FeedbackAttachmentGrantInput
}

type FeedbackAttachmentGrantInput struct {
	GrantID string `json:"grant_id"`
	OSSKey  string `json:"oss_key"`
}

type feedbackUploadGrant struct {
	OSSKey    string `json:"oss_key"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
}

func (s *FeedbackService) SubmitTicket(ctx context.Context, input SubmitTicketInput) (*model.FeedbackTicket, error) {
	if !validCategories[input.Category] {
		return nil, ErrFeedbackInvalidCategory
	}
	if input.Title == "" || input.Description == "" {
		return nil, ErrFeedbackTitleAndDescriptionReq
	}
	if len(input.Title) > 160 {
		return nil, ErrFeedbackTitleTooLong
	}

	if input.UserID == nil {
		if input.ContactEmail == "" {
			return nil, ErrFeedbackContactEmailRequired
		}
		if input.CaptchaToken == "" {
			return nil, ErrFeedbackCaptchaRequired
		}
		if s.captchaVerifier != nil {
			if err := s.captchaVerifier.Verify(ctx, input.CaptchaToken, ""); err != nil {
				return nil, ErrFeedbackCaptchaFailed
			}
		}
	}

	filteredDiag := filterDiagnostics(input.DiagnosticSummary)

	ticket := &model.FeedbackTicket{
		UserID:            input.UserID,
		ContactEmail:      input.ContactEmail,
		Category:          input.Category,
		Title:             input.Title,
		Description:       input.Description,
		DiagnosticSummary: filteredDiag,
		Status:            "open",
		Priority:          "normal",
	}

	attachments, consumedGrants, err := s.consumeAttachmentGrants(ctx, input)
	if err != nil {
		return nil, err
	}

	if err := s.moderateAttachments(ctx, "feedback_attachment", attachments); err != nil {
		s.restoreFeedbackUploadGrants(ctx, consumedGrants)
		return nil, err
	}

	if err := s.repo.CreateTicketWithAttachments(ticket, attachments); err != nil {
		s.restoreFeedbackUploadGrants(ctx, consumedGrants)
		return nil, err
	}

	return ticket, nil
}

// moderateAttachments runs the synchronous image review gate over feedback
// screenshot attachments before they are persisted. Tickets without
// attachments skip the gate entirely, so plain text feedback is never blocked
// by attachment review failure. A "block" result rejects the submission.
// Availability follows the A4 environment semantics: in release mode any
// review failure is fail-closed, while in local/test mode an unconfigured
// Green client is fail-open and must be recorded via structured logs.
func (s *FeedbackService) moderateAttachments(ctx context.Context, action string, attachments []model.FeedbackAttachment) error {
	if len(attachments) == 0 {
		return nil
	}

	if s.reviewSvc == nil {
		if s.isReleaseMode() {
			slog.Error("attachment image moderation unavailable, rejecting submission",
				"action", action, "env_mode", s.environmentMode(), "policy", "fail_closed", "reason", "review_service_not_wired")
			return ErrFeedbackAttachmentModerationUnavailable
		}
		slog.Warn("attachment image moderation skipped, submission allowed",
			"action", action, "env_mode", s.environmentMode(), "policy", "fail_open", "reason", "review_service_not_wired")
		return nil
	}

	for _, att := range attachments {
		result, err := s.reviewSvc.ReviewImageURL(ctx, s.resolveAttachmentScanURL(att.OSSKey))
		if err != nil {
			if !s.isReleaseMode() && errors.Is(err, aliyun.ErrGreenNotConfigured) {
				slog.Warn("attachment image moderation skipped, submission allowed",
					"action", action, "env_mode", s.environmentMode(), "policy", "fail_open", "reason", "green_not_configured")
				return nil
			}
			slog.Error("attachment image moderation unavailable, rejecting submission",
				"action", action, "env_mode", s.environmentMode(), "policy", "fail_closed", "reason", err.Error())
			return ErrFeedbackAttachmentModerationUnavailable
		}
		if result == "block" {
			return ErrFeedbackAttachmentBlocked
		}
	}
	return nil
}

func (s *FeedbackService) isReleaseMode() bool {
	return s.cfg != nil && s.cfg.Server.Mode == "release"
}

func (s *FeedbackService) environmentMode() string {
	if s.cfg == nil {
		return "unknown"
	}
	return s.cfg.Server.Mode
}

// resolveAttachmentScanURL maps a platform OSS object key to its delivery URL,
// mirroring ReviewService.resolveScanObjectURL.
func (s *FeedbackService) resolveAttachmentScanURL(ossKey string) string {
	if s.cfg != nil && strings.TrimSpace(s.cfg.OSS.Domain) != "" {
		return strings.TrimRight(strings.TrimSpace(s.cfg.OSS.Domain), "/") + "/" + strings.TrimLeft(ossKey, "/")
	}
	return ossKey
}

type PresignUploadInput struct {
	UserID       *int64
	FileName     string
	MimeType     string
	SizeBytes    int64
	CaptchaToken string
}

type PresignFeedbackUploadGrant struct {
	GrantID   string
	OSSKey    string
	UploadURL string
	ExpiresIn int64
}

func (s *FeedbackService) PresignUpload(ctx context.Context, input PresignUploadInput) (*PresignFeedbackUploadGrant, error) {
	if input.UserID == nil && input.CaptchaToken == "" {
		return nil, ErrFeedbackCaptchaRequired
	}
	if input.UserID == nil && s.captchaVerifier != nil {
		if err := s.captchaVerifier.Verify(ctx, input.CaptchaToken, ""); err != nil {
			return nil, ErrFeedbackCaptchaFailed
		}
	}

	validImageMime := map[string]bool{
		"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true,
	}
	if !validImageMime[input.MimeType] {
		return nil, ErrFeedbackInvalidMimeType
	}
	if input.SizeBytes > 20*1024*1024 {
		return nil, ErrFeedbackFileTooLarge
	}
	if s.ossSigner == nil {
		return nil, ErrOSSNotConfigured
	}
	if s.rdb == nil {
		return nil, ErrUploadGrantUnavailable
	}

	userIDForPath := int64(0)
	if input.UserID != nil {
		userIDForPath = *input.UserID
	}
	presign, err := s.ossSigner.GenerateFeedbackPresignUploadURL(ctx, PresignUploadRequest{
		FileName: input.FileName,
		FileType: "image",
		MimeType: input.MimeType,
		FileSize: input.SizeBytes,
	}, userIDForPath)
	if err != nil {
		return nil, err
	}

	grantID, err := generateFeedbackGrantID()
	if err != nil {
		return nil, err
	}
	grant := feedbackUploadGrant{
		OSSKey:    presign.OSSKey,
		MimeType:  input.MimeType,
		SizeBytes: input.SizeBytes,
	}
	payload, err := json.Marshal(grant)
	if err != nil {
		return nil, err
	}
	if err := s.rdb.Set(ctx, feedbackUploadGrantKey(grantID), payload, time.Duration(s.uploadGrantTTL)*time.Second).Err(); err != nil {
		return nil, err
	}
	return &PresignFeedbackUploadGrant{
		GrantID:   grantID,
		OSSKey:    presign.OSSKey,
		UploadURL: presign.UploadURL,
		ExpiresIn: presign.ExpiresIn,
	}, nil
}

func (s *FeedbackService) ListUserTickets(ctx context.Context, userID int64, page, pageSize int) ([]model.FeedbackTicket, int64, error) {
	return s.repo.ListByUser(userID, page, pageSize)
}

func (s *FeedbackService) GetTicketForUser(ctx context.Context, ticketID, userID int64) (*model.FeedbackTicket, error) {
	ticket, err := s.repo.FindTicketByID(ticketID)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, ErrFeedbackTicketNotFound
	}
	if ticket.UserID == nil || *ticket.UserID != userID {
		return nil, ErrFeedbackForbidden
	}
	return ticket, nil
}

func (s *FeedbackService) ListAdminFeedback(ctx context.Context, filter repository.AdminFeedbackFilter) ([]model.FeedbackTicket, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	return s.repo.ListAdminFeedback(filter)
}

func (s *FeedbackService) GetTicketForAdmin(ctx context.Context, ticketID int64) (*model.FeedbackTicket, error) {
	ticket, err := s.repo.FindTicketByIDForAdmin(ticketID)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, ErrFeedbackTicketNotFound
	}
	return ticket, nil
}

type AdminPatchFeedbackInput struct {
	Status          string
	Priority        string
	AssigneeAdminID *int64
}

func (s *FeedbackService) PatchTicket(ctx context.Context, ticketID int64, input AdminPatchFeedbackInput) (*model.FeedbackTicket, error) {
	ticket, err := s.patchTicket(ctx, ticketID, input)
	if err != nil {
		return nil, err
	}
	if err := s.NotifyPatchTicket(ctx, ticket, input); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *FeedbackService) PatchTicketTx(ctx context.Context, tx *gorm.DB, ticketID int64, input AdminPatchFeedbackInput) (*model.FeedbackTicket, error) {
	txSvc := *s
	txSvc.repo = s.repo.WithTx(tx)
	return txSvc.patchTicket(ctx, ticketID, input)
}

func (s *FeedbackService) patchTicket(ctx context.Context, ticketID int64, input AdminPatchFeedbackInput) (*model.FeedbackTicket, error) {
	ticket, err := s.repo.FindTicketByIDForAdmin(ticketID)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, ErrFeedbackTicketNotFound
	}

	validStatuses := map[string]bool{"open": true, "in_progress": true, "resolved": true, "closed": true, "reopened": true}
	if input.Status != "" {
		if !validStatuses[input.Status] {
			return nil, ErrFeedbackInvalidStatus
		}
		ticket.Status = input.Status
		if input.Status == "closed" && ticket.ResolvedAt == nil {
			now := time.Now()
			ticket.ResolvedAt = &now
		}
		if input.Status == "reopened" {
			ticket.ResolvedAt = nil
		}
	}

	validPriorities := map[string]bool{"low": true, "normal": true, "high": true, "urgent": true}
	if input.Priority != "" {
		if !validPriorities[input.Priority] {
			return nil, ErrFeedbackInvalidPriority
		}
		ticket.Priority = input.Priority
	}

	if input.AssigneeAdminID != nil {
		ticket.AssigneeAdminID = input.AssigneeAdminID
	}

	if err := s.repo.UpdateTicket(ticket); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *FeedbackService) NotifyPatchTicket(ctx context.Context, ticket *model.FeedbackTicket, input AdminPatchFeedbackInput) error {
	if input.Status == "closed" || input.Status == "reopened" {
		if err := s.deliverAdminFeedbackUpdate(ctx, ticket, 0, feedbackActionStatus(input.Status)); err != nil {
			return err
		}
	}
	return nil
}

type AdminReplyInput struct {
	TicketID       int64
	AuthorAdminID  int64
	Body           string
	IsInternalNote bool
}

func (s *FeedbackService) AdminReply(ctx context.Context, input AdminReplyInput) (*model.FeedbackReply, error) {
	reply, ticket, err := s.adminReply(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.NotifyAdminReply(ctx, ticket, input); err != nil {
		return nil, err
	}
	return reply, nil
}

func (s *FeedbackService) AdminReplyTx(ctx context.Context, tx *gorm.DB, input AdminReplyInput) (*model.FeedbackReply, *model.FeedbackTicket, error) {
	txSvc := *s
	txSvc.repo = s.repo.WithTx(tx)
	return txSvc.adminReply(ctx, input)
}

func (s *FeedbackService) adminReply(ctx context.Context, input AdminReplyInput) (*model.FeedbackReply, *model.FeedbackTicket, error) {
	if input.Body == "" {
		return nil, nil, ErrFeedbackBodyRequired
	}
	if len(input.Body) > 5000 {
		return nil, nil, ErrFeedbackBodyTooLong
	}

	ticket, err := s.repo.FindTicketByIDForAdmin(input.TicketID)
	if err != nil {
		return nil, nil, err
	}
	if ticket == nil {
		return nil, nil, ErrFeedbackTicketNotFound
	}

	reply := &model.FeedbackReply{
		TicketID:       input.TicketID,
		AuthorAdminID:  &input.AuthorAdminID,
		Body:           input.Body,
		IsInternalNote: input.IsInternalNote,
	}

	if err := s.repo.CreateReply(reply); err != nil {
		return nil, nil, err
	}
	return reply, ticket, nil
}

func (s *FeedbackService) NotifyAdminReply(ctx context.Context, ticket *model.FeedbackTicket, input AdminReplyInput) error {
	if !input.IsInternalNote {
		if err := s.deliverAdminFeedbackUpdate(ctx, ticket, input.AuthorAdminID, "reply"); err != nil {
			return err
		}
	}
	return nil
}

func (s *FeedbackService) deliverAdminFeedbackUpdate(ctx context.Context, ticket *model.FeedbackTicket, senderID int64, action string) error {
	if ticket == nil {
		return nil
	}
	if ticket.UserID != nil && *ticket.UserID > 0 {
		if s.notificationSvc != nil {
			title, body := feedbackNotificationText(action)
			s.notificationSvc.Notify(*ticket.UserID, "system", "system", title, body, "feedback_ticket", ticket.ID, senderID)
		}
		return nil
	}
	if ticket.ContactEmail == "" || s.mailSender == nil {
		return nil
	}
	subject, body := feedbackEmailText(action, ticket)
	if err := s.mailSender.SendFeedbackUpdate(ctx, ticket.ContactEmail, subject, body); err != nil {
		return ErrFeedbackDeliveryFailed
	}
	return nil
}

func (s *FeedbackService) CountOpenTickets(ctx context.Context) (int64, error) {
	return s.repo.CountByStatus("open")
}

func filterDiagnostics(raw map[string]interface{}) model.JSONMap {
	if raw == nil {
		return model.JSONMap{}
	}
	filtered := make(model.JSONMap)
	for k, v := range raw {
		if _, ok := allowedDiagnosticKeys[k]; ok {
			filtered[k] = v
		}
	}
	return filtered
}

func generateFeedbackGrantID() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(uploadGrantEntropyReader, b); err != nil {
		return "", fmt.Errorf("%w: entropy unavailable", ErrUploadGrantUnavailable)
	}
	return hex.EncodeToString(b), nil
}

func feedbackUploadGrantKey(grantID string) string {
	return fmt.Sprintf("feedback:upload_grant:%s", grantID)
}

type consumedFeedbackUploadGrant struct {
	GrantID string
	Grant   feedbackUploadGrant
}

func (s *FeedbackService) consumeAttachmentGrants(ctx context.Context, input SubmitTicketInput) ([]model.FeedbackAttachment, []consumedFeedbackUploadGrant, error) {
	if len(input.AttachmentOSSKeys) > 0 && len(input.AttachmentGrants) == 0 {
		return nil, nil, ErrFeedbackUploadGrantInvalid
	}
	if len(input.AttachmentGrants) == 0 {
		return nil, nil, nil
	}
	if s.rdb == nil {
		return nil, nil, ErrUploadGrantUnavailable
	}

	attachments := make([]model.FeedbackAttachment, 0, len(input.AttachmentGrants))
	consumed := make([]consumedFeedbackUploadGrant, 0, len(input.AttachmentGrants))
	for _, grantInput := range input.AttachmentGrants {
		grant, err := s.consumeUploadGrant(ctx, grantInput.GrantID, grantInput.OSSKey)
		if err != nil {
			s.restoreFeedbackUploadGrants(ctx, consumed)
			return nil, nil, err
		}
		consumed = append(consumed, consumedFeedbackUploadGrant{GrantID: grantInput.GrantID, Grant: *grant})
		attachments = append(attachments, model.FeedbackAttachment{
			OSSKey:    grant.OSSKey,
			FileType:  "screenshot",
			MimeType:  grant.MimeType,
			SizeBytes: grant.SizeBytes,
		})
	}
	return attachments, consumed, nil
}

func (s *FeedbackService) consumeUploadGrant(ctx context.Context, grantID, ossKey string) (*feedbackUploadGrant, error) {
	if grantID == "" || ossKey == "" {
		return nil, ErrFeedbackUploadGrantInvalid
	}
	script := redis.NewScript(`
local val = redis.call("GET", KEYS[1])
if not val then
  return nil
end
local decoded = cjson.decode(val)
if decoded["oss_key"] ~= ARGV[1] then
  return false
end
redis.call("DEL", KEYS[1])
return val
`)
	result, err := script.Run(ctx, s.rdb, []string{feedbackUploadGrantKey(grantID)}, ossKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrFeedbackUploadGrantInvalid
		}
		return nil, err
	}
	raw, ok := result.(string)
	if !ok || raw == "" {
		return nil, ErrFeedbackUploadGrantInvalid
	}

	var grant feedbackUploadGrant
	if err := json.Unmarshal([]byte(raw), &grant); err != nil {
		return nil, ErrFeedbackUploadGrantInvalid
	}
	if grant.MimeType == "" {
		grant.MimeType = "application/octet-stream"
	}
	return &grant, nil
}

func (s *FeedbackService) restoreFeedbackUploadGrants(ctx context.Context, grants []consumedFeedbackUploadGrant) {
	if s.rdb == nil {
		return
	}
	ttl := time.Duration(s.uploadGrantTTL) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	for _, consumed := range grants {
		payload, err := json.Marshal(consumed.Grant)
		if err != nil {
			continue
		}
		if err := s.rdb.SetNX(ctx, feedbackUploadGrantKey(consumed.GrantID), payload, ttl).Err(); err != nil {
			slog.Error("failed to restore feedback upload grant after ticket failure", "grant_id", consumed.GrantID, "error", err)
		}
	}
}

func feedbackActionStatus(status string) string {
	if status == "closed" {
		return "closed"
	}
	if status == "reopened" {
		return "reopened"
	}
	return "updated"
}

func feedbackNotificationText(action string) (string, string) {
	switch action {
	case "reply":
		return "Feedback reply received", "An admin replied to your feedback ticket."
	case "closed":
		return "Feedback ticket closed", "Your feedback ticket was closed by an admin."
	case "reopened":
		return "Feedback ticket reopened", "Your feedback ticket was reopened by an admin."
	default:
		return "Feedback ticket updated", "Your feedback ticket was updated by an admin."
	}
}

func feedbackEmailText(action string, ticket *model.FeedbackTicket) (string, string) {
	title, body := feedbackNotificationText(action)
	return title, fmt.Sprintf("%s\n\nTicket: %s", body, ticket.Title)
}
