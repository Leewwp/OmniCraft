package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// AgentImageGenerator is the provider seam behind the generate_image tool
// (SP-23 M1, #566). The CogView-4 client implements it against the
// OpenAI-compatible images endpoint; tests drive it with a stub.
type AgentImageGenerator interface {
	// GenerateImage produces one image for the prompt and returns the raw
	// bytes. Implementations must honor the context deadline (the caller
	// wraps it with agent.image.timeout_sec) and bound the response to
	// maxImageBytes.
	GenerateImage(ctx context.Context, prompt, size string, maxImageBytes int) ([]byte, error)
}

// ImageGenerateRequest is the OpenAI-compatible images/generations body.
type ImageGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Size   string `json:"size,omitempty"`
}

// ImageGenerateResponse is the subset of the images payload the tool reads.
type ImageGenerateResponse struct {
	Data []struct {
		URL     string `json:"url"`
		B64JSON string `json:"b64_json"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// CogViewClient talks to a Zhipu CogView-4 OpenAI-compatible images endpoint.
// External image URLs it returns are downloaded here and never leave the
// backend: the agent tool re-uploads the bytes to the platform's own OSS
// bucket before anything reaches the model or the UI.
type CogViewClient struct {
	APIBase string
	APIKey  string
	Model   string
	Client  *http.Client
	// AllowPrivateHosts relaxes the download host guard so tests can point
	// at httptest servers on 127.0.0.1; production leaves it false.
	AllowPrivateHosts bool
}

// maxRedirectHops bounds the redirect chain the download client follows
// (audit #8 / #814): every hop re-runs the host guard, and no chain may run
// longer than this.
const maxRedirectHops = 5

// NewCogViewClient builds the client; apiBase must not carry a trailing
// slash (the path is appended verbatim).
func NewCogViewClient(apiBase, apiKey, model string) *CogViewClient {
	c := &CogViewClient{
		APIBase: strings.TrimRight(strings.TrimSpace(apiBase), "/"),
		APIKey:  apiKey,
		Model:   model,
	}
	transport := &http.Transport{}
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = base.Clone()
	}
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	// Socket-level backstop behind the URL guards (DNS-rebinding depth):
	// even a public-looking hostname must not resolve-and-dial into a
	// private range. It reads AllowPrivateHosts live so tests can point at
	// loopback httptest servers by flipping the flag after construction.
	dialer.Control = func(_, address string, _ syscall.RawConn) error {
		if c.AllowPrivateHosts {
			return nil
		}
		return guardDialAddress(address)
	}
	transport.DialContext = dialer.DialContext
	c.Client = &http.Client{
		Timeout:       90 * time.Second,
		Transport:     transport,
		CheckRedirect: c.redirectPolicy,
	}
	return c
}

// redirectPolicy guards every redirect hop (audit #8 / #814): the first-hop
// URL check alone lets a hijacked provider answer the trusted origin with a
// 30x pointing at an intranet/metadata endpoint.
func (c *CogViewClient) redirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirectHops {
		return fmt.Errorf("image download exceeded %d redirect hops", maxRedirectHops)
	}
	return c.guardDownloadHost(req.URL)
}

// GenerateImage runs one images/generations call and downloads the resulting
// image bytes (url or base64 payload), bounded by maxImageBytes.
func (c *CogViewClient) GenerateImage(ctx context.Context, prompt, size string, maxImageBytes int) ([]byte, error) {
	body, err := json.Marshal(ImageGenerateRequest{Model: c.Model, Prompt: prompt, Size: size})
	if err != nil {
		return nil, fmt.Errorf("image request encode: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.APIBase+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("image request build: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("image provider call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("image provider read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image provider status %d: %s", resp.StatusCode, truncateForLog(string(raw), 200))
	}
	var parsed ImageGenerateResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("image provider decode: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("image provider error: %s", parsed.Error.Message)
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("image provider returned no image")
	}
	first := parsed.Data[0]
	if first.B64JSON != "" {
		return decodeBase64Image(first.B64JSON, maxImageBytes)
	}
	if first.URL == "" {
		return nil, fmt.Errorf("image provider returned neither url nor b64_json")
	}
	return c.download(ctx, first.URL, maxImageBytes)
}

func (c *CogViewClient) download(ctx context.Context, rawURL string, maxImageBytes int) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("image download url parse: %w", err)
	}
	if err := c.guardDownloadHost(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("image download build: %w", err)
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("image download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image download status %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, int64(maxImageBytes)+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("image download read: %w", err)
	}
	if len(raw) > maxImageBytes {
		return nil, fmt.Errorf("image exceeds max bytes (%d)", maxImageBytes)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("image download returned empty body")
	}
	return raw, nil
}

// guardDownloadHost enforces that a provider-supplied image URL points at a
// public host (SP-25 低-11 SSRF depth): a hijacked provider must not turn
// the backend into an intranet/metadata fetcher whose bytes get re-hosted
// on OSS. The provider's own api_base host is always trusted (it IS the
// configured trust anchor); private/loopback/link-local literals are
// rejected everywhere else.
func (c *CogViewClient) guardDownloadHost(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("image url scheme must be http(s), got %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("image url host is empty")
	}
	if c.AllowPrivateHosts {
		return nil
	}
	if base, err := url.Parse(c.APIBase); err == nil && sameOriginPort(base, u) {
		return nil
	}
	if isPrivateOrLocalHost(host) {
		return fmt.Errorf("image url host %q is a private/loopback/link-local address", host)
	}
	return nil
}

// sameOriginPort reports whether u matches the api_base trust anchor on
// scheme, host AND port (explicit or scheme-implied default) — the anchor is
// origin-scoped, not hostname-scoped (audit #8 / #814: same host on another
// port must not be exempt).
func sameOriginPort(base, u *url.URL) bool {
	if !strings.EqualFold(base.Scheme, u.Scheme) {
		return false
	}
	if !strings.EqualFold(base.Hostname(), u.Hostname()) {
		return false
	}
	return canonicalPort(base) == canonicalPort(u)
}

func canonicalPort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	return "80"
}

func isPrivateOrLocalHost(host string) bool {
	h := strings.ToLower(strings.Trim(host, "[]"))
	// Trailing-dot FQDNs ("localhost.") resolve to the same names as their
	// dotless form — normalize before suffix matching (audit #8 / #814).
	h = strings.Trim(h, ".")
	if h == "localhost" || strings.HasSuffix(h, ".localhost") ||
		strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") {
		return true
	}
	ip := net.ParseIP(h)
	if ip == nil {
		return false
	}
	return privateIPOrLocal(ip)
}

// privateIPOrLocal classifies literal/resolved IPs, including the CGNAT
// 100.64/10 block carrying the Aliyun IMDS endpoint 100.100.100.200 — the
// net.IP private helpers do not cover it (audit #8 / #814).
func privateIPOrLocal(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// guardDialAddress is the dial-time hook behind the URL guards: by the time
// the resolver has picked an address the URL string is already trusted, so
// this is the only place a rebinding-style resolution into a private range
// can still be stopped.
func guardDialAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("image client dial address %q: %w", address, err)
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return fmt.Errorf("image client dial address %q is not an IP literal", address)
	}
	if privateIPOrLocal(ip) {
		return fmt.Errorf("image client must not dial private/loopback/link-local address %s", address)
	}
	return nil
}

func decodeBase64Image(b64 string, maxImageBytes int) ([]byte, error) {
	// base64 inflates by ~4/3; reject before decoding to bound memory.
	if len(b64)*3/4 > maxImageBytes {
		return nil, fmt.Errorf("image exceeds max bytes (%d)", maxImageBytes)
	}
	out, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("image base64 decode: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("image base64 payload empty")
	}
	return out, nil
}

func truncateForLog(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max]) + "…"
}
