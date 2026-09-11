package router

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// SP-16 P0 (#446): the anonymous-surface registry. Every optAuth route
// declared in routes.go must be inventoried here with its visibility
// mechanism, and every inventoried route must appear in the exposure-matrix
// doc. Adding an anonymous endpoint without registering it (and without the
// audit entry in the matrix) turns this test red — that is the gate that
// keeps new public surface reviewed before it ships.

type optAuthRouteAudit struct {
	Method string
	Path   string
	// ReturnsContent marks endpoints whose response embeds content rows or
	// content-derived aggregates (lists, counts, tags, versions, PRs…).
	// These must name the mechanism that keeps non-public content out.
	ReturnsContent bool
	Mechanism      string
}

// Registry note: paths are absolute under /api/v1.
var optAuthRouteRegistry = []optAuthRouteAudit{
	{Method: "GET", Path: "/api/v1/users/:id", ReturnsContent: false, Mechanism: "public user projection (no email; role/ban flag are public governance signals)"},
	{Method: "GET", Path: "/api/v1/users/:id/reputation", ReturnsContent: false, Mechanism: "reputation log of public profile"},
	{Method: "GET", Path: "/api/v1/users/:id/contents", ReturnsContent: true, Mechanism: "ListContents + ApplyContentVisibilityScope(viewer)"},
	{Method: "GET", Path: "/api/v1/users/:id/followers", ReturnsContent: false, Mechanism: "follow graph rows (user projection)"},
	{Method: "GET", Path: "/api/v1/users/:id/following", ReturnsContent: false, Mechanism: "follow graph rows (ids only)"},
	{Method: "GET", Path: "/api/v1/users/search", ReturnsContent: false, Mechanism: "user search excludes banned/deleted users"},
	{Method: "GET", Path: "/api/v1/users/:id/discussions", ReturnsContent: true, Mechanism: "discussion status=published + parent-content visibility filter"},
	{Method: "GET", Path: "/api/v1/ips", ReturnsContent: true, Mechanism: "IP listing defaults to status=approved"},
	{Method: "GET", Path: "/api/v1/ips/:id", ReturnsContent: true, Mechanism: "IP detail gate: approved, or creator/admin viewer"},
	{Method: "GET", Path: "/api/v1/ips/:id/contents", ReturnsContent: true, Mechanism: "ListContents + ApplyContentVisibilityScope(viewer)"},
	{Method: "GET", Path: "/api/v1/ips/:id/proposals", ReturnsContent: true, Mechanism: "IP gate (approved or creator) — proposals inherit IP visibility"},
	{Method: "GET", Path: "/api/v1/ips/:id/proposals/:proposalId", ReturnsContent: true, Mechanism: "IP gate (approved or creator) — proposals inherit IP visibility"},
	{Method: "GET", Path: "/api/v1/ips/:id/versions", ReturnsContent: true, Mechanism: "IP gate (approved or creator); version snapshots are json:\"-\""},
	{Method: "GET", Path: "/api/v1/ips/:id/discussions", ReturnsContent: true, Mechanism: "discussion status=published + parent-content visibility filter"},
	{Method: "GET", Path: "/api/v1/ips/:id/discussions/search", ReturnsContent: true, Mechanism: "discussion status=published + parent-content visibility filter"},
	{Method: "GET", Path: "/api/v1/contents", ReturnsContent: true, Mechanism: "ApplyContentVisibilityScope(viewer) in content repo"},
	{Method: "GET", Path: "/api/v1/contents/:id", ReturnsContent: true, Mechanism: "contentVisibleToViewer: author/admin/judge-qualified; published+public+author-not-banned+IP-not-banned otherwise; source enrichment visibility-checked"},
	{Method: "GET", Path: "/api/v1/contents/:id/related-fanworks", ReturnsContent: true, Mechanism: "GetVisibleContent for source + scope on children"},
	{Method: "GET", Path: "/api/v1/contents/:id/versions", ReturnsContent: true, Mechanism: "content visibility gate before listing version lineage (body text lives in storage_key)"},
	{Method: "GET", Path: "/api/v1/contents/:id/prs", ReturnsContent: true, Mechanism: "content visibility gate before listing PRs"},
	{Method: "GET", Path: "/api/v1/contents/:id/guide", ReturnsContent: true, Mechanism: "content visibility gate (author/admin/anon-visible); merged template+specifics view, ETag+s-maxage (#447)"},
	{Method: "GET", Path: "/api/v1/contents/search", ReturnsContent: true, Mechanism: "ApplyContentVisibilityScope(viewer) in search repo"},
	{Method: "GET", Path: "/api/v1/versions/:id", ReturnsContent: true, Mechanism: "participant gate: author/proposed-submitter/admin (F-056)"},
	{Method: "GET", Path: "/api/v1/pr/:id", ReturnsContent: true, Mechanism: "participant gate: author/submitter/admin (FIX-21④)"},
	{Method: "GET", Path: "/api/v1/social/comments", ReturnsContent: true, Mechanism: "comment status=published + parent-content visibility filter"},
	{Method: "GET", Path: "/api/v1/social/discussions", ReturnsContent: true, Mechanism: "discussion status=published + parent-content visibility filter on content_id"},
	{Method: "GET", Path: "/api/v1/social/discussions/:id", ReturnsContent: true, Mechanism: "discussion detail gate: status=published (parity with /discussions/:id)"},
	{Method: "GET", Path: "/api/v1/social/reactions", ReturnsContent: false, Mechanism: "aggregate counts only"},
	{Method: "GET", Path: "/api/v1/collections", ReturnsContent: true, Mechanism: "collection is_public + ContentVisibilitySQL on item counts"},
	{Method: "GET", Path: "/api/v1/collections/:id", ReturnsContent: true, Mechanism: "collection is_public or owner; items filtered by ContentVisibilitySQL"},
	{Method: "GET", Path: "/api/v1/series/:id", ReturnsContent: true, Mechanism: "series items filtered by ContentVisibilitySQL (series metadata is public by design)"},
	{Method: "GET", Path: "/api/v1/judge/exam/:category", ReturnsContent: false, Mechanism: "login required (401 anonymous)"},
	{Method: "GET", Path: "/api/v1/judge/cases/:id/verdict", ReturnsContent: false, Mechanism: "governance transparency record (not content-derived); documented exposure"},
	{Method: "GET", Path: "/api/v1/stats/summary", ReturnsContent: true, Mechanism: "counts restricted to anonymous-visible content (visibility predicate)"},
	{Method: "GET", Path: "/api/v1/ips/stats/category_counts", ReturnsContent: false, Mechanism: "approved-IP category hash (not content-derived)"},
	{Method: "GET", Path: "/api/v1/categories", ReturnsContent: false, Mechanism: "category taxonomy"},
	{Method: "GET", Path: "/api/v1/openapi.json", ReturnsContent: false, Mechanism: "static contract document (no live data; #448)"},
	{Method: "POST", Path: "/api/v1/mcp", ReturnsContent: true, Mechanism: "MCP protocol endpoint; read tools apply ApplyContentVisibilityScope (viewer=0 anonymous, PAT viewer for scoped tools); write tools are PAT-scope gated per call - exposure review in mcpserver tests (#449/#451)"},
	{Method: "GET", Path: "/api/v1/mcp", ReturnsContent: false, Mechanism: "MCP protocol SSE stream channel (no tool data without POST)"},
	{Method: "DELETE", Path: "/api/v1/mcp", ReturnsContent: false, Mechanism: "MCP session termination"},
	{Method: "GET", Path: "/api/v1/tags/faceted", ReturnsContent: true, Mechanism: "co-occurrence counts restricted to anonymous-visible content"},
	{Method: "GET", Path: "/api/v1/tags/search", ReturnsContent: false, Mechanism: "global usage counters (not per-content)"},
	{Method: "GET", Path: "/api/v1/search/suggestions", ReturnsContent: true, Mechanism: "inline visibility predicate (status/deleted/author/ip/is_public)"},
	{Method: "GET", Path: "/api/v1/search/trending", ReturnsContent: true, Mechanism: "ApplyContentVisibilityScope(viewer)"},
	{Method: "GET", Path: "/api/v1/discussions/:id", ReturnsContent: true, Mechanism: "discussion detail gate: status=published"},
	{Method: "POST", Path: "/api/v1/feedback", ReturnsContent: false, Mechanism: "anonymous write: captcha-gated ticket intake (no read surface)"},
	{Method: "POST", Path: "/api/v1/feedback/attachments/presign", ReturnsContent: false, Mechanism: "anonymous write: captcha-gated image presign (no read surface)"},
}

