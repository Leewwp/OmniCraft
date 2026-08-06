package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAgentModerationRouteRemovedFromOrdinaryUsers asserts the ordinary
// authenticated Agent route group no longer exposes POST /agent/moderate/:id.
// Moderation remains an authorized admin/worker concern and cannot be
// model-selected; the endpoint must not even exist for ordinary users.
func TestAgentModerationRouteRemovedFromOrdinaryUsers(t *testing.T) {
	router, _, cleanup := buildRoutesSecurityRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/moderate/1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST /agent/moderate/1 status = %d, want 404 (route must be removed); body = %s", rec.Code, rec.Body.String())
	}
	if rec.Code == http.StatusOK || rec.Code == http.StatusBadRequest {
		t.Fatal("moderation route must not be registered for ordinary users")
	}
}
