package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

type fakeMailSender struct {
	mu       sync.Mutex
	sent     []string
	shouldOK bool
}

func newFakeMailSender() *fakeMailSender {
	return &fakeMailSender{shouldOK: true}
}

func (f *fakeMailSender) SendVerification(_ context.Context, to, link string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.shouldOK {
		return fmt.Errorf("smtp error")
	}
	f.sent = append(f.sent, "verify:"+to+":"+link)
	return nil
}

func (f *fakeMailSender) SendPasswordReset(_ context.Context, to, link string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.shouldOK {
		return fmt.Errorf("smtp error")
	}
	f.sent = append(f.sent, "reset:"+to+":"+link)
	return nil
}

func (f *fakeMailSender) getSent() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.sent))
	copy(out, f.sent)
	return out
}

func setupVerificationTest(t *testing.T) (*VerificationService, *fakeMailSender, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close(); mr.Close() })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sqlite db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	db.AutoMigrate(&model.User{})

	userRepo := repository.NewUserRepository(db)
	fakeMail := newFakeMailSender()
	cfg := &config.Config{
		Web: config.WebConfig{PublicBaseURL: "https://app.leeppp.online"},
		Verification: config.VerificationConfig{
			EmailTTLSec:       3600,
			ResetTTLSec:       3600,
			ResendCooldownSec: 60,
			PasswordMinLength: 8,
		},
	}

	svc := NewVerificationService(userRepo, rdb, fakeMail, cfg)
	return svc, fakeMail, mr
}

func createTestUser(t *testing.T, userRepo *repository.UserRepository) *model.User {
	t.Helper()
	user := &model.User{
		Email:        "test@example.com",
		PasswordHash: "$2a$10$fakehash",
		Username:     "testuser",
		Reputation:   10,
		Role:         "user",
	}
	if err := userRepo.CreateUser(user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func TestSendVerificationStoresHashedToken(t *testing.T) {
	svc, fakeMail, _ := setupVerificationTest(t)
	userRepo := svc.userRepo
	user := createTestUser(t, userRepo)

	ctx := context.Background()
	if err := svc.SendVerification(ctx, user); err != nil {
		t.Fatalf("SendVerification: %v", err)
	}

	sent := fakeMail.getSent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(sent))
	}

	digestKey := fmt.Sprintf("verify:email:user:%d", user.ID)
	digest, err := svc.rdb.Get(ctx, digestKey).Result()
	if err != nil {
		t.Fatalf("token digest not found in Redis: %v", err)
	}

	fullDigestKey := fmt.Sprintf("verify:email:%s", digest)
	storedUserID, err := svc.rdb.Get(ctx, fullDigestKey).Int64()
	if err != nil {
		t.Fatalf("digest key not found: %v", err)
	}
	if storedUserID != user.ID {
		t.Errorf("stored user ID = %d, want %d", storedUserID, user.ID)
	}
}

func TestVerifyEmailSingleUse(t *testing.T) {
	svc, fakeMail, _ := setupVerificationTest(t)
	userRepo := svc.userRepo
	user := createTestUser(t, userRepo)

	ctx := context.Background()
	svc.SendVerification(ctx, user)

	sent := fakeMail.getSent()
	rawToken := extractTokenFromLink(sent[0], "token=")

	if err := svc.VerifyEmail(ctx, rawToken); err != nil {
		t.Fatalf("VerifyEmail first use: %v", err)
	}

	if err := svc.VerifyEmail(ctx, rawToken); err != ErrInvalidToken {
		t.Errorf("second use: got %v, want ErrInvalidToken", err)
	}
}

