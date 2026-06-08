package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/internal/pkg/captcha"
)

type CaptchaHandler struct {
	providerVerifier captcha.CaptchaVerifier
	tickets          *captcha.TicketStore
}

func NewCaptchaHandler(providerVerifier captcha.CaptchaVerifier, tickets *captcha.TicketStore) *CaptchaHandler {
	return &CaptchaHandler{providerVerifier: providerVerifier, tickets: tickets}
}

func (h *CaptchaHandler) Verify(c *gin.Context) {
	var req struct {
		CaptchaVerifyParam string `json:"captcha_verify_param" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": "captcha verification parameter required", "captcha_result": false})
		return
	}

	if h.providerVerifier == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "CAPTCHA_UNAVAILABLE", "message": "captcha verification is temporarily unavailable", "captcha_result": false})
		return
	}
	if err := h.providerVerifier.Verify(c.Request.Context(), req.CaptchaVerifyParam, c.ClientIP()); err != nil {
		slog.Warn("captcha provider verification failed", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"code": "CAPTCHA_FAILED", "message": "captcha verification failed", "captcha_result": false})
		return
	}

	ticket, err := h.tickets.Issue(c.Request.Context())
	if err != nil {
		status := http.StatusServiceUnavailable
		if !errors.Is(err, captcha.ErrTicketStoreUnavailable) {
			status = http.StatusInternalServerError
		}
		slog.Error("captcha ticket issue failed", "error", err)
		c.JSON(status, gin.H{"code": "CAPTCHA_UNAVAILABLE", "message": "captcha verification is temporarily unavailable", "captcha_result": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"captcha_result": true,
		"captcha_token":  ticket,
	})
}
