package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/captcha"
	"omnicraft/backend/internal/repository"
	"os"
	"path/filepath"
	"time"

	"github.com/redis/go-redis/v9"
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

type FeedbackService struct {
	repo            *repository.FeedbackRepository
	userRepo        *repository.UserRepository
	rdb             *redis.Client
	captchaVerifier captcha.CaptchaVerifier
	uploadGrantTTL  int
	notificationSvc *NotificationService
	mailSender      FeedbackMailSender
	stagingDir      string
}

type FeedbackMailSender interface {
	SendFeedbackUpdate(ctx context.Context, to, subject, body string) error
}

func NewFeedbackService(
	repo *repository.FeedbackRepository,
	userRepo *repository.UserRepository,
	rdb *redis.Client,
	captchaVerifier captcha.CaptchaVerifier,
	uploadGrantTTL int,
) *FeedbackService {
	return &FeedbackService{
		repo:            repo,
		userRepo:        userRepo,
		rdb:             rdb,
		captchaVerifier: captchaVerifier,
		uploadGrantTTL:  uploadGrantTTL,
		stagingDir:      filepath.Join("data", "feedback-staging"),
	}
}

func (s *FeedbackService) SetNotificationService(notificationSvc *NotificationService) {
	s.notificationSvc = notificationSvc
}

func (s *FeedbackService) SetFeedbackMailSender(mailSender FeedbackMailSender) {
	s.mailSender = mailSender
}

func (s *FeedbackService) SetAttachmentStagingDir(stagingDir string) {
	if stagingDir != "" {
		s.stagingDir = stagingDir
	}
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
	Attachments       []FeedbackAttachmentGrantInput
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
		return nil, errors.New("INVALID_CATEGORY")
	}
	if input.Title == "" || input.Description == "" {
		return nil, errors.New("TITLE_AND_DESCRIPTION_REQUIRED")
	}
	if len(input.Title) > 160 {
		return nil, errors.New("TITLE_TOO_LONG")
	}

	if input.UserID == nil {
		if input.ContactEmail == "" {
			return nil, errors.New("CONTACT_EMAIL_REQUIRED_FOR_ANONYMOUS")
		}
		if input.CaptchaToken == "" {
			return nil, errors.New("CAPTCHA_REQUIRED_FOR_ANONYMOUS")
		}
		if s.captchaVerifier != nil {
			if err := s.captchaVerifier.Verify(ctx, input.CaptchaToken, ""); err != nil {
				return nil, errors.New("CAPTCHA_VERIFICATION_FAILED")
			}
		}
	}

	filteredDiag := filterDiagnostics(input.DiagnosticSummary)

	if len(input.Attachments) == 0 && len(input.AttachmentOSSKeys) > 0 {
		for _, ossKey := range input.AttachmentOSSKeys {
			input.Attachments = append(input.Attachments, FeedbackAttachmentGrantInput{OSSKey: ossKey})
		}
	}

	consumedAttachments := make([]feedbackUploadGrant, 0, len(input.Attachments))
	for _, attachment := range input.Attachments {
		grant, err := s.consumeAttachmentGrant(ctx, attachment)
		if err != nil {
			return nil, err
		}
		consumedAttachments = append(consumedAttachments, grant)
	}

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

	if err := s.repo.CreateTicket(ticket); err != nil {
		return nil, err
	}

	for _, grant := range consumedAttachments {
		att := &model.FeedbackAttachment{
			TicketID:  ticket.ID,
			OSSKey:    grant.OSSKey,
			FileType:  "screenshot",
			MimeType:  grant.MimeType,
			SizeBytes: grant.SizeBytes,
		}
		if err := s.repo.CreateAttachment(att); err != nil {
			return nil, err
		}
	}

	return ticket, nil
}

type PresignUploadInput struct {
	UserID       *int64
	FileName     string
	MimeType     string
	SizeBytes    int64
	CaptchaToken string
}

