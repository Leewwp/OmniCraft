package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSearchAbuseRejectLongQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	rejected := rejectLongQuery(c, strings.Repeat("中", 121), 120)

	if !rejected {
		t.Fatal("rejectLongQuery() = false, want true")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "QUERY_TOO_LONG") {
		t.Fatalf("body = %s, want QUERY_TOO_LONG", w.Body.String())
	}
}

func TestSearchLimitClamp(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{name: "too large", raw: "100000", want: 50},
		{name: "zero", raw: "0", want: 20},
		{name: "not number", raw: "not-a-number", want: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampLimit(tt.raw, 20, 50); got != tt.want {
				t.Fatalf("clampLimit(%q, 20, 50) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestSearchPageClamp(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{name: "too large", raw: "100000", want: 100},
		{name: "zero", raw: "0", want: 1},
		{name: "not number", raw: "not-a-number", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampPage(tt.raw, 100); got != tt.want {
				t.Fatalf("clampPage(%q, 100) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}
