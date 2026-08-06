// loki-gate is the authenticated operator access point for Loki. It accepts
// only requests carrying the operator token, forwards them to Loki over the
// internal network and appends every request to an audit file on a durable
// volume so operator query access is retained. It never exposes Loki to the
// public network; the recommended production access path is an SSH tunnel to
// the host port bound to 127.0.0.1.
package main

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	token := os.Getenv("GATE_TOKEN")
	upstreamRaw := os.Getenv("LOKI_UPSTREAM")
	auditDir := os.Getenv("AUDIT_DIR")
	if token == "" || upstreamRaw == "" || auditDir == "" {
		slog.Error("required gate configuration is missing")
		os.Exit(1)
	}
	if err := os.MkdirAll(auditDir, 0o755); err != nil {
		slog.Error("create audit dir failed", "error", err)
		os.Exit(1)
	}

	upstream, err := url.Parse(upstreamRaw)
	if err != nil {
		slog.Error("invalid Loki upstream", "error", err)
		os.Exit(1)
	}

	audit := &auditLog{dir: auditDir, file: filepath.Join(auditDir, "access-audit.jsonl")}

	proxy := httputil.NewSingleHostReverseProxy(upstream)
	handler := newGateHandler(token, proxy, audit)

	slog.Info("loki-gate listening", "addr", ":8080", "upstream", upstreamRaw)
	server := &http.Server{Addr: ":8080", Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	if err := server.ListenAndServe(); err != nil {
		slog.Error("loki-gate stopped", "error", err)
		os.Exit(1)
	}
}

func newGateHandler(token string, proxy http.Handler, audit *auditLog) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorized := subtle.ConstantTimeCompare([]byte(strings.TrimSpace(r.Header.Get("Authorization"))), []byte("Bearer "+token)) == 1
		if !authorized {
			if err := audit.record(r, http.StatusUnauthorized); err != nil {
				writeError(w, http.StatusServiceUnavailable, "AUDIT_UNAVAILABLE", "operator audit is unavailable")
				return
			}
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
			return
		}

		captured := newCaptureWriter()
		proxy.ServeHTTP(captured, r)
		if err := audit.record(r, captured.statusCode()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "AUDIT_UNAVAILABLE", "operator audit is unavailable")
			return
		}
		captured.commit(w)
	})
}

type captureWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newCaptureWriter() *captureWriter {
	return &captureWriter{header: make(http.Header)}
}

func (w *captureWriter) Header() http.Header { return w.header }

func (w *captureWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *captureWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(body)
}

func (w *captureWriter) Flush() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
}

func (w *captureWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *captureWriter) commit(destination http.ResponseWriter) {
	for key, values := range w.header {
		for _, value := range values {
			destination.Header().Add(key, value)
		}
	}
	destination.WriteHeader(w.statusCode())
	_, _ = destination.Write(w.body.Bytes())
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}

type auditLog struct {
	dir  string
	file string
	mu   sync.Mutex
}

func (a *auditLog) record(r *http.Request, status int) error {
	entry := map[string]any{
		"time":         time.Now().UTC().Format(time.RFC3339),
		"method":       r.Method,
		"path":         r.URL.Path,
		"status":       status,
		"has_token":    r.Header.Get("Authorization") != "",
		"gate_version": "1",
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	f, err := os.OpenFile(a.file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}
