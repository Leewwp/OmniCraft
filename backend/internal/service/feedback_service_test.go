package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/repository"
)

type fakeFeedbackOSSSigner struct{}

func (fakeFeedbackOSSSigner) GeneratePresignUploadURL(_ context.Context, req PresignUploadRequest, userID int64) (*PresignUploadResponse, error) {
	return &PresignUploadResponse{
		UploadURL: "https://oss.example.com/upload",
		OSSKey:    fmt.Sprintf("uploads/%d/image/%s", userID, req.FileName),
		ExpiresIn: 900,
	}, nil
}

func (fakeFeedbackOSSSigner) GenerateFeedbackPresignUploadURL(_ context.Context, req PresignUploadRequest, userID int64) (*PresignUploadResponse, error) {
	return &PresignUploadResponse{
		UploadURL: "https://oss.example.com/upload",
		OSSKey:    fmt.Sprintf("uploads/%d/feedback/%s", userID, req.FileName),
		ExpiresIn: 900,
	}, nil
}

func setupFeedbackServiceTest(t *testing.T) (*FeedbackService, *gorm.DB, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
		mr.Close()
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.FeedbackTicket{}, &model.FeedbackReply{}, &model.FeedbackAttachment{}, &model.Notification{}))

	repo := repository.NewFeedbackRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewFeedbackService(repo, userRepo, rdb, nil, 300, fakeFeedbackOSSSigner{})
	svc.SetNotificationService(NewNotificationService(repository.NewNotificationRepository(db)))
	return svc, db, mr
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
		return errors.New("mail failed")
	}
	f.sent = append(f.sent, to+":"+subject+":"+body)
	return nil
}

func (f *fakeFeedbackMailSender) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

