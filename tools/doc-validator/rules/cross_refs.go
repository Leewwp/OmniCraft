package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// CheckCrossRefs scans all .md files for file links and checks that
// target files exist on disk.
func CheckCrossRefs() []RuleIssue {
	var issues []RuleIssue

	// Regex for markdown links: [text](path)
	linkRe := regexp.MustCompile(`\[([^\]]*)\]\(([^)]+\.md[^)]*)\)`)
	// Regex for file:/// references
	fileRefRe := regexp.MustCompile(`file:///([^\s<>"'\\)]+)`)

	// Walk docs/, design/, .specify/ for .md files
	dirsToScan := []string{"docs", "design", ".specify"}

	for _, dir := range dirsToScan {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			// Check markdown links
			for _, match := range linkRe.FindAllStringSubmatch(string(content), -1) {
				target := match[2]
				// Remove anchor part (#section)
				if idx := strings.Index(target, "#"); idx >= 0 {
					target = target[:idx]
				}
				if target == "" {
					continue
				}

				// Resolve relative path
				fullTarget := filepath.Join(filepath.Dir(path), target)
				if _, err := os.Stat(fullTarget); os.IsNotExist(err) {
					issues = append(issues, RuleIssue{
						Severity: "WARNING",
						File:     path,
						Message:  fmt.Sprintf("broken link: [%s](%s) → target not found", match[1], target),
					})
				}
			}

			// Check file:/// references
			for _, match := range fileRefRe.FindAllStringSubmatch(string(content), -1) {
				ref := match[1]
				if _, err := os.Stat(ref); os.IsNotExist(err) {
					issues = append(issues, RuleIssue{
						Severity: "WARNING",
						File:     path,
						Message:  fmt.Sprintf("broken file reference: %s", ref),
					})
				}
			}

			return nil
		})
	}

	return issues
}

// CheckExpiredDocs scans docs/working/ for documents past their expiry date.
func CheckExpiredDocs() []RuleIssue {
	var issues []RuleIssue

	workingDir := filepath.Join("docs", "working")

	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		return nil // Directory doesn't exist yet, not an error
	}

	entries, err := os.ReadDir(workingDir)
	if err != nil {
		return []RuleIssue{{Severity: "ERROR", File: workingDir, Message: fmt.Sprintf("cannot read: %v", err)}}
	}

	// Regex to extract expiry date from document header: **预计失效日期**: YYYY-MM-DD
	expiryRe := regexp.MustCompile(`\*\*预计失效日期\*\*:\s*(\d{4}-\d{2}-\d{2})`)
	today := time.Now().Format("2006-01-02")

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(workingDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		match := expiryRe.FindStringSubmatch(string(content))
		if len(match) < 2 {
			// No expiry date declared — warn
			issues = append(issues, RuleIssue{
				Severity: "WARNING",
				File:     path,
				Message:  "no expiry date declared (add `**预计失效日期**: YYYY-MM-DD` in header)",
			})
			continue
		}

		if match[1] < today {
			issues = append(issues, RuleIssue{
				Severity: "WARNING",
				File:     path,
				Message:  fmt.Sprintf("expired on %s — move to docs/archive/", match[1]),
			})
		}
	}

	return issues
}
