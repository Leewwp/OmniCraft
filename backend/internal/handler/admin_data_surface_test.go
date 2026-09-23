package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func setupAdminDataSurfaceRouter(t *testing.T) (*gin.Engine, *gorm.DB, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.AdminAuditLog{},
		&model.Appeal{},
	))
	cfg := &config.Config{}
	cfg.JWT.Secret = "admin-data-surface-secret"

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	adminHandler := newAdminHandlerForTest(db, cfg, nil, auditSvc)
	authReq := middleware.AuthRequired(cfg, nil, db)

	admin := model.User{
		Email:        "admin-surface@example.com",
		Username:     "admin-surface",
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "admin",
	}
	require.NoError(t, db.Create(&admin).Error)
	token := mustToken(t, cfg, admin.ID, admin.Role)

	router := gin.New()
	group := router.Group("/api/v1/admin", authReq, middleware.AdminRequired())
	group.GET("/users", adminHandler.ListUsers)
	group.GET("/appeals", adminHandler.ListAppeals)
	return router, db, token
}

// FR-02（中-6）：admin 用户列表必须显式列投影——password_hash 永不出现在响应。
func TestAdminListUsersProjectionExcludesPasswordHash(t *testing.T) {
	router, db, token := setupAdminDataSurfaceRouter(t)

	now := time.Now()
	require.NoError(t, db.Create(&model.User{
		Email:           "victim@example.com",
		Username:        "victim",
		PasswordHash:    "$2a$10$supersecretbcryptdigest",
		Reputation:      10,
		Role:            "user",
		EmailVerifiedAt: &now,
	}).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=1&page_size=20", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Users []map[string]interface{} `json:"users"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Users)

	allowed := map[string]bool{
		"id": true, "email": true, "username": true, "avatar_url": true,
		"reputation": true, "role": true, "is_banned": true, "ban_reason": true,
		"email_verified_at": true, "created_at": true,
	}
	found := map[string]interface{}{}
	for _, u := range resp.Users {
		if u["username"] == "victim" {
			found = u
		}
		for key := range u {
			require.True(t, allowed[key], "admin users response leaks non-whitelisted key %q", key)
		}
	}
	require.NotEmpty(t, found, "victim user must appear in list")
	require.Equal(t, "victim@example.com", found["email"], "admin needs email for appeal handling")
	require.NotContains(t, rec.Body.String(), "supersecretbcryptdigest")
	require.NotContains(t, rec.Body.String(), "password_hash")
}

// FR-02（低-27 handler 位点）：appeals 查询故障必须返回错误而非空列表 200。
// （用 DropTable 注入故障：DB 连接须保持可用，否则 auth 中间件先 503 拦截。）
func TestAdminListAppealsReturnsErrorOnDBFailure(t *testing.T) {
	router, db, token := setupAdminDataSurfaceRouter(t)

	require.NoError(t, db.Create(&model.Appeal{
		UserID:     1,
		TargetType: "content",
		TargetID:   1,
		Reason:     "miscategorized",
		Status:     "pending",
	}).Error)
	require.NoError(t, db.Migrator().DropTable("appeals"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/appeals", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

// FR-02（低-7）：提示词版本并发冲突（哨兵 ErrPromptVersionExists）映射 409 而非 500。
// 用 BEFORE INSERT 触发器确定性制造 LatestVersion→CreateVersion 窗口内的冲突。
func TestAdminPromptCreateVersionConflictMaps409(t *testing.T) {
	router, db, token := setupAdminPromptRouter(t)

	require.NoError(t, db.Exec(`
		CREATE TRIGGER force_prompt_version_conflict
		BEFORE INSERT ON prompt_registry
		WHEN NEW.name = 'conversation_title_prompt' AND NEW.version = 2
		BEGIN
			INSERT INTO prompt_registry (name, version, content, required_placeholders)
			VALUES (NEW.name, NEW.version, 'conflict stub', '[]');
		END
	`).Error)

	rec := doPromptRequest(t, router, token, http.MethodPost, "/api/v1/admin/prompts/conversation_title_prompt/versions", map[string]string{
		"content": "生成标题：{{first_user_message}}",
	})
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Contains(t, rec.Body.String(), "PROMPT_VERSION_EXISTS")
}

// FR-02（低-7 附带）：trimNonEmpty 按 rune 截断，输出恒为合法 UTF-8。
func TestTrimNonEmptyTruncatesOnRuneBoundary(t *testing.T) {
	// 300 个三字节字符：按字节截 200 会切出非法序列；按 rune 截 200 合法。
	longCJK := strings.Repeat("创", 300)
	got := trimNonEmpty(longCJK)
	require.LessOrEqual(t, len([]rune(got)), 200)
	require.True(t, utf8.ValidString(got), "truncated value must remain valid UTF-8, got %q", got)

	ascii := strings.Repeat("a", 300)
	gotASCII := trimNonEmpty(ascii)
	require.Len(t, gotASCII, 200)

	// 短值原样返回。
	require.Equal(t, "hello", trimNonEmpty("hello"))
}