func TestFeedbackPresignUploadReturnsOSSURLAndGrant(t *testing.T) {
	svc, _, _ := setupFeedbackServiceTest(t)

	grant, err := svc.PresignUpload(context.Background(), PresignUploadInput{
		UserID:    ptrInt64(42),
		FileName:  "shot.png",
		MimeType:  "image/png",
		SizeBytes: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, grant.GrantID)
	require.Equal(t, "uploads/42/feedback/shot.png", grant.OSSKey)
	require.Equal(t, "https://oss.example.com/upload", grant.UploadURL)
	require.Equal(t, int64(900), grant.ExpiresIn)
}

func TestFeedbackPresignUploadRequiresCaptchaForAnonymous(t *testing.T) {
	svc, _, _ := setupFeedbackServiceTest(t)

	grant, err := svc.PresignUpload(context.Background(), PresignUploadInput{
		FileName:  "shot.png",
		MimeType:  "image/png",
		SizeBytes: 512,
	})

	require.Nil(t, grant)
	require.EqualError(t, err, "CAPTCHA_REQUIRED_FOR_ANONYMOUS")
}

func TestFeedbackPresignUploadRequiresConfiguredOSS(t *testing.T) {
	svc, _, _ := setupFeedbackServiceTest(t)
	svc.ossSigner = nil

	grant, err := svc.PresignUpload(context.Background(), PresignUploadInput{
		UserID:    ptrInt64(42),
		FileName:  "shot.png",
		MimeType:  "image/png",
		SizeBytes: 512,
	})

	require.Nil(t, grant)
	require.ErrorIs(t, err, ErrOSSNotConfigured)
}

func TestFeedbackPresignUploadFailsClosedWhenEntropyUnavailable(t *testing.T) {
	svc, _, _ := setupFeedbackServiceTest(t)

	previousReader := uploadGrantEntropyReader
	uploadGrantEntropyReader = failingReader{}
	t.Cleanup(func() { uploadGrantEntropyReader = previousReader })

	grant, err := svc.PresignUpload(context.Background(), PresignUploadInput{
		UserID:    ptrInt64(42),
		FileName:  "shot.png",
		MimeType:  "image/png",
		SizeBytes: 512,
	})

	require.Nil(t, grant)
	require.ErrorIs(t, err, ErrUploadGrantUnavailable)
}

func TestFeedbackUploadGrantIsConsumedOnce(t *testing.T) {
	svc, db, _ := setupFeedbackServiceTest(t)
	ctx := context.Background()

	grant, err := svc.PresignUpload(ctx, PresignUploadInput{
		UserID:    ptrInt64(42),
		FileName:  "shot.png",
		MimeType:  "image/png",
		SizeBytes: 512,
	})
	require.NoError(t, err)

	_, err = svc.SubmitTicket(ctx, SubmitTicketInput{
		UserID:      ptrInt64(42),
		Category:    "web_bug",
		Title:       "Broken page",
		Description: "The beta page failed",
		AttachmentGrants: []FeedbackAttachmentGrantInput{{
			GrantID: grant.GrantID,
			OSSKey:  grant.OSSKey,
		}},
	})
	require.NoError(t, err)

	var attached int64
	require.NoError(t, db.Model(&model.FeedbackAttachment{}).Where("oss_key = ? AND ticket_id > 0", grant.OSSKey).Count(&attached).Error)
	require.Equal(t, int64(1), attached)

	_, err = svc.SubmitTicket(ctx, SubmitTicketInput{
		UserID:      ptrInt64(42),
		Category:    "web_bug",
		Title:       "Second ticket",
		Description: "Should not reuse attachment",
		AttachmentGrants: []FeedbackAttachmentGrantInput{{
			GrantID: grant.GrantID,
			OSSKey:  grant.OSSKey,
		}},
	})
	require.EqualError(t, err, "UPLOAD_GRANT_INVALID")
}

func TestFeedbackUploadGrantMismatchDoesNotConsumeOriginalGrant(t *testing.T) {
	svc, db, _ := setupFeedbackServiceTest(t)
	ctx := context.Background()

	grant, err := svc.PresignUpload(ctx, PresignUploadInput{
		UserID:    ptrInt64(42),
		FileName:  "shot.png",
		MimeType:  "image/png",
		SizeBytes: 512,
	})
	require.NoError(t, err)

	_, err = svc.SubmitTicket(ctx, SubmitTicketInput{
		UserID:      ptrInt64(42),
		Category:    "web_bug",
		Title:       "Broken page",
		Description: "The beta page failed",
		AttachmentGrants: []FeedbackAttachmentGrantInput{{
			GrantID: grant.GrantID,
			OSSKey:  "uploads/42/image/other.png",
		}},
	})
	require.EqualError(t, err, "UPLOAD_GRANT_INVALID")

	_, err = svc.SubmitTicket(ctx, SubmitTicketInput{
		UserID:      ptrInt64(42),
		Category:    "web_bug",
		Title:       "Retry ticket",
		Description: "Correct grant should still work.",
		AttachmentGrants: []FeedbackAttachmentGrantInput{{
			GrantID: grant.GrantID,
			OSSKey:  grant.OSSKey,
		}},
	})
	require.NoError(t, err)

	var attached int64
	require.NoError(t, db.Model(&model.FeedbackAttachment{}).Where("oss_key = ?", grant.OSSKey).Count(&attached).Error)
	require.Equal(t, int64(1), attached)
}

func TestFeedbackUploadGrantRestoredWhenTicketCreateFails(t *testing.T) {
	svc, db, _ := setupFeedbackServiceTest(t)
	ctx := context.Background()

	grant, err := svc.PresignUpload(ctx, PresignUploadInput{
		UserID:    ptrInt64(42),
		FileName:  "shot.png",
		MimeType:  "image/png",
		SizeBytes: 512,
	})
	require.NoError(t, err)

	require.NoError(t, db.Migrator().DropTable(&model.FeedbackTicket{}))
	_, err = svc.SubmitTicket(ctx, SubmitTicketInput{
		UserID:      ptrInt64(42),
		Category:    "web_bug",
		Title:       "Broken page",
		Description: "The beta page failed",
		AttachmentGrants: []FeedbackAttachmentGrantInput{{
			GrantID: grant.GrantID,
			OSSKey:  grant.OSSKey,
		}},
	})
	require.Error(t, err)

	require.NoError(t, db.AutoMigrate(&model.FeedbackTicket{}))
	_, err = svc.SubmitTicket(ctx, SubmitTicketInput{
		UserID:      ptrInt64(42),
		Category:    "web_bug",
		Title:       "Retry ticket",
		Description: "The same screenshot grant should still be usable after a DB failure.",
		AttachmentGrants: []FeedbackAttachmentGrantInput{{
			GrantID: grant.GrantID,
			OSSKey:  grant.OSSKey,
		}},
	})
	require.NoError(t, err)
}

func TestContentUploadGrantCannotBeUsedAsFeedbackAttachment(t *testing.T) {
	svc, _, mr := setupFeedbackServiceTest(t)
	ctx := context.Background()

	contentGrants := NewUploadGrantService(svc.rdb, time.Minute)
	contentGrant, err := contentGrants.Issue(ctx, UploadGrant{
		UserID:   42,
		Purpose:  "content",
		OSSKey:   "uploads/42/image/file.png",
		FileType: "image",
		MimeType: "image/png",
		FileSize: 512,
	})
	require.NoError(t, err)

	_, err = svc.SubmitTicket(ctx, SubmitTicketInput{
		UserID:      ptrInt64(42),
		Category:    "web_bug",
		Title:       "Broken page",
		Description: "Content grants must not attach to feedback tickets.",
		AttachmentGrants: []FeedbackAttachmentGrantInput{{
			GrantID: contentGrant.ID,
			OSSKey:  contentGrant.OSSKey,
		}},
	})

	require.EqualError(t, err, "UPLOAD_GRANT_INVALID")
	require.True(t, mr.Exists("upload:grant:"+contentGrant.ID))
}

func TestFeedbackAdminPublicReplyNotifiesLoggedInTicketOwner(t *testing.T) {
	svc, db, _ := setupFeedbackServiceTest(t)

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
	require.NoError(t, db.Create(&owner).Error)
	require.NoError(t, db.Create(&admin).Error)

	ticket := model.FeedbackTicket{
		UserID:      &owner.ID,
		Category:    "web_bug",
		Title:       "Bug",
		Description: "Bug report",
		Status:      "open",
		Priority:    "normal",
	}
	require.NoError(t, db.Create(&ticket).Error)

	_, err := svc.AdminReply(context.Background(), AdminReplyInput{
		TicketID:      ticket.ID,
		AuthorAdminID: admin.ID,
		Body:          "We fixed this.",
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		var count int64
		err := db.Model(&model.Notification{}).
			Where("user_id = ? AND type = ? AND target_type = ? AND target_id = ?", owner.ID, "system", "feedback_ticket", ticket.ID).
			Count(&count).Error
		return err == nil && count == 1
	}, 500*time.Millisecond, 10*time.Millisecond)
}

func TestFeedbackPatchTicketCloseEmailsAnonymousContactAndSurfacesMailFailure(t *testing.T) {
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
	require.NoError(t, db.Create(&ticket).Error)

	_, err := svc.PatchTicket(context.Background(), ticket.ID, AdminPatchFeedbackInput{Status: "closed"})
	require.EqualError(t, err, "FEEDBACK_DELIVERY_FAILED")
	require.Equal(t, 0, mailer.count())
}

func TestFeedbackPatchTicketCloseEmailsAnonymousContact(t *testing.T) {
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
	require.NoError(t, db.Create(&ticket).Error)

	_, err := svc.PatchTicket(context.Background(), ticket.ID, AdminPatchFeedbackInput{Status: "closed"})
	require.NoError(t, err)
	require.Equal(t, 1, mailer.count())
}

func ptrInt64(v int64) *int64 {
	return &v
}

type fakeImageReviewer struct {
	result string
	err    error
	urls   []string
}

func (f *fakeImageReviewer) ReviewImageURL(_ context.Context, imageURL string) (string, error) {
	f.urls = append(f.urls, imageURL)
	if f.err != nil {
		return "", f.err
	}
	return f.result, nil
}

func feedbackModerationService(t *testing.T, mode string, reviewer ImageReviewer) *FeedbackService {
	t.Helper()
	svc, _, _ := setupFeedbackServiceTest(t)
	svc.SetConfig(&config.Config{Server: config.ServerConfig{Mode: mode}})
	svc.SetReviewService(reviewer)
	return svc
}

func issueFeedbackUploadGrant(t *testing.T, svc *FeedbackService) *PresignFeedbackUploadGrant {
	t.Helper()
	grant, err := svc.PresignUpload(context.Background(), PresignUploadInput{
		UserID:    ptrInt64(42),
		FileName:  "shot.png",
		MimeType:  "image/png",
		SizeBytes: 512,
	})
	require.NoError(t, err)
	return grant
}

func countFeedbackAttachments(t *testing.T, db *gorm.DB, ossKey string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&model.FeedbackAttachment{}).Where("oss_key = ?", ossKey).Count(&n).Error)
	return n
}