func (s *FeedbackService) PresignUpload(ctx context.Context, input PresignUploadInput) (string, string, error) {
	if input.UserID == nil && input.CaptchaToken == "" {
		return "", "", errors.New("CAPTCHA_REQUIRED_FOR_ANONYMOUS")
	}
	if input.UserID == nil && s.captchaVerifier != nil {
		if err := s.captchaVerifier.Verify(ctx, input.CaptchaToken, ""); err != nil {
			return "", "", errors.New("CAPTCHA_VERIFICATION_FAILED")
		}
	}

	validImageMime := map[string]bool{
		"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true,
	}
	if !validImageMime[input.MimeType] {
		return "", "", errors.New("INVALID_MIME_TYPE")
	}
	if input.SizeBytes > 20*1024*1024 {
		return "", "", errors.New("FILE_TOO_LARGE")
	}

	grantID := generateFeedbackGrantID()
	ossKey := fmt.Sprintf("feedback-staging/%s/%s", grantID, input.FileName)

	grantKey := feedbackUploadGrantKey(grantID)
	if s.rdb != nil {
		grant := feedbackUploadGrant{
			OSSKey:    ossKey,
			MimeType:  input.MimeType,
			SizeBytes: input.SizeBytes,
		}
		grantJSON, err := json.Marshal(grant)
		if err != nil {
			return "", "", err
		}
		if err := s.rdb.Set(ctx, grantKey, grantJSON, time.Duration(s.uploadGrantTTL)*time.Second).Err(); err != nil {
			return "", "", err
		}
	}

	return grantID, ossKey, nil
}

func (s *FeedbackService) HasAttachmentGrant(ctx context.Context, grantID string) bool {
	_, err := s.loadAttachmentGrant(ctx, grantID)
	return err == nil
}

func (s *FeedbackService) StageAttachmentUpload(ctx context.Context, grantID string, body io.Reader) error {
	grant, err := s.loadAttachmentGrant(ctx, grantID)
	if err != nil {
		return err
	}
	if body == nil {
		return errors.New("INVALID_ATTACHMENT_GRANT")
	}
	if err := os.MkdirAll(s.stagingDir, 0o755); err != nil {
		return err
	}

	stagingPath := filepath.Join(s.stagingDir, grantID+".bin")
	file, err := os.Create(stagingPath)
	if err != nil {
		return err
	}
	defer file.Close()

	written, err := io.Copy(file, io.LimitReader(body, grant.SizeBytes+1))
	if err != nil {
		return err
	}
	if grant.SizeBytes >= 0 && written > grant.SizeBytes {
		_ = file.Close()
		_ = os.Remove(stagingPath)
		return errors.New("FILE_TOO_LARGE")
	}
	return nil
}

func (s *FeedbackService) consumeAttachmentGrant(ctx context.Context, input FeedbackAttachmentGrantInput) (feedbackUploadGrant, error) {
	if input.GrantID == "" || input.OSSKey == "" || s.rdb == nil {
		return feedbackUploadGrant{}, errors.New("INVALID_ATTACHMENT_GRANT")
	}

	grantKey := feedbackUploadGrantKey(input.GrantID)
	raw, err := s.rdb.GetDel(ctx, grantKey).Result()
	if err != nil {
		return feedbackUploadGrant{}, errors.New("INVALID_ATTACHMENT_GRANT")
	}

	var grant feedbackUploadGrant
	if err := json.Unmarshal([]byte(raw), &grant); err != nil {
		grant = feedbackUploadGrant{OSSKey: raw}
	}
	if grant.OSSKey != input.OSSKey {
		return feedbackUploadGrant{}, errors.New("INVALID_ATTACHMENT_GRANT")
	}
	if grant.MimeType == "" {
		grant.MimeType = "application/octet-stream"
	}
	return grant, nil
}

