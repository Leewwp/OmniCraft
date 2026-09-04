package handler

// T31（FIX-27）申诉质量包：
// ① 目标存在性校验（content/comment → 404 TARGET_NOT_FOUND）
// ② F-099 HasPendingAppeal 错误 fail-closed（503，不再静默放行）
// ③ ResolveAppeal 非 pending → 409 APPEAL_ALREADY_RESOLVED
// ④ comment approved → {status:"published"}（隐藏态恢复）
// ⑤ 批准边界守卫：content/comment 作者被封禁（或 content 已删）→ 409；
//    account 目标已注销 → 409
// ⑥ admin ListAppeals status 筛选（默认 pending 兼容现状）

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

func setupT31AppealQualityRouter(t *testing.T) (*gin.Engine, *gin.Engine, *gorm.DB, *redis.Client, *miniredis.Miniredis) {
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
		&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentAttachment{}, &model.ContentTag{},
		&model.Appeal{}, &model.AdminAuditLog{}, &model.OutboxEvent{},
	))
	// comments 表手写建表（AutoMigrate 连带 discussions 的 DEFAULT NOW() 与
	// sqlite 不兼容，先例 social_delete_comment_test / social_service_test）。
	require.NoError(t, db.Exec(`CREATE TABLE comments (
		id integer PRIMARY KEY AUTOINCREMENT,
		content_item_id integer,
		discussion_id integer,
		parent_id integer,
		author_id integer NOT NULL,
		target_type text,
		target_id integer,
		content text,
		body text NOT NULL,
		status text NOT NULL DEFAULT 'published',
		like_count integer NOT NULL DEFAULT 0,
		created_at datetime,
		updated_at datetime
	)`).Error)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	previous := redisclient.Client
	redisclient.Client = rdb
	t.Cleanup(func() { redisclient.Client = previous })

	cfg := &config.Config{}
	cfg.JWT.Secret = "t31-appeal-secret"
	cfg.Cache.UserStatusTTL = 300

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	adminHandler := NewAdminHandler(db, cfg, rdb, auditSvc)

	appealHandler := NewAppealHandler(db)
	appealRouter := gin.New()
	appealReq := middleware.AuthRequired(cfg, rdb, db)
	v1 := appealRouter.Group("/api/v1")
	v1.POST("/appeals", appealReq, appealHandler.SubmitAppeal)

	adminRouter := gin.New()
	adminRouter.GET("/admin/appeals", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		c.Set(middleware.UserRoleKey, "admin")
		c.Set("trace_id", "trace-t31-test")
		adminHandler.ListAppeals(c)
	})
	adminRouter.POST("/admin/appeals/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		c.Set(middleware.UserRoleKey, "admin")
		c.Set("trace_id", "trace-t31-test")
		adminHandler.ResolveAppeal(c)
	})

	return appealRouter, adminRouter, db, rdb, mr
}

func t31Token(t *testing.T, cfg *config.Config, userID int64) string {
	t.Helper()
	pair, err := jwtutil.GenerateTokenPair(userID, "user", cfg.JWT.Secret, 120, 7)
	require.NoError(t, err)
	return pair.AccessToken
}

func t31SeedUser(t *testing.T, db *gorm.DB, id int64, banned bool) model.User {
	t.Helper()
	user := model.User{
		Email:           "t31-user-" + strconv.FormatInt(id, 10) + "@test.com",
		Username:        "t31-user-" + strconv.FormatInt(id, 10),
		PasswordHash:    "hash",
		Reputation:      10,
		Role:            "user",
		IsBanned:        banned,
		EmailVerifiedAt: ptrTimeT29(time.Now()),
	}
	user.ID = id
	require.NoError(t, db.Create(&user).Error)
	return user
}

func t31SubmitAppeal(t *testing.T, appealRouter *gin.Engine, cfg *config.Config, userID int64, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/appeals", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t31Token(t, cfg, userID))
	w := httptest.NewRecorder()
	appealRouter.ServeHTTP(w, req)
	return w
}

// ① content/comment 假目标 → 404 TARGET_NOT_FOUND；account 免校验（本人）。
func TestSubmitAppealRejectsMissingTarget(t *testing.T) {
	appealRouter, _, db, _, mr := setupT31AppealQualityRouter(t)
	defer mr.Close()
	t31SeedUser(t, db, 2, false)
	cfg := &config.Config{}
	cfg.JWT.Secret = "t31-appeal-secret"

	w := t31SubmitAppeal(t, appealRouter, cfg, 2, `{"target_type":"content","target_id":999999,"reason":"r"}`)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "TARGET_NOT_FOUND")

	w2 := t31SubmitAppeal(t, appealRouter, cfg, 2, `{"target_type":"comment","target_id":888888,"reason":"r"}`)
	require.Equal(t, http.StatusNotFound, w2.Code, w2.Body.String())
	require.Contains(t, w2.Body.String(), "TARGET_NOT_FOUND")
}

// ② F-099：HasPendingAppeal 查询失败必须 fail-closed（503，不创建申诉）。
func TestSubmitAppealFailsClosedWhenPendingCheckErrors(t *testing.T) {
	appealRouter, _, db, _, mr := setupT31AppealQualityRouter(t)
	defer mr.Close()
	user := t31SeedUser(t, db, 2, false)
	cfg := &config.Config{}
	cfg.JWT.Secret = "t31-appeal-secret"

	// 真实存在的 content（目标校验通过），但 appeals 表未建 → HasPendingAppeal 报错。
	content := model.ContentItem{AuthorID: user.ID, Title: "t31-fc", Status: "published"}
	require.NoError(t, db.Create(&content).Error)
	require.NoError(t, db.Migrator().DropTable("appeals"))

	w := t31SubmitAppeal(t, appealRouter, cfg, 2, `{"target_type":"content","target_id":`+strconv.FormatInt(content.ID, 10)+`,"reason":"r"}`)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
}

