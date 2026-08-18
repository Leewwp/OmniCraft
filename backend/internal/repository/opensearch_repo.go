package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrOpenSearchUnavailable = errors.New("opensearch unavailable")
var ErrOpenSearchAliasNotFound = errors.New("opensearch alias not found")

type SearchDocument struct {
	ID              string   `json:"-"`
	ChunkKey        string   `json:"chunk_key"`
	ContentID       int64    `json:"content_id"`
	ContentVersion  int      `json:"content_version"`
	ChunkingVersion int      `json:"chunking_version"`
	IndexVersion    int      `json:"index_version"`
	EmbeddingModel  string   `json:"embedding_model"`
	Title           string   `json:"title"`
	Heading         string   `json:"heading"`
	Text            string   `json:"text"`
	SourceStart     int      `json:"source_start"`
	SourceEnd       int      `json:"source_end"`
	Zone            string   `json:"zone"`
	ContentType     string   `json:"content_type"`
	Category        *string  `json:"category"`
	IP              *int64   `json:"ip"`
	Tags            []string `json:"tags"`
	Status          string   `json:"status"`
}

type OpenSearchRepository struct {
	baseURL              string
	client               *http.Client
	errorBodyMaxBytes    int64
	responseBodyMaxBytes int64
	healthPollInterval   time.Duration
}

type OpenSearchResponseLimits struct {
	ErrorBodyMaxBytes     int64
	ResponseBodyMaxBytes  int64
	HealthPollIntervalSec int
}

type openSearchResponseError struct {
	status int
	kind   string
}

type deleteByQueryResponse struct {
	TimedOut         bool              `json:"timed_out"`
	VersionConflicts int64             `json:"version_conflicts"`
	Failures         []json.RawMessage `json:"failures"`
}

func (*openSearchResponseError) Error() string { return ErrOpenSearchUnavailable.Error() }

func NewOpenSearchRepository(baseURL string, client *http.Client) *OpenSearchRepository {
	return NewOpenSearchRepositoryWithLimits(baseURL, client, OpenSearchResponseLimits{})
}

func NewOpenSearchRepositoryWithLimits(baseURL string, client *http.Client, limits OpenSearchResponseLimits) *OpenSearchRepository {
	if client == nil {
		client = http.DefaultClient
	}
	return &OpenSearchRepository{
		baseURL:              strings.TrimRight(baseURL, "/"),
		client:               client,
		errorBodyMaxBytes:    limits.ErrorBodyMaxBytes,
		responseBodyMaxBytes: limits.ResponseBodyMaxBytes,
		healthPollInterval:   time.Duration(limits.HealthPollIntervalSec) * time.Second,
	}
}

func (r *OpenSearchRepository) CreateIndex(ctx context.Context, index string) error {
	properties := map[string]any{}
	for field, fieldType := range fixedSearchMappingTypes() {
		properties[field] = map[string]any{"type": fieldType}
	}
	err := r.doJSON(ctx, http.MethodPut, "/"+index, map[string]any{
		"mappings": map[string]any{"dynamic": "strict", "properties": properties},
	}, nil)
	var responseErr *openSearchResponseError
	if errors.As(err, &responseErr) && responseErr.status == http.StatusBadRequest && responseErr.kind == "resource_already_exists_exception" {
		return r.validateMapping(ctx, index)
	}
	return err
}

func (r *OpenSearchRepository) ReplaceContent(ctx context.Context, index string, contentID int64, documents []SearchDocument) error {
	if len(documents) == 0 {
		return r.DeleteContent(ctx, index, contentID)
	}
	if err := r.UpsertContent(ctx, index, documents); err != nil {
		return err
	}
	return r.PruneStaleContent(ctx, index, contentID, documentIDs(documents))
}

func (r *OpenSearchRepository) UpsertContent(ctx context.Context, index string, documents []SearchDocument) error {
	if len(documents) == 0 {
		return nil
	}
	var payload bytes.Buffer
	encoder := json.NewEncoder(&payload)
	for _, document := range documents {
		if strings.TrimSpace(document.ID) == "" {
			return ErrOpenSearchUnavailable
		}
		if err := encoder.Encode(map[string]any{"index": map[string]any{"_index": index, "_id": document.ID}}); err != nil {
			return ErrOpenSearchUnavailable
		}
		if err := encoder.Encode(document); err != nil {
			return ErrOpenSearchUnavailable
		}
	}
	var response struct {
		Errors bool `json:"errors"`
	}
	if err := r.doRequest(ctx, http.MethodPost, "/_bulk?refresh=wait_for", "application/x-ndjson", &payload, &response); err != nil {
		return err
	}
	if response.Errors {
		return ErrOpenSearchUnavailable
	}
	return nil
}

