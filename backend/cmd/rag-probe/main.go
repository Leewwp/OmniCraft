package main

// rag-probe is the golden-set v2 authoring rig (#291 step 4): it drives the
// production HybridRetriever directly for a list of probe queries and dumps
// the ranked candidates as JSONL evidence. Retrieval-config variants are
// selected by launching the probe with CONFIG_OVERRIDE_PATH pointing at an
// override yaml that flips features.rag_{hybrid,query_expansion,rerank}_enabled
// (the A-04 ablation switches); the emitted header records the effective
// switch state so mined candidate unions stay attributable.
//
// It is an authoring/evidence tool only: nothing here writes to the DB, and
// it never runs as part of unit CI.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/container"
	"omnicraft/backend/internal/pkg/database"
	"omnicraft/backend/internal/pkg/llm"
	redisclient "omnicraft/backend/internal/pkg/redis"
)

type probeInput struct {
	Queries []string `json:"queries"`
	TopK    int      `json:"top_k"`
	Viewer  int64    `json:"viewer_id"`
}

type candidateRow struct {
	Rank          int    `json:"rank"`
	ContentID     int64  `json:"content_id"`
	ChunkKey      string `json:"chunk_key"`
	Title         string `json:"title"`
	Heading       string `json:"heading"`
	SourceStart   int    `json:"source_start"`
	SourceEnd     int    `json:"source_end"`
	ContentVersio int    `json:"content_version"`
}

type probeOutput struct {
	Query      string         `json:"query"`
	Degraded   string         `json:"degraded,omitempty"`
	Expanded   []string       `json:"expanded_queries,omitempty"`
	Candidates []candidateRow `json:"candidates"`
	Error      string         `json:"error,omitempty"`
}

func ctx0() context.Context { return context.Background() }

func main() {
	in := flag.String("in", "", "input json: {queries:[...], top_k, viewer_id}")
	out := flag.String("out", "", "output jsonl path")
	label := flag.String("label", "", "config label recorded in the header line")
	flag.Parse()
	if *in == "" || *out == "" {
		slog.Error("usage: rag-probe -in probes.json -out probes.out.jsonl -label cfg")
		os.Exit(2)
	}

	raw, err := os.ReadFile(*in)
	if err != nil {
		slog.Error("read input", "error", err)
		os.Exit(1)
	}
	var input probeInput
	if err := json.Unmarshal(raw, &input); err != nil {
		slog.Error("parse input", "error", err)
		os.Exit(1)
	}
	if input.TopK <= 0 {
		input.TopK = 20
	}

	cfg := config.Load()
	db := database.Init(cfg)
	rdb := redisclient.Init(cfg)
	defer rdb.Close()

	// startup self-check: the query embedder must be live, otherwise the
	// hybrid silently degrades to keyword-only and the mined evidence lies.
	embModel := cfg.RAG.Index.EmbeddingModel
	probeVec, embErr := llm.NewProvider(cfg).GetEmbedding(ctx0(), "self check")
	if embErr != nil {
		slog.Error("EMBEDDER SELF-CHECK FAILED", "model", embModel,
			"api_base", cfg.Agent.EmbeddingAPIBase, "error", embErr)
		os.Exit(3)
	}
	slog.Info("embedder self-check ok", "model", embModel, "dims", len(probeVec))

	ctr := container.NewContainer(db, rdb, cfg)

	fh, err := os.Create(*out)
	if err != nil {
		slog.Error("create output", "error", err)
		os.Exit(1)
	}
	defer fh.Close()
	enc := json.NewEncoder(fh)

	header, _ := json.Marshal(map[string]any{
		"kind": "rag-probe", "label": *label,
		"switches": map[string]bool{
			"hybrid":          cfg.Features.RAGHybridEnabled,
			"query_expansion": cfg.Features.RAGQueryExpansionEnabled,
			"rerank":          cfg.Features.RAGRerankEnabled,
		},
		// runtime identity echo: evidence files must be self-describing
		// (runtime-matrix.md conformance is asserted by gsv2_evidence.py;
		// API keys never leave the process).
		"runtime": map[string]any{
			"chat": map[string]string{
				"provider": cfg.Agent.LLMProvider, "model": cfg.Agent.LLMModel,
				"api_base": cfg.Agent.LLMAPIBase,
			},
			"embedding": map[string]string{
				"provider": cfg.Agent.EmbeddingProvider,
				"model":    cfg.RAG.Index.EmbeddingModel,
				"api_base": cfg.Agent.EmbeddingAPIBase,
			},
			"rerank": map[string]string{
				"provider": cfg.RAG.Rerank.Provider, "model": cfg.RAG.Rerank.Model,
				"api_base": cfg.RAG.Rerank.APIBase,
			},
			"keyword_source": cfg.RAG.Hybrid.KeywordSource,
		},
		"embedding_dims": len(probeVec),
		"top_k":          input.TopK, "viewer_id": input.Viewer,
	})
	fmt.Fprintln(fh, string(header))

	ctx := context.Background()
	for _, q := range input.Queries {
		row := probeOutput{Query: q}
		res, err := ctr.HybridRetriever.Retrieve(ctx, q, input.Viewer)
		if err != nil {
			row.Error = err.Error()
		} else {
			row.Degraded = res.Degraded
			row.Expanded = res.ExpandedQueries
			limit := res.Candidates
			if len(limit) > input.TopK {
				limit = limit[:input.TopK]
			}
			for i, c := range limit {
				row.Candidates = append(row.Candidates, candidateRow{
					Rank: i + 1, ContentID: c.ContentID, ChunkKey: c.ChunkKey,
					Title: c.Title, Heading: c.Heading,
					SourceStart: c.SourceStart, SourceEnd: c.SourceEnd,
					ContentVersio: c.ContentVersion,
				})
			}
		}
		if err := enc.Encode(row); err != nil {
			slog.Error("encode row", "error", err)
			os.Exit(1)
		}
	}
	slog.Info("rag-probe done", "queries", len(input.Queries), "out", *out)
}
