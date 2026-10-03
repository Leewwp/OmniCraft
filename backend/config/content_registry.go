package config

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// Content-type registry (ticket #687, spec
// docs/superpowers/specs/2026-09-27-omnicraft-content-registry-preview-batch-design.md).
//
// Two orthogonal axes live here as the single source of truth:
//
//   - content_types: the taxonomy axis (what a published work IS). Zones,
//     publish form shape, allowed attachment families and judge eligibility
//     are declared per type. Adding a category = one registry row + i18n
//     keys + an icon map entry; no business-branch code.
//   - upload_file_types: the upload-capability axis (what files the platform
//     ACCEPTS). Per family: extension policy, MIME rules, a max-size limit
//     key reference and the scannable flag.
//
// The section is optional in YAML: when absent the built-in default registry
// (shipped-baseline equivalent) applies via the Effective* accessors, so
// minimal test configs stay valid. When the section IS present every entry is
// validated at load (see validateContentRegistry) — a mistyped family
// reference or a missing judge_eligible refuses startup instead of silently
// hiding the type.
//
// Staging rule (spec v2): only families that exist TODAY are registered here.
// document joins with #688, model3d with #689 — referencing an unregistered
// family fails validation, which is the anti-typo property working as
// intended.

const (
	ContentFormText  = "text"
	ContentFormFile  = "file"
	ContentFormMedia = "media"

	ZoneOriginal = "original"
	ZoneFanwork  = "fanwork"

	// ExtensionsUnrestrictedKey marks a family whose extension policy is
	// "no explicit list" — admission is decided by the MIME rules alone
	// (today: video/image/avatar/text/mod). An explicit list (sheet_music
	// via extensions_key) is the other legal shape. Tightening an
	// unrestricted family to an explicit list is a deliberate behavior
	// change and needs its own ticket.
	ExtensionsUnrestrictedKey = ""
)

// ContentTypeEntry is one taxonomy row.
type ContentTypeEntry struct {
	Key             string   `mapstructure:"key" json:"key"`
	Zones           []string `mapstructure:"zones" json:"zones"`
	Form            string   `mapstructure:"form" json:"form"`
	UploadFileTypes []string `mapstructure:"upload_file_types" json:"upload_file_types"`
	// JudgeEligible is a pointer so "absent" is distinguishable from false:
	// the field is required-explicit per spec v2 — an entry without it
	// refuses startup (auditability over silent defaults).
	JudgeEligible    *bool                   `mapstructure:"judge_eligible" json:"judge_eligible"`
	AttachmentPolicy *AttachmentPolicyConfig `mapstructure:"attachment_policy,omitempty" json:"attachment_policy,omitempty"`
}

// AttachmentPolicyConfig constrains publish-time attachment composition.
// Declared and validated from T0; the generic evaluator lands with #688
// (existing types carry no policy → zero behavior change) and #690 turns it
// on by configuration alone.
type AttachmentPolicyConfig struct {
	// RequiredAnyOf: publishing requires at least one attachment whose
	// family is in this list.
	RequiredAnyOf []string `mapstructure:"required_any_of" json:"required_any_of"`
}

// UploadFileTypeEntry is one capability row. Exactly one extension-policy
// shape is legal: an explicit list inline, a legacy extensions_key reference
// (sheet_music → upload.sheet_music_extensions), or unrestricted (neither
// set). MIME rules are server-authoritative and never projected to clients.
type UploadFileTypeEntry struct {
	Key           string   `mapstructure:"key" json:"key"`
	Extensions    []string `mapstructure:"extensions,omitempty" json:"extensions,omitempty"`
	ExtensionsKey string   `mapstructure:"extensions_key,omitempty" json:"extensions_key,omitempty"`
	MimePrefixes  []string `mapstructure:"mime_prefixes,omitempty" json:"mime_prefixes,omitempty"`
	MimeExact     []string `mapstructure:"mime_exact,omitempty" json:"mime_exact,omitempty"`
	// MaxMBKey references a limits.* field name — resolved against the
	// live cfg.Limits at read time (never copied into the registry) so
	// admin runtime adjustments cannot fork into a second truth.
	MaxMBKey  string `mapstructure:"max_mb_key" json:"max_mb_key"`
	Scannable bool   `mapstructure:"scannable" json:"scannable"`
}

