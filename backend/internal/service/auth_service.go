package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	"omnicraft/backend/internal/repository"
)

var (
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserBanned         = errors.New("user is banned")
	ErrTokenInvalid       = errors.New("token is invalid")
	ErrEmailNotVerified   = errors.New("email not verified")
)

// PendingRegistration holds registration data stored in Redis before email verification.
type PendingRegistration struct {
	Email                  string `json:"email"`
	Username               string `json:"username"`
	PasswordHash           string `json:"password_hash"`
	Reputation             int    `json:"reputation"`
	Role                   string `json:"role"`
	PreferredLocale        string `json:"preferred_locale"`
	AcceptedTermsVersion   string `json:"accepted_terms_version,omitempty"`
	AcceptedTermsAt        int64  `json:"accepted_terms_at,omitempty"`
	AcceptedPrivacyVersion string `json:"accepted_privacy_version,omitempty"`
	AcceptedPrivacyAt      int64  `json:"accepted_privacy_at,omitempty"`
}

type AuthService struct {
	userRepo              *repository.UserRepository
	redis                 *redis.Client
	cfg                   *config.Config
	defaultCollectionRepo defaultCollectionEnsurer
}

func NewAuthService(userRepo *repository.UserRepository, rdb *redis.Client, cfg *config.Config) *AuthService {
	service := &AuthService{
		userRepo: userRepo,
		redis:    rdb,
		cfg:      cfg,
	}
	if userRepo != nil {
		service.defaultCollectionRepo = repository.NewCollectionRepository(userRepo.DB())
	}
	return service
}

type defaultCollectionEnsurer interface {
	EnsureDefaultCollection(ctx context.Context, userID int64, zone string) (*model.Collection, error)
}

func (s *AuthService) SetCollectionRepository(repo defaultCollectionEnsurer) {
	s.defaultCollectionRepo = repo
}

type RegisterInput struct {
	Email                  string `json:"email" binding:"required,email"`
	Username               string `json:"username" binding:"required,min=2,max=64"`
	Password               string `json:"password" binding:"required,min=6,max=128"`
	CaptchaToken           string `json:"captcha_token"`
	AcceptedTermsVersion   string `json:"accepted_terms_version"`
	AcceptedPrivacyVersion string `json:"accepted_privacy_version"`
}

type LoginInput struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required"`
	CaptchaToken string `json:"captcha_token"`
}

