package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"omnicraft/backend/internal/observability"
)

// RerankResult is one reranked document reference: the caller-supplied
// document index with its provider relevance score.
type RerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

// Reranker is the rerank seam consumed by the hybrid retrieval pipeline.
// Implementations must be safe for concurrent use.
type Reranker interface {
	Rerank(ctx context.Context, query string, documents []string, topN int) ([]RerankResult, error)
}

const (
	// DefaultDashScopeRerankBase is the DashScope (Bailian) native API host.
	DefaultDashScopeRerankBase = "https://dashscope.aliyuncs.com"
	// DefaultSiliconFlowBase is the SiliconFlow OpenAI-compatible host.
	DefaultSiliconFlowBase = "https://api.siliconflow.cn"
)

// rerankHTTP is the shared plumbing for the two wire formats: bearer auth,
// structured external-call observation and non-200 rejection.
func rerankHTTP(ctx context.Context, client *http.Client, url, apiKey string, payload any, out any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := retryDo(ctx, client, url, apiKey, b, 2)
	if err != nil {
		return err
	}
	started := time.Now()
	defer func() { observability.ObserveExternalCall("llm", started, nil) }()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rerank api error %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// DashScopeReranker calls the DashScope native text-rerank service
// (qwen3-rerank; gte-rerank was retired 2026-05-30 and must not be used).
type DashScopeReranker struct {
	apiKey string
	base   string
	model  string
	client *http.Client
}

func NewDashScopeReranker(apiKey, base, model string, timeout time.Duration) *DashScopeReranker {
	if strings.TrimSpace(base) == "" {
		base = DefaultDashScopeRerankBase
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &DashScopeReranker{
		apiKey: apiKey,
		base:   strings.TrimRight(base, "/"),
		model:  model,
		client: &http.Client{Timeout: timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
	}
}

type dashscopeRerankRequest struct {
	Model      string `json:"model"`
	Input      struct {
		Query     string   `json:"query"`
		Documents []string `json:"documents"`
	} `json:"input"`
	Parameters struct {
		ReturnDocuments bool `json:"return_documents"`
		TopN            int  `json:"top_n,omitempty"`
	} `json:"parameters"`
}

type dashscopeRerankResponse struct {
	Output struct {
		Results []RerankResult `json:"results"`
	} `json:"output"`
}

func (r *DashScopeReranker) Rerank(ctx context.Context, query string, documents []string, topN int) ([]RerankResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}
	var payload dashscopeRerankRequest
	payload.Model = r.model
	payload.Input.Query = query
	payload.Input.Documents = documents
	payload.Parameters.ReturnDocuments = false
	payload.Parameters.TopN = topN
	var out dashscopeRerankResponse
	if err := rerankHTTP(ctx, r.client, r.base+"/api/v1/services/rerank/text-rerank/text-rerank", r.apiKey, payload, &out); err != nil {
		return nil, err
	}
	return normalizeRerankResults(out.Output.Results, len(documents)), nil
}

// SiliconFlowReranker calls the SiliconFlow /v1/rerank endpoint
// (Cohere/Jina-compatible request shape; BAAI/bge-reranker-v2-m3).
type SiliconFlowReranker struct {
	apiKey string
	base   string
	model  string
	client *http.Client
}

func NewSiliconFlowReranker(apiKey, base, model string, timeout time.Duration) *SiliconFlowReranker {
	if strings.TrimSpace(base) == "" {
		base = DefaultSiliconFlowBase
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &SiliconFlowReranker{
		apiKey: apiKey,
		base:   strings.TrimRight(base, "/"),
		model:  model,
		client: &http.Client{Timeout: timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
	}
}

type siliconflowRerankRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            int      `json:"top_n,omitempty"`
	ReturnDocuments bool     `json:"return_documents"`
}

type siliconflowRerankResponse struct {
	Results []RerankResult `json:"results"`
}

func (r *SiliconFlowReranker) Rerank(ctx context.Context, query string, documents []string, topN int) ([]RerankResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}
	payload := siliconflowRerankRequest{
		Model: r.model, Query: query, Documents: documents,
		TopN: topN, ReturnDocuments: false,
	}
	var out siliconflowRerankResponse
	if err := rerankHTTP(ctx, r.client, r.base+"/v1/rerank", r.apiKey, payload, &out); err != nil {
		return nil, err
	}
	return normalizeRerankResults(out.Results, len(documents)), nil
}

// normalizeRerankResults drops out-of-range indices (defensive against
// provider quirks) and sorts by descending relevance so consumers can rely
// on order without re-sorting.
func normalizeRerankResults(results []RerankResult, documentCount int) []RerankResult {
	kept := make([]RerankResult, 0, len(results))
	for _, result := range results {
		if result.Index < 0 || result.Index >= documentCount {
			continue
		}
		kept = append(kept, result)
	}
	// stable sort by score descending; ties keep provider order
	for i := 1; i < len(kept); i++ {
		for j := i; j > 0 && kept[j].RelevanceScore > kept[j-1].RelevanceScore; j-- {
			kept[j], kept[j-1] = kept[j-1], kept[j]
		}
	}
	return kept
}

// FallbackReranker chains a primary reranker with a fallback: any primary
// failure (network, 5xx, malformed response) retries once on the fallback;
// if both fail the error surfaces so the caller can keep its pre-rerank
// order and mark the retrieval degraded.
type FallbackReranker struct {
	primary  Reranker
	fallback Reranker
}

func NewFallbackReranker(primary, fallback Reranker) *FallbackReranker {
	return &FallbackReranker{primary: primary, fallback: fallback}
}

func (r *FallbackReranker) Rerank(ctx context.Context, query string, documents []string, topN int) ([]RerankResult, error) {
	var primaryErr error
	if r.primary != nil {
		results, err := r.primary.Rerank(ctx, query, documents, topN)
		if err == nil {
			return results, nil
		}
		primaryErr = err
	}
	if r.fallback != nil {
		return r.fallback.Rerank(ctx, query, documents, topN)
	}
	return nil, primaryErr
}