func TestFeedbackSubmitTicketRejectsBlockedAttachmentAndRestoresGrant(t *testing.T) {
	svc, db, mr := setupFeedbackServiceTest(t)
	svc.SetConfig(&config.Config{Server: config.ServerConfig{Mode: "debug"}})
	reviewer := &fakeImageReviewer{result: "block"}
	svc.SetReviewService(reviewer)
	ctx := context.Background()

	grant := issueFeedbackUploadGrant(t, svc)
	_, err := svc.SubmitTicket(ctx, SubmitTicketInput{
		UserID:      ptrInt64(42),
		Category:    "web_bug",
		Title:       "Broken page",
		Description: "The beta page failed",
		AttachmentGrants: []FeedbackAttachmentGrantInput{{
			GrantID: grant.GrantID,
			OSSKey:  grant.OSSKey,
		}},
	})
	require.ErrorIs(t, err, ErrFeedbackAttachmentBlocked)
	require.Equal(t, int64(0), countFeedbackAttachments(t, db, grant.OSSKey))
	require.True(t, mr.Exists("feedback:upload_grant:"+grant.GrantID), "consumed grant must be restored after moderation rejection")

	var tickets int64
	require.NoError(t, db.Model(&model.FeedbackTicket{}).Count(&tickets).Error)
	require.Equal(t, int64(0), tickets)
}

