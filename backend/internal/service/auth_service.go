package service

import (
	"context"
	"errors"
	"fmt"
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
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserBanned         = errors.New("user is banned")
	ErrTokenInvalid       = errors.New("token is invalid")
)

type AuthService struct {
	userRepo *repository.UserRepository
	redis    *redis.Client
	cfg      *config.Config
}

func NewAuthService(userRepo *repository.UserRepository, rdb *redis.Client, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		redis:    rdb,
		cfg:      cfg,
	}
}

type RegisterInput struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=2,max=64"`
	Password string `json:"password" binding:"required,min=6,max=128"`
}

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (s *AuthService) Register(input RegisterInput) (*model.User, *jwtutil.TokenPair, error) {
	existingByEmail, err := s.userRepo.FindByEmail(input.Email)
	if err != nil {
		return nil, nil, err
	}
	if existingByEmail != nil {
		return nil, nil, ErrUserAlreadyExists
	}

	existingByUsername, err := s.userRepo.FindByUsername(input.Username)
	if err != nil {
		return nil, nil, err
	}
	if existingByUsername != nil {
		return nil, nil, errors.New("username already taken")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &model.User{
		Email:           input.Email,
		Username:        input.Username,
		PasswordHash:    string(hash),
		Reputation:      10,
		Role:            "user",
		PreferredLocale: "zh-CN",
	}

	if err := s.userRepo.CreateUser(user); err != nil {
		return nil, nil, fmt.Errorf("failed to create user: %w", err)
	}

	tokens, err := jwtutil.GenerateTokenPair(
		user.ID,
		user.Role,
		s.cfg.JWT.Secret,
		s.cfg.JWT.AccessTokenTTL,
		s.cfg.JWT.RefreshTokenTTL,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	return user, tokens, nil
}

func (s *AuthService) Login(input LoginInput) (*model.User, *jwtutil.TokenPair, error) {
	user, err := s.userRepo.FindByEmail(input.Email)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, ErrInvalidCredentials
	}
	if user.IsBanned {
		return nil, nil, ErrUserBanned
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	tokens, err := jwtutil.GenerateTokenPair(
		user.ID,
		user.Role,
		s.cfg.JWT.Secret,
		s.cfg.JWT.AccessTokenTTL,
		s.cfg.JWT.RefreshTokenTTL,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	return user, tokens, nil
}

func (s *AuthService) Logout(accessToken string) error {
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
	return s.redis.Set(ctx, key, "1", ttl).Err()
}

func (s *AuthService) IsTokenBlacklisted(accessToken string) bool {
	ctx := context.Background()
	key := fmt.Sprintf("blacklist:token:%s", accessToken)
	val, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return false
	}
	return val == "1"
}

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

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl > 0 {
		ctx := context.Background()
		key := fmt.Sprintf("blacklist:token:%s", refreshToken)
		s.redis.Set(ctx, key, "1", ttl)
	}

	return tokens, nil
}