func (s *FeedbackService) loadAttachmentGrant(ctx context.Context, grantID string) (feedbackUploadGrant, error) {
	if grantID == "" || s.rdb == nil {
		return feedbackUploadGrant{}, errors.New("INVALID_ATTACHMENT_GRANT")
	}

	raw, err := s.rdb.Get(ctx, feedbackUploadGrantKey(grantID)).Result()
	if err != nil {
		return feedbackUploadGrant{}, errors.New("INVALID_ATTACHMENT_GRANT")
	}

	var grant feedbackUploadGrant
	if err := json.Unmarshal([]byte(raw), &grant); err != nil {
		grant = feedbackUploadGrant{OSSKey: raw}
	}
	if grant.OSSKey == "" {
		return feedbackUploadGrant{}, errors.New("INVALID_ATTACHMENT_GRANT")
	}
	return grant, nil
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
		return nil, errors.New("TICKET_NOT_FOUND")
	}
	if ticket.UserID == nil || *ticket.UserID != userID {
		return nil, errors.New("FORBIDDEN")
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
		return nil, errors.New("TICKET_NOT_FOUND")
	}
	return ticket, nil
}

type AdminPatchFeedbackInput struct {
	Status          string
	Priority        string
	AssigneeAdminID *int64
}

func (s *FeedbackService) PatchTicket(ctx context.Context, ticketID int64, input AdminPatchFeedbackInput) (*model.FeedbackTicket, error) {
	ticket, err := s.repo.FindTicketByIDForAdmin(ticketID)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, errors.New("TICKET_NOT_FOUND")
	}

	validStatuses := map[string]bool{"open": true, "in_progress": true, "closed": true, "reopened": true}
	if input.Status != "" {
		if !validStatuses[input.Status] {
			return nil, errors.New("INVALID_STATUS")
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
			return nil, errors.New("INVALID_PRIORITY")
		}
		ticket.Priority = input.Priority
	}

	if input.AssigneeAdminID != nil {
		ticket.AssigneeAdminID = input.AssigneeAdminID
	}

	if err := s.repo.UpdateTicket(ticket); err != nil {
		return nil, err
	}
	if input.Status == "closed" || input.Status == "reopened" {
		if err := s.notifyFeedbackUpdate(ctx, ticket, 0, "feedback_status", "Feedback status updated", fmt.Sprintf("Feedback ticket status changed to %s.", ticket.Status)); err != nil {
			return nil, err
		}
	}
	return ticket, nil
}

type AdminReplyInput struct {
	TicketID       int64
	AuthorAdminID  int64
	Body           string
	IsInternalNote bool
}

func (s *FeedbackService) AdminReply(ctx context.Context, input AdminReplyInput) (*model.FeedbackReply, error) {
	if input.Body == "" {
		return nil, errors.New("BODY_REQUIRED")
	}
	if len(input.Body) > 5000 {
		return nil, errors.New("BODY_TOO_LONG")
	}

	ticket, err := s.repo.FindTicketByIDForAdmin(input.TicketID)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, errors.New("TICKET_NOT_FOUND")
	}

	reply := &model.FeedbackReply{
		TicketID:       input.TicketID,
		AuthorAdminID:  &input.AuthorAdminID,
		Body:           input.Body,
		IsInternalNote: input.IsInternalNote,
	}

	if err := s.repo.CreateReply(reply); err != nil {
		return nil, err
	}
	if !input.IsInternalNote {
		if err := s.notifyFeedbackUpdate(ctx, ticket, input.AuthorAdminID, "feedback_reply", "Feedback reply received", input.Body); err != nil {
			return nil, err
		}
	}
	return reply, nil
}

func (s *FeedbackService) notifyFeedbackUpdate(ctx context.Context, ticket *model.FeedbackTicket, senderID int64, notifType, subject, body string) error {
	if ticket == nil {
		return nil
	}
	if ticket.UserID != nil && *ticket.UserID > 0 {
		if s.notificationSvc != nil {
			s.notificationSvc.Notify(*ticket.UserID, "system", notifType, subject, body, "feedback_ticket", ticket.ID, senderID)
		}
		return nil
	}
	if ticket.ContactEmail == "" || s.mailSender == nil {
		return nil
	}
	if err := s.mailSender.SendFeedbackUpdate(ctx, ticket.ContactEmail, subject, body); err != nil {
		return errors.New("FEEDBACK_NOTIFICATION_FAILED")
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

func generateFeedbackGrantID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func feedbackUploadGrantKey(grantID string) string {
	return fmt.Sprintf("feedback:upload_grant:%s", grantID)
}
