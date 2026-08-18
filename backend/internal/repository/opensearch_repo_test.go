package repository

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenSearchRepositoryCreatesFixedStrictMapping(t *testing.T) {
	var mapping map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/omnicraft-rag-v7", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&mapping))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"acknowledged":true}`))
	}))
	t.Cleanup(server.Close)

	repo := NewOpenSearchRepository(server.URL, server.Client())
	require.NoError(t, repo.CreateIndex(context.Background(), "omnicraft-rag-v7"))

	mappings := mapping["mappings"].(map[string]any)
	require.Equal(t, "strict", mappings["dynamic"])
	properties := mappings["properties"].(map[string]any)
	for field, expectedType := range map[string]string{
		"chunk_key": "keyword", "content_id": "long", "content_version": "integer",
		"chunking_version": "integer", "index_version": "integer", "embedding_model": "keyword",
		"title": "text", "heading": "text", "text": "text", "source_start": "integer",
		"source_end": "integer", "zone": "keyword", "content_type": "keyword",
		"category": "keyword", "ip": "long", "tags": "keyword", "status": "keyword",
	} {
		require.Equal(t, expectedType, properties[field].(map[string]any)["type"], field)
	}
}

func TestOpenSearchRepositoryCreateIndexTreatsExistingGenerationAsReplay(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"type":"resource_already_exists_exception"},"status":400}`)
			return
		}
		_, _ = io.WriteString(w, validMappingResponse("omnicraft-rag-v2"))
	}))
	t.Cleanup(server.Close)
	repo := NewOpenSearchRepository(server.URL, server.Client())
	require.NoError(t, repo.CreateIndex(context.Background(), "omnicraft-rag-v2"))
}

func TestOpenSearchRepositoryRejectsExistingMappingDriftDuringCreate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"type":"resource_already_exists_exception"},"status":400}`)
			return
		}
		_, _ = io.WriteString(w, `{"omnicraft-rag-v2":{"mappings":{"dynamic":"true","properties":{}}}}`)
	}))
	t.Cleanup(server.Close)
	repo := NewOpenSearchRepository(server.URL, server.Client())
	require.ErrorIs(t, repo.CreateIndex(context.Background(), "omnicraft-rag-v2"), ErrOpenSearchUnavailable)
}

