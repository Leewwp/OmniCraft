package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type CrossRefOptions struct {
	IncludeArchive bool
	OnlyArchive    bool
}

var (
	markdownLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+\.md[^)]*)\)`)
)

// CheckCrossRefs scans all .md files for file links and checks that
// target files exist on disk.
func CheckCrossRefs(options CrossRefOptions) []RuleIssue {
	var issues []RuleIssue

	dirsToScan := []string{"docs", "design", ".specify"}
	if options.OnlyArchive {
		dirsToScan = []string{filepath.Join("docs", "archive")}
	}

	for _, dir := range dirsToScan {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil {
				return nil
			}
			if info.IsDir() {
				if !options.IncludeArchive && !options.OnlyArchive && isArchivePath(path) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".md") {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			// Check markdown links
			for _, match := range markdownLinkPattern.FindAllStringSubmatch(string(content), -1) {
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
			for _, uriPath := range fileURIPaths(string(content)) {
				ref := absoluteFileURIPath(uriPath)
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

func isArchivePath(path string) bool {
	archiveRoot := filepath.Clean(filepath.Join("docs", "archive"))
	cleanPath := filepath.Clean(path)
	return cleanPath == archiveRoot || strings.HasPrefix(cleanPath, archiveRoot+string(os.PathSeparator))
}

func absoluteFileURIPath(uriPath string) string {
	if len(uriPath) >= 3 && uriPath[1] == ':' && uriPath[2] == '/' {
		return filepath.FromSlash(uriPath)
	}
	return filepath.FromSlash("/" + uriPath)
}

func fileURIPaths(content string) []string {
	const prefix = "file:///"
	var paths []string

	for searchStart := 0; searchStart < len(content); {
		relativeStart := strings.Index(content[searchStart:], prefix)
		if relativeStart < 0 {
			break
		}
		pathStart := searchStart + relativeStart + len(prefix)
		uriPath, consumed, ok := parseFileURIPath(content[pathStart:])
		if ok {
			paths = append(paths, uriPath)
		}
		if consumed == 0 {
			_, consumed = utf8.DecodeRuneInString(content[pathStart:])
			if consumed == 0 {
				break
			}
		}
		searchStart = pathStart + consumed
	}

	return paths
}

func parseFileURIPath(text string) (string, int, bool) {
	if text == "" || !validFileURIPathStart(text) {
		return "", 0, false
	}

	parentheses := 0
	end := 0
	for offset, r := range text {
		if unicode.IsSpace(r) || strings.ContainsRune("<>\"'`", r) {
			break
		}
		if r == '(' {
			parentheses++
		} else if r == ')' {
			if parentheses == 0 {
				break
			}
			parentheses--
		}
		end = offset + utf8.RuneLen(r)
	}
	if end == 0 {
		return "", 0, false
	}
	return text[:end], end, true
}

func validFileURIPathStart(text string) bool {
	if len(text) >= 3 && isASCIILetter(text[0]) && text[1] == ':' && text[2] == '/' {
		return true
	}
	r, _ := utf8.DecodeRuneInString(text)
	return unicode.IsLetter(r) || unicode.IsNumber(r) || strings.ContainsRune("._~%-", r)
}

func isASCIILetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
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
