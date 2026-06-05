package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/mail"
	"omnicraft/backend/internal/repository"
)

var (
	ErrAlreadyVerified  = errors.New("email already verified")
	ErrInvalidToken     = errors.New("invalid or expired token")
	ErrResendCooldown   = errors.New("resend cooldown active")
	ErrPasswordTooShort = errors.New("password too short")
	ErrUserNotFound     = errors.New("user not found")
)

var consumeTokenScript = redis.NewScript(`
local value = redis.call("GET", KEYS[1])
if not value then
	return nil
end
local digest_key = ARGV[1] .. value
local stored_digest = redis.call("GET", digest_key)
if stored_digest ~= ARGV[2] then
	return nil
end
redis.call("DEL", KEYS[1], digest_key)
return value
`)

type VerificationService struct {
	userRepo   *repository.UserRepository
	rdb        *redis.Client
	mailSender mail.MailSender
	cfg        *config.Config
	authService *AuthService
}

func NewVerificationService(
	userRepo *repository.UserRepository,
	rdb *redis.Client,
	mailSender mail.MailSender,
	cfg *config.Config,
) *VerificationService {
	return &VerificationService{
		userRepo:   userRepo,
		rdb:        rdb,
		mailSender: mailSender,
		cfg:        cfg,
	}
}

// SetAuthService sets the auth service reference (to avoid circular dependency in constructor).
func (s *VerificationService) SetAuthService(as *AuthService) {
	s.authService = as
}

// SendVerificationForPending sends a verification email for a pending registration.
// The token maps to the registration ID in Redis, not a user ID.
func (s *VerificationService) SendVerificationForPending(ctx context.Context, regID string, email string) error {
	cooldownSec := s.cfg.Verification.ResendCooldownSec
	if cooldownSec <= 0 {
		cooldownSec = 60
	}
	cooldownKey := fmt.Sprintf("verify:resend:pending:%s", regID)
	if _, err := s.rdb.Get(ctx, cooldownKey).Result(); err == nil {
		return ErrResendCooldown
	}

	// Invalidate previous token for this registration
	prevDigestKey := fmt.Sprintf("verify:pending:reg:%s", regID)
	if prevDigest, err := s.rdb.Get(ctx, prevDigestKey).Result(); err == nil {
		s.rdb.Del(ctx, fmt.Sprintf("verify:pending:%s", prevDigest))
		s.rdb.Del(ctx, prevDigestKey)
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}
	rawToken := hex.EncodeToString(tokenBytes)
	digest := sha256HexLocal(rawToken)

	ttlSec := s.cfg.Verification.EmailTTLSec
	if ttlSec <= 0 {
		ttlSec = 3600
	}
	ttl := time.Duration(ttlSec) * time.Second

	digestKey := fmt.Sprintf("verify:pending:%s", digest)
	regDigestKey := fmt.Sprintf("verify:pending:reg:%s", regID)
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, digestKey, regID, ttl)
	pipe.Set(ctx, regDigestKey, digest, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to store verification token: %w", err)
	}

	cooldown := time.Duration(cooldownSec) * time.Second
	s.rdb.Set(ctx, cooldownKey, time.Now().Unix(), cooldown)

	baseURL := s.cfg.Web.PublicBaseURL
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}
	link := fmt.Sprintf("%s/verify-email?token=%s", baseURL, rawToken)

	if err := s.mailSender.SendVerification(ctx, email, link); err != nil {
		// Clean up token on email send failure
		s.rdb.Del(ctx, digestKey, regDigestKey)
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	return nil
}

// SendVerification sends a verification email for an existing unverified user in DB (legacy flow).
func (s *VerificationService) SendVerification(ctx context.Context, user *model.User) error {
	if user.EmailVerifiedAt != nil {
		return ErrAlreadyVerified
	}

	cooldownSec := s.cfg.Verification.ResendCooldownSec
	if cooldownSec <= 0 {
		cooldownSec = 60
	}
	cooldownKey := fmt.Sprintf("verify:resend:%d", user.ID)
	if _, err := s.rdb.Get(ctx, cooldownKey).Result(); err == nil {
		return ErrResendCooldown
	}

	prevDigestKey := fmt.Sprintf("verify:email:user:%d", user.ID)
	if prevDigest, err := s.rdb.Get(ctx, prevDigestKey).Result(); err == nil {
		s.rdb.Del(ctx, fmt.Sprintf("verify:email:%s", prevDigest))
		s.rdb.Del(ctx, prevDigestKey)
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}
	rawToken := hex.EncodeToString(tokenBytes)
	digest := sha256HexLocal(rawToken)

	ttlSec := s.cfg.Verification.EmailTTLSec
	if ttlSec <= 0 {
		ttlSec = 3600
	}
	ttl := time.Duration(ttlSec) * time.Second

	digestKey := fmt.Sprintf("verify:email:%s", digest)
	userDigestKey := fmt.Sprintf("verify:email:user:%d", user.ID)
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, digestKey, user.ID, ttl)
	pipe.Set(ctx, userDigestKey, digest, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to store verification token: %w", err)
	}

	cooldown := time.Duration(cooldownSec) * time.Second
	s.rdb.Set(ctx, cooldownKey, time.Now().Unix(), cooldown)

	baseURL := s.cfg.Web.PublicBaseURL
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}
	link := fmt.Sprintf("%s/verify-email?token=%s", baseURL, rawToken)

	if err := s.mailSender.SendVerification(ctx, user.Email, link); err != nil {
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	return nil
}

