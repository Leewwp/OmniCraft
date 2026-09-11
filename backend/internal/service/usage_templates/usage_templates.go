// Package usage_templates ships the system template layer of the usage-guide
// model (SP-16 #447 / spec D4): one thin generic install/usage template per
// content type, bilingual (zh/en), versioned with the repo and embedded into
// the backend binary. v1 is deliberately thin — per-game mod knowledge is
// explicitly out of scope (spec §8-5, Phase 2).
package usage_templates

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed templates/*.json
var templateFS embed.FS

// Version identifies the template bundle generation; it participates in the
// guide endpoint's ETag so shipping new templates invalidates caches.
const Version = "v1"

// SupportedLocales lists the locales the bundle ships.
var SupportedLocales = []string{"zh", "en"}

// ContentTypes lists the content types with a dedicated template. The order
// mirrors the studio type grid.
var ContentTypes = []string{
	"mod", "sheet_music", "template", "audio", "video",
	"image", "article", "prompt", "other",
}

// Template is the structured form of one template file. Requirements/Steps
// are plain string arrays; Notes is free Markdown; Safety is the
// platform-owned safety floor that content-level specifics never override.
type Template struct {
	ContentType  string   `json:"content_type"`
	Locale       string   `json:"locale"`
	Requirements []string `json:"requirements"`
	Steps        []string `json:"steps"`
	Notes        string   `json:"notes"`
	Safety       []string `json:"safety"`
}

// Get returns the template for a content type and locale. Unknown locales
// fall back to zh; unknown content types fall back to the generic "other"
// template so the guide endpoint can always serve a sane view.
func Get(contentType, locale string) (Template, error) {
	if locale != "zh" && locale != "en" {
		locale = "zh"
	}
	t, err := load(contentType, locale)
	if err != nil {
		other, otherErr := load("other", locale)
		if otherErr != nil {
			return Template{}, otherErr
		}
		return other, nil
	}
	return t, nil
}

func load(contentType, locale string) (Template, error) {
	raw, err := templateFS.ReadFile(fmt.Sprintf("templates/%s.%s.json", contentType, locale))
	if err != nil {
		return Template{}, fmt.Errorf("usage template %s/%s: %w", contentType, locale, err)
	}
	var t Template
	if err := json.Unmarshal(raw, &t); err != nil {
		return Template{}, fmt.Errorf("usage template %s/%s malformed: %w", contentType, locale, err)
	}
	if strings.TrimSpace(t.ContentType) == "" || len(t.Steps) == 0 {
		return Template{}, fmt.Errorf("usage template %s/%s incomplete", contentType, locale)
	}
	return t, nil
}