type ContentRegistryConfig struct {
	ContentTypes    []ContentTypeEntry    `mapstructure:"content_types" json:"content_types"`
	UploadFileTypes []UploadFileTypeEntry `mapstructure:"upload_file_types" json:"upload_file_types"`
}

func boolPtr(v bool) *bool { return &v }

// DefaultContentRegistry is the shipped baseline the YAML registry mirrors
// (config.yaml carries the explicit section as the operational truth; the
// equivalence is pinned by TestDefaultRegistryMatchesShippedYAML). Families
// encode validateUploadByType's current semantics; types encode the current
// zone/form/attachment reality, including the known audio→text upload
// mapping (audio link fix lands with #688).
//
// Canonical declaration order: image, article, video, audio, mod, prompt,
// template, sheet_music, 3d_print, other — each zone's subsequence equals
// its type_order so zone-filtered fallback lists keep the configured order.
// 3d_print (#690) is the first registry-pure-data category: its
// attachment_policy turns on by configuration alone (engine shipped with
// #688).
func DefaultContentRegistry() ContentRegistryConfig {
	return ContentRegistryConfig{
		ContentTypes: []ContentTypeEntry{
			{Key: "image", Zones: []string{ZoneOriginal, ZoneFanwork}, Form: ContentFormMedia, UploadFileTypes: []string{"image"}, JudgeEligible: boolPtr(true)},
			{Key: "article", Zones: []string{ZoneOriginal, ZoneFanwork}, Form: ContentFormText, UploadFileTypes: []string{}, JudgeEligible: boolPtr(true)},
			{Key: "video", Zones: []string{ZoneOriginal, ZoneFanwork}, Form: ContentFormMedia, UploadFileTypes: []string{"video"}, JudgeEligible: boolPtr(true)},
			{Key: "audio", Zones: []string{ZoneOriginal, ZoneFanwork}, Form: ContentFormFile, UploadFileTypes: []string{"audio"}, JudgeEligible: boolPtr(true)},
			{Key: "mod", Zones: []string{ZoneFanwork}, Form: ContentFormFile, UploadFileTypes: []string{"mod"}, JudgeEligible: boolPtr(false)},
			{Key: "prompt", Zones: []string{ZoneFanwork}, Form: ContentFormText, UploadFileTypes: []string{}, JudgeEligible: boolPtr(true)},
			{Key: "template", Zones: []string{ZoneOriginal}, Form: ContentFormFile, UploadFileTypes: []string{"text", "document", "model3d"}, JudgeEligible: boolPtr(true)},
			{Key: "sheet_music", Zones: []string{ZoneOriginal, ZoneFanwork}, Form: ContentFormFile, UploadFileTypes: []string{"sheet_music"}, JudgeEligible: boolPtr(true)},
			{Key: "3d_print", Zones: []string{ZoneOriginal, ZoneFanwork}, Form: ContentFormFile, UploadFileTypes: []string{"model3d", "text"}, JudgeEligible: boolPtr(true), AttachmentPolicy: &AttachmentPolicyConfig{RequiredAnyOf: []string{"model3d"}}},
			{Key: "other", Zones: []string{ZoneOriginal, ZoneFanwork}, Form: ContentFormText, UploadFileTypes: []string{}, JudgeEligible: boolPtr(true)},
		},
		UploadFileTypes: []UploadFileTypeEntry{
			{Key: "video", MimePrefixes: []string{"video/"}, MaxMBKey: "video_max_mb"},
			{Key: "image", MimePrefixes: []string{"image/"}, MaxMBKey: "image_max_mb"},
			// Profile avatars ride the presign chain under /avatar/ and
			// reuse the image size budget (validateUploadByType parity).
			{Key: "avatar", MimePrefixes: []string{"image/"}, MaxMBKey: "image_max_mb"},
			{Key: "text", MimePrefixes: []string{"text/"}, MimeExact: []string{"application/pdf"}, MaxMBKey: "text_max_mb"},
			{Key: "mod", MimeExact: []string{"application/zip", "application/x-zip-compressed"}, MaxMBKey: "mod_max_mb", Scannable: true},
			{Key: "sheet_music", ExtensionsKey: "sheet_music_extensions", MaxMBKey: "sheet_music_max_mb"},
			// #688 document family: .docx/.xlsx/.csv with the OpenXML/text
			// CSV MIME hints enforced per-extension in validateUploadByType
			// (octet-stream fallback while File.type may be empty); the
			// authoritative check is the server-side package-identity
			// validation at publish time. Macro containers (.docm/.xlsm/
			// .pptm) are rejected by extension AND by package content.
			{Key: "document", Extensions: []string{".docx", ".xlsx", ".csv"}, MaxMBKey: "document_max_mb", Scannable: true},
			// #688 audio family: this chain was broken before (file_type
			// mis-mapped to text and audio/* always failed the text MIME
			// rule) — first real opening, built to the new standard:
			// extension whitelist + header magic sniffing + scannable.
			{Key: "audio", Extensions: []string{".mp3", ".wav", ".flac", ".m4a", ".aac", ".ogg", ".opus"}, MaxMBKey: "audio_max_mb", Scannable: true},
			// #689 model3d family: extension-whitelist driven (MIME sniffing
			// is unreliable for STL/PLY — browsers report octet-stream);
			// 3MF is a ZIP container and walks the #688 structure +
			// package-identity pipeline (3D/*.model) post-upload.
			// .mtl uploads but never previews (V1: no OBJ+MTL material
			// matching — OSS keys are randomized, basename matching is
			// unimplementable on the current data model).
			{Key: "model3d", Extensions: []string{".stl", ".obj", ".3mf", ".gcode", ".ply", ".mtl"}, MaxMBKey: "model3d_max_mb", Scannable: true},
		},
	}
}

