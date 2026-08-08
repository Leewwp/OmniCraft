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

type PublicLegalDTO struct {
	CurrentTermsVersion   string `json:"current_terms_version"`
	CurrentPrivacyVersion string `json:"current_privacy_version"`
}

// PublicUploadDTO exposes only the non-sensitive upload limits the frontend
// needs to enforce media set sizing without duplicating constants. It must
// never include credentials, keys or rate limits.
type PublicUploadDTO struct {
	ImageGalleryMinItems int `json:"image_gallery_min_items"`
	ImageGalleryMaxItems int `json:"image_gallery_max_items"`
	VideoGalleryMinItems int `json:"video_gallery_min_items"`
	VideoGalleryMaxItems int `json:"video_gallery_max_items"`
}

type PublicConfigResponse struct {
	Features PublicFeaturesDTO `json:"features"`
	Captcha  PublicCaptchaDTO  `json:"captcha"`
	Client   PublicClientDTO   `json:"client"`
	Legal    PublicLegalDTO    `json:"legal"`
	Upload   PublicUploadDTO   `json:"upload"`
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
		Legal: PublicLegalDTO{
			CurrentTermsVersion:   h.cfg.Legal.CurrentTermsVersion,
			CurrentPrivacyVersion: h.cfg.Legal.CurrentPrivacyVersion,
		},
		Upload: PublicUploadDTO{
			ImageGalleryMinItems: h.cfg.Upload.ImageGalleryMinItems,
			ImageGalleryMaxItems: h.cfg.Upload.ImageGalleryMaxItems,
			VideoGalleryMinItems: h.cfg.Upload.VideoGalleryMinItems,
			VideoGalleryMaxItems: h.cfg.Upload.VideoGalleryMaxItems,
		},
	}
	c.JSON(http.StatusOK, resp)
}
