package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// Ticket #687: dual-axis content-type registry — validation matrix,
// shipped-YAML equivalence snapshot, extension derivation contract and the
// Load() refusal path for a broken registry section.

func boolPtrT(v bool) *bool { return &v }

// baseRegistryYAML mirrors the shipped section; test cases mutate one aspect
// and assert the exact rejection message.
const baseRegistryYAML = `
content_registry:
  content_types:
    - key: article
      zones: [original, fanwork]
      form: text
      upload_file_types: []
      judge_eligible: true
    - key: template
      zones: [original]
      form: file
      upload_file_types: [text]
      judge_eligible: true
    - key: sheet_music
      zones: [original, fanwork]
      form: file
      upload_file_types: [sheet_music]
      judge_eligible: true
    - key: mod
      zones: [fanwork]
      form: file
      upload_file_types: [mod]
      judge_eligible: false
  upload_file_types:
    - key: text
      mime_prefixes: ["text/"]
      mime_exact: ["application/pdf"]
      max_mb_key: text_max_mb
    - key: sheet_music
      extensions_key: sheet_music_extensions
      max_mb_key: sheet_music_max_mb
    - key: mod
      mime_exact: ["application/zip"]
      max_mb_key: mod_max_mb
      scannable: true
publish:
  type_order_original: ["article", "template", "sheet_music"]
  type_order_fanwork: ["article", "sheet_music", "mod"]
upload:
  sheet_music_extensions: [".mid", ".midi", ".xml", ".pdf"]
`

func loadRegistryConfig(t *testing.T, yamlText string) *Config {
	t.Helper()
	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(yamlText)))
	var cfg Config
	require.NoError(t, v.Unmarshal(&cfg))
	return &cfg
}

