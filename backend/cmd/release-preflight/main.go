// Command release-preflight validates the effective production configuration:
// the .env file, the repository default config.yaml and the operator override
// YAML are merged into one effective config (mirroring backend config.Load),
// then Config.ValidateRelease plus deployment-level checks (trusted-proxy
// topology, green callback seed/uid, frontend/API DNS consistency, placeholder
// scanning of environment values) are run. The result is written as a redacted
// JSON summary that never echoes secret values.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"

	"omnicraft/backend/config"
)

var (
	greenSeedFormat = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)
	greenUIDFormat  = regexp.MustCompile(`^\d+$`)
)

type check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

type summary struct {
	Tool       string   `json:"tool"`
	CheckedAt  string   `json:"checked_at"`
	EnvFile    string   `json:"env_file"`
	Override   string   `json:"override_file"`
	Schema     string   `json:"schema"`
	Checks     []check  `json:"checks"`
	OK         bool     `json:"ok"`
	Redacted   bool     `json:"redacted"`
	FieldNames []string `json:"redacted_field_names"`
}

func main() {
	envFile := flag.String("EnvironmentFile", "", "production .env file (required)")
	overrideFile := flag.String("OverrideFile", "", "operator config override YAML (required)")
	schemaFile := flag.String("Schema", "", "production-config.schema.json (optional)")
	reportDir := flag.String("ReportDir", "", "output directory for preflight-summary.json")
	repoRoot := flag.String("RepoRoot", "", "repository root (default: auto-detected)")
	composeFile := flag.String("ComposeFile", "", "docker-compose file checked for read-only config volume (default: repo docs/deploy/docker-compose.single-server.yml)")
	flag.Parse()

	if *envFile == "" || *overrideFile == "" {
		fmt.Fprintln(os.Stderr, "usage: release-preflight -EnvironmentFile <file> -OverrideFile <file> [-Schema <file>] [-ReportDir <dir>] [-ComposeFile <file>]")
		os.Exit(2)
	}
	if _, err := os.Stat(*envFile); err != nil {
		fmt.Fprintf(os.Stderr, "release-preflight: environment file not readable: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(*overrideFile); err != nil {
		fmt.Fprintf(os.Stderr, "release-preflight: override file not readable: %v\n", err)
		os.Exit(1)
	}

	var reportDirAbs string
	if *reportDir != "" {
		reportDirAbs, _ = filepath.Abs(*reportDir)
		_ = os.MkdirAll(reportDirAbs, 0o755)
	}

	envAbs, _ := filepath.Abs(*envFile)
	overrideAbs, _ := filepath.Abs(*overrideFile)
	root := *repoRoot
	if root == "" {
		root = autoRepoRoot()
	}
	compose := *composeFile
	if compose == "" {
		compose = filepath.Join(root, "docs", "deploy", "docker-compose.single-server.yml")
	}

	checks := runChecks(envAbs, overrideAbs, root, compose)

	ok := true
	for _, c := range checks {
		if !c.OK {
			ok = false
		}
	}

	sum := summary{
		Tool:      "backend/cmd/release-preflight",
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		EnvFile:   envAbs,
		Override:  overrideAbs,
		Schema:    *schemaFile,
		Checks:    checks,
		OK:        ok,
		Redacted:  true,
	}
	if reportDirAbs != "" {
		raw, _ := json.MarshalIndent(sum, "", "  ")
		if err := os.WriteFile(filepath.Join(reportDirAbs, "preflight-summary.json"), append(raw, '\n'), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "release-preflight: cannot write summary: %v\n", err)
			os.Exit(1)
		}
	}
	for _, c := range checks {
		mark := "ok "
		if !c.OK {
			mark = "FAIL"
		}
		fmt.Printf("  [%s] %s %s\n", mark, c.Name, c.Detail)
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "release-preflight: production configuration check failed")
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "release-preflight: all checks passed")
}