func TestVerifyEmailInvalidatesRuntimeUserStatusCache(t *testing.T) {
	svc, fakeMail, _ := setupVerificationTest(t)
	userRepo := svc.userRepo
	user := createTestUser(t, userRepo)

	ctx := context.Background()
	cache := NewRuntimeStatusCache(svc.rdb, svc.cfg)
	cache.Set(user.ID, &RuntimeUserStatus{
		ID:              user.ID,
		Role:            user.Role,
		IsBanned:        user.IsBanned,
		EmailVerifiedAt: nil,
		Reputation:      user.Reputation,
	})
	if _, ok := cache.Get(user.ID); !ok {
		t.Fatal("expected runtime status cache to be populated before verification")
	}

	if err := svc.SendVerification(ctx, user); err != nil {
		t.Fatalf("SendVerification: %v", err)
	}
	rawToken := extractTokenFromLink(fakeMail.getSent()[0], "token=")

	if err := svc.VerifyEmail(ctx, rawToken); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}

	if _, ok := cache.Get(user.ID); ok {
		t.Fatal("expected VerifyEmail to invalidate cached runtime user status")
	}
}

func TestVerifyEmailConcurrentDoubleSubmitConsumesTokenOnce(t *testing.T) {
	svc, fakeMail, _ := setupVerificationTest(t)
	userRepo := svc.userRepo
	user := createTestUser(t, userRepo)

	ctx := context.Background()
	if err := svc.SendVerification(ctx, user); err != nil {
		t.Fatalf("SendVerification: %v", err)
	}
	rawToken := extractTokenFromLink(fakeMail.getSent()[0], "token=")
	barrier := newRedisGetBarrier(fmt.Sprintf("verify:email:user:%d", user.ID))
	svc.rdb.AddHook(barrier)

	errs := runConcurrentVerifyEmail(ctx, svc, rawToken, barrier)
	assertExactlyOneSuccessAndOneInvalidToken(t, errs)
}

func TestVerifyEmailInvalidatesPreviousLink(t *testing.T) {
	svc, fakeMail, _ := setupVerificationTest(t)
	userRepo := svc.userRepo
	user := createTestUser(t, userRepo)

	ctx := context.Background()
	svc.SendVerification(ctx, user)
	firstToken := extractTokenFromLink(fakeMail.getSent()[0], "token=")

	svc.rdb.Del(ctx, fmt.Sprintf("verify:resend:%d", user.ID))
	svc.SendVerification(ctx, user)
	secondToken := extractTokenFromLink(fakeMail.getSent()[1], "token=")

	if err := svc.VerifyEmail(ctx, firstToken); err != ErrInvalidToken {
		t.Errorf("old token should be invalid: got %v", err)
	}

	if err := svc.VerifyEmail(ctx, secondToken); err != nil {
		t.Errorf("new token should work: %v", err)
	}
}

func TestSendVerificationResendCooldown(t *testing.T) {
	svc, _, _ := setupVerificationTest(t)
	userRepo := svc.userRepo
	user := createTestUser(t, userRepo)

	ctx := context.Background()
	if err := svc.SendVerification(ctx, user); err != nil {
		t.Fatalf("first send: %v", err)
	}

	if err := svc.SendVerification(ctx, user); err != ErrResendCooldown {
		t.Errorf("cooldown: got %v, want ErrResendCooldown", err)
	}
}

func TestSendVerificationAlreadyVerified(t *testing.T) {
	svc, _, _ := setupVerificationTest(t)
	userRepo := svc.userRepo
	user := createTestUser(t, userRepo)

	now := time.Now()
	user.EmailVerifiedAt = &now
	userRepo.UpdateFields(user.ID, map[string]interface{}{"email_verified_at": now})

	ctx := context.Background()
	if err := svc.SendVerification(ctx, user); err != ErrAlreadyVerified {
		t.Errorf("got %v, want ErrAlreadyVerified", err)
	}
}

func TestSendPasswordResetUnknownEmail(t *testing.T) {
	svc, fakeMail, _ := setupVerificationTest(t)

	ctx := context.Background()
	err := svc.SendPasswordReset(ctx, "nonexistent@example.com")
	if err != nil {
		t.Fatalf("SendPasswordReset for unknown email should not error: %v", err)
	}
	if len(fakeMail.getSent()) != 0 {
		t.Error("should not send email for unknown address")
	}
}

