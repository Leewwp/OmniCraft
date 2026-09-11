package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service/usage_templates"
)

var (
	ErrUsageGuideContentNotFound = errors.New("content not found")
	ErrUsageGuideForbidden       = errors.New("only the content author can edit the usage guide")
	ErrUsageGuideInvalidLocale   = errors.New("locale must be zh or en")
	ErrUsageGuideInvalidPayload  = errors.New("usage guide payload is invalid")
)

// UsageGuideService serves the merged system-template + content-specifics
// view (SP-16 #447 / spec D4). Merge rule per field: specifics win when the
// row exists; Safety always comes from the platform template (authors cannot
// remove the safety floor). Missing specifics degrade to the pure template.
type UsageGuideService struct {
	guideRepo   *repository.UsageGuideRepository
	contentRepo *repository.ContentRepository
}

func NewUsageGuideService(guideRepo *repository.UsageGuideRepository, contentRepo *repository.ContentRepository) *UsageGuideService {
	return &UsageGuideService{guideRepo: guideRepo, contentRepo: contentRepo}
}

// UsageGuideView is the wire contract of GET /api/v1/contents/:id/guide.
type UsageGuideView struct {
	ContentID   int64    `json:"content_id"`
	ContentType string   `json:"content_type"`
	Locale      string   `json:"locale"`
	Requirements []string `json:"requirements"`
	Steps       []string `json:"steps"`
	Notes       string   `json:"notes"`
	Safety      []string `json:"safety"`
	// Source reports the specifics provenance: "author" | "llm_assisted";
	// empty when the view degraded to the pure system template.
	Source           string  `json:"source,omitempty"`
	SpecificsUpdated *string `json:"specifics_updated_at,omitempty"`
	TemplateVersion  string  `json:"template_version"`
	ETag             string  `json:"-"`
}

func normalizeLocale(locale string) (string, error) {
	switch locale {
	case "", "zh":
		return "zh", nil
	case "en":
		return "en", nil
	default:
		return "", ErrUsageGuideInvalidLocale
	}
}

// GetMergedView builds the merged guide view for a viewer. The caller has
// already enforced content visibility (the anonymous GET route 404s on
// invisible content); admin/author viewers share this read path.
func (s *UsageGuideService) GetMergedView(ctx context.Context, contentID int64, locale string) (*UsageGuideView, error) {
	locale, err := normalizeLocale(locale)
	if err != nil {
		return nil, err
	}
	content, err := s.contentRepo.FindByID(contentID)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, ErrUsageGuideContentNotFound
	}

	tpl, err := usage_templates.Get(content.ContentType, locale)
	if err != nil {
		return nil, err
	}

	view := &UsageGuideView{
		ContentID:       content.ID,
		ContentType:     content.ContentType,
		Locale:          locale,
		Requirements:    tpl.Requirements,
		Steps:           tpl.Steps,
		Notes:           tpl.Notes,
		Safety:          tpl.Safety,
		TemplateVersion: usage_templates.Version,
	}

	var specificsUpdatedAt string
	if specifics, err := s.guideRepo.Find(ctx, contentID, locale); err != nil {
		return nil, err
	} else if specifics != nil {
		if req, ok := parseStringArray(specifics.Requirements); ok && len(req) > 0 {
			view.Requirements = req
		}
		if steps, ok := parseStringArray(specifics.Steps); ok && len(steps) > 0 {
			view.Steps = steps
		}
		if strings.TrimSpace(specifics.Notes) != "" {
			view.Notes = specifics.Notes
		}
		view.Source = specifics.Source
		updated := specifics.UpdatedAt.UTC().Format(time.RFC3339)
		view.SpecificsUpdated = &updated
		specificsUpdatedAt = updated
	}

	view.ETag = computeUsageGuideETag(view, specificsUpdatedAt)
	return view, nil
}

func computeUsageGuideETag(view *UsageGuideView, specificsUpdatedAt string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%s|%s|%s|%s", view.ContentID, view.Locale, view.TemplateVersion, view.Source, specificsUpdatedAt)
	return `"` + hex.EncodeToString(h.Sum(nil))[:32] + `"`
}

// SaveSpecifics is the studio write path (author-only): upserts the
// (content_id, locale) row after validating the payload shape.
func (s *UsageGuideService) SaveSpecifics(ctx context.Context, callerID int64, contentID int64, input UsageGuideInput) error {
	locale, err := normalizeLocale(input.Locale)
	if err != nil {
		return err
	}
	content, err := s.contentRepo.FindByID(contentID)
	if err != nil {
		return err
	}
	if content == nil {
		return ErrUsageGuideContentNotFound
	}
	if callerID != content.AuthorID {
		return ErrUsageGuideForbidden
	}

	source := model.UsageGuideSourceAuthor
	switch input.Source {
	case "", model.UsageGuideSourceAuthor:
	case model.UsageGuideSourceLLMAssisted:
		source = model.UsageGuideSourceLLMAssisted
	default:
		return ErrUsageGuideInvalidPayload
	}

	if len(input.Requirements) > 50 || len(input.Steps) > 50 {
		return ErrUsageGuideInvalidPayload
	}
	for _, item := range append(append([]string{}, input.Requirements...), input.Steps...) {
		if len(item) > 500 {
			return ErrUsageGuideInvalidPayload
		}
	}
	if len(input.Notes) > 5000 {
		return ErrUsageGuideInvalidPayload
	}

	reqJSON, err := marshalStringArray(input.Requirements)
	if err != nil {
		return err
	}
	stepsJSON, err := marshalStringArray(input.Steps)
	if err != nil {
		return err
	}

	row := &model.ContentUsageGuide{
		ContentItemID: contentID,
		Locale:        locale,
		Requirements:  reqJSON,
		Steps:         stepsJSON,
		Notes:         strings.TrimSpace(input.Notes),
		Source:        source,
	}
	return s.guideRepo.Upsert(ctx, row)
}

// UsageGuideInput is the PUT body of the studio guide editor.
type UsageGuideInput struct {
	Locale       string   `json:"locale"`
	Requirements []string `json:"requirements"`
	Steps        []string `json:"steps"`
	Notes        string   `json:"notes"`
	Source       string   `json:"source"`
}

// GetSpecifics serves the studio editor's initial state (author view of the
// saved row, nullable).
func (s *UsageGuideService) GetSpecifics(ctx context.Context, callerID int64, contentID int64, locale string) (*model.ContentUsageGuide, error) {
	locale, err := normalizeLocale(locale)
	if err != nil {
		return nil, err
	}
	content, err := s.contentRepo.FindByID(contentID)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, ErrUsageGuideContentNotFound
	}
	if callerID != content.AuthorID {
		return nil, ErrUsageGuideForbidden
	}
	return s.guideRepo.Find(ctx, contentID, locale)
}

func parseStringArray(raw string) ([]string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, false
	}
	var out []string
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, false
	}
	return out, true
}

func marshalStringArray(items []string) (string, error) {
	if items == nil {
		items = []string{}
	}
	data, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