// RegisterPending stores registration data in Redis and returns a registration ID.
// The user is NOT created in the database until email verification succeeds.
func (s *AuthService) RegisterPending(ctx context.Context, input RegisterInput) (string, error) {
	normalizedEmail := normalizeEmail(input.Email)
	lowerUsername := strings.ToLower(input.Username)

	// Check DB: only verified users occupy email/username
	existingByEmail, err := s.userRepo.FindByEmailVerified(normalizedEmail)
	if err != nil {
		return "", err
	}
	if existingByEmail != nil {
		return "", ErrUserAlreadyExists
	}

	existingByUsername, err := s.userRepo.FindByUsernameVerified(lowerUsername)
	if err != nil {
		return "", err
	}
	if existingByUsername != nil {
		return "", ErrUsernameTaken
	}

	// Check Redis: pending registrations also occupy email/username
	if s.redis != nil {
		emailKey := fmt.Sprintf("register:email:%s", sha256Hex(normalizedEmail))
		if exists, _ := s.redis.Exists(ctx, emailKey).Result(); exists == 1 {
			return "", ErrUserAlreadyExists
		}
		usernameKey := fmt.Sprintf("register:username:%s", sha256Hex(lowerUsername))
		if exists, _ := s.redis.Exists(ctx, usernameKey).Result(); exists == 1 {
			return "", ErrUsernameTaken
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	pending := &PendingRegistration{
		Email:                  normalizedEmail,
		Username:               input.Username,
		PasswordHash:           string(hash),
		Reputation:             10,
		Role:                   "user",
		PreferredLocale:        "zh-CN",
		AcceptedTermsVersion:   input.AcceptedTermsVersion,
		AcceptedPrivacyVersion: input.AcceptedPrivacyVersion,
	}
	if input.AcceptedTermsVersion != "" {
		pending.AcceptedTermsAt = time.Now().Unix()
	}
	if input.AcceptedPrivacyVersion != "" {
		pending.AcceptedPrivacyAt = time.Now().Unix()
	}

	// Generate registration ID
	regIDBytes := make([]byte, 16)
	if _, err := rand.Read(regIDBytes); err != nil {
		return "", fmt.Errorf("failed to generate registration ID: %w", err)
	}
	regID := hex.EncodeToString(regIDBytes)

	if err := s.storePendingRegistration(ctx, regID, pending); err != nil {
		return "", err
	}

	return regID, nil
}

// storePendingRegistration stores the pending registration data and email/username mappings in Redis.
func (s *AuthService) storePendingRegistration(ctx context.Context, regID string, pending *PendingRegistration) error {
	if s.redis == nil {
		return fmt.Errorf("redis is required for pending registration")
	}

	ttlSec := s.cfg.Verification.RegisterPendingTTLSec
	if ttlSec <= 0 {
		ttlSec = 86400
	}
	ttl := time.Duration(ttlSec) * time.Second

	data, err := json.Marshal(pending)
	if err != nil {
		return fmt.Errorf("failed to marshal pending registration: %w", err)
	}

	pendingKey := fmt.Sprintf("register:pending:%s", regID)
	emailKey := fmt.Sprintf("register:email:%s", sha256Hex(pending.Email))
	usernameKey := fmt.Sprintf("register:username:%s", sha256Hex(strings.ToLower(pending.Username)))

	pipe := s.redis.Pipeline()
	pipe.Set(ctx, pendingKey, data, ttl)
	pipe.Set(ctx, emailKey, regID, ttl)
	pipe.Set(ctx, usernameKey, regID, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to store pending registration: %w", err)
	}

	return nil
}

// GetPendingRegistration retrieves pending registration data from Redis.
func (s *AuthService) GetPendingRegistration(ctx context.Context, regID string) (*PendingRegistration, error) {
	if s.redis == nil {
		return nil, fmt.Errorf("redis is required for pending registration")
	}

	pendingKey := fmt.Sprintf("register:pending:%s", regID)
	data, err := s.redis.Get(ctx, pendingKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var pending PendingRegistration
	if err := json.Unmarshal(data, &pending); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pending registration: %w", err)
	}

	return &pending, nil
}

// DeletePendingRegistration removes pending registration data and email/username mappings from Redis.
func (s *AuthService) DeletePendingRegistration(ctx context.Context, regID string, pending *PendingRegistration) error {
	if s.redis == nil {
		return nil
	}

	pendingKey := fmt.Sprintf("register:pending:%s", regID)
	emailKey := fmt.Sprintf("register:email:%s", sha256Hex(pending.Email))
	usernameKey := fmt.Sprintf("register:username:%s", sha256Hex(strings.ToLower(pending.Username)))

	return s.redis.Del(ctx, pendingKey, emailKey, usernameKey).Err()
}

// FindPendingByEmail looks up a pending registration by email in Redis.
func (s *AuthService) FindPendingByEmail(ctx context.Context, email string) (string, *PendingRegistration, error) {
	if s.redis == nil {
		return "", nil, nil
	}

	normalizedEmail := normalizeEmail(email)
	emailKey := fmt.Sprintf("register:email:%s", sha256Hex(normalizedEmail))
	regID, err := s.redis.Get(ctx, emailKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil, nil
		}
		return "", nil, err
	}

	pending, err := s.GetPendingRegistration(ctx, regID)
	if err != nil {
		return "", nil, err
	}
	if pending == nil {
		return "", nil, nil
	}

	return regID, pending, nil
}

// CreateUserFromPending creates a verified user in DB from pending registration data.
func (s *AuthService) CreateUserFromPending(pending *PendingRegistration) (*model.User, error) {
	user := &model.User{
		Email:                  pending.Email,
		Username:               pending.Username,
		PasswordHash:           pending.PasswordHash,
		Reputation:             pending.Reputation,
		Role:                   pending.Role,
		PreferredLocale:        pending.PreferredLocale,
		AcceptedTermsVersion:   pending.AcceptedTermsVersion,
		AcceptedPrivacyVersion: pending.AcceptedPrivacyVersion,
	}

	now := time.Now()
	user.EmailVerifiedAt = &now
	if pending.AcceptedTermsAt > 0 {
		t := time.Unix(pending.AcceptedTermsAt, 0)
		user.AcceptedTermsAt = &t
	}
	if pending.AcceptedPrivacyAt > 0 {
		t := time.Unix(pending.AcceptedPrivacyAt, 0)
		user.AcceptedPrivacyAt = &t
	}

	if err := s.userRepo.CreateUser(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	s.ensureDefaultCollectionsForUser(user.ID)

	return user, nil
}

func (s *AuthService) ensureDefaultCollectionsForUser(userID int64) {
	if s.defaultCollectionRepo == nil {
		return
	}
	ctx := context.Background()
	for _, zone := range []string{"original", "fanwork"} {
		if _, err := s.defaultCollectionRepo.EnsureDefaultCollection(ctx, userID, zone); err != nil {
			slog.Error("failed to ensure default collection for new user",
				"user_id", userID,
				"zone", zone,
				"error", err,
			)
		}
	}
}

func (s *AuthService) Login(input LoginInput) (*model.User, *jwtutil.TokenPair, error) {
	normalizedEmail := normalizeEmail(input.Email)
	user, err := s.userRepo.FindByEmail(normalizedEmail)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, ErrInvalidCredentials
	}
	if user.IsBanned {
		return nil, nil, ErrUserBanned
	}
	if user.EmailVerifiedAt == nil {
		return nil, nil, ErrEmailNotVerified
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	tokens, err := s.IssueTokenPairForUser(user)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

func (s *AuthService) IssueTokenPairForUser(user *model.User) (*jwtutil.TokenPair, error) {
	tokens, err := jwtutil.GenerateTokenPair(
		user.ID,
		user.Role,
		s.cfg.JWT.Secret,
		s.cfg.JWT.AccessTokenTTL,
		s.cfg.JWT.RefreshTokenTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}
	if err := s.storeRefreshToken(user.ID, tokens.RefreshToken); err != nil {
		return nil, err
	}

	return tokens, nil
}

func (s *AuthService) Logout(accessToken string) error {
	if s.redis == nil {
		return nil
	}
	claims, err := jwtutil.ParseToken(accessToken, s.cfg.JWT.Secret)
	if err != nil {
		return nil
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil
	}
	ctx := context.Background()
	key := fmt.Sprintf("blacklist:token:%s", accessToken)
	if err := s.redis.Set(ctx, key, "1", ttl).Err(); err != nil {
		return err
	}
	if claims.Subject == "refresh" {
		return s.redis.Del(ctx, buildRefreshTokenKey(int64(claims.UserID), accessToken)).Err()
	}
	return nil
}

func (s *AuthService) IsTokenBlacklisted(accessToken string) bool {
	if s.redis == nil {
		return false
	}
	ctx := context.Background()
	key := fmt.Sprintf("blacklist:token:%s", accessToken)
	val, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return false
	}
	return val == "1"
}

// refreshReplayGraceSeconds：轮换后旧 token 的幂等重放窗口（秒）。并发换发、
// 或硬导航打断在途 refresh 导致浏览器错过 Set-Cookie 的请求，在此窗口内持旧
// token 重放时返回同一新 token 对而不是 401——服务端只发生一次真实轮换，其余
// 复用（#381；Auth0/IdentityServer 同款 rotation grace 语义）。
const refreshReplayGraceSeconds = 60

// luaRotateRefreshToken 原子完成「活跃校验 → 旧键转重放记录 → 写新白名单」，
// 消除检查与轮换之间的 TOCTOU 窗口：
//
//	KEYS[1] = 旧 token 白名单键（活跃="1"；轮换后=重放 JSON，TTL=grace）
//	KEYS[2] = 新 token 白名单键
//	KEYS[3] = user:tokens:<uid> 集合
//	ARGV[1] = 新 token 对 JSON；ARGV[2] = grace 秒；ARGV[3] = 新 token TTL 秒
//
// 返回 1 = 本次完成真实轮换；-1 = 旧键不存在（从未有效或已过 grace）；否则返回
// 存储的重放 JSON（调用方反序列化后原样返回）。
var luaRotateRefreshToken = redis.NewScript(`
local cur = redis.call('GET', KEYS[1])
if not cur then return -1 end
if cur == '1' then
	redis.call('SET', KEYS[1], ARGV[1], 'EX', tonumber(ARGV[2]))
	redis.call('SET', KEYS[2], '1', 'EX', tonumber(ARGV[3]))
	redis.call('SADD', KEYS[3], KEYS[2])
	redis.call('EXPIRE', KEYS[3], tonumber(ARGV[3]))
	return 1
end
return cur
`)

func (s *AuthService) RefreshToken(refreshToken string) (*jwtutil.TokenPair, error) {
	claims, err := jwtutil.ParseToken(refreshToken, s.cfg.JWT.Secret)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	if claims.Subject != "refresh" {
		return nil, ErrTokenInvalid
	}

	if s.IsTokenBlacklisted(refreshToken) {
		return nil, ErrTokenInvalid
	}

	user, err := s.userRepo.FindByID(int64(claims.UserID))
	if err != nil || user == nil {
		return nil, ErrTokenInvalid
	}
	if user.IsBanned {
		return nil, ErrUserBanned
	}

	tokens, err := jwtutil.GenerateTokenPair(
		user.ID,
		user.Role,
		s.cfg.JWT.Secret,
		s.cfg.JWT.AccessTokenTTL,
		s.cfg.JWT.RefreshTokenTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	if s.redis == nil {
		// 无 Redis 的测试/降级路径：无白名单可校验，保持既有放行语义。
		return tokens, nil
	}

	newPairJSON, err := json.Marshal(tokens)
	if err != nil {
		return nil, fmt.Errorf("marshal token pair: %w", err)
	}
	newTTL := time.Duration(s.cfg.JWT.RefreshTokenTTL) * 24 * time.Hour
	if newTTL <= 0 {
		newTTL = 7 * 24 * time.Hour
	}

	res, err := luaRotateRefreshToken.Run(
		context.Background(),
		s.redis,
		[]string{
			buildRefreshTokenKey(user.ID, refreshToken),
			buildRefreshTokenKey(user.ID, tokens.RefreshToken),
			fmt.Sprintf("user:tokens:%d", user.ID),
		},
		string(newPairJSON),
		strconv.Itoa(refreshReplayGraceSeconds),
		strconv.Itoa(int(newTTL/time.Second)),
	).Result()
	if err != nil {
		// Redis 基础设施故障：fail-closed（handler 会话失效），不得放行未校验轮换。
		return nil, fmt.Errorf("rotate refresh token: %w", err)
	}
	switch v := res.(type) {
	case int64:
		if v == 1 {
			return tokens, nil
		}
		return nil, ErrTokenInvalid
	case string:
		var replayed jwtutil.TokenPair
		if err := json.Unmarshal([]byte(v), &replayed); err != nil || replayed.RefreshToken == "" {
			return nil, ErrTokenInvalid
		}
		return &replayed, nil
	default:
		return nil, ErrTokenInvalid
	}
}

func (s *AuthService) storeRefreshToken(userID int64, refreshToken string) error {
	if s.redis == nil {
		return nil
	}
	ttl := time.Duration(s.cfg.JWT.RefreshTokenTTL) * 24 * time.Hour
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	ctx := context.Background()
	key := buildRefreshTokenKey(userID, refreshToken)
	if err := s.redis.Set(ctx, key, "1", ttl).Err(); err != nil {
		return err
	}
	tokenSetKey := fmt.Sprintf("user:tokens:%d", userID)
	s.redis.SAdd(ctx, tokenSetKey, key)
	s.redis.Expire(ctx, tokenSetKey, ttl)
	return nil
}

func (s *AuthService) ChangePassword(userID int64, newPassword string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	return s.userRepo.UpdatePassword(userID, string(hashed))
}

func buildRefreshTokenKey(userID int64, refreshToken string) string {
	sum := sha256.Sum256([]byte(refreshToken))
	return fmt.Sprintf("refresh_token:%d:%s", userID, hex.EncodeToString(sum[:]))
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
