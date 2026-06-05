package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"omnicraft/backend/config"
)

func setupInteractionTestEnv(t *testing.T) (*gin.Engine, *gorm.DB, *config.Config) {
	gin.SetMode(gin.TestMode)
	cfg := makeTestConfig()
	db := setupTestDB(t)
	r := gin.New()
	return r, db, cfg
}

func TestInteractionRequiredRejectsUnverifiedEmail(t *testing.T) {
	_, db, cfg := setupInteractionTestEnv(t)
	insertTestUser(db, 1, "user", false, false, 10, nil)

	policy := InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	}

	r := setupTestRouter()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, int64(1))
		c.Next()
	})
	r.Use(InteractionRequired(cfg, db, nil, policy))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 403, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "EMAIL_NOT_VERIFIED", body["code"])
}

func TestInteractionRequiredRejectsLowReputation(t *testing.T) {
	_, db, cfg := setupInteractionTestEnv(t)
	insertTestUser(db, 1, "user", false, true, 2, nil)

	policy := InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	}

	r := setupTestRouter()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, int64(1))
		c.Next()
	})
	r.Use(InteractionRequired(cfg, db, nil, policy))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 403, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "INSUFFICIENT_REPUTATION", body["code"])
}

func TestInteractionRequiredRejectsPublishFreezeWhenPolicyRequiresIt(t *testing.T) {
	_, db, cfg := setupInteractionTestEnv(t)
	insertTestUser(db, 1, "user", false, true, 10, nil)
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()
	mr.Set("user:publish_freeze:1", "1")
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	policy := InteractionPolicy{
		RequireVerifiedEmail:   true,
		RequireReputation:      true,
		RequireNoPublishFreeze: true,
	}

	r := setupTestRouter()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, int64(1))
		c.Next()
	})
	r.Use(InteractionRequired(cfg, db, rdb, policy))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 403, w.Code)
}

func TestInteractionRequiredFailsClosedWhenPublishFreezeStatusUnavailable(t *testing.T) {
	_, db, cfg := setupInteractionTestEnv(t)
	insertTestUser(db, 1, "user", false, true, 10, nil)

	policy := InteractionPolicy{
		RequireVerifiedEmail:   true,
		RequireReputation:      true,
		RequireNoPublishFreeze: true,
	}

	r := setupTestRouter()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, int64(1))
		c.Next()
	})
	r.Use(InteractionRequired(cfg, db, nil, policy))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 503, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "AUTH_STATUS_UNAVAILABLE", body["code"])
}

func TestInteractionRequiredAllowsValidUser(t *testing.T) {
	_, db, cfg := setupInteractionTestEnv(t)
	insertTestUser(db, 1, "user", false, true, 10, nil)

	policy := InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	}

	r := setupTestRouter()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, int64(1))
		c.Next()
	})
	r.Use(InteractionRequired(cfg, db, nil, policy))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestInteractionRequiredFailsOnConfigError(t *testing.T) {
	_, db, _ := setupInteractionTestEnv(t)
	insertTestUser(db, 1, "user", false, true, 10, nil)
	badCfg := makeTestConfig()
	badCfg.Reputation.MinScoreForInteraction = 0

	policy := InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	}

	r := setupTestRouter()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, int64(1))
		c.Next()
	})
	r.Use(InteractionRequired(badCfg, db, nil, policy))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 503, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "CONFIG_ERROR", body["code"])
}

func TestInteractionRequiredRejectsBannedUser(t *testing.T) {
	_, db, cfg := setupInteractionTestEnv(t)
	insertTestUser(db, 1, "user", true, true, 10, nil)

	policy := InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	}

	r := setupTestRouter()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, int64(1))
		c.Next()
	})
	r.Use(InteractionRequired(cfg, db, nil, policy))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestInteractionRequiredRejectsSoftDeletedUser(t *testing.T) {
	_, db, cfg := setupInteractionTestEnv(t)
	deletedAt := time.Now()
	insertTestUser(db, 1, "user", false, true, 10, &deletedAt)

	policy := InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	}

	r := setupTestRouter()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, int64(1))
		c.Next()
	})
	r.Use(InteractionRequired(cfg, db, nil, policy))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestInteractionRequiredRejectsUnauthenticated(t *testing.T) {
	_, db, cfg := setupInteractionTestEnv(t)

	policy := InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	}

	r := setupTestRouter()
	r.Use(InteractionRequired(cfg, db, nil, policy))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestInteractionRequiredZeroReputationIsNotSentinel(t *testing.T) {
	_, db, _ := setupInteractionTestEnv(t)
	insertTestUser(db, 1, "user", false, true, 0, nil)

	badCfg := makeTestConfig()
	badCfg.Reputation.MinScoreForInteraction = -5

	policy := InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	}

	r := setupTestRouter()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, int64(1))
		c.Next()
	})
	r.Use(InteractionRequired(badCfg, db, nil, policy))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 503, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "CONFIG_ERROR", body["code"])
}

func TestInteractionPolicyTableDriven(t *testing.T) {
	_, db, cfg := setupInteractionTestEnv(t)
	insertTestUser(db, 1, "user", false, true, 10, nil)
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	tests := []struct {
		name     string
		policy   InteractionPolicy
		expectOK bool
	}{
		{
			name: "full policy with valid user",
			policy: InteractionPolicy{
				RequireVerifiedEmail:   true,
				RequireReputation:      true,
				RequireNoPublishFreeze: true,
			},
			expectOK: true,
		},
		{
			name: "no requirements",
			policy: InteractionPolicy{
				RequireVerifiedEmail:   false,
				RequireReputation:      false,
				RequireNoPublishFreeze: false,
			},
			expectOK: true,
		},
		{
			name: "reputation only with valid user",
			policy: InteractionPolicy{
				RequireVerifiedEmail:   false,
				RequireReputation:      true,
				RequireNoPublishFreeze: false,
			},
			expectOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupTestRouter()
			r.Use(func(c *gin.Context) {
				c.Set(UserIDKey, int64(1))
				c.Next()
			})
			r.Use(InteractionRequired(cfg, db, rdb, tt.policy))
			r.POST("/test", func(c *gin.Context) {
				c.JSON(200, gin.H{"ok": true})
			})

			req := httptest.NewRequest("POST", "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if tt.expectOK {
				assert.Equal(t, 200, w.Code)
			} else {
				assert.NotEqual(t, 200, w.Code)
			}
		})
	}
}
