package handler

// T30（FIX-20）注销账号改软删除：
// ① DeleteAccount 改 deleted_at 软删除 + 匿名化清写，不再伪装封禁（is_banned 不设）
// ② 注销后 /auth/me 走 deleted 语义 401（"user not found or deleted"），非 USER_BANNED
// ③ 匿名化后原邮箱在库中不存在——密码重置走「不存在」防枚举路径
// ④ 迁移 076：存量 ban_reason='self_deleted' 用户幂等修正（置 deleted_at + 清 is_banned）

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"omnicraft/backend/internal/testutil"
)

func setupT30DeleteRouter(t *testing.T) (*gin.Engine, *gin.Engine, *gorm.DB, *config.Config, *miniredis.Miniredis) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.Follow{}))

	cfg := &config.Config{
		Server:     config.ServerConfig{Mode: "debug"},
		JWT:        config.JWTConfig{Secret: "t30-delete-secret", AccessTokenTTL: 120, RefreshTokenTTL: 7},
		Reputation: config.ReputationConfig{MinScoreForInteraction: 3},
		Cache:      config.CacheConfig{UserStatusTTL: 300},
	}
	userRepo := repository.NewUserRepository(db)
	authSvc := service.NewAuthService(userRepo, rdb, cfg)
	verificationSvc := service.NewVerificationService(userRepo, rdb, noopMailSender{}, cfg)
	userHandler := NewUserHandler(db, authSvc, rdb, cfg)
	authHandler := NewAuthHandler(authSvc, verificationSvc, userRepo, nil, rdb, cfg)

	authReq := middleware.AuthRequired(cfg, rdb, db)
	userRouter := gin.New()
	userRouter.DELETE("/api/v1/users/me", authReq, userHandler.DeleteAccount)
	authRouter := gin.New()
	authRouter.GET("/api/v1/auth/me", authReq, authHandler.Me)
	return userRouter, authRouter, db, cfg, mr
}