// EffectiveContentTypes returns the configured registry, falling back to the
// built-in baseline when the YAML section is absent. Nil-receiver safe: the
// default keeps registry consumers working with hand-built test configs.
func (c *Config) EffectiveContentTypes() []ContentTypeEntry {
	if c != nil && len(c.ContentRegistry.ContentTypes) > 0 {
		return c.ContentRegistry.ContentTypes
	}
	return DefaultContentRegistry().ContentTypes
}

// EffectiveUploadFileTypes is the capability-axis twin of
// EffectiveContentTypes.
func (c *Config) EffectiveUploadFileTypes() []UploadFileTypeEntry {
	if c != nil && len(c.ContentRegistry.UploadFileTypes) > 0 {
		return c.ContentRegistry.UploadFileTypes
	}
	return DefaultContentRegistry().UploadFileTypes
}

// IsRegisteredContentType reports taxonomy membership.
func (c *Config) IsRegisteredContentType(key string) bool {
	for _, entry := range c.EffectiveContentTypes() {
		if entry.Key == key {
			return true
		}
	}
	return false
}

// ContentTypeLookup returns the registry row for a type key.
func (c *Config) ContentTypeLookup(key string) (ContentTypeEntry, bool) {
	for _, entry := range c.EffectiveContentTypes() {
		if entry.Key == key {
			return entry, true
		}
	}
	return ContentTypeEntry{}, false
}

// UploadFileTypeLookup returns the capability row for a family key.
func (c *Config) UploadFileTypeLookup(key string) (UploadFileTypeEntry, bool) {
	for _, entry := range c.EffectiveUploadFileTypes() {
		if entry.Key == key {
			return entry, true
		}
	}
	return UploadFileTypeEntry{}, false
}

// FamilyExtensions resolves a family's explicit extension list, or nil when
// the family is unrestricted. Legacy extensions_key references resolve
// against the existing upload section so the list keeps a single truth.
func (c *Config) FamilyExtensions(key string) []string {
	family, ok := c.UploadFileTypeLookup(key)
	if !ok {
		return nil
	}
	if len(family.Extensions) > 0 {
		return family.Extensions
	}
	if family.ExtensionsKey == "sheet_music_extensions" && c != nil {
		return c.Upload.SheetMusicExtensions
	}
	return nil
}

