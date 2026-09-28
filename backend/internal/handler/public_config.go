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

// PublicAttachmentPolicyDTO mirrors the registry's attachment_policy so the
// publish form can pre-validate client-side (#688 evaluator consumes it;
// #690 turns it on by configuration).
type PublicAttachmentPolicyDTO struct {
	RequiredAnyOf []string `json:"required_any_of"`
}

// PublicContentTypeDTO projects one content_types registry row (#687
// additive projection). ClientAccept carries the server-derived file-input
// accept attribute for the type's attachment set: "*" when any allowed
// family is extension-unrestricted (never silently tighten), otherwise the
// explicit extension union; empty when the type takes no attachments.
type PublicContentTypeDTO struct {
	Key               string                    `json:"key"`
	Zones             []string                  `json:"zones"`
	Form              string                    `json:"form"`
	UploadFileTypes   []string                  `json:"upload_file_types"`
	JudgeEligible     bool                      `json:"judge_eligible"`
	ClientAccept      string                    `json:"client_accept,omitempty"`
	AttachmentPolicy  *PublicAttachmentPolicyDTO `json:"attachment_policy,omitempty"`
}

// PublicUploadFileTypeDTO projects the safe slice of one upload capability
// row: the resolved explicit extension list (nil = unrestricted,
// MIME-driven) and the live size cap resolved through the limit key. MIME
// rules and the scannable flag stay server-authoritative.
type PublicUploadFileTypeDTO struct {
	Key        string   `json:"key"`
	Extensions []string `json:"extensions"`
	MaxMB      int      `json:"max_mb"`
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
	ContentTypes  []PublicContentTypeDTO  `json:"content_types"`
	UploadFileTypes []PublicUploadFileTypeDTO `json:"upload_file_types"`
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
		ContentTypes:    buildPublicContentTypes(h.cfg),
		UploadFileTypes: buildPublicUploadFileTypes(h.cfg),
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

// buildPublicContentTypes projects the effective content_types registry.
func buildPublicContentTypes(cfg *config.Config) []PublicContentTypeDTO {
	entries := cfg.EffectiveContentTypes()
	out := make([]PublicContentTypeDTO, 0, len(entries))
	for _, entry := range entries {
		dto := PublicContentTypeDTO{
			Key:             entry.Key,
			Zones:           entry.Zones,
			Form:            entry.Form,
			UploadFileTypes: entry.UploadFileTypes,
			JudgeEligible:   entry.JudgeEligible != nil && *entry.JudgeEligible,
			ClientAccept:    cfg.ClientAcceptForContentType(entry.Key),
		}
		if entry.AttachmentPolicy != nil {
			dto.AttachmentPolicy = &PublicAttachmentPolicyDTO{
				RequiredAnyOf: entry.AttachmentPolicy.RequiredAnyOf,
			}
		}
		out = append(out, dto)
	}
	return out
}

// buildPublicUploadFileTypes projects the safe slice of the capability
// registry: resolved extension list (nil = unrestricted) + live max_mb.
func buildPublicUploadFileTypes(cfg *config.Config) []PublicUploadFileTypeDTO {
	families := cfg.EffectiveUploadFileTypes()
	out := make([]PublicUploadFileTypeDTO, 0, len(families))
	for _, family := range families {
		// nil extensions (= JSON null) means unrestricted/MIME-driven; an
		// explicit array is the whitelist. Do not collapse nil to [].
		out = append(out, PublicUploadFileTypeDTO{
			Key:        family.Key,
			Extensions: cfg.FamilyExtensions(family.Key),
			MaxMB:      cfg.FamilyMaxMB(family.Key),
		})
	}
	return out
}
