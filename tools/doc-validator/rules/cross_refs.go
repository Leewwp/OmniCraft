package rules

// CheckCrossRefs validates that file links in documentation point to existing files.
// Stub implementation — full implementation in Task 16.
func CheckCrossRefs() []RuleIssue {
	// TODO (Task 16): Scan doc files for markdown links and cross-references,
	// verify the target files exist.
	return nil
}

// CheckExpiredDocs checks for expired docs/working/ documents against their stated expiry dates.
// Stub implementation — full implementation in Task 16.
func CheckExpiredDocs() []RuleIssue {
	// TODO (Task 16): Parse docs/working/ files for YAML front-matter
	// containing expiry dates; warn if any have passed.
	return nil
}