// IsUnrestrictedFamily reports whether admission is MIME-driven (no explicit
// extension list).
func (c *Config) IsUnrestrictedFamily(key string) bool {
	return c.FamilyExtensions(key) == nil
}

// ScannableUploadFamilies lists the families that join the ClamAV pipeline
// (registry capability axis). Wiring threads this into the scan gate, the
// scan repository and publish-time job creation.
func (c *Config) ScannableUploadFamilies() []string {
	var families []string
	for _, entry := range c.EffectiveUploadFileTypes() {
		if entry.Scannable {
			families = append(families, entry.Key)
		}
	}
	return families
}

// MaxMBByKey resolves a limits.* field name to its live value. The registry
// stores the key, never the number, so admin runtime adjustments stay
// authoritative (spec v2.2 item 9).
func (c *Config) MaxMBByKey(key string) (int, bool) {
	if c == nil {
		return 0, false
	}
	switch key {
	case "video_max_mb":
		return c.Limits.VideoMaxMB, true
	case "image_max_mb":
		return c.Limits.ImageMaxMB, true
	case "text_max_mb":
		return c.Limits.TextMaxMB, true
	case "mod_max_mb":
		return c.Limits.ModMaxMB, true
	case "sheet_music_max_mb":
		return c.Limits.SheetMusicMaxMB, true
	case "audio_max_mb":
		return c.Limits.AudioMaxMB, true
	case "document_max_mb":
		return c.Limits.DocumentMaxMB, true
	case "model3d_max_mb":
		return c.Limits.Model3DMaxMB, true
	default:
		return 0, false
	}
}

// FamilyMaxMB resolves a family's size cap through its limit key.
func (c *Config) FamilyMaxMB(key string) int {
	family, ok := c.UploadFileTypeLookup(key)
	if !ok {
		return 0
	}
	mb, _ := c.MaxMBByKey(family.MaxMBKey)
	return mb
}