func TestFeedbackSubmitTicketAllowsPassAndReviewAttachments(t *testing.T) {
	for _, result := range []string{"pass", "review"} {
		t.Run(result, func(t *testing.T) {
			svc, db, _ := setupFeedbackServiceTest(t)
			svc.SetConfig(&config.Config{Server: config.ServerConfig{Mode: "debug"}})
			reviewer := &fakeImageReviewer{result: result}
			svc.SetReviewService(reviewer)
			ctx := context.Background()

			grant := issueFeedbackUploadGrant(t, svc)
			_, err := svc.SubmitTicket(ctx, SubmitTicketInput{
				UserID:      ptrInt64(42),
				Category:    "web_bug",
				Title:       "Broken page",
				Description: "The beta page failed",
				AttachmentGrants: []FeedbackAttachmentGrantInput{{
					GrantID: grant.GrantID,
					OSSKey:  grant.OSSKey,
				}},
			})
			require.NoError(t, err)
			require.Equal(t, int64(1), countFeedbackAttachments(t, db, grant.OSSKey))
		})
	}
}

func TestFeedbackSubmitTicketFailsClosedInReleaseWhenModerationUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name     string
		reviewer ImageReviewer
	}{
		{name: "reviewer error", reviewer: &fakeImageReviewer{err: errors.New("green upstream down")}},
		{name: "reviewer not wired", reviewer: nil},
		{name: "green not configured in release", reviewer: &fakeImageReviewer{err: aliyun.ErrGreenNotConfigured}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _ := setupFeedbackServiceTest(t)
			svc.SetConfig(&config.Config{Server: config.ServerConfig{Mode: "release"}})
			svc.SetReviewService(tc.reviewer)
			ctx := context.Background()

			grant := issueFeedbackUploadGrant(t, svc)
			_, err := svc.SubmitTicket(ctx, SubmitTicketInput{
				UserID:      ptrInt64(42),
				Category:    "web_bug",
				Title:       "Broken page",
				Description: "The beta page failed",
				AttachmentGrants: []FeedbackAttachmentGrantInput{{
					GrantID: grant.GrantID,
					OSSKey:  grant.OSSKey,
				}},
			})
			require.ErrorIs(t, err, ErrFeedbackAttachmentModerationUnavailable)
		})
	}
}

