package handler

import (
	"errors"
	"net/http"
	"omnicraft/backend/internal/pkg/response"
	"strconv"

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
		switch {
		case errors.Is(err, service.ErrAgentTokenNameInvalid),
			errors.Is(err, service.ErrAgentTokenScopesInvalid):
			response.Error(c, http.StatusBadRequest, "AGENT_TOKEN_INVALID", "invalid token name or scopes")
		case errors.Is(err, service.ErrAgentTokenLimitReached):
			response.Error(c, http.StatusConflict, "AGENT_TOKEN_LIMIT_REACHED", "too many active tokens, revoke one first")
		default:
			response.Error(c, http.StatusInternalServerError, "DB_ERROR", "failed to create token")
		}
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
	tokenID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid token id")
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), userID, tokenID); err != nil {
		if errors.Is(err, service.ErrAgentTokenNotFound) {
			response.Error(c, http.StatusNotFound, "AGENT_TOKEN_NOT_FOUND", "token not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "DB_ERROR", "failed to revoke token")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "revoked"})
}