func TestOpenSearchRepositoryDeleteMissingProjectionIsIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"type":"index_not_found_exception"},"status":404}`)
	}))
	t.Cleanup(server.Close)
	repo := NewOpenSearchRepository(server.URL, server.Client())
	require.NoError(t, repo.DeleteContent(context.Background(), "omnicraft-rag-v1", 42))
	require.NoError(t, repo.DeleteIndex(context.Background(), "omnicraft-rag-v1"))
}

func TestOpenSearchRepositoryRejectsPartialDeleteByQuerySuccess(t *testing.T) {
	for _, body := range []string{
		`{"timed_out":true,"failures":[]}`,
		`{"timed_out":false,"failures":[{"cause":{"type":"unavailable_shards_exception"}}]}`,
		`{"timed_out":false,"version_conflicts":1,"failures":[]}`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, body)
		}))
		repo := NewOpenSearchRepository(server.URL, server.Client())
		require.ErrorIs(t, repo.DeleteContent(context.Background(), "omnicraft-rag-v1", 42), ErrOpenSearchUnavailable)
		server.Close()
	}
}

func TestOpenSearchRepositoryReplacesContentWithStableBulkDocumentIDs(t *testing.T) {
	var deletedContentID int64
	var bulkLines []map[string]any
	var requestOrder []string
	var deleteQuery map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/omnicraft-rag-v3/_delete_by_query":
			requestOrder = append(requestOrder, "prune")
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&deleteQuery))
			filter := deleteQuery["query"].(map[string]any)["bool"].(map[string]any)["filter"].([]any)
			deletedContentID = int64(filter[0].(map[string]any)["term"].(map[string]any)["content_id"].(float64))
			_, _ = w.Write([]byte(`{"deleted":2}`))
		case "/_bulk":
			requestOrder = append(requestOrder, "bulk")
			require.Equal(t, "wait_for", r.URL.Query().Get("refresh"))
			scanner := bufio.NewScanner(r.Body)
			for scanner.Scan() {
				var line map[string]any
				require.NoError(t, json.Unmarshal(scanner.Bytes(), &line))
				bulkLines = append(bulkLines, line)
			}
			require.NoError(t, scanner.Err())
			_, _ = io.WriteString(w, `{"errors":false,"items":[]}`)
		default:
			t.Fatalf("unexpected OpenSearch request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	repo := NewOpenSearchRepository(server.URL, server.Client())
	documents := []SearchDocument{
		{ID: "chunk-a", ChunkKey: "chunk-a", ContentID: 42, ContentVersion: 2, Status: "published"},
		{ID: "chunk-b", ChunkKey: "chunk-b", ContentID: 42, ContentVersion: 2, Status: "published"},
	}
	require.NoError(t, repo.ReplaceContent(context.Background(), "omnicraft-rag-v3", 42, documents))
	require.Equal(t, []string{"bulk", "prune"}, requestOrder, "new stable ids must be upserted before stale ids are pruned")
	require.Equal(t, int64(42), deletedContentID)
	require.Contains(t, string(mustJSON(t, deleteQuery)), `"chunk-a"`)
	require.Contains(t, string(mustJSON(t, deleteQuery)), `"chunk-b"`)
	require.Len(t, bulkLines, 4)
	firstAction := bulkLines[0]["index"].(map[string]any)
	require.Equal(t, "omnicraft-rag-v3", firstAction["_index"])
	require.Equal(t, "chunk-a", firstAction["_id"])
	require.Equal(t, "chunk-a", bulkLines[1]["chunk_key"])
	secondAction := bulkLines[2]["index"].(map[string]any)
	require.Equal(t, "chunk-b", secondAction["_id"])
}

func TestOpenSearchRepositorySkipsEmptyBulkUpsert(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	t.Cleanup(server.Close)
	repo := NewOpenSearchRepository(server.URL, server.Client())
	require.NoError(t, repo.UpsertContent(context.Background(), "omnicraft-rag-v1", nil))
	require.Zero(t, requests, "OpenSearch rejects empty bulk payloads")
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return encoded
}

func TestOpenSearchRepositoryHealthAcceptsSingleNodeYellow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/_cluster/health", r.URL.Path)
		_, _ = io.WriteString(w, `{"status":"yellow","timed_out":false}`)
	}))
	t.Cleanup(server.Close)

	repo := NewOpenSearchRepository(server.URL, server.Client())
	require.NoError(t, repo.Health(context.Background()))
}

func TestOpenSearchRepositoryHealthPollsUntilClusterIsUsable(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/_cluster/health", r.URL.Path)
		attempts++
		if attempts == 1 {
			_, _ = io.WriteString(w, `{"status":"red","timed_out":false}`)
			return
		}
		_, _ = io.WriteString(w, `{"status":"yellow","timed_out":false}`)
	}))
	t.Cleanup(server.Close)

	repo := NewOpenSearchRepositoryWithLimits(server.URL, server.Client(), OpenSearchResponseLimits{HealthPollIntervalSec: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, repo.Health(ctx))
	require.Equal(t, 2, attempts)
}

func TestOpenSearchRepositoryHealthRejectsUnknownStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"blue","timed_out":false}`)
	}))
	t.Cleanup(server.Close)
	repo := NewOpenSearchRepository(server.URL, server.Client())
	require.ErrorIs(t, repo.Health(context.Background()), ErrOpenSearchUnavailable)
}

func TestOpenSearchRepositoryValidateIndexRejectsUnexpectedDocumentCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/_refresh"):
			_, _ = io.WriteString(w, `{}`)
		case strings.HasSuffix(r.URL.Path, "/_count"):
			_, _ = io.WriteString(w, `{"count":2}`)
		case strings.HasSuffix(r.URL.Path, "/_mapping"):
			_, _ = io.WriteString(w, validMappingResponse("omnicraft-rag-v2"))
		default:
			t.Fatalf("unexpected request %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	repo := NewOpenSearchRepository(server.URL, server.Client())
	require.ErrorIs(t, repo.ValidateIndex(context.Background(), "omnicraft-rag-v2", 1), ErrOpenSearchUnavailable)
}

