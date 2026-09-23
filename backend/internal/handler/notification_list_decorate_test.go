package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/model"
)

// SP-18 #509 契约：GET /notifications 列表装饰（响应只增不改）。
// 每条通知新增 sender{id,username,avatar_url}（批量 join，免前端 N+1）与
// target_summary{kind,title,url}（原内容引用块渲染与深链跳转，映射与
// frontend lib/notification-url.ts 同源）。老字段（id/channel/title/body/
// target_type/target_id/sender_id/is_read/created_at）原样保留。

type decoratedNotification struct {
	ID         int64           `json:"id"`
	Channel    string          `json:"channel"`
	Type       string          `json:"type"`
	Title      *string         `json:"title"`
	Body       *string         `json:"body"`
	TargetType *string         `json:"target_type"`
	TargetID   *int64          `json:"target_id"`
	SenderID   *int64          `json:"sender_id"`
	IsRead     bool            `json:"is_read"`
	CreatedAt  string          `json:"created_at"`
	Sender     *senderJSON     `json:"sender"`
	Summary    *targetSummary  `json:"target_summary"`
}

type senderJSON struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

type targetSummary struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

func setupNotificationDecorateRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.User{}, &model.Notification{}, &model.ContentItem{}, &model.IP{}, &model.PullRequest{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// discussions 的 last_active_at 生产 tag 是 DEFAULT NOW()（Postgres 方言），
	// sqlite AutoMigrate 不认——此处手建等价表（仅测试态）。
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS discussions (
		id integer PRIMARY KEY AUTOINCREMENT,
		ip_id integer, content_item_id integer,
		author_id integer NOT NULL,
		title text NOT NULL, body text,
		status text NOT NULL DEFAULT 'published',
		is_pinned numeric NOT NULL DEFAULT 0,
		view_count integer NOT NULL DEFAULT 0,
		reply_count integer NOT NULL DEFAULT 0,
		last_active_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
		created_at datetime, updated_at datetime
	)`).Error; err != nil {
		t.Fatalf("create discussions: %v", err)
	}

	handler := NewNotificationHandler(repository.NewNotificationRepository(db))
	router := gin.New()
	router.GET("/api/v1/notifications", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.ListNotifications(c)
	})
	return router, db
}

func TestListNotificationsDecoratesSenderAndTargetSummary(t *testing.T) {
	router, db := setupNotificationDecorateRouter(t)

	seedNotificationDecorateFixtures(t, db)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?page=1&page_size=20", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Notifications []decoratedNotification `json:"notifications"`
		Total         int64                   `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(payload.Notifications) != 7 {
		t.Fatalf("notifications = %d, want 7", len(payload.Notifications))
	}
	byID := map[int64]*decoratedNotification{}
	for i := range payload.Notifications {
		byID[payload.Notifications[i].ID] = &payload.Notifications[i]
	}

	// 回复我的（评论，target=content）：sender + content 标题引用块 + 内容页深链。
	n := byID[1]
	if n.Sender == nil || n.Sender.Username != "commenter" || n.Sender.ID != 2 || n.Sender.AvatarURL != "https://cdn.example/a.png" {
		t.Fatalf("notification 1 sender = %+v", n.Sender)
	}
	if n.Summary == nil || n.Summary.Kind != "content" || n.Summary.Title != "灵感笔记" || n.Summary.URL != "/content/101" {
		t.Fatalf("notification 1 summary = %+v", n.Summary)
	}
	if n.Channel != "reply" || n.TargetType == nil || *n.TargetType != "content" || n.TargetID == nil || *n.TargetID != 101 {
		t.Fatalf("notification 1 old fields mutated: %+v", n)
	}

	// 回复我的（讨论回复， target=discussion）：标题引用块 + IP Hub 讨论浮层深链（免前端二跳）。
	n = byID[2]
	if n.Summary == nil || n.Summary.Kind != "discussion" || n.Summary.Title != "系列灵感 08" || n.Summary.URL != "/ip/300?tab=discussions&d=201" {
		t.Fatalf("notification 2 summary = %+v", n.Summary)
	}

	// 收到的赞：content 引用块。
	n = byID[3]
	if n.Summary == nil || n.Summary.Kind != "content" || n.Summary.Title != "灵感笔记" {
		t.Fatalf("notification 3 summary = %+v", n.Summary)
	}

	// 关注：kind=user 深链，无标题引用块。
	n = byID[4]
	if n.Summary == nil || n.Summary.Kind != "user" || n.Summary.Title != "" || n.Summary.URL != "/user/1" {
		t.Fatalf("notification 4 summary = %+v", n.Summary)
	}

	// PR：所属内容标题引用块 + PR 页深链。
	n = byID[5]
	if n.Summary == nil || n.Summary.Kind != "pr" || n.Summary.Title != "灵感笔记" || n.Summary.URL != "/studio/pr-requests" {
		t.Fatalf("notification 5 summary = %+v", n.Summary)
	}

	// 系统消息：有 sender（管理员触发）但无 target 装饰。
	n = byID[6]
	if n.Sender == nil || n.Sender.Username != "site_admin" {
		t.Fatalf("notification 6 sender = %+v", n.Sender)
	}
	if n.Summary != nil {
		t.Fatalf("notification 6 summary = %+v, want nil", n.Summary)
	}

	// 广播：无 sender、无 target 装饰，老字段原样。
	n = byID[7]
	if n.Sender != nil || n.Summary != nil {
		t.Fatalf("notification 7 must stay undecorated: sender=%+v summary=%+v", n.Sender, n.Summary)
	}
	if n.Title == nil || *n.Title != "每周精选" {
		t.Fatalf("notification 7 title = %v", n.Title)
	}
}

