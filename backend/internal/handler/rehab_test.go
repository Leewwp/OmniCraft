package handler

import (
	"encoding/json"
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
)

func TestCompleteCourseReturnsRecoveredReputation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.RehabCourse{}, &model.RehabCompletion{}, &model.ReputationLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	user := model.User{Email: "rehab-handler@test.com", Username: "rehab-handler", PasswordHash: "hash", Reputation: 2, Role: "user", EmailVerifiedAt: &now}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	course := model.RehabCourse{ViolationType: "handler", MinReadingSec: 0, RewardPoints: 1}
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
	cfg := &config.Config{Reputation: config.ReputationConfig{MinScoreForInteraction: 3}, Cache: config.CacheConfig{UserStatusTTL: 300}}
	handler := NewRehabHandler(db, rdb, cfg)

	r := gin.New()
	r.POST("/rehab/courses/:id/complete", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, user.ID)
		handler.CompleteCourse(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/rehab/courses/"+strconv.FormatInt(course.ID, 10)+"/complete", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("complete course: got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["reputation"] != float64(3) {
		t.Fatalf("response reputation=%v body=%s", body["reputation"], w.Body.String())
	}
}

func TestCompleteCourseRetryReturnsCommittedResultAfterCacheRecovers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.RehabCourse{}, &model.RehabCompletion{}, &model.ReputationLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	user := model.User{Email: "rehab-retry@test.com", Username: "rehab-retry", PasswordHash: "hash", Reputation: 2, Role: "user", EmailVerifiedAt: &now}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	course := model.RehabCourse{ViolationType: "retry", MinReadingSec: 0, RewardPoints: 1}
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
	handler := NewRehabHandler(db, rdb, &config.Config{Cache: config.CacheConfig{UserStatusTTL: 300}})
	r := gin.New()
	r.POST("/rehab/courses/:id/complete", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, user.ID)
		handler.CompleteCourse(c)
	})
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/rehab/courses/"+strconv.FormatInt(course.ID, 10)+"/complete", nil))
		return w
	}

	mr.Close()
	first := request()
	if first.Code != http.StatusServiceUnavailable {
		t.Fatalf("first attempt with cache unavailable: got %d body=%s", first.Code, first.Body.String())
	}
	if err := mr.Restart(); err != nil {
		t.Fatalf("restart miniredis: %v", err)
	}

	retry := request()
	if retry.Code != http.StatusOK {
		t.Fatalf("retry after cache recovery: got %d body=%s", retry.Code, retry.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(retry.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode retry response: %v", err)
	}
	if body["reputation"] != float64(3) {
		t.Fatalf("retry reputation=%v body=%s", body["reputation"], retry.Body.String())
	}
	var stored model.User
	if err := db.First(&stored, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.Reputation != 3 {
		t.Fatalf("retry applied the reward more than once: reputation=%d", stored.Reputation)
	}
}
