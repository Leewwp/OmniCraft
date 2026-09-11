package handler

import (
	"bytes"
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	sqlitedriver "github.com/glebarez/go-sqlite"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// T12（FIX-18）讨论发帖/回复统一走审核门的 handler 级测试。
// 场景对应票面验收：rep=1 经 /ips/:id/discussions → 403；正常用户 → 201 且
// 楼主收到通知；回复 content 字段映射不 400；未发布讨论详情不透出（F-106）。

// registerDiscussionTestNow makes Postgres-flavored NOW() available on the
// sqlite driver for IncrementDiscussionReplyCount's gorm.Expr("NOW()").
func registerDiscussionTestNow() {
	_ = sqlitedriver.RegisterScalarFunction("NOW", 0, func(_ *sqlitedriver.FunctionContext, _ []driver.Value) (driver.Value, error) {
		return time.Now().Format("2006-01-02 15:04:05.999999999"), nil
	})
}

func newDiscussionModerationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	registerDiscussionTestNow()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// :memory: sqlite is per-connection: cap the pool at one so the async
	// notification goroutine lands rows in the same database the assertions
	// read (mirrors setupSocialServiceTestDB in the service package).
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Notification{}))
	// comments/discussions 携带 DEFAULT NOW() DDL，sqlite 不认，手写兼容表。
	require.NoError(t, db.Exec(`
		CREATE TABLE comments (
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
	require.NoError(t, db.Exec(`
		CREATE TABLE discussions (
			id integer PRIMARY KEY AUTOINCREMENT,
			ip_id integer,
			content_item_id integer,
			author_id integer NOT NULL,
			title text NOT NULL,
			body text,
			status text NOT NULL DEFAULT 'published',
			is_pinned numeric NOT NULL DEFAULT 0,
			view_count integer NOT NULL DEFAULT 0,
			reply_count integer NOT NULL DEFAULT 0,
			last_active_at datetime NOT NULL DEFAULT (datetime('now')),
			created_at datetime,
			updated_at datetime
		)`).Error)
	return db
}

type discussionTestCase struct {
	db      *gorm.DB
	router  *gin.Engine
	ownerID int64 // 楼主
	lowID   int64 // rep=1 低信誉号
	normID  int64 // 正常用户
}

func setupDiscussionModerationCase(t *testing.T, reviewer service.TextReviewer) *discussionTestCase {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := newDiscussionModerationTestDB(t)

	owner := model.User{Email: "owner@d.local", Username: "owner", Reputation: 10}
	require.NoError(t, db.Create(&owner).Error)
	low := model.User{Email: "low@d.local", Username: "lowrep", Reputation: 1}
	require.NoError(t, db.Create(&low).Error)
	norm := model.User{Email: "norm@d.local", Username: "normal", Reputation: 10}
	require.NoError(t, db.Create(&norm).Error)

	cfg := &config.Config{Server: config.ServerConfig{Mode: "debug"}}
	socialSvc := service.NewSocialServiceWithRedis(
		repository.NewSocialRepository(db),
		repository.NewContentRepository(db),
		repository.NewUserRepository(db),
		cfg,
		nil,
		reviewer,
	)
	socialSvc.SetNotificationService(service.NewNotificationService(repository.NewNotificationRepository(db)))

	h := NewDiscussionHandler(db, socialSvc)

	r := gin.New()
	setUID := func(id int64) gin.HandlerFunc {
		return func(c *gin.Context) { c.Set(middleware.UserIDKey, id); c.Next() }
	}
	r.POST("/ips/:id/discussions", setUID(norm.ID), h.CreateDiscussion)
	r.POST("/ips-low/:id/discussions", setUID(low.ID), h.CreateDiscussion)
	r.POST("/discussions/:id/comments", setUID(norm.ID), h.ReplyToDiscussion)
	r.POST("/discussions-low/:id/comments", setUID(low.ID), h.ReplyToDiscussion)
	r.GET("/discussions/:id", h.GetDiscussion)
	return &discussionTestCase{db: db, router: r, ownerID: owner.ID, lowID: low.ID, normID: norm.ID}
}

func (tc *discussionTestCase) doJSON(t *testing.T, method, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	tc.router.ServeHTTP(rec, req)
	return rec
}

func countDiscussionsRows(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&model.Discussion{}).Count(&n).Error)
	return n
}

// rep=1 低信誉用户经前端实际调用的 /ips/:id/discussions 发帖必须被信誉门拦下
// （F-027：此前直写 repo 无守卫返回 201）。
func TestCreateDiscussionLowReputationForbidden(t *testing.T) {
	tc := setupDiscussionModerationCase(t, &fakeHandlerTextReviewer{result: "pass"})
	rec := tc.doJSON(t, http.MethodPost, "/ips-low/42/discussions", `{"title":"绕门帖","body":"rep=1 也要发"}`)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	require.Equal(t, int64(0), countDiscussionsRows(t, tc.db), "low-reputation post must not persist")
}

// 正常用户发帖走 SocialService.PostDiscussion：Green block 结果拒绝落库
// （F-028：此前无审核）；pass 结果 201 且 status=published。
func TestCreateDiscussionRoutesThroughModeration(t *testing.T) {
	blocked := setupDiscussionModerationCase(t, &fakeHandlerTextReviewer{result: "block"})
	rec := blocked.doJSON(t, http.MethodPost, "/ips/42/discussions", `{"title":"好帖","body":"但命中违禁词"}`)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	require.Equal(t, int64(0), countDiscussionsRows(t, blocked.db), "blocked text must not persist")

	passed := setupDiscussionModerationCase(t, &fakeHandlerTextReviewer{result: "pass"})
	rec = passed.doJSON(t, http.MethodPost, "/ips/42/discussions", `{"title":"正常帖","body":"正常正文"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Equal(t, int64(1), countDiscussionsRows(t, passed.db))
	var d model.Discussion
	require.NoError(t, passed.db.First(&d).Error)
	require.Equal(t, "published", d.Status)
}

// 正常用户回复讨论：content 字段映射不 400（前提①回归）；楼主收到通知；
// reply_count 由 service 统一递增。
func TestReplyToDiscussionMapsContentFieldAndNotifiesOwner(t *testing.T) {
	tc := setupDiscussionModerationCase(t, &fakeHandlerTextReviewer{result: "pass"})
	owner := model.User{Email: "owner2@d.local", Username: "owner2", Reputation: 10}
	require.NoError(t, tc.db.Create(&owner).Error)
	disc := model.Discussion{IPID: nil, AuthorID: owner.ID, Title: "楼主帖", Body: "楼主正文", Status: "published"}
	require.NoError(t, tc.db.Create(&disc).Error)

	rec := tc.doJSON(t, http.MethodPost, "/discussions/"+strconv.FormatInt(disc.ID, 10)+"/comments", `{"content":"恭喜发财"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var comment model.Comment
	require.NoError(t, tc.db.Where("discussion_id = ?", disc.ID).First(&comment).Error)
	require.Equal(t, "恭喜发财", comment.Body, "content field must map to comment body")

	var stored model.Discussion
	require.NoError(t, tc.db.First(&stored, disc.ID).Error)
	require.Equal(t, 1, stored.ReplyCount, "reply_count must increment exactly once")

	// 通知走 Noop-producer 的 GoSafe 落库，短轮询等副作用生效。
	var notifs []model.Notification
	for i := 0; i < 20 && len(notifs) == 0; i++ {
		require.NoError(t, tc.db.Where("user_id = ?", owner.ID).Find(&notifs).Error)
		time.Sleep(50 * time.Millisecond)
	}
	require.Len(t, notifs, 1, "discussion owner must be notified")
	require.Equal(t, "reply", notifs[0].Channel)
	require.NotNil(t, notifs[0].TargetType)
	require.Equal(t, "discussion", *notifs[0].TargetType)
}

// rep=1 回复讨论同样必须被信誉门拦下（handler 路由测试不挂 commentsGuard，
// 断言的是 SocialService.ensureCanInteract 这一层门真实生效）。
func TestReplyToDiscussionLowReputationForbidden(t *testing.T) {
	tc := setupDiscussionModerationCase(t, &fakeHandlerTextReviewer{result: "pass"})
	disc := model.Discussion{AuthorID: tc.ownerID, Title: "楼主帖", Body: "正文", Status: "published"}
	require.NoError(t, tc.db.Create(&disc).Error)

	rec := tc.doJSON(t, http.MethodPost, "/discussions-low/"+strconv.FormatInt(disc.ID, 10)+"/comments", `{"content":"低信誉回复"}`)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	var n int64
	require.NoError(t, tc.db.Model(&model.Comment{}).Count(&n).Error)
	require.Equal(t, int64(0), n, "low-reputation reply must not persist")
}

// F-106 顺带收口：未发布讨论详情不透出。
func TestGetDiscussionHidesUnpublished(t *testing.T) {
	tc := setupDiscussionModerationCase(t, &fakeHandlerTextReviewer{result: "pass"})
	disc := model.Discussion{AuthorID: tc.ownerID, Title: "待审帖", Body: "正文", Status: "under_review"}
	require.NoError(t, tc.db.Create(&disc).Error)

	rec := tc.doJSON(t, http.MethodGet, "/discussions/"+strconv.FormatInt(disc.ID, 10), "")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

// fakeHandlerTextReviewer mirrors the service-package fake: minimal moderation
// stub for handler tests (i18n fixture honesty: mirrors real catalog shape).
type fakeHandlerTextReviewer struct {
	result string
}

func (f *fakeHandlerTextReviewer) ReviewText(_ context.Context, _ string) (string, error) {
	return f.result, nil
}
