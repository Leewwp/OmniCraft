package handler

import (
	"net/http"
	"omnicraft/backend/internal/pkg/response"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/service"
)

// AgentAccessTokenHandler serves the settings-page token management
// endpoints. These routes are JWT-only by design (RequireJWTChannel in the
// route wiring): a leaked PAT must never be able to mint more PATs.
type AgentAccessTokenHandler struct {
	svc *service.AgentAccessTokenService
}

func NewAgentAccessTokenHandler(svc *service.AgentAccessTokenService) *AgentAccessTokenHandler {
	return &AgentAccessTokenHandler{svc: svc}
}

// createAgentTokenErrMap 是 POST .../tokens（Create 签发）的 sentinel→HTTP
// 合同：name/scopes 校验失败同 400（迁移前单 case 双 sentinel，拆为同响
// 应两条规则，语义等价）、活跃上限 409；兜底保留历史固定文案 500
// DB_ERROR。
var createAgentTokenErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrAgentTokenNameInvalid, status: http.StatusBadRequest, code: "AGENT_TOKEN_INVALID", message: "invalid token name or scopes"},
		{sentinel: service.ErrAgentTokenScopesInvalid, status: http.StatusBadRequest, code: "AGENT_TOKEN_INVALID", message: "invalid token name or scopes"},
		{sentinel: service.ErrAgentTokenLimitReached, status: http.StatusConflict, code: "AGENT_TOKEN_LIMIT_REACHED", message: "too many active tokens, revoke one first"},
	},
	fallback: errFallback{
		status:  fallbackDB.status,
		code:    fallbackDB.code,
		message: "failed to create token",
	},
}

// revokeAgentTokenErrMap 是 DELETE .../tokens/:id（Revoke 撤销）的
// sentinel→HTTP 合同；兜底保留历史固定文案 500 DB_ERROR。
var revokeAgentTokenErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrAgentTokenNotFound, status: http.StatusNotFound, code: "AGENT_TOKEN_NOT_FOUND", message: "token not found"},
	},
	fallback: errFallback{
		status:  fallbackDB.status,
		code:    fallbackDB.code,
		message: "failed to revoke token",
	},
}

// List returns the caller's live tokens, newest first, without secrets.
func (h *AgentAccessTokenHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	tokens, err := h.svc.List(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "DB_ERROR", "failed to list tokens")
		return
	}
	c.JSON(http.StatusOK, gin.H{"tokens": tokens})
}

type createAgentTokenRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

// Create issues a new PAT. The plaintext token appears exactly once in this
// response and is never persisted or logged.
func (h *AgentAccessTokenHandler) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	var req createAgentTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	issued, err := h.svc.Issue(c.Request.Context(), userID, req.Name, req.Scopes)
	if err != nil {
		createAgentTokenErrMap.write(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"token":      issued.Token,
		"token_info": issued.Info,
	})
}

// Revoke immediately disables the caller's own token (leak stop lever).
func (h *AgentAccessTokenHandler) Revoke(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	tokenID, ok := pathID(c, "id", "invalid token id")
	if !ok {
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), userID, tokenID); err != nil {
		revokeAgentTokenErrMap.write(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "revoked"})
}