// pruneStaleContent runs only after every new stable document id has been
// acknowledged. A prune failure may temporarily leave old and new chunks
// together, but it never removes the last searchable projection.
func (r *OpenSearchRepository) PruneStaleContent(ctx context.Context, index string, contentID int64, keepIDs []string) error {
	if len(keepIDs) == 0 {
		return r.DeleteContent(ctx, index, contentID)
	}
	var response deleteByQueryResponse
	err := r.doJSON(ctx, http.MethodPost, "/"+index+"/_delete_by_query?refresh=true&conflicts=proceed", map[string]any{
		"query": map[string]any{"bool": map[string]any{
			"filter":   []any{map[string]any{"term": map[string]any{"content_id": contentID}}},
			"must_not": []any{map[string]any{"ids": map[string]any{"values": keepIDs}}},
		}},
	}, &response)
	if isOpenSearchNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return validateDeleteByQuery(response)
}

func documentIDs(documents []SearchDocument) []string {
	ids := make([]string, len(documents))
	for i := range documents {
		ids[i] = documents[i].ID
	}
	return ids
}

func (r *OpenSearchRepository) DeleteContent(ctx context.Context, index string, contentID int64) error {
	var response deleteByQueryResponse
	err := r.doJSON(ctx, http.MethodPost, "/"+index+"/_delete_by_query?refresh=true&conflicts=proceed", map[string]any{
		"query": map[string]any{"term": map[string]any{"content_id": contentID}},
	}, &response)
	if isOpenSearchNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return validateDeleteByQuery(response)
}

func validateDeleteByQuery(response deleteByQueryResponse) error {
	if response.TimedOut || response.VersionConflicts > 0 || len(response.Failures) > 0 {
		return ErrOpenSearchUnavailable
	}
	return nil
}

func (r *OpenSearchRepository) Health(ctx context.Context) error {
	if err := r.healthOnce(ctx); err == nil {
		return nil
	}
	if r.healthPollInterval <= 0 {
		return ErrOpenSearchUnavailable
	}
	pollCtx := ctx
	cancel := func() {}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && r.client.Timeout > 0 {
		pollCtx, cancel = context.WithTimeout(ctx, r.client.Timeout)
	}
	defer cancel()
	if _, hasDeadline := pollCtx.Deadline(); !hasDeadline {
		return ErrOpenSearchUnavailable
	}
	ticker := time.NewTicker(r.healthPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-pollCtx.Done():
			return ErrOpenSearchUnavailable
		case <-ticker.C:
			if err := r.healthOnce(pollCtx); err == nil {
				return nil
			}
		}
	}
}

func (r *OpenSearchRepository) healthOnce(ctx context.Context) error {
	var health struct {
		Status   string `json:"status"`
		TimedOut bool   `json:"timed_out"`
	}
	if err := r.doJSON(ctx, http.MethodGet, "/_cluster/health", nil, &health); err != nil {
		return ErrOpenSearchUnavailable
	}
	if health.TimedOut || (health.Status != "green" && health.Status != "yellow") {
		return ErrOpenSearchUnavailable
	}
	return nil
}

func (r *OpenSearchRepository) ValidateIndex(ctx context.Context, index string, expectedDocuments int64) error {
	if err := r.doJSON(ctx, http.MethodPost, "/"+index+"/_refresh", nil, nil); err != nil {
		return err
	}
	var count struct {
		Count int64 `json:"count"`
	}
	if err := r.doJSON(ctx, http.MethodGet, "/"+index+"/_count", nil, &count); err != nil {
		return err
	}
	if count.Count != expectedDocuments {
		return ErrOpenSearchUnavailable
	}
	return r.validateMapping(ctx, index)
}

func (r *OpenSearchRepository) validateMapping(ctx context.Context, index string) error {
	var mappings map[string]struct {
		Mappings struct {
			Dynamic    string `json:"dynamic"`
			Properties map[string]struct {
				Type string `json:"type"`
			} `json:"properties"`
		} `json:"mappings"`
	}
	if err := r.doJSON(ctx, http.MethodGet, "/"+index+"/_mapping", nil, &mappings); err != nil {
		return err
	}
	mapping, ok := mappings[index]
	expected := fixedSearchMappingTypes()
	if !ok || mapping.Mappings.Dynamic != "strict" || len(mapping.Mappings.Properties) != len(expected) {
		return ErrOpenSearchUnavailable
	}
	for field, fieldType := range expected {
		if mapping.Mappings.Properties[field].Type != fieldType {
			return ErrOpenSearchUnavailable
		}
	}
	return nil
}