// ③ 已处理申诉重复 Resolve → 409 APPEAL_ALREADY_RESOLVED。
func TestResolveAppealRejectsAlreadyResolved(t *testing.T) {
	_, adminRouter, db, _, mr := setupT31AppealQualityRouter(t)
	defer mr.Close()
	user := t31SeedUser(t, db, 2, false)
	cfg := &config.Config{}
	cfg.JWT.Secret = "t31-appeal-secret"

	// 建一个真实 content 再申诉，走通 201。
	content := model.ContentItem{AuthorID: user.ID, Title: "t31", Status: "banned"}
	require.NoError(t, db.Create(&content).Error)
	appeal := model.Appeal{UserID: user.ID, TargetType: "content", TargetID: content.ID, Reason: "r", Status: "approved"}
	require.NoError(t, db.Create(&appeal).Error)

	req := httptest.NewRequest(http.MethodPost, "/admin/appeals/"+strconv.FormatInt(appeal.ID, 10),
		strings.NewReader(`{"status":"rejected","admin_response":"again"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	adminRouter.ServeHTTP(w, req)
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "APPEAL_ALREADY_RESOLVED")
}

// ④ 批准评论申诉 → 评论恢复 published。
func TestResolveCommentAppealApprovedRestoresComment(t *testing.T) {
	_, adminRouter, db, _, mr := setupT31AppealQualityRouter(t)
	defer mr.Close()
	user := t31SeedUser(t, db, 2, false)

	comment := model.Comment{AuthorID: user.ID, Body: "c", Status: "hidden"}
	require.NoError(t, db.Create(&comment).Error)
	appeal := model.Appeal{UserID: user.ID, TargetType: "comment", TargetID: comment.ID, Reason: "r", Status: "pending"}
	require.NoError(t, db.Create(&appeal).Error)

	req := httptest.NewRequest(http.MethodPost, "/admin/appeals/"+strconv.FormatInt(appeal.ID, 10),
		strings.NewReader(`{"status":"approved","admin_response":"恢复"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	adminRouter.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var restored model.Comment
	require.NoError(t, db.First(&restored, comment.ID).Error)
	require.Equal(t, "published", restored.Status)
}

// ⑤ 批准边界：content 作者被封禁 → 409；account 目标已注销 → 409。
func TestResolveAppealRejectsUnavailableTarget(t *testing.T) {
	_, adminRouter, db, _, mr := setupT31AppealQualityRouter(t)
	defer mr.Close()
	author := t31SeedUser(t, db, 2, true) // 被封禁作者
	goneUser := t31SeedUser(t, db, 3, false)
	deletedAt := time.Now()
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", goneUser.ID).Update("deleted_at", &deletedAt).Error)

	content := model.ContentItem{AuthorID: author.ID, Title: "t31-gone", Status: "banned"}
	require.NoError(t, db.Create(&content).Error)
	contentAppeal := model.Appeal{UserID: author.ID, TargetType: "content", TargetID: content.ID, Reason: "r", Status: "pending"}
	require.NoError(t, db.Create(&contentAppeal).Error)

	accountAppeal := model.Appeal{UserID: goneUser.ID, TargetType: "account", TargetID: goneUser.ID, Reason: "r", Status: "pending"}
	require.NoError(t, db.Create(&accountAppeal).Error)

	resolve := func(id int64) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/admin/appeals/"+strconv.FormatInt(id, 10),
			strings.NewReader(`{"status":"approved","admin_response":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		adminRouter.ServeHTTP(w, req)
		return w
	}

	w := resolve(contentAppeal.ID)
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "APPEAL_TARGET_GONE")

	w2 := resolve(accountAppeal.ID)
	require.Equal(t, http.StatusConflict, w2.Code, w2.Body.String())
	require.Contains(t, w2.Body.String(), "APPEAL_TARGET_GONE")

	// 被拒路径不得改动目标状态。
	var still model.ContentItem
	require.NoError(t, db.First(&still, content.ID).Error)
	require.Equal(t, "banned", still.Status)
}

// ⑥ admin ListAppeals status 筛选：默认 pending（兼容），all/approved 可选。
func TestListAppealsStatusFilter(t *testing.T) {
	_, adminRouter, db, _, mr := setupT31AppealQualityRouter(t)
	defer mr.Close()
	user := t31SeedUser(t, db, 2, false)

	pending := model.Appeal{UserID: user.ID, TargetType: "account", TargetID: user.ID, Reason: "r", Status: "pending"}
	approved := model.Appeal{UserID: user.ID, TargetType: "account", TargetID: user.ID, Reason: "r", Status: "approved"}
	require.NoError(t, db.Create(&pending).Error)
	require.NoError(t, db.Create(&approved).Error)

	list := func(status string) map[string]any {
		req := httptest.NewRequest(http.MethodGet, "/admin/appeals?page=1&page_size=20"+status, nil)
		w := httptest.NewRecorder()
		adminRouter.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		return body
	}

	require.EqualValues(t, 1, list("")["total"], "默认仍只列 pending")
	require.EqualValues(t, 1, list("&status=approved")["total"])
	require.EqualValues(t, 2, list("&status=all")["total"])
	require.EqualValues(t, 0, list("&status=rejected")["total"])
}
