package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testHandler(t *testing.T) (http.Handler, string) {
	t.Helper()
	sinkFile := filepath.Join(t.TempDir(), "events.jsonl")
	handler, err := newHandler(sinkFile, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("newHandler: %v", err)
	}
	return handler, sinkFile
}

func TestHealthz(t *testing.T) {
	handler, _ := testHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz returned %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("healthz content type %q", rec.Header().Get("Content-Type"))
	}
}

func TestPostAndGetEvents(t *testing.T) {
	handler, sinkFile := testHandler(t)
	body := `{"status":"firing","alerts":[{"labels":{"alertname":"TestAlert"}}]}`
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST returned %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/events", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET returned %d", rec.Code)
	}
	var recorded []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &recorded); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(recorded) != 1 || recorded[0]["status"] != "firing" {
		t.Fatalf("unexpected events: %v", recorded)
	}
	if _, ok := recorded[0]["received_at"].(string); !ok {
		t.Fatalf("received_at missing: %v", recorded[0])
	}
	raw, err := os.ReadFile(sinkFile)
	if err != nil {
		t.Fatalf("read sink file: %v", err)
	}
	if !strings.Contains(string(raw), "TestAlert") {
		t.Fatalf("sink file missing payload: %q", raw)
	}
}

func TestPostInvalidJSON(t *testing.T) {
	handler, _ := testHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString("not-json"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assertError(t, rec, http.StatusBadRequest, "INVALID_JSON")
}

func TestPostRejectsNull(t *testing.T) {
	handler, _ := testHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString("null"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assertError(t, rec, http.StatusBadRequest, "INVALID_JSON")
}

func TestPostRejectsMultipleJSONValues(t *testing.T) {
	handler, _ := testHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(`{} {}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assertError(t, rec, http.StatusBadRequest, "INVALID_JSON")
}

func TestPostRejectsOversizedPayload(t *testing.T) {
	handler, _ := testHandler(t)
	body := `{"payload":"` + strings.Repeat("x", maxRequestBytes) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assertError(t, rec, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE")
}

func TestMethodNotAllowed(t *testing.T) {
	handler, _ := testHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/events", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assertError(t, rec, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status %d, want %d: %s", rec.Code, status, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content type %q", rec.Header().Get("Content-Type"))
	}
	var envelope map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid error JSON: %v", err)
	}
	if envelope["code"] != code || envelope["message"] == "" {
		t.Fatalf("unexpected error envelope: %v", envelope)
	}
}
