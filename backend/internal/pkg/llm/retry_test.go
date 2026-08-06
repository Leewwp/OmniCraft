package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestProviderRetries429AndRetryable5xxThenSucceeds covers bounded retries for
// configured network/429/5xx conditions: the request succeeds and the server
// observes exactly one attempt per status in the sequence.
func TestProviderRetries429AndRetryable5xxThenSucceeds(t *testing.T) {
	for _, sequence := range [][]int{{429, 200}, {500, 503, 200}, {504, 200}} {
		var calls int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			idx := int(atomic.AddInt32(&calls, 1)) - 1
			if idx < len(sequence) {
				code := sequence[idx]
				if code >= 200 && code < 300 {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
					return
				}
				w.WriteHeader(code)
				_, _ = io.WriteString(w, "attempt")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
		}))

		p := NewOpenAICompatProvider("k", server.URL, "m", "e", WithMaxRetries(4), WithTimeout(5*time.Second))
		_, err := p.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
		server.Close()
		if err != nil {
			t.Fatalf("statuses %v: Chat err = %v, want eventual success", sequence, err)
		}
		if got := atomic.LoadInt32(&calls); int(got) != len(sequence) {
			t.Fatalf("statuses %v: server calls = %d, want %d (one per attempt, bounded)", sequence, got, len(sequence))
		}
	}
}

// TestProviderDoesNotRetryNonRetryable4xx covers the 4xx no-retry rule.
func TestProviderDoesNotRetryNonRetryable4xx(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 422} {
		var calls int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(status)
			_, _ = io.WriteString(w, "bad request")
		}))

		p := NewOpenAICompatProvider("k", server.URL, "m", "e", WithMaxRetries(4), WithTimeout(5*time.Second))
		_, err := p.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
		server.Close()
		if err == nil {
			t.Fatalf("status %d: want error", status)
		}
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Fatalf("status %d: server calls = %d, want exactly 1 (non-retryable 4xx)", status, got)
		}
	}
}

// TestProviderRetriesNetworkErrorThenSucceeds covers retry on transport-level
// failure (connection dropped mid-handshake).
func TestProviderRetriesNetworkErrorThenSucceeds(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))

	p := NewOpenAICompatProvider("k", server.URL, "m", "e", WithMaxRetries(2), WithTimeout(5*time.Second))
	_, err := p.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	server.Close()
	if err != nil {
		t.Fatalf("Chat err = %v, want success after network retry", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("server calls = %d, want 2", got)
	}
}

// TestProviderTimeoutCovers cfg.ProviderTimeoutSec enforcement.
func TestProviderTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer server.Close()

	p := NewOpenAICompatProvider("k", server.URL, "m", "e", WithMaxRetries(0), WithTimeout(100*time.Millisecond))
	start := time.Now()
	_, err := p.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("want timeout error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("timeout not enforced: elapsed = %v", elapsed)
	}
}