func TestContentRegistryValidationMatrix(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(yamlText string) string
		wantErr string
	}{
		{
			name: "unknown form rejects",
			mutate: func(s string) string {
				return strings.Replace(s, "form: file\n      upload_file_types: [text]", "form: document\n      upload_file_types: [text]", 1)
			},
			wantErr: `unknown form "document"`,
		},
		{
			name: "unregistered family reference rejects",
			mutate: func(s string) string {
				return strings.Replace(s, "upload_file_types: [text]\n      judge_eligible: true", "upload_file_types: [document]\n      judge_eligible: true", 1)
			},
			wantErr: `references unregistered family "document"`,
		},
		{
			name: "missing judge_eligible rejects",
			mutate: func(s string) string {
				return strings.Replace(s, "upload_file_types: []\n      judge_eligible: true", "upload_file_types: []", 1)
			},
			wantErr: "judge_eligible is required-explicit",
		},
		{
			name: "duplicate type key rejects",
			mutate: func(s string) string {
				return strings.Replace(s, "    - key: mod\n      zones: [fanwork]", "    - key: article\n      zones: [fanwork]", 1)
			},
			wantErr: `duplicate type key "article"`,
		},
		{
			name: "unknown max_mb_key rejects",
			mutate: func(s string) string {
				return strings.Replace(s, "max_mb_key: text_max_mb", "max_mb_key: text_max_gigabytes", 1)
			},
			wantErr: `unknown max_mb_key "text_max_gigabytes"`,
		},
		{
			name: "extensions plus extensions_key mutually exclusive",
			mutate: func(s string) string {
				return strings.Replace(s, "extensions_key: sheet_music_extensions", "extensions_key: sheet_music_extensions\n      extensions: [.mid]", 1)
			},
			wantErr: "mutually exclusive",
		},
		{
			name: "type order duplicate entry rejects",
			mutate: func(s string) string {
				return strings.Replace(s, `type_order_original: ["article", "template", "sheet_music"]`, `type_order_original: ["article", "article", "template", "sheet_music"]`, 1)
			},
			wantErr: `duplicate entry "article"`,
		},
		{
			name: "type order missing publishable type rejects",
			mutate: func(s string) string {
				return strings.Replace(s, `type_order_original: ["article", "template", "sheet_music"]`, `type_order_original: ["article", "template"]`, 1)
			},
			wantErr: `publishable type "sheet_music" is missing`,
		},
		{
			name: "type order entry from wrong zone rejects",
			mutate: func(s string) string {
				return strings.Replace(s, `type_order_original: ["article", "template", "sheet_music"]`, `type_order_original: ["article", "template", "sheet_music", "mod"]`, 1)
			},
			wantErr: `entry "mod" is not publishable in this zone`,
		},
		{
			name: "type in no order rejects",
			mutate: func(s string) string {
				return strings.Replace(s, `type_order_fanwork: ["article", "sheet_music", "mod"]`, `type_order_fanwork: ["article", "sheet_music"]`, 1)
			},
			wantErr: `appears in no publish type order`,
		},
		{
			name: "one-sided type order rejects",
			mutate: func(s string) string {
				return strings.Replace(s, `type_order_original: ["article", "template", "sheet_music"]`, `type_order_original: []`, 1)
			},
			wantErr: "must be configured together",
		},
		{
			name: "extension ambiguity within one allowed set rejects",
			mutate: func(s string) string {
				// sheet_music's allowed set gains a second family that
				// explicitly lists .pdf, clashing with sheet_music's own
				// resolved list (also contains .pdf via extensions_key).
				out := strings.Replace(s,
					"      upload_file_types: [sheet_music]\n      judge_eligible: true",
					"      upload_file_types: [sheet_music, pdf_dup]\n      judge_eligible: true", 1)
				return strings.Replace(out, "    - key: mod\n      mime_exact", "    - key: pdf_dup\n      extensions: [.pdf]\n      max_mb_key: text_max_mb\n    - key: mod\n      mime_exact", 1)
			},
			wantErr: `extension .pdf matches families`,
		},
		{
			name: "two unrestricted families in one allowed set rejects",
			mutate: func(s string) string {
				// template's set becomes [text, mod]: both extension-
				// unrestricted → the fallback would be ambiguous.
				return strings.Replace(s, "upload_file_types: [text]\n      judge_eligible: true", "upload_file_types: [text, mod]\n      judge_eligible: true", 1)
			},
			wantErr: "unrestricted families in one allowed set",
		},
		{
			name: "attachment policy unregistered family rejects",
			mutate: func(s string) string {
				return strings.Replace(s,
					"      upload_file_types: [mod]\n      judge_eligible: false",
					"      upload_file_types: [mod]\n      judge_eligible: false\n      attachment_policy:\n        required_any_of: [model3d]", 1)
			},
			wantErr: `required_any_of references unregistered family "model3d"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := loadRegistryConfig(t, tc.mutate(baseRegistryYAML))
			err := cfg.Validate()
			// Validate also reports the unrelated missing required fields of
			// this fragment; only the registry finding is asserted.
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}

	t.Run("baseline fragment registry itself validates", func(t *testing.T) {
		cfg := loadRegistryConfig(t, baseRegistryYAML)
		err := cfg.validateContentRegistryRaw()
		require.NoError(t, err)
	})
}

// validateContentRegistryRaw runs only the registry block so the baseline
// fragment (which lacks the required core fields) can be proven clean.
func (c *Config) validateContentRegistryRaw() error {
	var errs []string
	c.validateContentRegistry(&errs)
	if len(errs) > 0 {
		return &registryValidationError{errs}
	}
	return nil
}

type registryValidationError struct{ errs []string }

func (e *registryValidationError) Error() string { return strings.Join(e.errs, "; ") }

func TestDefaultRegistryMatchesShippedYAML(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)

	def := DefaultContentRegistry()
	require.Equal(t, def.ContentTypes, cfg.ContentRegistry.ContentTypes,
		"config.yaml content_registry.content_types must mirror DefaultContentRegistry exactly (shipped baseline)")
	require.Equal(t, def.UploadFileTypes, cfg.ContentRegistry.UploadFileTypes,
		"config.yaml content_registry.upload_file_types must mirror DefaultContentRegistry exactly")

	// Effective accessors return the configured section when present.
	require.Equal(t, def.ContentTypes, cfg.EffectiveContentTypes())
	require.Equal(t, def.UploadFileTypes, cfg.EffectiveUploadFileTypes())

	// And fall back to the baseline when the section is absent (nil-receiver
	// safe: hand-built test configs keep registry consumers working).
	empty := &Config{}
	require.Equal(t, def.ContentTypes, empty.EffectiveContentTypes())
	var nilCfg *Config
	require.Equal(t, def.ContentTypes, nilCfg.EffectiveContentTypes())
	require.True(t, nilCfg.IsRegisteredContentType("article"))
	require.Equal(t, "*", nilCfg.ClientAcceptForContentType("mod"))
}

// TestShippedYAMLRegistryValidates loads the real config.yaml and proves the
// full registry + type_order contract holds on the shipped baseline.
func TestShippedYAMLRegistryValidates(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)
	require.NoError(t, cfg.validateContentRegistryRaw())
}

func TestExtensionDerivationContract(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)

	// Explicit extension match wins: .pdf inside sheet_music context is
	// sheet_music (the v2.1 .pdf double-membership resolves per content
	// type), while inside template context the unrestricted text family is
	// the fallback.
	require.Equal(t, "sheet_music", cfg.ResolveUploadFamilyForExtension("sheet_music", "song.pdf"))
	require.Equal(t, "text", cfg.ResolveUploadFamilyForExtension("template", "manual.pdf"))
	require.Equal(t, "text", cfg.ResolveUploadFamilyForExtension("template", "notes.doc"))
	require.Equal(t, "mod", cfg.ResolveUploadFamilyForExtension("mod", "pack.zip"))
	// .zip is explicit nowhere → falls to mod's unrestricted... mod IS
	// MIME-restricted but extension-unrestricted: the derivation contract
	// is a client-side hint; presign MIME checks stay authoritative.
	require.Equal(t, "", cfg.ResolveUploadFamilyForExtension("sheet_music", "song.docx"), "no explicit match and no unrestricted family in the set")
	require.Equal(t, "", cfg.ResolveUploadFamilyForExtension("article", "x.pdf"), "text-form types take no attachments")
	require.Equal(t, "", cfg.ResolveUploadFamilyForExtension("nope", "x.pdf"), "unknown content type")
}

func TestClientAcceptProjection(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)

	// Any unrestricted family in the allowed set → "*" (never silently
	// tighten the accept list).
	require.Equal(t, "*", cfg.ClientAcceptForContentType("mod"))
	require.Equal(t, "*", cfg.ClientAcceptForContentType("template"), "text family stays unrestricted in template's set")
	require.Equal(t, ".aac,.flac,.m4a,.mp3,.ogg,.opus,.wav", cfg.ClientAcceptForContentType("audio"), "#688: audio family is an explicit extension whitelist")
	// Explicit-only set → sorted explicit union resolved from the legacy
	// extensions key.
	require.Equal(t, ".mid,.midi,.mscx,.mscz,.mxl,.pdf,.xml", cfg.ClientAcceptForContentType("sheet_music"))
	// No attachments → empty.
	require.Equal(t, "", cfg.ClientAcceptForContentType("article"))
}

func TestMaxMBByKeyResolvesLiveLimits(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)
	cfg.Limits.ModMaxMB = 4321
	mb, ok := cfg.MaxMBByKey("mod_max_mb")
	require.True(t, ok)
	require.Equal(t, 4321, mb, "registry holds the key, the live Limits value resolves")
	_, ok = cfg.MaxMBByKey("nonexistent")
	require.False(t, ok)
}

// TestLoadExitsOnBrokenContentRegistry proves the refusal-to-boot path
// through the subprocess re-exec harness (ticket #671 pattern): Load() must
// exit 1 naming the registry finding before any broken type ships.
func TestLoadExitsOnBrokenContentRegistry(t *testing.T) {
	if os.Getenv("LOAD_EXIT_CHILD") == "1" {
		return // child role handled in TestMain
	}

	// Minimal-valid core + a registry referencing an unregistered family.
	extra := `
content_registry:
  content_types:
    - key: article
      zones: [original, fanwork]
      form: text
      upload_file_types: [document]
      judge_eligible: true
  upload_file_types:
    - key: text
      mime_prefixes: ["text/"]
      max_mb_key: text_max_mb
`

	dir := t.TempDir()
	override := filepath.Join(dir, "override.yaml")
	require.NoError(t, os.WriteFile(override, []byte(minimalValidCoreYAML+extra), 0o600))

	exe, err := os.Executable()
	require.NoError(t, err)
	cmd := exec.Command(exe, "-test.run", "TestMain", "-test.timeout", "30s")
	cmd.Dir = t.TempDir() // no config.yaml: the override must carry the damage
	cmd.Env = append(os.Environ(),
		"LOAD_EXIT_CHILD=1",
		"CONFIG_OVERRIDE_PATH="+override,
		"OMNICRAFT_TEST_MODE=",
	)
	out, _ := cmd.CombinedOutput()
	require.Equal(t, 1, cmd.ProcessState.ExitCode(),
		"Load must refuse to boot on an unregistered upload_file_types reference; output: %s", out)
	require.Contains(t, string(out), "references unregistered family", "output: %s", out)
}
