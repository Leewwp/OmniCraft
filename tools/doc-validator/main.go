package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"omnicraft/tools/doc-validator/rules"
)

type Issue struct {
	Severity string // ERROR, WARNING
	File     string
	Message  string
}

type checkProfile struct {
	Name             string
	RunReleaseChecks bool
	RunExpiryChecks  bool
	CrossRefs        rules.CrossRefOptions
}

func main() {
	fix := flag.Bool("fix", false, "Auto-fix architecture.md auto-generated sections")
	check := flag.Bool("check", false, "Run consistency checks")
	diff := flag.Bool("diff", false, "Only check files changed in git diff")
	profileName := flag.String("profile", "release", "Validation profile: release, archive, or all")
	flag.Parse()
	profile, err := resolveProfile(*profileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	projectRoot := findProjectRoot()
	if projectRoot == "" {
		fmt.Fprintf(os.Stderr, "error: cannot find project root (no architecture.md with backend/config.yaml)\n")
		os.Exit(1)
	}

	if err := os.Chdir(projectRoot); err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot change to project root %s: %v\n", projectRoot, err)
		os.Exit(1)
	}

	if *fix {
		runFix()
	} else if *check {
		fmt.Printf("Profile: %s\n", profile.Name)
		issues := runCheck(*diff, profile)
		if len(issues) > 0 {
			for _, issue := range issues {
				fmt.Printf("[%s] %s: %s\n", issue.Severity, issue.File, issue.Message)
			}
			fmt.Printf("\n%d issue(s) found\n", len(issues))
			os.Exit(1)
		} else {
			fmt.Println("All checks passed")
		}
	} else {
		fmt.Println("Usage: doc-validator --check | --fix [--diff] [--profile release|archive|all]")
		os.Exit(1)
	}
}

func resolveProfile(name string) (checkProfile, error) {
	if name == "" {
		name = "release"
	}

	switch name {
	case "release":
		return checkProfile{
			Name:             name,
			RunReleaseChecks: true,
			RunExpiryChecks:  true,
		}, nil
	case "archive":
		return checkProfile{
			Name:      name,
			CrossRefs: rules.CrossRefOptions{OnlyArchive: true},
		}, nil
	case "all":
		return checkProfile{
			Name:             name,
			RunReleaseChecks: true,
			RunExpiryChecks:  true,
			CrossRefs:        rules.CrossRefOptions{IncludeArchive: true},
		}, nil
	default:
		return checkProfile{}, fmt.Errorf("unknown profile %q (expected release, archive, or all)", name)
	}
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "architecture.md")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "backend", "config.yaml")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func runFix() {
	fmt.Println("Fixing architecture.md auto-generated sections...")
	if err := rules.SyncConfigFields(); err != nil {
		fmt.Fprintf(os.Stderr, "config sync: %v\n", err)
	}
	if err := rules.SyncSchemaDocs(); err != nil {
		fmt.Fprintf(os.Stderr, "schema sync: %v\n", err)
	}
	if err := rules.SyncRouteList(); err != nil {
		fmt.Fprintf(os.Stderr, "route sync: %v\n", err)
	}
	fmt.Println("Done. Review changes to architecture.md before committing.")
}

func runCheck(diffOnly bool, profile checkProfile) []Issue {
	var issues []Issue
	if profile.RunReleaseChecks {
		syncIssues := rules.CheckConfigSync()
		for _, i := range syncIssues {
			issues = append(issues, Issue{Severity: i.Severity, File: i.File, Message: i.Message})
		}
		schemaIssues := rules.CheckSchemaSync()
		for _, i := range schemaIssues {
			issues = append(issues, Issue{Severity: i.Severity, File: i.File, Message: i.Message})
		}
		routeIssues := rules.CheckRouteSync()
		for _, i := range routeIssues {
			issues = append(issues, Issue{Severity: i.Severity, File: i.File, Message: i.Message})
		}
		tokenIssues := rules.CheckTokenRefs()
		for _, i := range tokenIssues {
			issues = append(issues, Issue{Severity: i.Severity, File: i.File, Message: i.Message})
		}
	}
	crossIssues := rules.CheckCrossRefs(profile.CrossRefs)
	for _, i := range crossIssues {
		issues = append(issues, Issue{Severity: i.Severity, File: i.File, Message: i.Message})
	}
	if profile.RunExpiryChecks {
		expiredIssues := rules.CheckExpiredDocs()
		for _, i := range expiredIssues {
			issues = append(issues, Issue{Severity: i.Severity, File: i.File, Message: i.Message})
		}
	}
	return issues
}
