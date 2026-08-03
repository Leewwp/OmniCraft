package rules

import (
	"fmt"
	"os"
	"regexp"
)

// definedTokensIn extracts --token definitions in both prose (`--name: value`)
// and backtick-quoted table cell (`--name`) forms.
func definedTokensIn(content string) map[string]bool {
	tokenDefRe := regexp.MustCompile(`--([a-zA-Z][a-zA-Z0-9-]*)\s*:`)
	tokenCellRe := regexp.MustCompile("`--([a-zA-Z][a-zA-Z0-9-]*)`")
	defined := make(map[string]bool)
	for _, match := range tokenDefRe.FindAllStringSubmatch(content, -1) {
		defined["--"+match[1]] = true
	}
	for _, match := range tokenCellRe.FindAllStringSubmatch(content, -1) {
		defined["--"+match[1]] = true
	}
	return defined
}

// CheckTokenRefs scans ui-spec.md for CSS variable references and checks
// that each referenced token is defined in design-system.md.
func CheckTokenRefs() []RuleIssue {
	var issues []RuleIssue

	// Read design-system.md to build set of defined tokens
	dsPath := "design/design-system.md"
	dsContent, err := os.ReadFile(dsPath)
	if err != nil {
		return []RuleIssue{{Severity: "ERROR", File: dsPath, Message: fmt.Sprintf("cannot read: %v", err)}}
	}

	definedTokens := definedTokensIn(string(dsContent))

	// Read ui-spec.md for token references
	uiPath := "design/ui-spec.md"
	uiContent, err := os.ReadFile(uiPath)
	if err != nil {
		return []RuleIssue{{Severity: "ERROR", File: uiPath, Message: fmt.Sprintf("cannot read: %v", err)}}
	}

	// Extract all var(--xxx) references from ui-spec.md
	varRefRe := regexp.MustCompile(`var\(--([a-zA-Z][a-zA-Z0-9-]*)\)`)
	seen := make(map[string]bool)
	for _, match := range varRefRe.FindAllStringSubmatch(string(uiContent), -1) {
		tokenName := "--" + match[1]
		if seen[tokenName] {
			continue
		}
		seen[tokenName] = true
		if !definedTokens[tokenName] {
			issues = append(issues, RuleIssue{
				Severity: "WARNING",
				File:     uiPath,
				Message:  fmt.Sprintf("token %s referenced in ui-spec.md but not defined in design-system.md", tokenName),
			})
		}
	}

	return issues
}
