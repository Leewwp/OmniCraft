package handler

import (
	"errors"
	"net/http"
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
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "authentication required"})
		return
	}
	tokens, err := h.svc.List(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to list tokens"})
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
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "authentication required"})
		return
	}
	var req createAgentTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_REQUEST", "message": "invalid request body"})
		return
	}
	issued, err := h.svc.Issue(c.Request.Context(), userID, req.Name, req.Scopes)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAgentTokenNameInvalid),
			errors.Is(err, service.ErrAgentTokenScopesInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"code": "AGENT_TOKEN_INVALID", "message": "invalid token name or scopes"})
		case errors.Is(err, service.ErrAgentTokenLimitReached):
			c.JSON(http.StatusConflict, gin.H{"code": "AGENT_TOKEN_LIMIT_REACHED", "message": "too many active tokens, revoke one first"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to create token"})
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
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "authentication required"})
		return
	}
	tokenID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid token id"})
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), userID, tokenID); err != nil {
		if errors.Is(err, service.ErrAgentTokenNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "AGENT_TOKEN_NOT_FOUND", "message": "token not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "failed to revoke token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "revoked"})
}
