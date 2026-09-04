package handler

// T33（FIX-37）①：处罚日志（reputation）只允许本人与 admin 查看——
// ai_violation 等处罚 reason 不对他人外泄。他人 → 403 FORBIDDEN。

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	jwtutil "omnicraft/backend/internal/pkg/jwt"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
)

func setupT33ReputationRouter(t *testing.T) (*gin.Engine, *gorm.DB, *config.Config, *miniredis.Miniredis) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.Follow{}, &model.ReputationLog{}))

	cfg := &config.Config{
		Server:     config.ServerConfig{Mode: "debug"},
		JWT:        config.JWTConfig{Secret: "t33-secret", AccessTokenTTL: 120, RefreshTokenTTL: 7},
		Reputation: config.ReputationConfig{MinScoreForInteraction: 3},
		Cache:      config.CacheConfig{UserStatusTTL: 300},
	}
	userHandler := NewUserHandler(db, nil, rdb, cfg)

	r := gin.New()
	authReq := middleware.AuthRequired(cfg, rdb, db)
	r.GET("/api/v1/users/:id/reputation", authReq, userHandler.GetReputation)
	return r, db, cfg, mr
}

func t33SeedReputationUser(t *testing.T, db *gorm.DB, id int64, role string) {
	t.Helper()
	user := model.User{
		Email:        "t33-" + role + "-" + strconv.FormatInt(id, 10) + "@example.test",
		Username:     "t33-" + role + "-" + strconv.FormatInt(id, 10),
		PasswordHash: "hash",
		Reputation:   10,
		Role:         role,
	}
	user.ID = id
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&model.ReputationLog{UserID: id, Delta: -2, Reason: "ai_violation"}).Error)
}

func t33TokenFor(t *testing.T, cfg *config.Config, userID int64) string {
	t.Helper()
	pair, err := jwtutil.GenerateTokenPair(userID, "user", cfg.JWT.Secret, 120, 7)
	require.NoError(t, err)
	return pair.AccessToken
}

func t33GetReputation(r *gin.Engine, token, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGetReputationRequiresSelfOrAdmin(t *testing.T) {
	r, db, cfg, mr := setupT33ReputationRouter(t)
	defer mr.Close()
	t33SeedReputationUser(t, db, 1, "user")
	t33SeedReputationUser(t, db, 2, "user")
	t33SeedReputationUser(t, db, 3, "admin")

	aliceToken := t33TokenFor(t, cfg, 1)
	adminToken := t33TokenFor(t, cfg, 3)

	// 他人查询 → 403（处罚 reason 不外泄）
	wOther := t33GetReputation(r, t33TokenFor(t, cfg, 2), "/api/v1/users/1/reputation")
	require.Equal(t, http.StatusForbidden, wOther.Code, wOther.Body.String())
	require.Contains(t, wOther.Body.String(), "FORBIDDEN")

	// 本人 → 200
	wSelf := t33GetReputation(r, aliceToken, "/api/v1/users/1/reputation")
	require.Equal(t, http.StatusOK, wSelf.Code, wSelf.Body.String())
	require.Contains(t, wSelf.Body.String(), "ai_violation")

	// admin → 200
	wAdmin := t33GetReputation(r, adminToken, "/api/v1/users/1/reputation")
	require.Equal(t, http.StatusOK, wAdmin.Code, wAdmin.Body.String())
}