func TestOpenSearchRepositoryValidateIndexRejectsMappingDrift(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/_refresh"):
			_, _ = io.WriteString(w, `{}`)
		case strings.HasSuffix(r.URL.Path, "/_count"):
			_, _ = io.WriteString(w, `{"count":1}`)
		case strings.HasSuffix(r.URL.Path, "/_mapping"):
			_, _ = io.WriteString(w, `{"omnicraft-rag-v2":{"mappings":{"dynamic":"true","properties":{}}}}`)
		default:
			t.Fatalf("unexpected request %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	repo := NewOpenSearchRepository(server.URL, server.Client())
	require.ErrorIs(t, repo.ValidateIndex(context.Background(), "omnicraft-rag-v2", 1), ErrOpenSearchUnavailable)
}

func TestOpenSearchRepositoryAtomicallySwapsAndResolvesReadAlias(t *testing.T) {
	var actions []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_aliases":
			require.Equal(t, http.MethodPost, r.Method)
			var request struct {
				Actions []map[string]any `json:"actions"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			actions = request.Actions
			_, _ = io.WriteString(w, `{"acknowledged":true}`)
		case "/_alias/omnicraft-rag-read":
			require.Equal(t, http.MethodGet, r.Method)
			_, _ = io.WriteString(w, `{"omnicraft-rag-v8":{"aliases":{"omnicraft-rag-read":{}}}}`)
		default:
			t.Fatalf("unexpected OpenSearch request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	repo := NewOpenSearchRepository(server.URL, server.Client())
	require.NoError(t, repo.SwapAlias(context.Background(), "omnicraft-rag-read", "omnicraft-rag-v8"))
	require.Len(t, actions, 2)
	require.Equal(t, map[string]any{"index": "*", "alias": "omnicraft-rag-read", "must_exist": false}, actions[0]["remove"])
	require.Equal(t, map[string]any{"index": "omnicraft-rag-v8", "alias": "omnicraft-rag-read"}, actions[1]["add"])
	target, err := repo.AliasTarget(context.Background(), "omnicraft-rag-read")
	require.NoError(t, err)
	require.Equal(t, "omnicraft-rag-v8", target)
}

func TestOpenSearchRepositoryListsFixedPrefixIndexes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/_resolve/index/omnicraft-rag-v*", r.URL.Path)
		_, _ = io.WriteString(w, `{"indices":[{"name":"omnicraft-rag-v1"},{"name":"omnicraft-rag-v2"},{"name":"unrelated"}]}`)
	}))
	t.Cleanup(server.Close)
	repo := NewOpenSearchRepository(server.URL, server.Client())
	indexes, err := repo.ListIndexes(context.Background(), "omnicraft-rag-v")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"omnicraft-rag-v1", "omnicraft-rag-v2"}, indexes)
}

func TestOpenSearchRepositoryDistinguishesMissingAliasFromTransientFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"type":"alias_missing_exception"},"status":404}`)
	}))
	t.Cleanup(server.Close)
	repo := NewOpenSearchRepository(server.URL, server.Client())
	_, err := repo.AliasTarget(context.Background(), "omnicraft-rag-read")
	require.ErrorIs(t, err, ErrOpenSearchAliasNotFound)
}

func TestOpenSearchRepositoryRealContainerContract(t *testing.T) {
	baseURL := os.Getenv("OMNICRAFT_OPENSEARCH_URL")
	if baseURL == "" {
		t.Skip("set OMNICRAFT_OPENSEARCH_URL for the full-infra OpenSearch contract")
	}
	repo := NewOpenSearchRepository(baseURL, &http.Client{Timeout: 10 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, repo.Health(ctx))
	suffix := time.Now().UTC().Format("20060102150405")
	index := "omnicraft-rag-contract-v" + suffix
	alias := "omnicraft-rag-contract-read-" + suffix
	t.Cleanup(func() { _ = repo.DeleteIndex(context.Background(), index) })
	require.NoError(t, repo.CreateIndex(ctx, index))
	require.NoError(t, repo.ReplaceContent(ctx, index, 42, []SearchDocument{{
		ID: "stable-contract-chunk", ChunkKey: "stable-contract-chunk", ContentID: 42,
		ContentVersion: 1, ChunkingVersion: 1, IndexVersion: 1,
		EmbeddingModel: "contract", Title: "Contract", Text: "real OpenSearch",
		SourceStart: 0, SourceEnd: 15, Zone: "original", ContentType: "guide",
		Tags: []string{"contract"}, Status: "published",
	}}))
	require.NoError(t, repo.ValidateIndex(ctx, index, 1))
	require.NoError(t, repo.SwapAlias(ctx, alias, index))
	target, err := repo.AliasTarget(ctx, alias)
	require.NoError(t, err)
	require.Equal(t, index, target)
	require.NoError(t, repo.DeleteContent(ctx, index, 42))
}

func validMappingResponse(index string) string {
	properties := make(map[string]any)
	for field, fieldType := range fixedSearchMappingTypes() {
		properties[field] = map[string]any{"type": fieldType}
	}
	payload, _ := json.Marshal(map[string]any{index: map[string]any{
		"mappings": map[string]any{"dynamic": "strict", "properties": properties},
	}})
	return string(payload)
}
