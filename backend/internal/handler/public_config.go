package handler

import (
	"net/http"
	"strings"

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

// PublicCollaborationDTO exposes only the publish-time invitee cap the
// frontend needs to size the collaborator picker. Daily limits, expiry and
// contributor capacity are server-only.
type PublicCollaborationDTO struct {
	MaxInviteesPerPublish int `json:"max_invitees_per_publish"`
}

// PublicPublishDTO exposes the operational publish type ordering so the
// frontend type grid follows runtime config instead of a baked-in list
// (T25 / FIX-41). Empty lists mean "not configured" and clients fall back.
type PublicPublishDTO struct {
	TypeOrderOriginal []string `json:"type_order_original"`
	TypeOrderFanwork  []string `json:"type_order_fanwork"`
}

// PublicLimitsDTO exposes only the per-type upload size caps the frontend
// enforces client-side (T25 / FIX-41). These are non-sensitive numeric caps;
// durations, reputations and rate numbers stay server-only.
type PublicLimitsDTO struct {
	VideoMaxMB      int `json:"video_max_mb"`
	ImageMaxMB      int `json:"image_max_mb"`
	TextMaxMB       int `json:"text_max_mb"`
	ModMaxMB        int `json:"mod_max_mb"`
	SheetMusicMaxMB int `json:"sheet_music_max_mb"`
}

// PublicSocialDTO exposes the comment fold ratio the frontend needs for
// noise reduction display (T47 / FIX-29c). Auto-hide rates and other
// moderation knobs stay server-only.
type PublicSocialDTO struct {
	CommentFoldThreshold float64 `json:"comment_fold_threshold"`
}

// PublicAgentDTO exposes the non-sensitive agent model identity so clients
// and evidence tooling can show what is actually running. Provider and model
// names only — never credentials, keyed endpoints or limits.
type PublicAgentDTO struct {
	ChatProvider      string `json:"chat_provider"`
	ChatModel         string `json:"chat_model"`
	EmbeddingProvider string `json:"embedding_provider,omitempty"`
	EmbeddingModel    string `json:"embedding_model,omitempty"`
}

type PublicConfigResponse struct {
	Features      PublicFeaturesDTO      `json:"features"`
	Captcha       PublicCaptchaDTO       `json:"captcha"`
	Client        PublicClientDTO        `json:"client"`
	Legal         PublicLegalDTO         `json:"legal"`
	Upload        PublicUploadDTO        `json:"upload"`
	Collaboration PublicCollaborationDTO `json:"collaboration"`
	Publish       PublicPublishDTO       `json:"publish"`
	Limits        PublicLimitsDTO        `json:"limits"`
	Social        PublicSocialDTO        `json:"social"`
	Agent         PublicAgentDTO         `json:"agent"`
	// OSSDomain is the configured object delivery domain (#111). Clients use
	// it to compose stable object URLs from upload grants (e.g. avatar_url =
	// oss_domain + "/" + oss_key). Empty when delivery is not configured.
	OSSDomain string `json:"oss_domain"`
}

type PublicConfigHandler struct {
	cfg *config.Config
}

func NewPublicConfigHandler(cfg *config.Config) *PublicConfigHandler {
	return &PublicConfigHandler{cfg: cfg}
}

func (h *PublicConfigHandler) GetPublicConfig(c *gin.Context) {
	upload := h.cfg.Upload.NormalizedGalleryLimits()
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
			ImageGalleryMinItems: upload.ImageGalleryMinItems,
			ImageGalleryMaxItems: upload.ImageGalleryMaxItems,
			VideoGalleryMinItems: upload.VideoGalleryMinItems,
			VideoGalleryMaxItems: upload.VideoGalleryMaxItems,
		},
		Collaboration: PublicCollaborationDTO{
			MaxInviteesPerPublish: h.cfg.Collaboration.MaxInviteesPerPublish,
		},
		Publish: PublicPublishDTO{
			TypeOrderOriginal: h.cfg.Publish.TypeOrderOriginal,
			TypeOrderFanwork:  h.cfg.Publish.TypeOrderFanwork,
		},
		Limits: PublicLimitsDTO{
			VideoMaxMB:      h.cfg.Limits.VideoMaxMB,
			ImageMaxMB:      h.cfg.Limits.ImageMaxMB,
			TextMaxMB:       h.cfg.Limits.TextMaxMB,
			ModMaxMB:        h.cfg.Limits.ModMaxMB,
			SheetMusicMaxMB: h.cfg.Limits.SheetMusicMaxMB,
		},
		Social: PublicSocialDTO{
			CommentFoldThreshold: h.cfg.Social.CommentFoldThreshold,
		},
		Agent: PublicAgentDTO{
			ChatProvider:      h.cfg.Agent.LLMProvider,
			ChatModel:         h.cfg.Agent.LLMModel,
			EmbeddingProvider: strings.TrimSpace(h.cfg.Agent.EmbeddingProvider),
			EmbeddingModel:    h.cfg.Agent.EmbeddingModel,
		},
		OSSDomain: strings.TrimRight(strings.TrimSpace(h.cfg.OSS.Domain), "/"),
	}
	c.JSON(http.StatusOK, resp)
}