func fixedSearchMappingTypes() map[string]string {
	return map[string]string{
		"chunk_key": "keyword", "content_id": "long", "content_version": "integer",
		"chunking_version": "integer", "index_version": "integer", "embedding_model": "keyword",
		"title": "text", "heading": "text", "text": "text", "source_start": "integer",
		"source_end": "integer", "zone": "keyword", "content_type": "keyword",
		"category": "keyword", "ip": "long", "tags": "keyword", "status": "keyword",
	}
}

func (r *OpenSearchRepository) SwapAlias(ctx context.Context, alias string, index string) error {
	return r.doJSON(ctx, http.MethodPost, "/_aliases", map[string]any{
		"actions": []any{
			map[string]any{"remove": map[string]any{"index": "*", "alias": alias, "must_exist": false}},
			map[string]any{"add": map[string]any{"index": index, "alias": alias}},
		},
	}, nil)
}

func (r *OpenSearchRepository) RemoveAlias(ctx context.Context, alias string) error {
	return r.doJSON(ctx, http.MethodPost, "/_aliases", map[string]any{
		"actions": []any{map[string]any{
			"remove": map[string]any{"index": "*", "alias": alias, "must_exist": false},
		}},
	}, nil)
}

func (r *OpenSearchRepository) AliasTarget(ctx context.Context, alias string) (string, error) {
	var aliases map[string]json.RawMessage
	if err := r.doJSON(ctx, http.MethodGet, "/_alias/"+alias, nil, &aliases); err != nil {
		var responseErr *openSearchResponseError
		if errors.As(err, &responseErr) && responseErr.status == http.StatusNotFound {
			return "", ErrOpenSearchAliasNotFound
		}
		return "", err
	}
	if len(aliases) != 1 {
		return "", ErrOpenSearchUnavailable
	}
	for index := range aliases {
		return index, nil
	}
	return "", nil
}

func (r *OpenSearchRepository) ListIndexes(ctx context.Context, prefix string) ([]string, error) {
	var response struct {
		Indices []struct {
			Name string `json:"name"`
		} `json:"indices"`
	}
	if err := r.doJSON(ctx, http.MethodGet, "/_resolve/index/"+prefix+"*", nil, &response); err != nil {
		if isOpenSearchNotFound(err) {
			return []string{}, nil
		}
		return nil, err
	}
	indexes := make([]string, 0, len(response.Indices))
	for _, index := range response.Indices {
		if strings.HasPrefix(index.Name, prefix) {
			indexes = append(indexes, index.Name)
		}
	}
	return indexes, nil
}

func (r *OpenSearchRepository) DeleteIndex(ctx context.Context, index string) error {
	err := r.doJSON(ctx, http.MethodDelete, "/"+index, nil, nil)
	if isOpenSearchNotFound(err) {
		return nil
	}
	return err
}

func isOpenSearchNotFound(err error) bool {
	var responseErr *openSearchResponseError
	return errors.As(err, &responseErr) && responseErr.status == http.StatusNotFound && responseErr.kind == "index_not_found_exception"
}

func (r *OpenSearchRepository) doJSON(ctx context.Context, method, path string, input any, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode opensearch request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	return r.doRequest(ctx, method, path, "application/json", body, output)
}

func (r *OpenSearchRepository) doRequest(ctx context.Context, method, path, contentType string, body io.Reader, output any) (resultErr error) {
	req, err := http.NewRequestWithContext(ctx, method, r.baseURL+path, body)
	if err != nil {
		return ErrOpenSearchUnavailable
	}
	if body != nil {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return ErrOpenSearchUnavailable
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && resultErr == nil {
			resultErr = ErrOpenSearchUnavailable
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorBody := io.Reader(resp.Body)
		if r.errorBodyMaxBytes > 0 {
			errorBody = io.LimitReader(resp.Body, r.errorBodyMaxBytes)
		}
		body, err := io.ReadAll(errorBody)
		if err != nil {
			return ErrOpenSearchUnavailable
		}
		var failure struct {
			Error struct {
				Type string `json:"type"`
			} `json:"error"`
		}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &failure); err != nil {
				return &openSearchResponseError{status: resp.StatusCode}
			}
		}
		return &openSearchResponseError{status: resp.StatusCode, kind: failure.Error.Type}
	}
	if output != nil {
		responseBody := io.Reader(resp.Body)
		if r.responseBodyMaxBytes > 0 {
			responseBody = io.LimitReader(resp.Body, r.responseBodyMaxBytes)
		}
		if err := json.NewDecoder(responseBody).Decode(output); err != nil {
			return ErrOpenSearchUnavailable
		}
	}
	return nil
}