func t30SeedUser(t *testing.T, db *gorm.DB) model.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("T30Password#2026"), bcrypt.MinCost)
	require.NoError(t, err)
	verifiedAt := time.Now()
	user := model.User{
		Email:           "t30-leaver@seed.omnicraft.local",
		Username:        "t30-leaver",
		PasswordHash:    string(hash),
		Reputation:      10,
		Role:            "user",
		EmailVerifiedAt: &verifiedAt,
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func t30TokenFor(cfg *config.Config, userID int64) string {
	pair, err := jwtutil.GenerateTokenPair(userID, "user", cfg.JWT.Secret, 120, 7)
	require.NoError(nil, err)
	return pair.AccessToken
}

func t30DeleteAccount(t *testing.T, userRouter *gin.Engine, token, password string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/me",
		strings.NewReader(`{"password":"`+password+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	userRouter.ServeHTTP(w, req)
	return w
}

// ① 注销 = 软删除 + 匿名化，不再伪装封禁。
func TestDeleteAccountSoftDeletesInsteadOfBan(t *testing.T) {
	userRouter, _, db, cfg, mr := setupT30DeleteRouter(t)
	defer mr.Close()
	user := t30SeedUser(t, db)

	w := t30DeleteAccount(t, userRouter, t30TokenFor(cfg, int64(user.ID)), "T30Password#2026")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var after model.User
	require.NoError(t, db.First(&after, user.ID).Error)
	require.NotNil(t, after.DeletedAt, "注销必须写 deleted_at 软删除")
	require.False(t, after.IsBanned, "注销不得再伪装成封禁（F-014）")
	require.Empty(t, after.BanReason)
	require.Contains(t, after.Username, "已注销用户", "username 匿名化")
	require.Equal(t, "deleted_"+strconv.FormatInt(int64(user.ID), 10)+"@anon.local", after.Email, "email 匿名化防 PII 残留")
	require.Empty(t, after.AvatarURL)
	require.Empty(t, after.Bio)
}

// ② 注销后 /auth/me 走 deleted 语义 401，不是 USER_BANNED。
func TestMeAfterDeleteReturnsDeletedSemantics(t *testing.T) {
	userRouter, authRouter, db, cfg, mr := setupT30DeleteRouter(t)
	defer mr.Close()
	user := t30SeedUser(t, db)
	token := t30TokenFor(cfg, int64(user.ID))

	w := t30DeleteAccount(t, userRouter, token, "T30Password#2026")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	middleware.InvalidateUserStatusCache(nil, int64(user.ID))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	authRouter.ServeHTTP(w2, req)
	require.Equal(t, http.StatusUnauthorized, w2.Code, w2.Body.String())
	require.Contains(t, w2.Body.String(), "user not found or deleted", "deleted 语义（middleware 现成分支）")
	require.NotContains(t, w2.Body.String(), "USER_BANNED")
}

// ③ 匿名化后原邮箱在库中不存在 → 密码重置自然走「不存在」防枚举路径。
func TestPasswordResetForDeletedUserGoesMissingPath(t *testing.T) {
	userRouter, _, db, cfg, mr := setupT30DeleteRouter(t)
	defer mr.Close()
	user := t30SeedUser(t, db)

	w := t30DeleteAccount(t, userRouter, t30TokenFor(cfg, int64(user.ID)), "T30Password#2026")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	userRepo := repository.NewUserRepository(db)
	found, err := userRepo.FindByEmail("t30-leaver@seed.omnicraft.local")
	require.NoError(t, err)
	require.Nil(t, found, "原邮箱已匿名化，重置流程对原邮箱走不存在路径")
}

// ④ 迁移 076 幂等：存量 self_deleted 用户置 deleted_at + 清 is_banned；其他封禁不动。
func TestMigration076SelfDeletedBackfillIdempotent(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)

	require.NoError(t, db.Exec(`CREATE TABLE users (
		id bigserial PRIMARY KEY,
		email text NOT NULL,
		password_hash text NOT NULL,
		username text NOT NULL,
		avatar_url text,
		bio text,
		reputation integer NOT NULL DEFAULT 10,
		preferred_locale text NOT NULL DEFAULT 'zh-CN',
		role text NOT NULL DEFAULT 'user',
		is_banned boolean NOT NULL DEFAULT FALSE,
		ban_reason text,
		email_verified_at timestamptz,
		deleted_at timestamptz,
		created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error)
	t.Cleanup(func() {
		_ = db.Exec(`DROP TABLE IF EXISTS users`).Error
	})

	require.NoError(t, db.Exec(`INSERT INTO users (email, password_hash, username, is_banned, ban_reason) VALUES
		('legacy1@test.com','h','legacy1',TRUE,'self_deleted'),
		('legacy2@test.com','h','legacy2',TRUE,'self_deleted'),
		('realban@test.com','h','realban',TRUE,'spam')`).Error)

	testutil.ApplyMigrationFile(t, db, "../../migrations/076_self_deleted_soft_delete.sql")

	var legacy1Deleted *time.Time
	var legacy1Banned bool
	require.NoError(t, db.Raw(`SELECT deleted_at, is_banned FROM users WHERE email='legacy1@test.com'`).Row().Scan(&legacy1Deleted, &legacy1Banned))
	require.NotNil(t, legacy1Deleted, "存量 self_deleted 必须回填 deleted_at")
	require.False(t, legacy1Banned, "存量 self_deleted 必须清除伪装封禁")

	var realBanReason string
	var realBanned bool
	require.NoError(t, db.Raw(`SELECT ban_reason, is_banned FROM users WHERE email='realban@test.com'`).Row().Scan(&realBanReason, &realBanned))
	require.True(t, realBanned)
	require.Equal(t, "spam", realBanReason, "真实封禁不受迁移影响")

	// 幂等：重复应用不报错、不产生变化。
	testutil.ApplyMigrationFile(t, db, "../../migrations/076_self_deleted_soft_delete.sql")
	var legacy2Deleted *time.Time
	require.NoError(t, db.Raw(`SELECT deleted_at FROM users WHERE email='legacy2@test.com'`).Row().Scan(&legacy2Deleted))
	require.NotNil(t, legacy2Deleted)
}
