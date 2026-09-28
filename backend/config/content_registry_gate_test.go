package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ticket #687 source-contract gate: outside the allowlist below, no backend
// Go file may enumerate the full content-type vocabulary — the registry
// (config/content_registry.go + YAML section) is the single taxonomy source.
// Precedent: internal/router/routes_test.go source-contract style.
//
// Detection: a 25-line sliding window containing quoted occurrences of at
// least 8 of the 9 canonical type names is an enumeration site (arrays,
// map literals and switch cases all satisfy this shape).
//
// Allowlist entries carry their exemption reason; adding a new entry needs
// a census-table justification in the PR (#687 迁/留裁决账).
var contentTypeEnumerationAllowlist = map[string]string{
	"content_registry.go": "DefaultContentRegistry — the sanctioned single definition of the shipped baseline (mirrors config.yaml; equivalence pinned by TestDefaultRegistryMatchesShippedYAML)",
	"usage_templates.go":  "embedded usage-template bundle manifest — enumerates shipped template JSON files, not the taxonomy; unknown types fall back to the other template",
}

var canonicalContentTypeNames = []string{
	"image", "article", "video", "audio", "template", "sheet_music", "mod", "prompt", "other",
}

func TestNoContentTypeEnumerationOutsideAllowlist(t *testing.T) {
	roots := []string{".." + string(filepath.Separator) + "internal", ".." + string(filepath.Separator) + "cmd"}
	violations := []string{}

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			base := filepath.Base(path)
			if _, exempt := contentTypeEnumerationAllowlist[base]; exempt {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if windowContainsEnumeration(string(raw)) {
				rel, _ := filepath.Rel("..", path)
				violations = append(violations, rel)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	for _, violation := range violations {
		t.Errorf("full content-type enumeration found in %s — consume the registry (config.Effective*/IsRegisteredContentType) or add a census-justified allowlist entry", violation)
	}
}

// windowContainsEnumeration reports whether any 25-line window quotes at
// least 8 of the 9 canonical type names.
func windowContainsEnumeration(source string) bool {
	lines := strings.Split(source, "\n")
	const window = 25
	for start := 0; start < len(lines); start += 1 {
		end := start + window
		if end > len(lines) {
			end = len(lines)
		}
		found := map[string]bool{}
		for _, line := range lines[start:end] {
			for _, name := range canonicalContentTypeNames {
				if strings.Contains(line, `"`+name+`"`) {
					found[name] = true
				}
			}
		}
		if len(found) >= 8 {
			return true
		}
	}
	return false
}
