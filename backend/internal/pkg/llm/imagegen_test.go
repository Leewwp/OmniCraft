package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCogViewClientURLRoundTrip(t *testing.T) {
	var gotPath string
	var gotBody ImageGenerateRequest
	var gotAuth string
	imageBytes := []byte("fake-png-body")
	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer dl.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"url": dl.URL + "/img.png"}},
		})
	}))
	defer srv.Close()

	c := NewCogViewClient(srv.URL, "sk-test", "cogview-4")
	out, err := c.GenerateImage(context.Background(), "灯塔夜景", "1024x1024", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/images/generations" {
		t.Fatalf("endpoint path = %s", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("auth header = %q", gotAuth)
	}
	if gotBody.Model != "cogview-4" || gotBody.Prompt != "灯塔夜景" || gotBody.Size != "1024x1024" {
		t.Fatalf("request body = %+v", gotBody)
	}
	if string(out) != string(imageBytes) {
		t.Fatalf("downloaded bytes mismatch")
	}
}

func TestCogViewClientB64Payload(t *testing.T) {
	payload := []byte("b64-image-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString(payload)}},
		})
	}))
	defer srv.Close()
	c := NewCogViewClient(srv.URL, "k", "cogview-4")
	out, err := c.GenerateImage(context.Background(), "p", "", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(payload) {
		t.Fatalf("b64 decode mismatch")
	}
}

func TestCogViewClientLimitsAndErrors(t *testing.T) {
	// oversize download is rejected, not truncated silently
	big := strings.Repeat("x", 100)
	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(big))
	}))
	defer dl.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"url": dl.URL + "/big.png"}},
		})
	}))
	defer srv.Close()
	c := NewCogViewClient(srv.URL, "k", "cogview-4")
	if _, err := c.GenerateImage(context.Background(), "p", "", 10); err == nil || !strings.Contains(err.Error(), "max bytes") {
		t.Fatalf("oversize must be rejected: %v", err)
	}

	// provider error payload surfaces
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "bad prompt"}})
	}))
	defer errSrv.Close()
	c2 := NewCogViewClient(errSrv.URL, "k", "cogview-4")
	if _, err := c2.GenerateImage(context.Background(), "p", "", 1<<20); err == nil || !strings.Contains(err.Error(), "bad prompt") {
		t.Fatalf("provider error must surface: %v", err)
	}

	// empty data array
	emptySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer emptySrv.Close()
	c3 := NewCogViewClient(emptySrv.URL, "k", "cogview-4")
	if _, err := c3.GenerateImage(context.Background(), "p", "", 1<<20); err == nil || !strings.Contains(err.Error(), "no image") {
		t.Fatalf("empty data must fail: %v", err)
	}
}
