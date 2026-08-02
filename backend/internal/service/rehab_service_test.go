package service_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/service"
)

func TestCompleteCourseImmediatelyUnlocksCapabilityAndInteractionGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.RehabCourse{}, &model.RehabCompletion{}, &model.ReputationLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now()
	user := model.User{
		Email:           "rehab-unlock@test.com",
		Username:        "rehab-unlock",
		PasswordHash:    "hash",
		Reputation:      2,
		Role:            "user",
		EmailVerifiedAt: &now,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	course := model.RehabCourse{ViolationType: "interaction", MinReadingSec: 0, RewardPoints: 1}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("create course: %v", err)
	}
	startedAt := now.Add(-time.Minute)
	if err := db.Create(&model.RehabCompletion{UserID: user.ID, CourseID: course.ID, StartedAt: &startedAt}).Error; err != nil {
		t.Fatalf("create completion: %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	cfg := &config.Config{
		Reputation: config.ReputationConfig{MinScoreForInteraction: 3},
		Cache:      config.CacheConfig{UserStatusTTL: 300},
	}
	cache := service.NewRuntimeStatusCache(rdb, cfg)
	cache.Set(user.ID, &service.RuntimeUserStatus{
		ID:              user.ID,
		Role:            user.Role,
		EmailVerifiedAt: user.EmailVerifiedAt,
		Reputation:      2,
	})
	if !mr.Exists("user:status:" + strconv.FormatInt(user.ID, 10)) {
		t.Fatal("expected stale runtime status fixture")
	}

	rehab := service.NewRehabService(db, cache)
	reputation, err := rehab.CompleteCourse(user.ID, course.ID)
	if err != nil {
		t.Fatalf("complete course: %v", err)
	}
	if reputation != 3 {
		t.Fatalf("recovered reputation=%d, want 3", reputation)
	}
	if mr.Exists("user:status:" + strconv.FormatInt(user.ID, 10)) {
		t.Fatal("successful reputation recovery must invalidate runtime status cache")
	}

	status, err := service.ResolveRuntimeUserStatus(t.Context(), db, cache, user.ID)
	if err != nil {
		t.Fatalf("resolve recovered status: %v", err)
	}
	decision := service.EvaluateInteractionAccess(status, cfg, true, true)
	if !decision.Allowed {
		t.Fatalf("recovered capability remained locked: %+v", decision)
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.UserIDKey, user.ID)
		c.Next()
	})
	r.Use(middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	}))
	r.POST("/interact", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/interact", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("recovered action gate remained locked: %d %s", w.Code, w.Body.String())
	}
}

func TestCompleteCourseDoesNotReportSuccessWhenCacheInvalidationFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.RehabCourse{}, &model.RehabCompletion{}, &model.ReputationLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	user := model.User{Email: "rehab-cache-failure@test.com", Username: "rehab-cache-failure", PasswordHash: "hash", Reputation: 2, Role: "user", EmailVerifiedAt: &now}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	course := model.RehabCourse{ViolationType: "cache-failure", MinReadingSec: 0, RewardPoints: 1}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("create course: %v", err)
	}
	startedAt := now.Add(-time.Minute)
	if err := db.Create(&model.RehabCompletion{UserID: user.ID, CourseID: course.ID, StartedAt: &startedAt}).Error; err != nil {
		t.Fatalf("create completion: %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	cache := service.NewRuntimeStatusCache(rdb, &config.Config{Cache: config.CacheConfig{UserStatusTTL: 300}})
	mr.Close()

	if _, err := service.NewRehabService(db, cache).CompleteCourse(user.ID, course.ID); err == nil {
		t.Fatal("cache invalidation failure must not be reported as successful recovery")
	}
}
