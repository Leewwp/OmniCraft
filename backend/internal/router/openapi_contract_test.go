package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"omnicraft/backend/internal/handler"
)

// SP-16 #448: the OpenAPI contract cannot drift from the live surface.
// Every documented path+method must exist as a real route, be anonymously
// reachable, and the repo browsing copy must match the served bytes.

type openAPIPathItem map[string]map[string]any

func parseOpenAPIV1Spec(t *testing.T) (map[string]openAPIPathItem, []byte) {
	t.Helper()
	raw := handler.OpenAPIV1Spec()
	var spec struct {
		OpenAPI string                  `json:"openapi"`
		Paths   map[string]openAPIPathItem `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("openapi spec is not valid JSON: %v", err)
	}
	if spec.OpenAPI == "" || !regexp.MustCompile(`^3\.1\.`).MatchString(spec.OpenAPI) {
		t.Fatalf("openapi field = %q, want a 3.1.x document", spec.OpenAPI)
	}
	if len(spec.Paths) == 0 {
		t.Fatal("openapi spec has no paths")
	}
	return spec.Paths, raw
}

func TestOpenAPIDocsCopyMatchesEmbeddedSpec(t *testing.T) {
	_, raw := parseOpenAPIV1Spec(t)
	docsPath := filepath.Join("..", "..", "..", "docs", "reference", "openapi.v1.json")
	docsRaw, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("docs copy missing at %s: %v", docsPath, err)
	}
	if !bytes.Equal(raw, docsRaw) {
		t.Fatalf("docs/reference/openapi.v1.json differs from the served spec — resync the browsing copy (they must stay byte-identical)")
	}
}

func TestOpenAPIPathsExistAndAreAnonymouslyReachable(t *testing.T) {
	paths, _ := parseOpenAPIV1Spec(t)
	router, _, _, fx := buildOptAuthLeakAuditStack(t)

	// gin param form + per-path fixture substitution for concrete requests.
	ginPath := regexp.MustCompile(`\{(\w+)\}`)
	pathFixtures := map[string]map[string]string{
		"/contents/{id}":          {"id": fmt.Sprintf("%d", fx.cPub)},
		"/contents/{id}/versions": {"id": fmt.Sprintf("%d", fx.cPub)},
		"/contents/{id}/guide":    {"id": fmt.Sprintf("%d", fx.cPub)},
		"/users/{id}":             {"id": fmt.Sprintf("%d", fx.authorID)},
	}

	registered := map[string]bool{}
	for _, r := range router.Routes() {
		registered[r.Method+" "+r.Path] = true
	}

	for path, item := range paths {
		gin := "/api/v1" + ginPath.ReplaceAllString(path, ":$1")
		for method := range item {
			if method == "parameters" {
				continue
			}
			httpMethod := strings.ToUpper(method)
			if !registered[httpMethod+" "+gin] {
				t.Errorf("openapi documents %s %s but no such route exists (gin form %s) — contract drift", httpMethod, path, gin)
				continue
			}
			fixtures := pathFixtures[path]
			requestPath := "/api/v1" + ginPath.ReplaceAllStringFunc(path, func(m string) string {
				name := m[1 : len(m)-1]
				if v, ok := fixtures[name]; ok {
					return v
				}
				return "1"
			})
			req := httptest.NewRequest(httpMethod, requestPath, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
				t.Errorf("%s %s anonymous reachability broken: status %d", method, requestPath, rec.Code)
			}
		}
	}
}

func TestContractedAnonymousGETsHonourCacheContract(t *testing.T) {
	router, _, _, fx := buildOptAuthLeakAuditStack(t)

	get := func(path string, ifNoneMatch string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if ifNoneMatch != "" {
			req.Header.Set("If-None-Match", ifNoneMatch)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	contracted := []string{
		"/api/v1/contents?page=1",
		fmt.Sprintf("/api/v1/contents/%d", fx.cPub),
		fmt.Sprintf("/api/v1/contents/%d/versions", fx.cPub),
		fmt.Sprintf("/api/v1/contents/%d/guide", fx.cPub),
		"/api/v1/categories",
		"/api/v1/tags/faceted",
		"/api/v1/ips",
		fmt.Sprintf("/api/v1/users/%d", fx.authorID),
		"/api/v1/stats/summary",
	}
	for _, path := range contracted {
		first := get(path, "")
		if first.Code != http.StatusOK {
			t.Errorf("[%s] status %d, want 200", path, first.Code)
			continue
		}
		cc := first.Header().Get("Cache-Control")
		etag := first.Header().Get("ETag")
		if !strings.Contains(cc, "s-maxage=") {
			t.Errorf("[%s] Cache-Control = %q, want s-maxage present", path, cc)
		}
		if etag == "" {
			t.Errorf("[%s] ETag missing on contracted anonymous GET", path)
			continue
		}
		second := get(path, etag)
		if second.Code != http.StatusNotModified {
			t.Errorf("[%s] conditional revalidation status %d, want 304 (etag %s)", path, second.Code, etag)
		}
		if second.Body.Len() != 0 {
			t.Errorf("[%s] 304 must carry an empty body, got %d bytes", path, second.Body.Len())
		}
	}
}
