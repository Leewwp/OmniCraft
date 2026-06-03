package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

func setupFeedbackServiceTest(t *testing.T) (*FeedbackService, *gorm.DB, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		rdb.Close()
		mr.Close()
	})

	db, err := gorm.Open(sqlite.Open("file:feedback_service_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sqlite db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.FeedbackTicket{}, &model.FeedbackReply{}, &model.FeedbackAttachment{}, &model.Notification{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := repository.NewFeedbackRepository(db)
	userRepo := repository.NewUserRepository(db)
	return NewFeedbackService(repo, userRepo, rdb, nil, 300), db, mr
}

type fakeFeedbackMailSender struct {
	mu         sync.Mutex
	shouldFail bool
	sent       []string
}

func (f *fakeFeedbackMailSender) SendFeedbackUpdate(_ context.Context, to, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.shouldFail {
		return fmt.Errorf("mail failed")
	}
	f.sent = append(f.sent, to+":"+subject+":"+body)
	return nil
}

func (f *fakeFeedbackMailSender) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

func TestSubmitTicketConsumesAttachmentGrantOnce(t *testing.T) {
	svc, db, _ := setupFeedbackServiceTest(t)
	ctx := context.Background()

	grantID, ossKey, err := svc.PresignUpload(ctx, PresignUploadInput{
		UserID:    ptrInt64(42),
		FileName:  "shot.png",
		MimeType:  "image/png",
		SizeBytes: 512,
	})
	if err != nil {
		t.Fatalf("PresignUpload: %v", err)
	}

	_, err = svc.SubmitTicket(ctx, SubmitTicketInput{
		UserID:      ptrInt64(42),
		Category:    "web_bug",
		Title:       "Broken page",
		Description: "The beta page failed",
		Attachments: []FeedbackAttachmentGrantInput{{GrantID: grantID, OSSKey: ossKey}},
	})
	if err != nil {
		t.Fatalf("SubmitTicket first use: %v", err)
	}

	var attached int64
	if err := db.Model(&model.FeedbackAttachment{}).Where("oss_key = ? AND ticket_id > 0", ossKey).Count(&attached).Error; err != nil {
		t.Fatalf("count attachment: %v", err)
	}
	if attached != 1 {
		t.Fatalf("attached count = %d, want 1", attached)
	}

	_, err = svc.SubmitTicket(ctx, SubmitTicketInput{
		UserID:      ptrInt64(42),
		Category:    "web_bug",
		Title:       "Second ticket",
		Description: "Should not reuse attachment",
		Attachments: []FeedbackAttachmentGrantInput{{GrantID: grantID, OSSKey: ossKey}},
	})
	if err == nil || !strings.Contains(err.Error(), "INVALID_ATTACHMENT_GRANT") {
		t.Fatalf("second use err = %v, want INVALID_ATTACHMENT_GRANT", err)
	}
}

func TestSubmitTicketRejectsAttachmentGrantKeyMismatch(t *testing.T) {
	svc, _, _ := setupFeedbackServiceTest(t)
	ctx := context.Background()

	grantID, _, err := svc.PresignUpload(ctx, PresignUploadInput{
		UserID:    ptrInt64(42),
		FileName:  "shot.png",
		MimeType:  "image/png",
		SizeBytes: 512,
	})
	if err != nil {
		t.Fatalf("PresignUpload: %v", err)
	}

	_, err = svc.SubmitTicket(ctx, SubmitTicketInput{
		UserID:      ptrInt64(42),
		Category:    "web_bug",
		Title:       "Broken page",
		Description: "The beta page failed",
		Attachments: []FeedbackAttachmentGrantInput{{GrantID: grantID, OSSKey: "feedback-staging/other/file.png"}},
	})
	if err == nil || !strings.Contains(err.Error(), "INVALID_ATTACHMENT_GRANT") {
		t.Fatalf("mismatch err = %v, want INVALID_ATTACHMENT_GRANT", err)
	}
}

func TestAdminPublicReplyNotifiesLoggedInTicketOwner(t *testing.T) {
	svc, db, _ := setupFeedbackServiceTest(t)
	notifRepo := repository.NewNotificationRepository(db)
	svc.SetNotificationService(NewNotificationService(notifRepo))

	owner := model.User{
		Email:        "owner@example.com",
		Username:     "owner",
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
	}
	admin := model.User{
		Email:        "admin@example.com",
		Username:     "admin",
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "admin",
	}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	ticket := model.FeedbackTicket{
		UserID:      &owner.ID,
		Category:    "web_bug",
		Title:       "Bug",
		Description: "Bug report",
		Status:      "open",
		Priority:    "normal",
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	if _, err := svc.AdminReply(context.Background(), AdminReplyInput{
		TicketID:      ticket.ID,
		AuthorAdminID: admin.ID,
		Body:          "We fixed this.",
	}); err != nil {
		t.Fatalf("AdminReply: %v", err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		var count int64
		if err := db.Model(&model.Notification{}).
			Where("user_id = ? AND type = ? AND target_type = ? AND target_id = ?", owner.ID, "feedback_reply", "feedback_ticket", ticket.ID).
			Count(&count).Error; err != nil {
			t.Fatalf("count notifications: %v", err)
		}
		if count == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("notification count = %d, want 1", count)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPatchTicketCloseEmailsAnonymousContactAndSurfacesMailFailure(t *testing.T) {
	svc, db, _ := setupFeedbackServiceTest(t)
	mailer := &fakeFeedbackMailSender{shouldFail: true}
	svc.SetFeedbackMailSender(mailer)

	ticket := model.FeedbackTicket{
		ContactEmail: "anon@example.com",
		Category:     "web_bug",
		Title:        "Bug",
		Description:  "Bug report",
		Status:       "open",
		Priority:     "normal",
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	_, err := svc.PatchTicket(context.Background(), ticket.ID, AdminPatchFeedbackInput{Status: "closed"})
	if err == nil || !strings.Contains(err.Error(), "FEEDBACK_NOTIFICATION_FAILED") {
		t.Fatalf("PatchTicket err = %v, want FEEDBACK_NOTIFICATION_FAILED", err)
	}
	if mailer.count() != 0 {
		t.Fatalf("successful mail count = %d, want 0", mailer.count())
	}
}

func TestPatchTicketCloseEmailsAnonymousContact(t *testing.T) {
	svc, db, _ := setupFeedbackServiceTest(t)
	mailer := &fakeFeedbackMailSender{}
	svc.SetFeedbackMailSender(mailer)

	ticket := model.FeedbackTicket{
		ContactEmail: "anon@example.com",
		Category:     "web_bug",
		Title:        "Bug",
		Description:  "Bug report",
		Status:       "open",
		Priority:     "normal",
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	if _, err := svc.PatchTicket(context.Background(), ticket.ID, AdminPatchFeedbackInput{Status: "closed"}); err != nil {
		t.Fatalf("PatchTicket: %v", err)
	}
	if mailer.count() != 1 {
		t.Fatalf("mail count = %d, want 1", mailer.count())
	}
}

func TestStageAttachmentUploadStoresBodyForExistingGrant(t *testing.T) {
	svc, _, _ := setupFeedbackServiceTest(t)
	stagingDir := t.TempDir()
	svc.SetAttachmentStagingDir(stagingDir)
	ctx := context.Background()

	grantID, _, err := svc.PresignUpload(ctx, PresignUploadInput{
		UserID:    ptrInt64(42),
		FileName:  "shot.png",
		MimeType:  "image/png",
		SizeBytes: 512,
	})
	if err != nil {
		t.Fatalf("PresignUpload: %v", err)
	}

	if err := svc.StageAttachmentUpload(ctx, grantID, strings.NewReader("image-bytes")); err != nil {
		t.Fatalf("StageAttachmentUpload: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(stagingDir, grantID+".bin"))
	if err != nil {
		t.Fatalf("read staged upload: %v", err)
	}
	if string(data) != "image-bytes" {
		t.Fatalf("staged body = %q, want image-bytes", string(data))
	}
}

func TestStageAttachmentUploadRejectsBodyLargerThanGrant(t *testing.T) {
	svc, _, _ := setupFeedbackServiceTest(t)
	stagingDir := t.TempDir()
	svc.SetAttachmentStagingDir(stagingDir)
	ctx := context.Background()

	grantID, _, err := svc.PresignUpload(ctx, PresignUploadInput{
		UserID:    ptrInt64(42),
		FileName:  "shot.png",
		MimeType:  "image/png",
		SizeBytes: 4,
	})
	if err != nil {
		t.Fatalf("PresignUpload: %v", err)
	}

	err = svc.StageAttachmentUpload(ctx, grantID, strings.NewReader("too-large"))
	if err == nil || !strings.Contains(err.Error(), "FILE_TOO_LARGE") {
		t.Fatalf("StageAttachmentUpload err = %v, want FILE_TOO_LARGE", err)
	}

	if _, err := os.Stat(filepath.Join(stagingDir, grantID+".bin")); !os.IsNotExist(err) {
		t.Fatalf("oversized upload should not leave staged file, stat err = %v", err)
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}
