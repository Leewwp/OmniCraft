package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckCrossRefsReleaseSkipsOnlyArchive(t *testing.T) {
	withCrossRefFixture(t, map[string]string{
		"docs/current.md":     "[active](missing-active.md)",
		"docs/archive/old.md": "[archive](missing-archive.md)",
	}, func() {
		issues := CheckCrossRefs(CrossRefOptions{})
		if len(issues) != 1 {
			t.Fatalf("release issues = %d, want 1: %#v", len(issues), issues)
		}
		if filepath.Clean(issues[0].File) != filepath.Clean("docs/current.md") {
			t.Fatalf("release issue file = %q, want active document", issues[0].File)
		}
	})
}

func TestCheckCrossRefsArchiveChecksOnlyArchive(t *testing.T) {
	withCrossRefFixture(t, map[string]string{
		"docs/current.md":     "[active](missing-active.md)",
		"docs/archive/old.md": "[archive](missing-archive.md)",
	}, func() {
		issues := CheckCrossRefs(CrossRefOptions{OnlyArchive: true})
		if len(issues) != 1 {
			t.Fatalf("archive issues = %d, want 1: %#v", len(issues), issues)
		}
		if filepath.Clean(issues[0].File) != filepath.Clean("docs/archive/old.md") {
			t.Fatalf("archive issue file = %q, want archived document", issues[0].File)
		}
	})
}

func TestCheckCrossRefsAllIncludesActiveAndArchive(t *testing.T) {
	withCrossRefFixture(t, map[string]string{
		"docs/current.md":     "[active](missing-active.md)",
		"docs/archive/old.md": "[archive](missing-archive.md)",
	}, func() {
		issues := CheckCrossRefs(CrossRefOptions{IncludeArchive: true})
		if len(issues) != 2 {
			t.Fatalf("all issues = %d, want 2: %#v", len(issues), issues)
		}
	})
}

func TestCheckCrossRefsReleaseKeepsEveryActiveDocumentationScope(t *testing.T) {
	withCrossRefFixture(t, map[string]string{
		"docs/current.md":                     "[missing](missing.md)",
		"docs/superpowers/specs/spec.md":      "[missing](missing.md)",
		"docs/superpowers/plans/plan.md":      "[missing](missing.md)",
		"docs/working/guide.md":               "[missing](missing.md)",
		"design/ui-spec.md":                   "[missing](missing.md)",
		".specify/memory/constitution.md":     "[missing](missing.md)",
		"docs/archive/ignored/archive-doc.md": "[missing](missing.md)",
	}, func() {
		issues := CheckCrossRefs(CrossRefOptions{})
		if len(issues) != 6 {
			t.Fatalf("release active-scope issues = %d, want 6: %#v", len(issues), issues)
		}
	})
}

func TestFileReferenceRequiresAnAbsolutePathAfterScheme(t *testing.T) {
	withCrossRefFixture(t, map[string]string{
		"docs/current.md": strings.Join([]string{
			"Windows: file:///C:/definitely-missing/omnicraft.md",
			"POSIX: file:///definitely-missing/omnicraft.md",
			"Parentheses: [route](file:///C:/definitely-missing/(protected)/missing.md)",
			"Bare prose: file:///），检查目标文件是否存在。",
		}, "\n"),
	}, func() {
		issues := CheckCrossRefs(CrossRefOptions{})
		if len(issues) != 3 {
			t.Fatalf("file URI issues = %d, want 3: %#v", len(issues), issues)
		}
		foundParenthesizedPath := false
		for _, issue := range issues {
			if strings.Contains(issue.Message, "检查目标文件") {
				t.Fatalf("bare file URI prose was treated as a path: %s", issue.Message)
			}
			if strings.Contains(filepath.ToSlash(issue.Message), "(protected)/missing.md") {
				foundParenthesizedPath = true
			}
		}
		if !foundParenthesizedPath {
			t.Fatal("absolute file URI containing balanced parentheses was not checked")
		}
	})
}

func TestMarkdownReferenceAnchorIsNotPartOfFilename(t *testing.T) {
	withCrossRefFixture(t, map[string]string{
		"docs/current.md": "[target](target.md#section)",
		"docs/target.md":  "# Section",
	}, func() {
		if issues := CheckCrossRefs(CrossRefOptions{}); len(issues) != 0 {
			t.Fatalf("anchor-only reference produced issues: %#v", issues)
		}
	})
}

func withCrossRefFixture(t *testing.T, files map[string]string, run func()) {
	t.Helper()
	root := t.TempDir()
	for relativePath, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir fixture: %v", err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	run()
}
