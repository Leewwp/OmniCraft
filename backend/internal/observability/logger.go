package observability

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"omnicraft/backend/config"
)

const (
	serviceName = "omnicraft-backend"

	// hashPrefixLength is the 128-bit prefix of the HMAC-SHA256 digest that
	// becomes the client_ip log value (32 lowercase hex characters).
	hashPrefixLength = 16

	// unresolvedClientIP is the bounded marker used when no key is available;
	// a raw IP is never emitted.
	unresolvedClientIP = "00000000000000000000000000000000"
)

// IPHasher derives a keyed, bounded client IP identifier. The raw client IP
// never appears in logs: the first 128 bits of HMAC-SHA256(secret, ip) are
// hex-encoded to a fixed 32-character lowercase string, and every log line
// records the non-secret key ID so hashes can be rotated and correlated.
type IPHasher struct {
	currentSecret  []byte
	currentKeyID   string
	previousSecret []byte
	previousKeyID  string
	activeFrom     time.Time
	activeUntil    time.Time
	now            func() time.Time
}

// NewIPHasher builds an IPHasher. An empty current secret disables hashing
// (the placeholder marker is used) but never falls back to the raw IP.
// Callers in release mode must validate config first so a missing key fails
// closed before any request is served.
func NewIPHasher(cfg config.ObservabilityConfig) (*IPHasher, error) {
	h := &IPHasher{
		currentSecret: []byte(cfg.LogIPHashSecret),
		currentKeyID:  cfg.LogIPKeyID,
		now:           time.Now,
	}
	rot := cfg.IPKeyRotation
	if strings.TrimSpace(rot.PreviousSecret) != "" || strings.TrimSpace(rot.PreviousKeyID) != "" ||
		strings.TrimSpace(rot.ActiveFrom) != "" || strings.TrimSpace(rot.ActiveUntil) != "" {
		from, err := time.Parse(time.RFC3339, rot.ActiveFrom)
		if err != nil {
			return nil, fmt.Errorf("active_from must be RFC3339: %w", err)
		}
		until, err := time.Parse(time.RFC3339, rot.ActiveUntil)
		if err != nil {
			return nil, fmt.Errorf("active_until must be RFC3339: %w", err)
		}
		if !until.After(from) {
			return nil, errors.New("active_until must be after active_from")
		}
		h.previousSecret = []byte(rot.PreviousSecret)
		h.previousKeyID = rot.PreviousKeyID
		h.activeFrom = from
		h.activeUntil = until
	}
	return h, nil
}

func hashWith(key []byte, ip string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil)[:hashPrefixLength])
}

// Hash returns the bounded identifier for ip using the current key. When no
// secret is configured a fixed placeholder is returned; the raw IP is never
// used.
func (h *IPHasher) Hash(ip string) string {
	if len(h.currentSecret) == 0 {
		return unresolvedClientIP
	}
	return hashWith(h.currentSecret, ip)
}

// KeyID is the non-secret identifier of the key currently producing hashes.
func (h *IPHasher) KeyID() string {
	if h.currentKeyID == "" {
		return "none"
	}
	return h.currentKeyID
}

// HashPrevious hashes ip with the previous key, but only while the explicit
// rotation window is active. Outside the window the previous key is refused
// so stale hashes cannot be correlated indefinitely.
func (h *IPHasher) HashPrevious(ip string, at time.Time) (string, error) {
	if len(h.previousSecret) == 0 {
		return "", errors.New("no previous IP hash key configured")
	}
	if at.Before(h.activeFrom) || at.After(h.activeUntil) {
		return "", fmt.Errorf("previous IP hash key is outside its rotation window (%s..%s)", h.activeFrom.Format(time.RFC3339), h.activeUntil.Format(time.RFC3339))
	}
	return hashWith(h.previousSecret, ip), nil
}

// ErrorClass maps an HTTP status to a bounded error category used as the
// error_class log field. Non-error statuses yield "none".
func ErrorClass(status int) string {
	switch {
	case status >= 500:
		return "internal"
	case status >= 400:
		return "client"
	default:
		return "none"
	}
}

// stableAttributes returns the fixed identity attributes attached to every
// structured log line.
func stableAttributes(cfg config.Config) map[string]string {
	return map[string]string{
		"service":     serviceName,
		"environment": cfg.Server.Mode,
		"version":     Version,
	}
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// newLoggerWithAttrs attaches the stable identity attributes to a handler.
func newLoggerWithAttrs(handler slog.Handler, cfg config.Config) *slog.Logger {
	attrs := stableAttributes(cfg)
	return slog.New(handler).With(
		"service", attrs["service"],
		"environment", attrs["environment"],
		"version", attrs["version"],
	)
}

// NewLogger builds the JSON slog logger used by the server. It writes to
// stdout and always carries service/environment/version identity fields.
func NewLogger(cfg config.Config) (*slog.Logger, error) {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.Observability.LogLevel)})
	return newLoggerWithAttrs(handler, cfg), nil
}

// WriteTextfile writes Prometheus textfile-format content atomically (temp
// file + rename) into dir. lines must already be fully formatted (including
// # HELP/# TYPE headers). Used by migration/backup/recovery scripts so their
// success/failure/last-success state is scrapeable by a textfile collector
// without embedding a Prometheus client in every script.
func WriteTextfile(dir, name string, lines []string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create textfile dir: %w", err)
	}
	var builder strings.Builder
	for _, line := range lines {
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	tmp, err := os.CreateTemp(dir, ".textfile-*.prom")
	if err != nil {
		return fmt.Errorf("create textfile temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(builder.String()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write textfile temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close textfile temp: %w", err)
	}
	if err := os.Rename(tmpName, filepath.Join(dir, name)); err != nil {
		return fmt.Errorf("rename textfile into place: %w", err)
	}
	return nil
}
