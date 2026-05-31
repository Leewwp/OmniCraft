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
	ErrAlreadyVerified      = errors.New("email already verified")
	ErrInvalidToken         = errors.New("invalid or expired token")
	ErrResendCooldown       = errors.New("resend cooldown active")
	ErrPasswordTooShort     = errors.New("password too short")
	ErrUserNotFound         = errors.New("user not found")
)

type VerificationService struct {
	userRepo   *repository.UserRepository
	rdb        *redis.Client
	mailSender mail.MailSender
	cfg        *config.Config
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
	digest := sha256Hex(rawToken)

	ttlSec := s.cfg.Verification.EmailTTLSec
	if ttlSec <= 0 {
		ttlSec = 3600
	}
	ttl := time.Duration(ttlSec) * time.Second

	digestKey := fmt.Sprintf("verify:email:%s", digest)
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, digestKey, user.ID, ttl)
	pipe.Set(ctx, prevDigestKey, digest, ttl)
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

func (s *VerificationService) VerifyEmail(ctx context.Context, rawToken string) error {
	digest := sha256Hex(rawToken)
	digestKey := fmt.Sprintf("verify:email:%s", digest)

	userID, err := s.rdb.Get(ctx, digestKey).Int64()
	if err != nil {
		return ErrInvalidToken
	}

	userDigestKey := fmt.Sprintf("verify:email:user:%d", userID)
	storedDigest, err := s.rdb.Get(ctx, userDigestKey).Result()
	if err != nil {
		return ErrInvalidToken
	}

	if !constantTimeEqual(storedDigest, digest) {
		return ErrInvalidToken
	}

	pipe := s.rdb.Pipeline()
	pipe.Del(ctx, digestKey)
	pipe.Del(ctx, userDigestKey)
	pipe.Exec(ctx)

	now := time.Now()
	if err := s.userRepo.UpdateFields(userID, map[string]interface{}{
		"email_verified_at": now,
	}); err != nil {
		return fmt.Errorf("failed to verify email: %w", err)
	}

	return nil
}

func (s *VerificationService) SendPasswordReset(ctx context.Context, email string) error {
	normalized := normalizeEmail(email)
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
	digest := sha256Hex(rawToken)

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

func (s *VerificationService) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	minLen := s.cfg.Verification.PasswordMinLength
	if minLen <= 0 {
		minLen = 8
	}
	if len(newPassword) < minLen {
		return ErrPasswordTooShort
	}

	digest := sha256Hex(rawToken)
	digestKey := fmt.Sprintf("reset:password:%s", digest)

	userID, err := s.rdb.Get(ctx, digestKey).Int64()
	if err != nil {
		return ErrInvalidToken
	}

	userDigestKey := fmt.Sprintf("reset:password:user:%d", userID)
	storedDigest, err := s.rdb.Get(ctx, userDigestKey).Result()
	if err != nil {
		return ErrInvalidToken
	}

	if !constantTimeEqual(storedDigest, digest) {
		return ErrInvalidToken
	}

	pipe := s.rdb.Pipeline()
	pipe.Del(ctx, digestKey)
	pipe.Del(ctx, userDigestKey)
	pipe.Exec(ctx)

	if err := s.userRepo.UpdateFields(userID, map[string]interface{}{
		"password_hash": hashPassword(newPassword),
	}); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func normalizeEmail(email string) string {
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

func hashPassword(password string) string {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}
	return string(hashed)
}
