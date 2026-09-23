package router

// #658 组合根收口行为验证（漂移①②）：全栈（容器 + RegisterRoutes +
// miniredis + 临时 Postgres）驱动真实 HTTP 路由，断言落库结果——
// ① 管理员恢复内容在审计事务内重发 TopicContentPublished（此前
// SetContentOutbox 从未接线，outbox 恒 nil、重发被静默跳过，恢复的内容
// 不再进入检索投影）；
// ② 扫描结果回调走容器审核服务后，审核终态同事务落发件箱事件并通知
// 作者（此前 internal handler 自建裸审核服务，两样都缺）。
// 契约先例：judge_wiring_test.go（F-A001 防再犯集成门）。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/container"
	"omnicraft/backend/internal/mcpserver"
	"omnicraft/backend/internal/model"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/testutil"
)

func setupCompositionWiringStack(t *testing.T) (*gin.Engine, *gorm.DB, *config.Config, *container.ServiceContainer) {
	t.Helper()

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.ContentItem{}, &model.ContentAttachment{}, &model.Notification{}, &model.OutboxEvent{},
		&model.AdminAuditLog{}, &model.AIReviewRecord{}, &model.ReputationLog{}, &model.JudgeCase{},
		&model.ContentTag{}, &model.Tag{}, &model.Category{}, &model.ContentUsageGuide{}, &model.IP{},
		&model.ContentVersion{},
	))
	// 迁移 068 的唯一索引是 recordAIReview ON CONFLICT 子句的绑定目标，
	// AutoMigrate 不声明它（ai_callback_test.go 同款补建）。
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_review_records_provider_task
		ON ai_review_records (provider, provider_task_id)`).Error)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })
	previous := redisclient.Client
	redisclient.Client = rdb
	t.Cleanup(func() { redisclient.Client = previous })

	cfg := &config.Config{
		JWT:        config.JWTConfig{Secret: "composition-wiring-test-secret"},
		Reputation: config.ReputationConfig{MinScoreForInteraction: 1},
		// Queue 关闭：回调走同步 ProcessAICallback，通知走直写 DB 路径。
		Green: config.GreenConfig{Seed: "composition-seed", UID: "composition-uid"},
	}
	ctr, err := container.NewContainer(db, rdb, cfg)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterRoutes(engine.Group("/api/v1"), cfg, ctr)
	return engine, db, cfg, ctr
}

func seedCompositionUser(t *testing.T, db *gorm.DB, id int64, role string) {
	t.Helper()
	verifiedAt := time.Now()
	require.NoError(t, db.Create(&model.User{
		ID:              id,
		Email:           fmt.Sprintf("composition-%d@example.test", id),
		Username:        fmt.Sprintf("composition_%d", id),
		PasswordHash:    "hash",
		Role:            role,
		Reputation:      10,
		EmailVerifiedAt: &verifiedAt,
	}).Error)
}

func compositionAuthHeader(t *testing.T, cfg *config.Config, userID int64, role string) string {
	t.Helper()
	token, err := jwtutil.GenerateAccessToken(userID, role, cfg.JWT.Secret, 30)
	require.NoError(t, err)
	return "Bearer " + token
}

func countOutboxEvents(t *testing.T, db *gorm.DB, eventType string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&model.OutboxEvent{}).Where("event_type = ?", eventType).Count(&n).Error)
	return n
}

// 漂移①：管理员恢复非 published 内容 → 同事务补发 content.published。
func TestAdminRestoreReemitsContentPublishedEvent(t *testing.T) {
	engine, db, cfg, _ := setupCompositionWiringStack(t)
	const adminID int64 = 6501
	const authorID int64 = 6502
	seedCompositionUser(t, db, adminID, "admin")
	seedCompositionUser(t, db, authorID, "user")

	deletedAt := time.Now()
	content := model.ContentItem{
		Title:       "composition restore content",
		AuthorID:    authorID,
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		Status:      "banned",
		IsPublic:    true,
		DeletedAt:   &deletedAt,
	}
	require.NoError(t, db.Create(&content).Error)

	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/contents/%d/restore", content.ID), nil)
	req.Header.Set("Authorization", compositionAuthHeader(t, cfg, adminID, "admin"))
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "restore: %s", rec.Body.String())

	var restored model.ContentItem
	require.NoError(t, db.First(&restored, content.ID).Error)
	require.Equal(t, "published", restored.Status)
	require.Nil(t, restored.DeletedAt)

	require.Equal(t, int64(1), countOutboxEvents(t, db, "content.published"),
		"恢复必须在审计事务内重发内容发布事件（检索投影依赖）")

	// 重复 restore 幂等：已 published 的内容不再重发。
	req2 := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/contents/%d/restore", content.ID), nil)
	req2.Header.Set("Authorization", compositionAuthHeader(t, cfg, adminID, "admin"))
	rec2 := httptest.NewRecorder()
	engine.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code, "second restore: %s", rec2.Body.String())
	require.Equal(t, int64(1), countOutboxEvents(t, db, "content.published"), "重复 restore 不得重复发事件")
}

// 漂移②：扫描结果回调 → 审核终态落发件箱事件 + 作者通知。
func TestAICallbackWritesOutboxEventAndNotifiesAuthor(t *testing.T) {
	engine, db, cfg, _ := setupCompositionWiringStack(t)
	const authorID int64 = 6512
	seedCompositionUser(t, db, authorID, "user")

	content := model.ContentItem{
		Title:       "composition callback content",
		AuthorID:    authorID,
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		Status:      "under_review",
		IsPublic:    true,
	}
	require.NoError(t, db.Create(&content).Error)

	callbackContent := fmt.Sprintf(
		`{"dataId":"content:%d","taskId":"task-composition-1","code":200,"message":"OK","results":[{"scene":"text","label":"normal","suggestion":"pass"}]}`,
		content.ID,
	)
	// 丢单独放 handler 的 sha256 帮手不可达，测试内联同款摘要。
	sum := compositionSha256Hex(cfg.Green.UID + cfg.Green.Seed + callbackContent)
	form := url.Values{"checksum": {sum}, "content": {callbackContent}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/ai-callback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "ai-callback: %s", rec.Body.String())

	var published model.ContentItem
	require.NoError(t, db.First(&published, content.ID).Error)
	require.Equal(t, "published", published.Status, "pass 回调应发布内容")

	require.Equal(t, int64(1), countOutboxEvents(t, db, "content.published"),
		"审核终态必须在同事务落发件箱索引事件（此前裸审核服务静默跳过）")

	require.Eventually(t, func() bool {
		var notified int64
		require.NoError(t, db.Model(&model.Notification{}).
			Where("user_id = ? AND type = ? AND target_id = ?", authorID, "content_status", content.ID).
			Count(&notified).Error)
		return notified >= 1
	}, 3*time.Second, 50*time.Millisecond, "审核终态应通知作者（此前裸审核服务静默跳过）")
}

func compositionSha256Hex(s string) string {
	// 复用 internal.go 的 sha256Hex 语义（handler 包未导出）。
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// MCP 写路径（#658 唯一预期行为变更点）：容器 StudioContentService 被 MCP
// 写通道与 HTTP 共用（含事务性发件箱）。外部 Agent 经 omnicraft_create_content
// 写入的内容走同一审核终态链路——回调 pass 后落 content.published 索引事件，
// 与 HTTP 发布通道一致。
func TestMCPWritePathEmitsIndexEvents(t *testing.T) {
	engine, db, cfg, ctr := setupCompositionWiringStack(t)
	const agentAuthorID int64 = 6522
	seedCompositionUser(t, db, agentAuthorID, "user")

	// upload-scope PAT 身份直挂 MCP handler（routes.go mcpIdentity 中间件
	// 的等价物），真 Streamable HTTP 会话驱动工具调用。
	mcpRoot := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := mcpserver.Identity{UserID: agentAuthorID, Scopes: []string{"upload"}}
		ctr.MCPHandler.ServeHTTP(w, r.WithContext(mcpserver.WithIdentity(r.Context(), id)))
	})
	srv := httptest.NewServer(mcpRoot)
	t.Cleanup(srv.Close)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "composition-wiring-test", Version: "v1"}, nil)
	session, err := client.Connect(context.Background(), &sdkmcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	require.NoError(t, err, "MCP connect")
	t.Cleanup(func() { _ = session.Close() })

	args, _ := json.Marshal(map[string]any{
		"title": "mcp wiring draft", "description": "via mcp", "zone": "original",
		"content_type": "article", "category": "game", "is_public": true, "allow_copy": true,
	})
	res, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "omnicraft_create_content", Arguments: json.RawMessage(args),
	})
	require.NoError(t, err, "create_content call")
	require.False(t, res.IsError, "create_content errored")
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			text += tc.Text
		}
	}
	var created struct {
		ContentID int64  `json:"content_id"`
		Status    string `json:"status"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &created), "decode %q", text)
	require.NotZero(t, created.ContentID)
	require.Equal(t, "pending", created.Status, "外部上传走审核，无豁免")

	// 审核终态：回调 pass → published + 同事务索引事件（共享发件箱）。
	callbackContent := fmt.Sprintf(
		`{"dataId":"content:%d","taskId":"task-composition-mcp-1","code":200,"message":"OK","results":[{"scene":"text","label":"normal","suggestion":"pass"}]}`,
		created.ContentID,
	)
	form := url.Values{"checksum": {compositionSha256Hex(cfg.Green.UID + cfg.Green.Seed + callbackContent)}, "content": {callbackContent}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/ai-callback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "ai-callback after mcp create: %s", rec.Body.String())

	var published model.ContentItem
	require.NoError(t, db.First(&published, created.ContentID).Error)
	require.Equal(t, "published", published.Status)
	require.Equal(t, int64(1), countOutboxEvents(t, db, "content.published"),
		"MCP 写入内容的审核终态必须落索引事件（与 HTTP 通道一致）")
}
