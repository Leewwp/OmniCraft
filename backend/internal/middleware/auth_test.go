package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL,
			avatar_url TEXT,
			bio TEXT,
			reputation INTEGER NOT NULL DEFAULT 10,
			preferred_locale VARCHAR(10) NOT NULL DEFAULT 'zh-CN',
			role VARCHAR(20) NOT NULL DEFAULT 'user',
			is_banned BOOLEAN NOT NULL DEFAULT FALSE,
			ban_reason TEXT,
			email_verified_at DATETIME,
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error)
	return db
}

func insertTestUser(db *gorm.DB, id int64, role string, isBanned bool, emailVerified bool, reputation int, deletedAt *time.Time) {
	var verifiedAt interface{}
	if emailVerified {
		now := time.Now()
		verifiedAt = now.Format("2006-01-02 15:04:05")
	} else {
		verifiedAt = nil
	}
	db.Exec("INSERT INTO users (id, email, password_hash, username, role, is_banned, reputation, email_verified_at, deleted_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, "testuser@test.com", "hash", "testuser", role, isBanned, reputation, verifiedAt, deletedAt)
}

func makeTestToken(cfg *config.Config, userID int64, role string) string {
	pair, _ := jwtutil.GenerateTokenPair(userID, role, cfg.JWT.Secret, 120, 7)
	return pair.AccessToken
}

func makeTestConfig() *config.Config {
	return &config.Config{
		JWT: config.JWTConfig{
			Secret:          "test-secret-key-for-testing-only",
			AccessTokenTTL:  7200,
			RefreshTokenTTL: 604800,
		},
		Reputation: config.ReputationConfig{
			MinScoreForInteraction: 3,
		},
		Cache: config.CacheConfig{
			UserStatusTTL:    300,
			PublishFreezeTTL: 604800,
		},
	}
}

func TestAuthRequiredRejectsWhenRedisAndDBCannotConfirmStatus(t *testing.T) {
	cfg := makeTestConfig()
	token := makeTestToken(cfg, 1, "user")

	r := setupTestRouter()
	r.Use(AuthRequired(cfg, nil))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 503, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "AUTH_STATUS_UNAVAILABLE", body["code"])
}

func TestAuthRequiredFailsClosedWhenRedisBlacklistCheckFails(t *testing.T) {
	cfg := makeTestConfig()
	db := setupTestDB(t)
	insertTestUser(db, 1, "user", false, true, 10, nil)
	token := makeTestToken(cfg, 1, "user")
	rdb := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 10 * time.Millisecond,
		ReadTimeout: 10 * time.Millisecond,
	})
	t.Cleanup(func() { _ = rdb.Close() })

	r := setupTestRouter()
	r.Use(AuthRequired(cfg, rdb, db))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 503, w.Code)
	assert.Contains(t, w.Body.String(), "AUTH_STATUS_UNAVAILABLE")
}

func TestAuthRequiredUsesFreshDBRoleInsteadOfJWTClaim(t *testing.T) {
	cfg := makeTestConfig()
	db := setupTestDB(t)
	insertTestUser(db, 1, "admin", false, true, 10, nil)

	token := makeTestToken(cfg, 1, "user")

	r := setupTestRouter()
	r.Use(AuthRequired(cfg, nil, db))
	r.GET("/test", func(c *gin.Context) {
		role, _ := c.Get(UserRoleKey)
		c.JSON(200, gin.H{"role": role})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "admin", body["role"])
}

func TestAuthRequiredRejectsMissingOrSoftDeletedUser(t *testing.T) {
	cfg := makeTestConfig()
	db := setupTestDB(t)

	deletedAt := time.Now()
	insertTestUser(db, 1, "user", false, true, 10, &deletedAt)

	token := makeTestToken(cfg, 1, "user")

	r := setupTestRouter()
	r.Use(AuthRequired(cfg, nil, db))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestAuthRequiredRejectsBannedUser(t *testing.T) {
	cfg := makeTestConfig()
	db := setupTestDB(t)
	insertTestUser(db, 1, "user", true, true, 10, nil)

	token := makeTestToken(cfg, 1, "user")

	r := setupTestRouter()
	r.Use(AuthRequired(cfg, nil, db))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "USER_BANNED", body["code"])
}

func TestOptionalAuthDowngradesToAnonymousWhenStatusCannotBeConfirmed(t *testing.T) {
	cfg := makeTestConfig()
	token := makeTestToken(cfg, 1, "admin")

	r := setupTestRouter()
	r.Use(OptionalAuth(cfg, nil))
	r.GET("/test", func(c *gin.Context) {
		userID := GetUserID(c)
		role, _ := c.Get(UserRoleKey)
		c.JSON(200, gin.H{"user_id": userID, "role": role})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, float64(0), body["user_id"])
}

func TestOptionalAuthConfirmsStatusFromDB(t *testing.T) {
	cfg := makeTestConfig()
	db := setupTestDB(t)
	insertTestUser(db, 1, "admin", false, true, 10, nil)

	token := makeTestToken(cfg, 1, "user")

	r := setupTestRouter()
	r.Use(OptionalAuth(cfg, nil, db))
	r.GET("/test", func(c *gin.Context) {
		userID := GetUserID(c)
		role, _ := c.Get(UserRoleKey)
		c.JSON(200, gin.H{"user_id": userID, "role": role})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, float64(1), body["user_id"])
	assert.Equal(t, "admin", body["role"])
}