func TestResetPasswordSingleUse(t *testing.T) {
	svc, fakeMail, _ := setupVerificationTest(t)
	userRepo := svc.userRepo
	user := createTestUser(t, userRepo)

	ctx := context.Background()
	svc.SendPasswordReset(ctx, user.Email)

	sent := fakeMail.getSent()
	rawToken := extractTokenFromLink(sent[0], "token=")

	if _, err := svc.ResetPassword(ctx, rawToken, "newpassword123"); err != nil {
		t.Fatalf("ResetPassword first use: %v", err)
	}

	if _, err := svc.ResetPassword(ctx, rawToken, "anotherpass456"); err != ErrInvalidToken {
		t.Errorf("second use: got %v, want ErrInvalidToken", err)
	}
}

func TestResetPasswordConcurrentDoubleSubmitConsumesTokenOnce(t *testing.T) {
	svc, fakeMail, _ := setupVerificationTest(t)
	userRepo := svc.userRepo
	user := createTestUser(t, userRepo)

	ctx := context.Background()
	if err := svc.SendPasswordReset(ctx, user.Email); err != nil {
		t.Fatalf("SendPasswordReset: %v", err)
	}
	rawToken := extractTokenFromLink(fakeMail.getSent()[0], "token=")
	barrier := newRedisGetBarrier(fmt.Sprintf("reset:password:user:%d", user.ID))
	svc.rdb.AddHook(barrier)

	errs := runConcurrentResetPassword(ctx, svc, rawToken, barrier)
	assertExactlyOneSuccessAndOneInvalidToken(t, errs)
}

func TestResetPasswordTooShort(t *testing.T) {
	svc, fakeMail, _ := setupVerificationTest(t)
	userRepo := svc.userRepo
	user := createTestUser(t, userRepo)

	ctx := context.Background()
	svc.SendPasswordReset(ctx, user.Email)

	sent := fakeMail.getSent()
	rawToken := extractTokenFromLink(sent[0], "token=")

	if _, err := svc.ResetPassword(ctx, rawToken, "short"); err != ErrPasswordTooShort {
		t.Errorf("got %v, want ErrPasswordTooShort", err)
	}
}

func TestResetPasswordInvalidatesPreviousLink(t *testing.T) {
	svc, fakeMail, _ := setupVerificationTest(t)
	userRepo := svc.userRepo
	user := createTestUser(t, userRepo)

	ctx := context.Background()
	svc.SendPasswordReset(ctx, user.Email)
	firstToken := extractTokenFromLink(fakeMail.getSent()[0], "token=")

	svc.SendPasswordReset(ctx, user.Email)
	secondToken := extractTokenFromLink(fakeMail.getSent()[1], "token=")

	if _, err := svc.ResetPassword(ctx, firstToken, "newpassword123"); err != ErrInvalidToken {
		t.Errorf("old reset token should be invalid: got %v", err)
	}

	if _, err := svc.ResetPassword(ctx, secondToken, "newpassword123"); err != nil {
		t.Errorf("new reset token should work: %v", err)
	}
}

func TestTokenHashedNotStoredRaw(t *testing.T) {
	svc, fakeMail, mr := setupVerificationTest(t)
	userRepo := svc.userRepo
	user := createTestUser(t, userRepo)

	ctx := context.Background()
	svc.SendVerification(ctx, user)

	sent := fakeMail.getSent()
	rawToken := extractTokenFromLink(sent[0], "token=")

	for _, key := range mr.Keys() {
		val, _ := mr.Get(key)
		if val == rawToken {
			t.Errorf("raw token found in Redis key %s — tokens must be hashed", key)
		}
	}

	digest := testSHA256Hex(rawToken)
	digestKey := fmt.Sprintf("verify:email:%s", digest)
	if _, err := svc.rdb.Get(ctx, digestKey).Result(); err != nil {
		t.Errorf("hashed digest key not found: %v", err)
	}
}

