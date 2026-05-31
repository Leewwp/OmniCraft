package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/captcha"
	"omnicraft/backend/internal/repository"
	"time"

	"github.com/redis/go-redis/v9"
)

var allowedDiagnosticKeys = map[string]struct{}{
	"app_version": {},
	"platform":   {},
	"route":      {},
	"error_code": {},
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
	}
}

type SubmitTicketInput struct {
	UserID             *int64
	ContactEmail       string
	Category           string
	Title              string
	Description        string
	DiagnosticSummary  map[string]interface{}
	CaptchaToken       string
	AttachmentOSSKeys  []string
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

	ticket := &model.FeedbackTicket{
		UserID:             input.UserID,
		ContactEmail:       input.ContactEmail,
		Category:           input.Category,
		Title:              input.Title,
		Description:        input.Description,
		DiagnosticSummary:  filteredDiag,
		Status:             "open",
		Priority:           "normal",
	}

	if err := s.repo.CreateTicket(ticket); err != nil {
		return nil, err
	}

	for _, ossKey := range input.AttachmentOSSKeys {
		att, err := s.repo.FindAttachmentByOSSKey(ossKey)
		if err != nil || att == nil {
			continue
		}
		att.TicketID = ticket.ID
		if err := s.repo.CreateAttachment(att); err != nil {
			continue
		}
	}

	return ticket, nil
}

type PresignUploadInput struct {
	UserID      *int64
	FileName    string
	MimeType    string
	SizeBytes   int64
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

	grantKey := fmt.Sprintf("feedback:upload_grant:%s", grantID)
	if s.rdb != nil {
		if err := s.rdb.Set(ctx, grantKey, ossKey, time.Duration(s.uploadGrantTTL)*time.Second).Err(); err != nil {
			return "", "", err
		}
	}

	att := &model.FeedbackAttachment{
		OSSKey:    ossKey,
		FileType:  "screenshot",
		MimeType:  input.MimeType,
		SizeBytes: input.SizeBytes,
	}
	if err := s.repo.CreateAttachment(att); err != nil {
		return "", "", err
	}

	return grantID, ossKey, nil
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
	Status         string
	Priority       string
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
	return reply, nil
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