func TestFeedbackSubmitTicketFailsOpenWhenGreenNotConfiguredOutsideRelease(t *testing.T) {
	svc, db, _ := setupFeedbackServiceTest(t)
	svc.SetConfig(&config.Config{Server: config.ServerConfig{Mode: "debug"}})
	reviewer := &fakeImageReviewer{err: aliyun.ErrGreenNotConfigured}
	svc.SetReviewService(reviewer)
	ctx := context.Background()

	grant := issueFeedbackUploadGrant(t, svc)
	_, err := svc.SubmitTicket(ctx, SubmitTicketInput{
		UserID:      ptrInt64(42),
		Category:    "web_bug",
		Title:       "Broken page",
		Description: "The beta page failed",
		AttachmentGrants: []FeedbackAttachmentGrantInput{{
			GrantID: grant.GrantID,
			OSSKey:  grant.OSSKey,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), countFeedbackAttachments(t, db, grant.OSSKey))
}

func TestFeedbackTextOnlyTicketBypassesAttachmentModeration(t *testing.T) {
	for _, tc := range []struct {
		name     string
		reviewer ImageReviewer
		mode     string
	}{
		{name: "no reviewer", reviewer: nil, mode: "release"},
		{name: "blocking reviewer", reviewer: &fakeImageReviewer{result: "block"}, mode: "debug"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, _ := setupFeedbackServiceTest(t)
			svc.SetConfig(&config.Config{Server: config.ServerConfig{Mode: tc.mode}})
			if tc.reviewer != nil {
				svc.SetReviewService(tc.reviewer)
			}
			ctx := context.Background()

			_, err := svc.SubmitTicket(ctx, SubmitTicketInput{
				UserID:      ptrInt64(42),
				Category:    "web_bug",
				Title:       "Broken page",
				Description: "The beta page failed",
			})
			require.NoError(t, err)
			var tickets int64
			require.NoError(t, db.Model(&model.FeedbackTicket{}).Count(&tickets).Error)
			require.Equal(t, int64(1), tickets)
		})
	}
}

func TestFeedbackSubmitTicketPassesAttachmentScanURLFromOSSKey(t *testing.T) {
	svc, _, _ := setupFeedbackServiceTest(t)
	svc.SetConfig(&config.Config{
		Server: config.ServerConfig{Mode: "debug"},
		OSS:    config.OSSConfig{Domain: "https://cdn.example.com/"},
	})
	reviewer := &fakeImageReviewer{result: "pass"}
	svc.SetReviewService(reviewer)
	ctx := context.Background()

	grant := issueFeedbackUploadGrant(t, svc)
	_, err := svc.SubmitTicket(ctx, SubmitTicketInput{
		UserID:      ptrInt64(42),
		Category:    "web_bug",
		Title:       "Broken page",
		Description: "The beta page failed",
		AttachmentGrants: []FeedbackAttachmentGrantInput{{
			GrantID: grant.GrantID,
			OSSKey:  grant.OSSKey,
		}},
	})
	require.NoError(t, err)
	require.Len(t, reviewer.urls, 1)
	require.Equal(t, "https://cdn.example.com/"+grant.OSSKey, reviewer.urls[0])
}