func TestMailSenderFailure(t *testing.T) {
	svc, fakeMail, _ := setupVerificationTest(t)
	userRepo := svc.userRepo
	user := createTestUser(t, userRepo)

	fakeMail.shouldOK = false

	ctx := context.Background()
	if err := svc.SendVerification(ctx, user); err == nil {
		t.Error("expected error when mail sender fails")
	}
}

func testSHA256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func extractTokenFromLink(link, param string) string {
	idx := strings.Index(link, param)
	if idx < 0 {
		return ""
	}
	return link[idx+len(param):]
}

type redisGetBarrier struct {
	targetKey string
	hits      chan struct{}
	release   chan struct{}
	once      sync.Once
}

func newRedisGetBarrier(targetKey string) *redisGetBarrier {
	return &redisGetBarrier{
		targetKey: targetKey,
		hits:      make(chan struct{}, 2),
		release:   make(chan struct{}),
	}
}

func (b *redisGetBarrier) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (b *redisGetBarrier) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		err := next(ctx, cmd)
		if err != nil || strings.ToLower(cmd.FullName()) != "get" || len(cmd.Args()) < 2 {
			return err
		}
		if fmt.Sprint(cmd.Args()[1]) != b.targetKey {
			return err
		}
		b.hits <- struct{}{}
		<-b.release
		return err
	}
}

func (b *redisGetBarrier) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func (b *redisGetBarrier) releaseWhenReadyOrDone(done <-chan struct{}) {
	defer b.once.Do(func() { close(b.release) })
	hits := 0
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	for hits < 2 {
		select {
		case <-b.hits:
			hits++
		case <-done:
			return
		case <-timer.C:
			return
		}
	}
}

func runConcurrentVerifyEmail(ctx context.Context, svc *VerificationService, rawToken string, barrier *redisGetBarrier) []error {
	var wg sync.WaitGroup
	errs := make([]error, 2)
	done := make(chan struct{})

	wg.Add(2)
	for i := range errs {
		go func(i int) {
			defer wg.Done()
			errs[i] = svc.VerifyEmail(ctx, rawToken)
		}(i)
	}
	go func() {
		wg.Wait()
		close(done)
	}()
	barrier.releaseWhenReadyOrDone(done)
	return waitForConcurrentResults(done, errs)
}

func runConcurrentResetPassword(ctx context.Context, svc *VerificationService, rawToken string, barrier *redisGetBarrier) []error {
	var wg sync.WaitGroup
	errs := make([]error, 2)
	done := make(chan struct{})

	wg.Add(2)
	for i := range errs {
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.ResetPassword(ctx, rawToken, fmt.Sprintf("newpassword%d", i+123))
		}(i)
	}
	go func() {
		wg.Wait()
		close(done)
	}()
	barrier.releaseWhenReadyOrDone(done)
	return waitForConcurrentResults(done, errs)
}

func waitForConcurrentResults(done <-chan struct{}, errs []error) []error {
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		panic("concurrent token consumption test timed out")
	}
	return errs
}

func assertExactlyOneSuccessAndOneInvalidToken(t *testing.T, errs []error) {
	t.Helper()
	sort.Slice(errs, func(i, j int) bool {
		return fmt.Sprint(errs[i]) < fmt.Sprint(errs[j])
	})
	successes := 0
	invalids := 0
	for _, err := range errs {
		switch err {
		case nil:
			successes++
		case ErrInvalidToken:
			invalids++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if successes != 1 || invalids != 1 {
		t.Fatalf("got successes=%d invalid_tokens=%d, want exactly one each; errs=%v", successes, invalids, errs)
	}
}