func autoRepoRoot() string {
	wd, _ := os.Getwd()
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "backend", "config.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}

// runChecks merges env + override over the repository default config and runs
// ValidateRelease plus deployment-level checks. Secret values are never
// included in the returned checks; only field names and statuses are.
func runChecks(envFile, overrideFile, repoRoot, composeFile string) []check {
	var checks []check
	add := func(name string, ok bool, detail string) {
		checks = append(checks, check{Name: name, OK: ok, Detail: detail})
	}

	// Load the .env file into the process environment, mirroring the backend
	// (loadDotEnvFiles uses godotenv.Overload).
	loadErr := godotenv.Overload(envFile)
	add("env_file.load", loadErr == nil, errString(loadErr))
	if loadErr != nil {
		return checks
	}

	cfg := loadEffectiveConfig(repoRoot, overrideFile, &checks)

	if len(checks) > 0 && !checks[0].OK {
		return checks
	}

	if cfg != nil {
		cfg.Server.Mode = "release"
		verr := cfg.ValidateRelease()
		if verr != nil {
			// ValidateRelease aggregates all findings; split them for display.
			msg := verr.Error()
			add("validate_release", false, msg)
		} else {
			add("validate_release", true, "all release-mode config rules passed")
		}
	}

	checkTrustedProxies(cfg, add)
	checkGreenAuthFields(cfg, add)
	checkFrontendURLs(add)
	checkPlaceholdersInEnv(add)
	checkConfigVolumeReadOnly(composeFile, add)

	return checks
}

// checkConfigVolumeReadOnly verifies the compose template mounts the operator
// config override read-only (:ro), so a compromised backend process cannot
// rewrite its own production configuration.
func checkConfigVolumeReadOnly(composeFile string, add func(string, bool, string)) {
	if composeFile == "" {
		add("config_volume.read_only", false, "compose file not specified")
		return
	}
	raw, err := os.ReadFile(composeFile)
	if err != nil {
		add("config_volume.read_only", false, fmt.Sprintf("cannot read compose file: %v", err))
		return
	}
	content := string(raw)
	found := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "config_override.yaml") {
			continue
		}
		if !strings.HasPrefix(trimmed, "-") {
			continue
		}
		found = true
		if !strings.HasSuffix(trimmed, ":ro") && !strings.Contains(trimmed, ":ro") {
			add("config_volume.read_only", false,
				"backend config_override.yaml volume must be mounted read-only (:ro) in the compose file")
			return
		}
	}
	if !found {
		add("config_volume.read_only", false,
			"compose file must mount the operator config override (config_override.yaml) into the backend service")
		return
	}
	add("config_volume.read_only", true, "operator config volume is mounted read-only")
}

func loadEffectiveConfig(repoRoot, overrideFile string, checks *[]check) *config.Config {
	baseConfig := filepath.Join(repoRoot, "backend", "config.yaml")
	v := viper.New()
	v.SetConfigFile(baseConfig)
	if err := v.ReadInConfig(); err != nil {
		*checks = append(*checks, check{Name: "config.yaml.load", OK: false, Detail: errString(err)})
		return nil
	}
	cfg := &config.Config{}
	if err := v.Unmarshal(cfg); err != nil {
		*checks = append(*checks, check{Name: "config.yaml.unmarshal", OK: false, Detail: errString(err)})
		return nil
	}
	// Match config.Load: the operator override is layered onto config.yaml
	// first, then explicit environment variables remain the final runtime
	// authority. The preflight must validate the same effective values that the
	// backend will actually start with.
	config.LoadOverride(cfg, overrideFile)
	config.OverrideFromEnv(cfg)
	return cfg
}

func checkTrustedProxies(cfg *config.Config, add func(string, bool, string)) {
	if cfg == nil {
		return
	}
	proxies := cfg.Security.TrustedProxies
	if len(proxies) == 0 {
		add("trusted_proxies.nonempty", false, "security.trusted_proxies must be explicit in production topology")
		return
	}
	loopbackOnly := true
	for _, p := range proxies {
		ip := net.ParseIP(strings.TrimSpace(p))
		if ip != nil && !ip.IsLoopback() {
			loopbackOnly = false
		}
		_, _, err := net.ParseCIDR(strings.TrimSpace(p))
		if err == nil && !isLoopbackCIDR(p) {
			loopbackOnly = false
		}
	}
	add("trusted_proxies.topology", !loopbackOnly,
		"at least one non-loopback trusted proxy (nginx container IP/CIDR) is required")
}

func isLoopbackCIDR(cidr string) bool {
	_, ipNet, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil {
		return false
	}
	return ipNet.IP.IsLoopback() || (ipNet.IP.IsUnspecified() && strings.Contains(cidr, "/0"))
}