// VerifyEmail handles email verification for both pending registrations and legacy DB users.
func (s *VerificationService) VerifyEmail(ctx context.Context, rawToken string) error {
	digest := sha256HexLocal(rawToken)

	// Try pending registration flow first (new registrations)
	regID, err := s.consumePendingToken(ctx, digest)
	if err == nil && regID != "" {
		return s.verifyPendingRegistration(ctx, regID)
	}

	// Fall back to legacy flow (existing unverified users in DB)
	userID, err := s.consumeTokenAtomic(ctx, "verify:email", digest)
	if err != nil {
		return ErrInvalidToken
	}

	now := time.Now()
	if err := s.userRepo.UpdateFields(userID, map[string]interface{}{
		"email_verified_at": now,
	}); err != nil {
		return fmt.Errorf("failed to verify email: %w", err)
	}
	NewRuntimeStatusCache(s.rdb, s.cfg).Invalidate(userID)

	return nil
}

// consumePendingToken tries to consume a pending registration verification token.
func (s *VerificationService) consumePendingToken(ctx context.Context, digest string) (string, error) {
	if s.rdb == nil {
		return "", ErrInvalidToken
	}
	digestKey := fmt.Sprintf("verify:pending:%s", digest)
	regDigestKeyPrefix := "verify:pending:reg:"
	regID, err := consumeTokenScript.Run(ctx, s.rdb, []string{digestKey}, regDigestKeyPrefix, digest).Text()
	if err != nil {
		return "", ErrInvalidToken
	}
	return regID, nil
}

// verifyPendingRegistration creates a verified user from pending registration data.
func (s *VerificationService) verifyPendingRegistration(ctx context.Context, regID string) error {
	if s.authService == nil {
		return fmt.Errorf("auth service not configured")
	}

	pending, err := s.authService.GetPendingRegistration(ctx, regID)
	if err != nil {
		return fmt.Errorf("failed to get pending registration: %w", err)
	}
	if pending == nil {
		return ErrInvalidToken
	}

	// Create the verified user in DB
	user, err := s.authService.CreateUserFromPending(pending)
	if err != nil {
		return fmt.Errorf("failed to create user from pending registration: %w", err)
	}

	// Clean up pending registration data from Redis
	if err := s.authService.DeletePendingRegistration(ctx, regID, pending); err != nil {
		// Log but don't fail - user is already created
		_ = err
	}

	// Invalidate runtime status cache
	NewRuntimeStatusCache(s.rdb, s.cfg).Invalidate(user.ID)

	return nil
}

func (s *VerificationService) SendPasswordReset(ctx context.Context, email string) error {
	normalized := normalizeEmailLocal(email)
	user, err := s.userRepo.FindByEmail(normalized)
	if err != nil || user == nil {
		return nil
	}

	prevDigestKey := fmt.Sprintf("reset:password:user:%d", user.ID)
	if prevDigest, err := s.rdb.Get(ctx, prevDigestKey).Result(); err == nil {
		s.rdb.Del(ctx, fmt.Sprintf("reset:password:%s", prevDigest))
		s.rdb.Del(ctx, prevDigestKey)
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}
	rawToken := hex.EncodeToString(tokenBytes)
	digest := sha256HexLocal(rawToken)

	ttlSec := s.cfg.Verification.ResetTTLSec
	if ttlSec <= 0 {
		ttlSec = 3600
	}
	ttl := time.Duration(ttlSec) * time.Second

	digestKey := fmt.Sprintf("reset:password:%s", digest)
	userDigestKey := fmt.Sprintf("reset:password:user:%d", user.ID)
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, digestKey, user.ID, ttl)
	pipe.Set(ctx, userDigestKey, digest, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to store reset token: %w", err)
	}

	baseURL := s.cfg.Web.PublicBaseURL
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}
	link := fmt.Sprintf("%s/reset-password?token=%s", baseURL, rawToken)

	if err := s.mailSender.SendPasswordReset(ctx, user.Email, link); err != nil {
		return fmt.Errorf("failed to send reset email: %w", err)
	}

	return nil
}

func (s *VerificationService) ResetPassword(ctx context.Context, rawToken, newPassword string) (int64, error) {
	minLen := s.cfg.Verification.PasswordMinLength
	if minLen <= 0 {
		minLen = 8
	}
	if len(newPassword) < minLen {
		return 0, ErrPasswordTooShort
	}

	digest := sha256HexLocal(rawToken)
	userID, err := s.consumeTokenAtomic(ctx, "reset:password", digest)
	if err != nil {
		return 0, err
	}

	if err := s.userRepo.UpdateFields(userID, map[string]interface{}{
		"password_hash": hashPasswordLocal(newPassword),
	}); err != nil {
		return 0, fmt.Errorf("failed to update password: %w", err)
	}

	return userID, nil
}

func (s *VerificationService) consumeTokenAtomic(ctx context.Context, prefix, digest string) (int64, error) {
	if s.rdb == nil {
		return 0, ErrInvalidToken
	}
	digestKey := fmt.Sprintf("%s:%s", prefix, digest)
	userDigestKeyPrefix := fmt.Sprintf("%s:user:", prefix)
	userID, err := consumeTokenScript.Run(ctx, s.rdb, []string{digestKey}, userDigestKeyPrefix, digest).Int64()
	if err != nil {
		return 0, ErrInvalidToken
	}
	return userID, nil
}

func sha256HexLocal(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func normalizeEmailLocal(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

func hashPasswordLocal(password string) string {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}
	return string(hashed)
}
