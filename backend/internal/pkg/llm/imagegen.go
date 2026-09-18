package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
}

// NewCogViewClient builds the client; apiBase must not carry a trailing
// slash (the path is appended verbatim).
func NewCogViewClient(apiBase, apiKey, model string) *CogViewClient {
	return &CogViewClient{
		APIBase: strings.TrimRight(strings.TrimSpace(apiBase), "/"),
		APIKey:  apiKey,
		Model:   model,
		Client:  &http.Client{Timeout: 90 * time.Second},
	}
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

func (c *CogViewClient) download(ctx context.Context, url string, maxImageBytes int) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
