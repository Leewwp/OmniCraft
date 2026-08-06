package observability

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server is the internal observability endpoint. It listens only on an
// internal port that is never published to the public network and exposes
// /metrics, liveness /healthz and dependency-aware /readyz. Readiness never
// returns connection details; it only reports ready/unavailable.
type Server struct {
	mux               *http.ServeMux
	readHeaderTimeout time.Duration
}

// NewServer builds the internal metrics/readiness server. ready must return
// nil only when all required dependencies are healthy.
func NewServer(reg *prometheus.Registry, ready func() error, readHeaderTimeout time.Duration) *Server {
	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := ready(); err != nil {
			// Deliberately opaque: dependency identity/errors must not leak.
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unavailable"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	return &Server{mux: mux, readHeaderTimeout: readHeaderTimeout}
}

// Serve runs the internal HTTP server on addr until the server errors.
func (s *Server) Serve(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: s.readHeaderTimeout,
	}
	slog.Info("observability server listening", "addr", addr)
	if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
