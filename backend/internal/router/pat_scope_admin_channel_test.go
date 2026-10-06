package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// run-1 审计 #11/#12：series/collections 变更族与 contents PATCH/DELETE 此前
// 不在 PAT scope 词表内（download-only PAT 可硬删系列）；admin 组与组外
// discussion pin 仅查角色——泄露的 admin PAT 即全权机器通道。修复后：
// 变更族统一 RequireScopeForPAT("upload")，admin 面 RequireJWTChannel()。

func doPATRequest(t *testing.T, engine *gin.Engine, method, path, authHeader, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestSeriesAndCollectionMutationsRequireUploadScopeForPAT(t *testing.T) {
	router, db, cfg, fx := buildOptAuthLeakAuditStack(t)
	require.NoError(t, db.AutoMigrate(&model.AgentAccessToken{}))
	// 该栈 cfg 为空对象：interaction 守卫要求 reputation.min_score_for_interaction
	// 已配置（EvaluateInteractionAccess 的 CONFIG_ERROR 路径），请求前补上。
	cfg.Reputation.MinScoreForInteraction = 3

	patSvc := service.NewAgentAccessTokenService(repository.NewAgentAccessTokenRepository(db), cfg)
	downloadOnly, err := patSvc.Issue(t.Context(), fx.viewerID, "run1 scope dl", []string{service.AgentTokenScopeDownload})
	require.NoError(t, err)
	uploadPAT, err := patSvc.Issue(t.Context(), fx.viewerID, "run1 scope ul", []string{service.AgentTokenScopeUpload})
	require.NoError(t, err)
	jwtToken := makeRoutesSecurityToken(cfg, fx.viewerID, "user")

	mutations := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/series"},
		{http.MethodPut, fmt.Sprintf("/api/v1/series/%d", optAuthTestSeriesID)},
		{http.MethodDelete, fmt.Sprintf("/api/v1/series/%d", optAuthTestSeriesID)},
		{http.MethodPost, fmt.Sprintf("/api/v1/series/%d/items", optAuthTestSeriesID)},
		{http.MethodDelete, fmt.Sprintf("/api/v1/series/%d/items/%d", optAuthTestSeriesID, 4310)},
		{http.MethodPut, fmt.Sprintf("/api/v1/series/%d/items/reorder", optAuthTestSeriesID)},
		{http.MethodPost, "/api/v1/collections"},
		{http.MethodPut, fmt.Sprintf("/api/v1/collections/%d", optAuthTestCollectionID)},
		{http.MethodDelete, fmt.Sprintf("/api/v1/collections/%d", optAuthTestCollectionID)},
		{http.MethodPost, fmt.Sprintf("/api/v1/collections/%d/items", optAuthTestCollectionID)},
		{http.MethodDelete, fmt.Sprintf("/api/v1/collections/%d/items/%d", optAuthTestCollectionID, 4210)},
		{http.MethodPut, fmt.Sprintf("/api/v1/collections/%d/items/%d", optAuthTestCollectionID, 4210)},
		// run-1 审计 #11 顺带收口：contents PATCH/DELETE 同为内容组织写。
		{http.MethodPatch, fmt.Sprintf("/api/v1/contents/%d", fx.cPub)},
		{http.MethodDelete, fmt.Sprintf("/api/v1/contents/%d", fx.cPub)},
	}

	for _, m := range mutations {
		rec := doPATRequest(t, router, m.method, m.path, "Bearer "+downloadOnly.Token, `{}`)
		require.Equal(t, http.StatusForbidden, rec.Code, "%s %s by download-only PAT: %s", m.method, m.path, rec.Body.String())
		require.Contains(t, rec.Body.String(), "PAT_SCOPE_REQUIRED", "%s %s body: %s", m.method, m.path, rec.Body.String())
	}

	// upload scope 与 JWT 会话不受影响（对照：创建路径真实走通）。
	for label, authHeader := range map[string]string{
		"upload-pat": "Bearer " + uploadPAT.Token,
		"jwt":        "Bearer " + jwtToken,
	} {
		rec := doPATRequest(t, router, http.MethodPost, "/api/v1/series", authHeader, `{"title":"pat gate `+label+`","zone":"original"}`)
		require.Equal(t, http.StatusCreated, rec.Code, "%s create series: %s", label, rec.Body.String())
		rec = doPATRequest(t, router, http.MethodPost, "/api/v1/collections", authHeader, `{"title":"pat gate `+label+`","zone":"original","is_public":true}`)
		require.Equal(t, http.StatusCreated, rec.Code, "%s create collection: %s", label, rec.Body.String())
	}
}

func TestAdminSurfaceRequiresJWTChannel(t *testing.T) {
	router, db, cfg, fx := buildOptAuthLeakAuditStack(t)
	require.NoError(t, db.AutoMigrate(&model.AgentAccessToken{}))

	verifiedAt := time.Now()
	adminUser := model.User{
		ID: 504, Email: "sp16a_admin@example.com", Username: "sp16a_admin",
		PasswordHash: "hash", Reputation: 10, Role: "admin", EmailVerifiedAt: &verifiedAt,
	}
	require.NoError(t, db.Create(&adminUser).Error)

	patSvc := service.NewAgentAccessTokenService(repository.NewAgentAccessTokenRepository(db), cfg)
	adminPAT, err := patSvc.Issue(t.Context(), adminUser.ID, "run1 admin pat", []string{service.AgentTokenScopeDownload, service.AgentTokenScopeUpload})
	require.NoError(t, err)
	adminJWT := makeRoutesSecurityToken(cfg, adminUser.ID, "admin")

	// PAT bearer（即使是 admin 用户的全 scope PAT）在 admin 面一律 403。
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/admin/users"},
		{http.MethodGet, "/api/v1/admin/ips"},
		{http.MethodPost, "/api/v1/admin/ips/401/approve"},
		// 组外唯一 admin 路由：discussion pin 同款缺口。
		{http.MethodPatch, fmt.Sprintf("/api/v1/discussions/%d/pin", fx.discHidden)},
	} {
		rec := doPATRequest(t, router, tc.method, tc.path, "Bearer "+adminPAT.Token, `{"pinned":true}`)
		require.Equal(t, http.StatusForbidden, rec.Code, "%s %s by admin PAT: %s", tc.method, tc.path, rec.Body.String())
		require.Contains(t, rec.Body.String(), "JWT_CHANNEL_REQUIRED", "%s %s body: %s", tc.method, tc.path, rec.Body.String())
	}

	// JWT admin 不受影响：list 200，pin 200。
	rec := doPATRequest(t, router, http.MethodGet, "/api/v1/admin/users", "Bearer "+adminJWT, "")
	require.Equal(t, http.StatusOK, rec.Code, "admin JWT list users: %s", rec.Body.String())

	rec = doPATRequest(t, router, http.MethodPatch, fmt.Sprintf("/api/v1/discussions/%d/pin", fx.discHidden), "Bearer "+adminJWT, `{"pinned":true}`)
	require.Equal(t, http.StatusOK, rec.Code, "admin JWT pin: %s", rec.Body.String())
}
