package usage_templates

import (
	"fmt"
	"testing"
)

// SP-16 #447: the system template bundle must cover every content type in
// both locales with a usable (non-empty) template — the guide endpoint and
// the MCP guide tool both degrade onto this floor.
func TestTemplateBundleCoversAllTypesInBothLocales(t *testing.T) {
	for _, contentType := range ContentTypes {
		for _, locale := range SupportedLocales {
			tpl, err := Get(contentType, locale)
			if err != nil {
				t.Errorf("Get(%s, %s): %v", contentType, locale, err)
				continue
			}
			if tpl.ContentType != contentType && contentType != "other" {
				t.Errorf("Get(%s, %s) returned template for %q", contentType, locale, tpl.ContentType)
			}
			if tpl.Locale != locale {
				t.Errorf("Get(%s, %s) returned locale %q", contentType, locale, tpl.Locale)
			}
			if len(tpl.Steps) == 0 {
				t.Errorf("template %s/%s has no steps", contentType, locale)
			}
			if len(tpl.Safety) == 0 {
				t.Errorf("template %s/%s has no safety notes", contentType, locale)
			}
			if len(tpl.Requirements) == 0 {
				t.Errorf("template %s/%s has no requirements", contentType, locale)
			}
		}
	}
}

func TestGetFallsBackToOtherTemplateAndZhLocale(t *testing.T) {
	tpl, err := Get("does_not_exist", "zh")
	if err != nil {
		t.Fatalf("unknown content type should fall back to other: %v", err)
	}
	if tpl.ContentType != "other" {
		t.Fatalf("fallback returned %q, want other", tpl.ContentType)
	}

	tpl, err = Get("mod", "fr")
	if err != nil {
		t.Fatalf("unknown locale should fall back to zh: %v", err)
	}
	if tpl.Locale != "zh" {
		t.Fatalf("locale fallback returned %q, want zh", tpl.Locale)
	}
}

func TestVersionIsStableIdentifier(t *testing.T) {
	if Version == "" || fmt.Sprint(Version) == "" {
		t.Fatal("template bundle version must be a stable non-empty identifier (participates in ETags)")
	}
}