func TestListNotificationsChannelFilterKeepsDecoration(t *testing.T) {
	router, db := setupNotificationDecorateRouter(t)
	seedNotificationDecorateFixtures(t, db)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?channel=reply", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Notifications []decoratedNotification `json:"notifications"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(payload.Notifications) != 2 {
		t.Fatalf("filtered notifications = %d, want 2", len(payload.Notifications))
	}
	for _, n := range payload.Notifications {
		if n.Sender == nil || n.Summary == nil {
			t.Fatalf("channel-filtered notification %d lost decoration", n.ID)
		}
	}
}

func seedNotificationDecorateFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()

	users := []model.User{
		{ID: 1, Username: "recipient", Email: "r@example.com", PasswordHash: "x"},
		{ID: 2, Username: "commenter", Email: "c@example.com", PasswordHash: "x", AvatarURL: "https://cdn.example/a.png"},
		{ID: 9, Username: "site_admin", Email: "a@example.com", PasswordHash: "x"},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}

	if err := db.Create(&model.ContentItem{ID: 101, Title: "灵感笔记", AuthorID: 1, Zone: "original", ContentType: "article"}).Error; err != nil {
		t.Fatalf("seed content: %v", err)
	}
	ipID := int64(300)
	if err := db.Create(&model.IP{ID: 300, Name: "宝可梦", Slug: "ip-300", Status: "approved"}).Error; err != nil {
		t.Fatalf("seed ip: %v", err)
	}
	if err := db.Create(&model.Discussion{ID: 201, IPID: &ipID, AuthorID: 2, Title: "系列灵感 08", Status: "published"}).Error; err != nil {
		t.Fatalf("seed discussion: %v", err)
	}
	if err := db.Create(&model.PullRequest{ID: 401, ContentItemID: 101, SubmitterID: 2, BaseVersionID: 1, Status: "open"}).Error; err != nil {
		t.Fatalf("seed pr: %v", err)
	}

	strPtr := func(s string) *string { return &s }
	intPtr := func(v int64) *int64 { return &v }
	notifications := []model.Notification{
		{ID: 1, UserID: 1, Type: "comment", Channel: "reply", Title: strPtr("新评论"), Body: strPtr("写得真好"), TargetType: strPtr("content"), TargetID: intPtr(101), SenderID: intPtr(2)},
		{ID: 2, UserID: 1, Type: "comment", Channel: "reply", Title: strPtr("新回复"), Body: strPtr("同感"), TargetType: strPtr("discussion"), TargetID: intPtr(201), SenderID: intPtr(2)},
		{ID: 3, UserID: 1, Type: "like", Channel: "like", Title: strPtr("新的赞"), TargetType: strPtr("content"), TargetID: intPtr(101), SenderID: intPtr(2)},
		{ID: 4, UserID: 1, Type: "follow", Channel: "follow", Title: strPtr("你有新粉丝"), TargetType: strPtr("user"), TargetID: intPtr(1), SenderID: intPtr(2)},
		{ID: 5, UserID: 1, Type: "pr_merged", Channel: "pr", Title: strPtr("PR 已合并：灵感笔记"), TargetType: strPtr("pr"), TargetID: intPtr(401), SenderID: intPtr(2)},
		{ID: 6, UserID: 1, Type: "content_status", Channel: "system", Title: strPtr("审核通过"), Body: strPtr("已发布"), SenderID: intPtr(9)},
		{ID: 7, UserID: 1, Type: "system", Channel: "broadcast", Title: strPtr("每周精选"), Body: strPtr("本周内容合集")},
	}
	if err := db.Create(&notifications).Error; err != nil {
		t.Fatalf("seed notifications: %v", err)
	}
}
