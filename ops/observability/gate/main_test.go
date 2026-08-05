package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGateHandlerUsesSafeUnauthorizedEnvelopeAndRedactsAuditInputs(t *testing.T) {
	auditDir := t.TempDir()
	audit := &auditLog{dir: auditDir, file: filepath.Join(auditDir, "access-audit.jsonl")}
	handler := newGateHandler("operator-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), audit)

	req := httptest.NewRequest(http.MethodGet, "/loki/api/v1/query_range?query=secret-message", nil)
	req.RemoteAddr = "203.0.113.7:1234"
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if strings.Contains(string(body), "secret-message") || strings.Contains(string(body), "error") {
		t.Fatalf("response exposed unsafe/error-shaped data: %s", body)
	}

	auditBytes, err := os.ReadFile(audit.file)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	auditText := string(auditBytes)
	if strings.Contains(auditText, "203.0.113.7") || strings.Contains(auditText, "secret-message") {
		t.Fatalf("audit retained raw request data: %s", auditText)
	}
}

func TestGateHandlerFailsClosedWhenAuditCannotBeWritten(t *testing.T) {
	parent := t.TempDir()
	badParent := filepath.Join(parent, "not-a-directory")
	if err := os.WriteFile(badParent, []byte("blocker"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	audit := &auditLog{dir: badParent, file: filepath.Join(badParent, "access-audit.jsonl")}
	handler := newGateHandler("operator-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), audit)

	req := httptest.NewRequest(http.MethodGet, "/loki/api/v1/query_range", nil)
	req.Header.Set("Authorization", "Bearer operator-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when audit fails", rec.Code)
	}
}

func TestGateHandlerAuditsUpstreamStatus(t *testing.T) {
	auditDir := t.TempDir()
	audit := &auditLog{dir: auditDir, file: filepath.Join(auditDir, "access-audit.jsonl")}
	handler := newGateHandler("operator-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}), audit)

	req := httptest.NewRequest(http.MethodGet, "/loki/api/v1/query_range", nil)
	req.Header.Set("Authorization", "Bearer operator-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	auditBytes, err := os.ReadFile(audit.file)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if !strings.Contains(string(auditBytes), `"status":502`) {
		t.Fatalf("audit did not retain upstream status: %s", auditBytes)
	}
}
