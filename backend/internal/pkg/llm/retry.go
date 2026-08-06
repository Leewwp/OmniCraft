package llm

import (
	"bytes"
	"context"
	"net/http"
	"time"
)

// retryableStatus reports whether an HTTP status qualifies for bounded retry:
// 429 and retryable 5xx (500/502/503/504). Non-retryable 4xx never retries.
func retryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// retryDo performs an HTTP POST with bounded retries for retryable network,
// 429 and 5xx conditions. Retries happen inside one request, so they never
// reserve a second quota unit. The caller's context cancels retries too.
func retryDo(ctx context.Context, client *http.Client, url, apiKey string, body []byte, maxRetries int) (*http.Response, error) {
	attempt := 0
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(req)
		if err != nil {
			if attempt < maxRetries && !isContextDone(ctx) {
				attempt++
				backoff(ctx, attempt)
				continue
			}
			return nil, err
		}
		if retryableStatus(resp.StatusCode) && attempt < maxRetries && !isContextDone(ctx) {
			resp.Body.Close()
			attempt++
			backoff(ctx, attempt)
			continue
		}
		return resp, nil
	}
}

func isContextDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// backoff sleeps a bounded, increasing delay between retry attempts.
func backoff(ctx context.Context, attempt int) {
	delay := time.Duration(150*attempt) * time.Millisecond
	if delay > 2*time.Second {
		delay = 2 * time.Second
	}
	select {
	case <-ctx.Done():
	case <-time.After(delay):
	}
}
