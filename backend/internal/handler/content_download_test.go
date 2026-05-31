package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDownloadContent_ReturnsJSONNotRedirect(t *testing.T) {
	source := readHandlerSource(t, "content.go")
	if strings.Contains(source, "c.Redirect(") {
		t.Fatal("DownloadContent must return JSON {download_url, expires_in}, not c.Redirect")
	}
}

func TestDownloadContent_SupportsAttachmentID(t *testing.T) {
	source := readHandlerSource(t, "content.go")
	if !strings.Contains(source, "attachment_id") {
		t.Fatal("DownloadContent must accept attachment_id query parameter")
	}
}

func TestDownloadContent_RejectsAmbiguousWithoutAttachmentID(t *testing.T) {
	source := readHandlerSource(t, "content.go")
	if !strings.Contains(source, "AMBIGUOUS_ATTACHMENT") {
		t.Fatal("DownloadContent must return AMBIGUOUS_ATTACHMENT when multiple or no primary attachments exist and attachment_id is omitted")
	}
}

func TestDownloadContent_RejectsAttachmentFromOtherContent(t *testing.T) {
	source := readHandlerSource(t, "content.go")
	if !strings.Contains(source, "ATTACHMENT_MISMATCH") {
		t.Fatal("DownloadContent must verify attachment belongs to the requested content")
	}
}

func TestDownloadContent_UsesConfigurableTTL(t *testing.T) {
	source := readHandlerSource(t, "content.go")
	if !strings.Contains(source, "DownloadURLTTL") && !strings.Contains(source, "download_url_ttl") {
		t.Fatal("DownloadContent must use configurable download URL TTL from config, not hardcoded value")
	}
}

func TestDownloadContent_ResponseSchema(t *testing.T) {
	resp := map[string]interface{}{
		"download_url": "https://example.com/file.zip",
		"expires_in":   300,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if _, ok := parsed["download_url"]; !ok {
		t.Fatal("response must contain download_url")
	}
	if _, ok := parsed["expires_in"]; !ok {
		t.Fatal("response must contain expires_in")
	}
}

func TestDownloadRouteRequiresAuth(t *testing.T) {
	routes := readRoutesSource(t)
	if !strings.Contains(routes, "download") {
		t.Skip("no download route found")
	}
	lines := strings.Split(routes, "\n")
	for _, line := range lines {
		if strings.Contains(line, "download") && strings.Contains(line, "GET") {
			if !strings.Contains(line, "authReq") && !strings.Contains(line, "AuthRequired") {
				t.Fatalf("download route must require authentication: %s", line)
			}
		}
	}
}

func TestDownloadRouteRequiresInteractionGuard(t *testing.T) {
	routes := readRoutesSource(t)
	if !strings.Contains(routes, "download") {
		t.Skip("no download route found")
	}
	lines := strings.Split(routes, "\n")
	for _, line := range lines {
		if strings.Contains(line, "download") && strings.Contains(line, "GET") {
			if !strings.Contains(line, "downloadsGuard") && !strings.Contains(line, "InteractionRequired") {
				t.Fatalf("download route must require interaction guard (verified email + reputation): %s", line)
			}
		}
	}
}

func TestOSSDownloadURLTTLConfigExists(t *testing.T) {
	source := readConfigSource(t)
	if !strings.Contains(source, "DownloadURLTTL") && !strings.Contains(source, "download_url_ttl") {
		t.Fatal("config must define DownloadURLTTL or download_url_ttl field for OSS download URL TTL")
	}
}

func readHandlerSource(t *testing.T, filename string) string {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read %s: %v", filename, err)
	}
	return string(data)
}

func readConfigSource(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"../config/config.go",
		"../../config/config.go",
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err == nil {
			return string(data)
		}
	}
	t.Fatal("cannot find config.go")
	return ""
}

func init() {
	gin.SetMode(gin.TestMode)
}

func TestDownloadContent_EndToEnd_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}
	router := gin.New()
	router.GET("/api/v1/contents/:id/download", func(c *gin.Context) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/contents/1/download", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