var (
	optAuthGroupDeclRe = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*:=\s*v1\.Group\("([^"]*)"`)
	optAuthRouteDeclRe = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\.(GET|POST|PUT|PATCH|DELETE)\("([^"]*)",\s*optAuth\b`)
)

// parseOptAuthRoutes extracts the machine-readable anonymous-route list from
// the routes.go source. Group variables resolve to their v1 prefix; routes
// declared directly on v1 resolve with no prefix.
func parseOptAuthRoutes(source string) []optAuthRouteAudit {
	prefixes := map[string]string{"v1": ""}
	for _, m := range optAuthGroupDeclRe.FindAllStringSubmatch(source, -1) {
		prefixes[m[1]] = m[2]
	}

	var routes []optAuthRouteAudit
	for _, m := range optAuthRouteDeclRe.FindAllStringSubmatch(source, -1) {
		prefix, ok := prefixes[m[1]]
		if !ok {
			continue
		}
		routes = append(routes, optAuthRouteAudit{
			Method: m[2],
			Path:   "/api/v1" + prefix + m[3],
		})
	}
	return routes
}

func TestOptAuthRouteRegistryMatchesRoutesSource(t *testing.T) {
	parsed := parseOptAuthRoutes(readRoutesSource(t))
	if len(parsed) == 0 {
		t.Fatal("optAuth route parser found no routes — parser regex drifted from routes.go style")
	}

	parsedSet := map[string]bool{}
	for _, r := range parsed {
		key := r.Method + " " + r.Path
		if parsedSet[key] {
			t.Errorf("duplicate optAuth route declaration parsed: %s", key)
		}
		parsedSet[key] = true
	}
	registrySet := map[string]optAuthRouteAudit{}
	for _, r := range optAuthRouteRegistry {
		key := r.Method + " " + r.Path
		if _, dup := registrySet[key]; dup {
			t.Errorf("duplicate registry entry: %s", key)
		}
		registrySet[key] = r
	}

	for key := range parsedSet {
		if _, ok := registrySet[key]; !ok {
			t.Errorf("optAuth route %q is declared in routes.go but NOT registered in the audit registry (optAuthRouteRegistry) — every anonymous endpoint must pass visibility review before shipping; add the registry entry plus a matrix-doc row", key)
		}
	}
	for key := range registrySet {
		if !parsedSet[key] {
			t.Errorf("registry entry %q no longer exists in routes.go — remove the stale registry entry and its matrix-doc row", key)
		}
	}

	for _, r := range optAuthRouteRegistry {
		if r.ReturnsContent && strings.TrimSpace(r.Mechanism) == "" {
			t.Errorf("registry entry %s %s returns content data but documents no visibility mechanism", r.Method, r.Path)
		}
	}
}

