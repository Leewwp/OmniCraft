package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/config"
)

type PublicFeaturesDTO struct {
	WebAgentEnabled       bool `json:"web_agent_enabled"`
	PaymentEnabled        bool `json:"payment_enabled"`
	CreatorSupportEnabled bool `json:"creator_support_enabled"`
	DesktopDeployEnabled  bool `json:"desktop_deploy_enabled"`
}

type PublicCaptchaDTO struct {
	Provider string `json:"provider"`
	Prefix   string `json:"prefix"`
	SceneID  string `json:"scene_id"`
	Region   string `json:"region"`
}

type PublicClientDTO struct {
	DownloadEnabled bool   `json:"download_enabled"`
	DownloadURL     string `json:"download_url"`
	LatestVersion   string `json:"latest_version"`
}

type PublicConfigResponse struct {
	Features PublicFeaturesDTO `json:"features"`
	Captcha  PublicCaptchaDTO  `json:"captcha"`
	Client   PublicClientDTO   `json:"client"`
}

type PublicConfigHandler struct {
	cfg *config.Config
}

func NewPublicConfigHandler(cfg *config.Config) *PublicConfigHandler {
	return &PublicConfigHandler{cfg: cfg}
}

func (h *PublicConfigHandler) GetPublicConfig(c *gin.Context) {
	resp := PublicConfigResponse{
		Features: PublicFeaturesDTO{
			WebAgentEnabled:       h.cfg.Agent.WebAgentEnabled,
			PaymentEnabled:        h.cfg.Features.PaymentEnabled,
			CreatorSupportEnabled: h.cfg.Features.CreatorSupportEnabled,
			DesktopDeployEnabled:  h.cfg.Features.DesktopDeployEnabled,
		},
		Captcha: PublicCaptchaDTO{
			Provider: h.cfg.Captcha.Provider,
			Prefix:   h.cfg.Captcha.Prefix,
			SceneID:  h.cfg.Captcha.SceneID,
			Region:   h.cfg.Captcha.Region,
		},
		Client: PublicClientDTO{
			DownloadEnabled: h.cfg.Client.DownloadEnabled,
			DownloadURL:     h.cfg.Client.DownloadURL,
			LatestVersion:   h.cfg.Client.LatestVersion,
		},
	}
	c.JSON(http.StatusOK, resp)
}
