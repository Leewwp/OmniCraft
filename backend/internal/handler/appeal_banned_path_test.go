package handler

// T29（FIX-15）：封禁用户申诉出路——account 目标申诉端到端。
// ① banned 用户 POST /appeals（target_type=account，target_id 免填/强制为本人）
// ② admin ResolveAppeal account approved → is_banned=false + ban_reason='' +
//    用户状态缓存失效 + 审计（与 appeal 解析同事务）。

import (
	"encoding/json"
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
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func setupAppealBannedTestRouter(t *testing.T) (*gin.Engine, *gin.Engine, *gorm.DB, *redis.Client, *miniredis.Miniredis) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Appeal{}, &model.AdminAuditLog{}, &model.OutboxEvent{},
	))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	previous := redisclient.Client
	redisclient.Client = rdb
	t.Cleanup(func() { redisclient.Client = previous })

	cfg := &config.Config{}
	cfg.JWT.Secret = "t29-appeal-secret"
	cfg.Cache.UserStatusTTL = 300

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	adminHandler := NewAdminHandler(db, cfg, rdb, auditSvc)

	appealRouter := gin.New()
	appealReq := middleware.AuthRequired(cfg, rdb, db)
	v1 := appealRouter.Group("/api/v1")
	v1.POST("/appeals", appealReq, (&AppealHandler{appealRepo: repository.NewAppealRepository(db), contentRepo: repository.NewContentRepository(db)}).SubmitAppeal)
	v1.GET("/appeals/me", appealReq, (&AppealHandler{appealRepo: repository.NewAppealRepository(db), contentRepo: repository.NewContentRepository(db)}).GetMyAppeals)

	adminRouter := gin.New()
	adminRouter.POST("/admin/appeals/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		c.Set(middleware.UserRoleKey, "admin")
		c.Set("trace_id", "trace-t29-test")
		adminHandler.ResolveAppeal(c)
	})

	return appealRouter, adminRouter, db, rdb, mr
}

func seedT29BannedUser(t *testing.T, db *gorm.DB, id int64) model.User {
	t.Helper()
	user := model.User{
		Email:           "banned-appeal@test.com",
		Username:        "banned-appealer",
		PasswordHash:    "hash",
		Reputation:      10,
		Role:            "user",
		IsBanned:        true,
		BanReason:       "spam",
		EmailVerifiedAt: ptrTimeT29(time.Now()),
	}
	user.ID = id
	require.NoError(t, db.Create(&user).Error)
	return user
}

func ptrTimeT29(t time.Time) *time.Time { return &t }

// 封禁用户提交 account 申诉：target_id 免填，服务端强制为本人 callerID。
func TestBannedUserSubmitAccountAppealForcesSelfTarget(t *testing.T) {
	appealRouter, _, db, _, mr := setupAppealBannedTestRouter(t)
	defer mr.Close()
	user := seedT29BannedUser(t, db, 2)

	cfg := &config.Config{}
	cfg.JWT.Secret = "t29-appeal-secret"
	pair, err := jwtutil.GenerateTokenPair(int64(user.ID), "user", cfg.JWT.Secret, 120, 7)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/appeals",
		strings.NewReader(`{"target_type":"account","reason":"误封申诉，请复核"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	w := httptest.NewRecorder()
	appealRouter.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var body struct {
		Appeal model.Appeal `json:"appeal"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "account", body.Appeal.TargetType)
	require.Equal(t, int64(2), body.Appeal.TargetID, "account 申诉 target_id 必须强制为本人")
	require.Equal(t, int64(2), body.Appeal.UserID)

	// content/comment 仍要求 target_id>0。
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/appeals",
		strings.NewReader(`{"target_type":"content","reason":"no target"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	w2 := httptest.NewRecorder()
	appealRouter.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusBadRequest, w2.Code, w2.Body.String())
}

// admin 批准 account 申诉 → 解封 + 清 ban_reason + 状态缓存失效 + 审计。
func TestResolveAccountAppealApprovedUnbansUser(t *testing.T) {
	appealRouter, adminRouter, db, rdb, mr := setupAppealBannedTestRouter(t)
	defer mr.Close()
	user := seedT29BannedUser(t, db, 2)

	cfg := &config.Config{}
	cfg.JWT.Secret = "t29-appeal-secret"
	pair, err := jwtutil.GenerateTokenPair(int64(user.ID), "user", cfg.JWT.Secret, 120, 7)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/appeals",
		strings.NewReader(`{"target_type":"account","reason":"误封申诉"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	w := httptest.NewRecorder()
	appealRouter.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var created struct {
		Appeal model.Appeal `json:"appeal"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	appealID := created.Appeal.ID

	// 预热状态缓存，批准后必须失效（解封即时生效）。
	cache := service.NewRuntimeStatusCache(rdb, cfg)
	cache.Set(int64(user.ID), &service.RuntimeUserStatus{ID: int64(user.ID), Role: "user", IsBanned: true, Reputation: 10})

	resolveReq := httptest.NewRequest(http.MethodPost, "/admin/appeals/"+strconv.FormatInt(appealID, 10),
		strings.NewReader(`{"status":"approved","admin_response":"复核通过，予以解封"}`))
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveW := httptest.NewRecorder()
	adminRouter.ServeHTTP(resolveW, resolveReq)
	require.Equal(t, http.StatusOK, resolveW.Code, resolveW.Body.String())

	var refreshed model.User
	require.NoError(t, db.First(&refreshed, user.ID).Error)
	require.False(t, refreshed.IsBanned, "批准 account 申诉必须解封")
	require.Empty(t, refreshed.BanReason, "解封必须清空 ban_reason")

	var resolved model.Appeal
	require.NoError(t, db.First(&resolved, appealID).Error)
	require.Equal(t, "approved", resolved.Status)
	require.NotNil(t, resolved.ResolvedAt)

	// 审计：appeal_resolve 必须落库（metadata 带决策与 target_type）。
	var auditCount int64
	require.NoError(t, db.Model(&model.AdminAuditLog{}).Where("action = ?", "appeal_resolve").Count(&auditCount).Error)
	require.Equal(t, int64(1), auditCount, "account 分支必须写审计")

	// 状态缓存已失效：Set 写入的键被 Invalidate 清除。
	statusKey := "user:status:2"
	_, getErr := rdb.Get(t.Context(), statusKey).Result()
	require.Error(t, getErr, "解封后用户状态缓存必须失效")
}