// checkGreenAuthFields validates green.seed/green.uid early (missing,
// placeholder, format) so a misconfigured callback signature is caught before
// the server starts; every message is actionable for the deployer.
func checkGreenAuthFields(cfg *config.Config, add func(string, bool, string)) {
	if cfg == nil {
		return
	}
	var problems []string
	seed := strings.TrimSpace(cfg.Green.Seed)
	if seed == "" {
		problems = append(problems, "green.seed is missing: set GREEN_SEED to a random string of [A-Za-z0-9_], max 64 chars (e.g. openssl rand -base64 36 | tr -dc 'A-Za-z0-9_' | cut -c1-48); deployers may replace the generated template value before first launch")
	} else if containsPlaceholder(seed) {
		problems = append(problems, "green.seed must not contain placeholders")
	} else if !greenSeedFormat.MatchString(seed) {
		problems = append(problems, "green.seed must be 1-64 characters of [A-Za-z0-9_]")
	}

	uid := strings.TrimSpace(cfg.Green.UID)
	if uid == "" {
		problems = append(problems, "green.uid is missing: set GREEN_UID to the Aliyun MAIN account UID, found in the Aliyun console top-right account info (the RAM user UID is NOT accepted)")
	} else if containsPlaceholder(uid) {
		problems = append(problems, "green.uid must not contain placeholders")
	} else if !greenUIDFormat.MatchString(uid) {
		problems = append(problems, "green.uid must be digits only (the Aliyun main account UID from the console account info)")
	}

	add("green.auth_fields", len(problems) == 0, strings.Join(problems, "; "))
}

func checkFrontendURLs(add func(string, bool, string)) {
	apiURL := strings.TrimSpace(os.Getenv("NEXT_PUBLIC_API_URL"))
	siteURL := strings.TrimSpace(os.Getenv("NEXT_PUBLIC_SITE_URL"))
	internalURL := strings.TrimSpace(os.Getenv("INTERNAL_API_URL"))

	for _, item := range []struct{ name, value string }{
		{"NEXT_PUBLIC_API_URL", apiURL},
		{"NEXT_PUBLIC_SITE_URL", siteURL},
		{"INTERNAL_API_URL", internalURL},
	} {
		if item.value == "" {
			add("frontend_urls."+strings.ToLower(item.name), false, item.name+" is required in production build environment")
			continue
		}
		u, err := url.Parse(item.value)
		if err != nil || u.Scheme != "https" {
			add("frontend_urls."+strings.ToLower(item.name), false, item.name+" must be an https URL")
			continue
		}
		if isLoopbackHost(u.Hostname()) {
			add("frontend_urls."+strings.ToLower(item.name), false, item.name+" must not use a loopback host")
			continue
		}
		add("frontend_urls."+strings.ToLower(item.name), true, "")
	}

	if apiURL != "" && internalURL != "" {
		apiHost, _ := url.Parse(apiURL)
		internalHost, _ := url.Parse(internalURL)
		same := apiHost != nil && internalHost != nil && apiHost.Hostname() == internalHost.Hostname()
		add("frontend_urls.api_consistency", same,
			"NEXT_PUBLIC_API_URL and INTERNAL_API_URL must resolve to the same API host")
	}
	if siteURL != "" && apiURL != "" {
		siteHost, _ := url.Parse(siteURL)
		apiHost, _ := url.Parse(apiURL)
		diff := siteHost != nil && apiHost != nil && siteHost.Hostname() != apiHost.Hostname()
		add("frontend_urls.site_api_separate", diff,
			"NEXT_PUBLIC_SITE_URL and NEXT_PUBLIC_API_URL should use separate subdomains (app vs api)")
	}
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// checkPlaceholdersInEnv rejects template tokens in environment values that
// are not covered by Config.ValidateRelease (which validates the merged config
// fields, not raw env values like POSTGRES_PASSWORD or frontend URLs).
func checkPlaceholdersInEnv(add func(string, bool, string)) {
	keys := []string{
		"POSTGRES_PASSWORD", "DB_DSN", "REDIS_PASSWORD", "JWT_SECRET",
		"LLM_KEY_ENCRYPTION_SECRET", "OSS_ACCESS_KEY_ID", "OSS_ACCESS_KEY_SECRET",
		"OSS_BUCKET_NAME", "GREEN_ACCESS_KEY_ID", "GREEN_ACCESS_KEY_SECRET",
		"CAPTCHA_ACCESS_KEY_ID", "CAPTCHA_ACCESS_KEY_SECRET", "SMTP_PASSWORD",
		"AGENT_LLM_API_KEY", "NEXT_PUBLIC_API_URL", "NEXT_PUBLIC_SITE_URL",
		"INTERNAL_API_URL", "ALLOWED_ORIGINS",
	}
	bad := []string{}
	for _, k := range keys {
		v := strings.TrimSpace(os.Getenv(k))
		if v != "" && containsPlaceholder(v) {
			bad = append(bad, k)
		}
	}
	add("env.placeholders", len(bad) == 0, fmt.Sprintf("placeholder values must not appear in production env: %v", bad))
}

func containsPlaceholder(value string) bool {
	lower := strings.ToLower(value)
	for _, token := range []string{"<", ">", "change_me", "replace_me", "placeholder", "your-", "example.com", "example.org", "xxx"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