// TestOptAuthRegistryCoversExposureMatrixDoc keeps the human-readable
// exposure matrix (docs/reference/agent-access-exposure-matrix.md, the P3
// MCP-tool baseline) in lockstep with the registry: every anonymous route
// must appear in the doc with its canonical "METHOD /api/v1/…" spelling.
func TestOptAuthRegistryCoversExposureMatrixDoc(t *testing.T) {
	docPath := filepath.Join("..", "..", "..", "docs", "reference", "agent-access-exposure-matrix.md")
	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("exposure matrix doc missing at %s: %v — the P0 audit deliverable must ship with the registry test", docPath, err)
	}
	doc := string(raw)

	for _, r := range optAuthRouteRegistry {
		canonical := r.Method + " " + r.Path
		if !strings.Contains(doc, canonical) {
			t.Errorf("exposure matrix doc %s does not cover anonymous route %q — add the matrix row", docPath, canonical)
		}
	}
	// Drift guard in the other direction: a doc row naming a route that no
	// longer exists misleads the P3 MCP implementation.
	docRouteKeys := map[string]bool{}
	for _, m := range regexp.MustCompile(`\| (GET|POST|PUT|PATCH|DELETE) (/api/v1/[^\s|]+)`).FindAllStringSubmatch(doc, -1) {
		docRouteKeys[m[1]+" "+m[2]] = true
	}
	registryKeys := map[string]bool{}
	for _, r := range optAuthRouteRegistry {
		registryKeys[r.Method+" "+r.Path] = true
	}
	docKeys := make([]string, 0, len(docRouteKeys))
	for k := range docRouteKeys {
		docKeys = append(docKeys, k)
	}
	sort.Strings(docKeys)
	for _, k := range docKeys {
		if !registryKeys[k] {
			t.Errorf("exposure matrix doc lists route %q that is not an optAuth route in the registry — stale doc row or unregistered route", k)
		}
	}
}
