package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"omnicraft/backend/internal/pkg/response"

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
		response.CaptchaError(c, http.StatusBadRequest, "VALIDATION_ERROR", "captcha verification parameter required")
		return
	}

	if h.providerVerifier == nil {
		response.CaptchaError(c, http.StatusServiceUnavailable, "CAPTCHA_UNAVAILABLE", "captcha verification is temporarily unavailable")
		return
	}
	if err := h.providerVerifier.Verify(c.Request.Context(), req.CaptchaVerifyParam, c.ClientIP()); err != nil {
		slog.Warn("captcha provider verification failed", "error", err)
		response.CaptchaError(c, http.StatusBadRequest, "CAPTCHA_FAILED", "captcha verification failed")
		return
	}

	ticket, err := h.tickets.Issue(c.Request.Context())
	if err != nil {
		status := http.StatusServiceUnavailable
		if !errors.Is(err, captcha.ErrTicketStoreUnavailable) {
			status = http.StatusInternalServerError
		}
		slog.Error("captcha ticket issue failed", "error", err)
		response.CaptchaError(c, status, "CAPTCHA_UNAVAILABLE", "captcha verification is temporarily unavailable")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"captcha_result": true,
		"captcha_token":  ticket,
	})
}
