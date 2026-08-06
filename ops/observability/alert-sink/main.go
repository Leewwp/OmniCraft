package main

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxRequestBytes = 4 << 20

type eventSink struct {
	mu     sync.Mutex
	path   string
	events []map[string]any
	logger *slog.Logger
}

func newHandler(path string, logger *slog.Logger) (http.Handler, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}

	sink := &eventSink{path: path, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", sink.handleHealth)
	mux.HandleFunc("/events", sink.handleEvents)
	return mux, nil
}

func sinkFilePath() string {
	if path := os.Getenv("ALERT_SINK_FILE"); path != "" {
		return path
	}
	return "/events/events.jsonl"
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler, err := newHandler(sinkFilePath(), logger)
	if err != nil {
		logger.Error("initialize alert sink", "error", err)
		os.Exit(1)
	}

	addr := os.Getenv("ALERT_SINK_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	logger.Info("alert sink listening", "addr", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("serve alert sink", "error", err)
		os.Exit(1)
	}
}

func (s *eventSink) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", s.logger)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}, s.logger)
}

func (s *eventSink) handleEvents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handlePost(w, r)
	case http.MethodGet:
		s.handleGet(w)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", s.logger)
	}
}

func (s *eventSink) handlePost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body exceeds 4 MiB", s.logger)
			return
		}
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body must be a JSON object", s.logger)
		return
	}
	if payload == nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body must be a JSON object", s.logger)
		return
	}
	if err := ensureEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body must contain one JSON object", s.logger)
		return
	}
	payload["received_at"] = time.Now().UTC().Format(time.RFC3339)

	line, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("marshal alert payload", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to record event", s.logger)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		s.logger.Error("open alert sink file", "error", err)
		writeError(w, http.StatusInternalServerError, "STORAGE_ERROR", "failed to record event", s.logger)
		return
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			s.logger.Error("close alert sink file after write failure", "error", closeErr)
		}
		s.logger.Error("write alert sink file", "error", err)
		writeError(w, http.StatusInternalServerError, "STORAGE_ERROR", "failed to record event", s.logger)
		return
	}
	if err := f.Close(); err != nil {
		s.logger.Error("close alert sink file", "error", err)
		writeError(w, http.StatusInternalServerError, "STORAGE_ERROR", "failed to record event", s.logger)
		return
	}
	s.events = append(s.events, payload)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "recorded"}, s.logger)
}

func (s *eventSink) handleGet(w http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, http.StatusOK, s.events, s.logger)
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	}
	return errors.New("additional JSON value")
}

func writeError(w http.ResponseWriter, status int, code, message string, logger *slog.Logger) {
	writeJSON(w, status, map[string]string{"code": code, "message": message}, logger)
}

func writeJSON(w http.ResponseWriter, status int, value any, logger *slog.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		logger.Error("write JSON response", "error", err)
	}
}