// ClientAcceptForContentType derives the file-input accept attribute for a
// content type's allowed attachment set (spec v2.2 item 7): any unrestricted
// family → "*" (never silently tighten); otherwise the explicit extension
// union. Empty string = the type takes no attachments.
func (c *Config) ClientAcceptForContentType(contentType string) string {
	entry, ok := c.ContentTypeLookup(contentType)
	if !ok {
		return ""
	}
	parts := make([]string, 0, 4)
	for _, family := range entry.UploadFileTypes {
		exts := c.FamilyExtensions(family)
		if exts == nil {
			return "*"
		}
		parts = append(parts, exts...)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// ResolveUploadFamilyForExtension implements the extension→family derivation
// contract (spec v2.1): explicit extension match wins and must be unique
// within the allowed set (the ambiguity invariant enforces uniqueness at
// load); with no explicit match the single unrestricted family is the
// fallback. Empty string = unsupported extension for this content type.
// ext may be a bare extension (".pdf") or a filename ("song.pdf") — the
// extension is extracted and lowercased first. Server-side presign/verify
// stays MIME-authoritative; this contract exists for client-side per-file
// typing (#688).
func (c *Config) ResolveUploadFamilyForExtension(contentType, ext string) string {
	entry, ok := c.ContentTypeLookup(contentType)
	if !ok {
		return ""
	}
	normalized := normalizeExtensionToken(path.Ext(strings.TrimSpace(ext)))
	if normalized == "" {
		return ""
	}
	fallback := ""
	for _, family := range entry.UploadFileTypes {
		for _, candidate := range c.FamilyExtensions(family) {
			if normalizeExtensionToken(candidate) == normalized {
				return family
			}
		}
		if c.IsUnrestrictedFamily(family) {
			if fallback != "" {
				return "" // >1 unrestricted family: ambiguous, caller rejects
			}
			fallback = family
		}
	}
	return fallback
}

// normalizeExtensionToken lowercases an extension token that is already an
// extension (".pdf"), tolerating a missing leading dot. Filenames must be
// reduced with path.Ext first.
func normalizeExtensionToken(ext string) string {
	e := strings.ToLower(strings.TrimSpace(ext))
	if e == "" {
		return ""
	}
	if !strings.HasPrefix(e, ".") {
		e = "." + e
	}
	return e
}

// validateContentRegistry enforces the registry contracts on the CONFIGURED
// section (defaults are valid by construction and never reach here as a
// section). Wired into validateStructure so every boot mode gates on it.
func (c *Config) validateContentRegistry(errs *[]string) {
	if len(c.ContentRegistry.ContentTypes) == 0 && len(c.ContentRegistry.UploadFileTypes) == 0 {
		return
	}

	contentKeys := map[string]bool{}
	families := map[string]bool{}
	for i, family := range c.ContentRegistry.UploadFileTypes {
		if strings.TrimSpace(family.Key) == "" {
			*errs = append(*errs, fmt.Sprintf("content_registry.upload_file_types[%d].key is required", i))
			continue
		}
		if families[family.Key] {
			*errs = append(*errs, fmt.Sprintf("content_registry.upload_file_types: duplicate family key %q", family.Key))
			continue
		}
		families[family.Key] = true
		if _, ok := c.MaxMBByKey(family.MaxMBKey); !ok {
			*errs = append(*errs, fmt.Sprintf("content_registry.upload_file_types[%q]: unknown max_mb_key %q", family.Key, family.MaxMBKey))
		}
		hasExtensions := len(family.Extensions) > 0
		hasExtensionsKey := strings.TrimSpace(family.ExtensionsKey) != ""
		if hasExtensions && hasExtensionsKey {
			*errs = append(*errs, fmt.Sprintf("content_registry.upload_file_types[%q]: extensions and extensions_key are mutually exclusive", family.Key))
		}
		if hasExtensionsKey && family.ExtensionsKey != "sheet_music_extensions" {
			*errs = append(*errs, fmt.Sprintf("content_registry.upload_file_types[%q]: unknown extensions_key %q", family.Key, family.ExtensionsKey))
		}
		if hasExtensionsKey && len(c.Upload.SheetMusicExtensions) == 0 {
			*errs = append(*errs, fmt.Sprintf("content_registry.upload_file_types[%q]: extensions_key %q resolves to an empty list", family.Key, family.ExtensionsKey))
		}
		if !hasExtensions && !hasExtensionsKey && len(family.MimePrefixes) == 0 && len(family.MimeExact) == 0 {
			*errs = append(*errs, fmt.Sprintf("content_registry.upload_file_types[%q]: needs at least one admission rule (extensions, extensions_key, mime_prefixes or mime_exact)", family.Key))
		}
	}

	for i, entry := range c.ContentRegistry.ContentTypes {
		if strings.TrimSpace(entry.Key) == "" {
			*errs = append(*errs, fmt.Sprintf("content_registry.content_types[%d].key is required", i))
			continue
		}
		if contentKeys[entry.Key] {
			*errs = append(*errs, fmt.Sprintf("content_registry.content_types: duplicate type key %q", entry.Key))
			continue
		}
		contentKeys[entry.Key] = true

		switch entry.Form {
		case ContentFormText, ContentFormFile, ContentFormMedia:
		default:
			*errs = append(*errs, fmt.Sprintf("content_registry.content_types[%q]: unknown form %q (must be text, file or media)", entry.Key, entry.Form))
		}
		if len(entry.Zones) == 0 {
			*errs = append(*errs, fmt.Sprintf("content_registry.content_types[%q]: zones must not be empty", entry.Key))
		}
		seenZones := map[string]bool{}
		for _, zone := range entry.Zones {
			if zone != ZoneOriginal && zone != ZoneFanwork {
				*errs = append(*errs, fmt.Sprintf("content_registry.content_types[%q]: unknown zone %q", entry.Key, zone))
			}
			if seenZones[zone] {
				*errs = append(*errs, fmt.Sprintf("content_registry.content_types[%q]: duplicate zone %q", entry.Key, zone))
			}
			seenZones[zone] = true
		}
		if entry.JudgeEligible == nil {
			*errs = append(*errs, fmt.Sprintf("content_registry.content_types[%q]: judge_eligible is required-explicit (set true or false)", entry.Key))
		}
		for _, family := range entry.UploadFileTypes {
			if !families[family] {
				*errs = append(*errs, fmt.Sprintf("content_registry.content_types[%q]: upload_file_types references unregistered family %q", entry.Key, family))
			}
		}
		if entry.AttachmentPolicy != nil {
			for _, family := range entry.AttachmentPolicy.RequiredAnyOf {
				if !families[family] {
					*errs = append(*errs, fmt.Sprintf("content_registry.content_types[%q].policy.required_any_of references unregistered family %q", entry.Key, family))
				}
			}
		}
	}

	c.validateTypeOrderInvariants(errs)
	c.validateExtensionAmbiguity(errs)
}

// validateTypeOrderInvariants checks publish.type_order_* against the
// effective registry (spec v2 item 15): orders are all-or-nothing — either
// both zones are configured and each is a duplicate-free exact cover of its
// zone's publishable types (with every entry actually belonging to the
// zone), or neither is configured and the frontend fallback grid applies.
func (c *Config) validateTypeOrderInvariants(errs *[]string) {
	original := c.Publish.TypeOrderOriginal
	fanwork := c.Publish.TypeOrderFanwork
	if len(original) == 0 && len(fanwork) == 0 {
		return
	}
	if len(original) == 0 || len(fanwork) == 0 {
		*errs = append(*errs, "publish.type_order_original and publish.type_order_fanwork must be configured together (one-sided ordering is rejected)")
		return
	}

	zoneTypes := map[string]map[string]bool{ZoneOriginal: {}, ZoneFanwork: {}}
	for _, entry := range c.EffectiveContentTypes() {
		for _, zone := range entry.Zones {
			if zone == ZoneOriginal || zone == ZoneFanwork {
				zoneTypes[zone][entry.Key] = true
			}
		}
	}

	orderedAnywhere := map[string]bool{}
	for zone, order := range map[string][]string{ZoneOriginal: original, ZoneFanwork: fanwork} {
		seen := map[string]bool{}
		for _, key := range order {
			if seen[key] {
				*errs = append(*errs, fmt.Sprintf("publish.type_order_%s: duplicate entry %q", zone, key))
				continue
			}
			seen[key] = true
			if !zoneTypes[zone][key] {
				*errs = append(*errs, fmt.Sprintf("publish.type_order_%s: entry %q is not publishable in this zone (registry zones must include it)", zone, key))
			}
			orderedAnywhere[key] = true
		}
		for key := range zoneTypes[zone] {
			if !seen[key] {
				*errs = append(*errs, fmt.Sprintf("publish.type_order_%s: publishable type %q is missing from the order", zone, key))
			}
		}
	}
	allTypes := map[string]bool{}
	for zone := range zoneTypes {
		for key := range zoneTypes[zone] {
			allTypes[key] = true
		}
	}
	for key := range allTypes {
		if !orderedAnywhere[key] {
			*errs = append(*errs, fmt.Sprintf("content_registry.content_types: type %q appears in no publish type order", key))
		}
	}
}

// validateExtensionAmbiguity enforces the derivation contract's precondition
// (spec v2.1 item 3): within one content type's allowed family set an
// extension may explicitly match at most one family, and at most one
// unrestricted family may act as the fallback.
func (c *Config) validateExtensionAmbiguity(errs *[]string) {
	for _, entry := range c.EffectiveContentTypes() {
		owner := map[string]string{}
		unrestricted := 0
		for _, family := range entry.UploadFileTypes {
			exts := c.FamilyExtensions(family)
			if exts == nil {
				unrestricted++
				continue
			}
			for _, ext := range exts {
				normalized := normalizeExtensionToken(ext)
				if prior, clash := owner[normalized]; clash {
					*errs = append(*errs, fmt.Sprintf("content_registry.content_types[%q]: extension %s matches families %q and %q in the same allowed set", entry.Key, normalized, prior, family))
				} else {
					owner[normalized] = family
				}
			}
		}
		if unrestricted > 1 {
			*errs = append(*errs, fmt.Sprintf("content_registry.content_types[%q]: %d unrestricted families in one allowed set — the extension fallback would be ambiguous (at most one)", entry.Key, unrestricted))
		}
	}
}
