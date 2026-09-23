package main

// rag-contextual is the SP-24 R4 pilot driver (#575): contextual retrieval
// with a DeepSeek-written situation prefix per chunk.
//
//	rag-contextual -estimate   chunk the whole published corpus offline and
//	                           print the token/cost/ETA estimate (no LLM call)
//	rag-contextual -rebuild    run the production projection rebuild through
//	                           the real container wiring (annotation included)
//
// Cost discipline (#575): the estimate runs first and the rebuild refuses to
// start unless rag.contextual.enabled is on and an annotation key resolved.
// Configuration comes from config.yaml plus CONFIG_OVERRIDE_PATH (rag-eval
// convention): the pilot override turns contextual on and bumps
// rag.chunking.version, producing a new projection generation (the A/B pair).
//
// The A/B comparison itself is measured with cmd/rag-eval -skip-generation
// before and after the rebuild; this tool only moves the index.

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/container"
	"omnicraft/backend/internal/pkg/database"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	ragservice "omnicraft/backend/internal/service/rag"
)

// Estimate-only pricing constants (yuan per million tokens). Deliberately
// conservative V3-era anchors: deepseek-chat (which the endpoint now aliases
// to the cheaper flash tier) lists well below these, and prompt caching
// discounts the repeated per-document prefix further. The estimate therefore
// bounds the worst case; the invoice follows the provider, not this table.
const (
	estimateInputYuanPerMTok  = 2.0 // deepseek-chat input (cache miss)
	estimateOutputYuanPerMTok = 8.0 // deepseek-chat output
	estimateEmbedYuanPerMTok  = 0.5 // dashscope text-embedding-v4
	estimateCallLatencySec    = 1.5 // assumed annotation round trip
)

func main() {
	estimate := flag.Bool("estimate", false, "print the corpus chunk/token/cost estimate and exit (no provider calls)")
	rebuild := flag.Bool("rebuild", false, "run the projection rebuild through the container wiring (annotation included)")
	flag.Parse()
	if *estimate == *rebuild {
		slog.Error("usage: rag-contextual -estimate | rag-contextual -rebuild")
		os.Exit(2)
	}

	cfg := config.Load()
	db := database.Init(cfg)
	defer func() { _, _ = db.DB() }()

	if *estimate {
		runEstimate(cfg, db)
		return
	}
	runRebuild(cfg, db)
}

type corpusStats struct {
	contents     int
	chunks       int
	docTokens    int64 // Σ per-chunk document-context tokens (the prompt's doc field)
	chunkTokens  int64 // Σ chunk text tokens (the prompt's chunk field)
	embedTokens  int64 // Σ (chunk + prefix) tokens the rebuild would embed
	emptyContent int
}

// runEstimate chunks the published corpus offline with the production
// chunker and prints what a contextual rebuild would cost. No provider is
// called: the annotation tokens are budget-derived (max_prefix_tokens out,
// doc-context + chunk + chrome in), same as the annotator would spend.
func runEstimate(cfg *config.Config, db *gorm.DB) {
	if !cfg.RAG.Contextual.Enabled {
		slog.Warn("rag.contextual.enabled is off: this estimate assumes the pilot override (contextual on) is loaded via CONFIG_OVERRIDE_PATH")
	}
	chunker := ragservice.NewChunker(ragservice.ChunkerConfig{
		MaxTokens: cfg.RAG.Chunking.MaxTokens, OverlapTokens: cfg.RAG.Chunking.OverlapTokens,
		ChunkingVersion: cfg.RAG.Chunking.ChunkingVersion, TokenizerEncoding: cfg.RAG.Chunking.TokenizerEncoding,
	})
	versions := service.NewVersionService(repository.NewVersionRepository(db), repository.NewContentRepository(db))
	ctx := context.Background()

	type contentRow struct {
		ID    int64
		Title string
	}
	var contents []contentRow
	if err := db.Table("content_items").Select("id, title").
		Where("status = ? AND deleted_at IS NULL", "published").
		Order("id ASC").Find(&contents).Error; err != nil {
		slog.Error("list published contents", "error", err)
		os.Exit(1)
	}
	stats := corpusStats{}
	started := time.Now()
	for _, row := range contents {
		_, text, err := versions.LoadLatestPublishedContent(ctx, row.ID)
		if err != nil {
			stats.emptyContent++
			continue
		}
		stats.contents++
		docContext := ragservice.BoundDocContext(text, cfg.RAG.Contextual.DocContextChars)
		docTok := int64(chunker.CountTokens(row.Title)) + int64(chunker.CountTokens(docContext))
		chunks, err := chunker.Chunk(ragservice.SourceDocument{ContentID: row.ID, Title: row.Title, Text: text})
		if err != nil {
			slog.Error("chunk content", "content_id", row.ID, "error", err)
			os.Exit(1)
		}
		for _, chunk := range chunks {
			stats.chunks++
			chunkTok := int64(chunker.CountTokens(chunk.Text))
			stats.docTokens += docTok
			stats.chunkTokens += chunkTok
			stats.embedTokens += chunkTok + int64(cfg.RAG.Contextual.MaxPrefixTokens)
		}
	}

	inputTok := stats.docTokens + stats.chunkTokens + int64(stats.chunks)*ragservice.ContextualPromptOverhead
	outputTok := int64(stats.chunks) * int64(cfg.RAG.Contextual.MaxPrefixTokens)
	annotateCost := float64(inputTok)/1e6*estimateInputYuanPerMTok + float64(outputTok)/1e6*estimateOutputYuanPerMTok
	embedCost := float64(stats.embedTokens) / 1e6 * estimateEmbedYuanPerMTok
	concurrency := cfg.RAG.Contextual.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	perCall := time.Duration(cfg.RAG.Contextual.RequestIntervalMS) * time.Millisecond
	throughputPerSec := float64(concurrency) / estimateCallLatencySec
	if perCall > 0 {
		if paced := 1.0 / (perCall.Seconds()); paced < throughputPerSec {
			throughputPerSec = paced
		}
	}
	eta := time.Duration(float64(stats.chunks)/throughputPerSec) * time.Second

	fmt.Println("=== SP-24 R4 contextual rebuild estimate (cl100k proxy, list prices) ===")
	fmt.Printf("published contents : %d (unversioned skipped: %d)\n", stats.contents, stats.emptyContent)
	fmt.Printf("chunks (v%d)       : %d\n", cfg.RAG.Chunking.ChunkingVersion, stats.chunks)
	fmt.Printf("annotation model   : %s/%s, concurrency %d, interval %dms\n",
		cfg.RAG.Contextual.Provider, cfg.RAG.Contextual.Model, concurrency, cfg.RAG.Contextual.RequestIntervalMS)
	fmt.Printf("annotation input   : %d tok (doc+title %d + chunk %d + chrome %d)\n",
		inputTok, stats.docTokens, stats.chunkTokens, int64(stats.chunks)*ragservice.ContextualPromptOverhead)
	fmt.Printf("annotation output  : %d tok (budget %d/chunk)\n", outputTok, cfg.RAG.Contextual.MaxPrefixTokens)
	fmt.Printf("annotation cost    : ~¥%.2f (¥%.1f/M in + ¥%.1f/M out, cache miss worst case)\n",
		annotateCost, estimateInputYuanPerMTok, estimateOutputYuanPerMTok)
	fmt.Printf("re-embedding       : %d tok ~¥%.2f (¥%.1f/M)\n", stats.embedTokens, embedCost, estimateEmbedYuanPerMTok)
	fmt.Printf("total estimate     : ~¥%.2f\n", annotateCost+embedCost)
	fmt.Printf("annotation ETA     : %s (~%.1f calls/s)\n", eta.Round(time.Second), throughputPerSec)
	fmt.Printf("offline pass took  : %s\n", time.Since(started).Round(time.Millisecond))
	fmt.Println("estimate only; actual billing follows provider pricing and cache hits.")
}

// runRebuild executes the production projection rebuild (admin endpoint
// path) through the real container so annotation, embedding and generation
// promotion all run exactly as they would behind POST /admin/rag/rebuild.
func runRebuild(cfg *config.Config, db *gorm.DB) {
	if !cfg.RAG.Contextual.Enabled {
		slog.Error("refusing to rebuild: rag.contextual.enabled is off (load the pilot override via CONFIG_OVERRIDE_PATH; a plain rebuild belongs to the admin endpoint)")
		os.Exit(2)
	}
	if strings.TrimSpace(cfg.RAG.Contextual.APIKey) == "" {
		slog.Error("refusing to rebuild: no annotation API key resolved (RAG_CONTEXTUAL_API_KEY / AGENT_MODEL_DEEPSEEK_API_KEY)")
		os.Exit(2)
	}
	before, err := currentGeneration(db)
	if err != nil {
		slog.Error("read current generation", "error", err)
		os.Exit(1)
	}

	rdb := redisclient.Init(cfg)
	defer rdb.Close()
	ctr, err := container.NewContainer(db, rdb, cfg)
	if err != nil {
		slog.Error("composition root wiring incomplete", "error", err)
		os.Exit(1)
	}

	started := time.Now()
	slog.Info("contextual rebuild starting",
		"from_generation", before,
		"chunking_version", cfg.RAG.Chunking.ChunkingVersion,
		"annotation", fmt.Sprintf("%s/%s", cfg.RAG.Contextual.Provider, cfg.RAG.Contextual.Model))
	if err := ctr.RAGProjection.Rebuild(context.Background()); err != nil {
		slog.Error("contextual rebuild failed", "error", err, "elapsed", time.Since(started).Round(time.Second))
		os.Exit(1)
	}
	after, err := currentGeneration(db)
	if err != nil {
		slog.Error("read new generation", "error", err)
		os.Exit(1)
	}
	var chunks int64
	if err := db.Table("rag_chunks").Where("index_version = ?", after).Count(&chunks).Error; err != nil {
		slog.Error("count new generation chunks", "error", err)
	}
	slog.Info("contextual rebuild complete",
		"generation", fmt.Sprintf("%d -> %d", before, after),
		"chunks", chunks,
		"elapsed", time.Since(started).Round(time.Second))
}

func currentGeneration(db *gorm.DB) (int, error) {
	var version int
	if err := db.Table("index_projection_status").
		Select("COALESCE(MAX(index_version), 0)").Where("is_current = ?", true).Scan(&version).Error; err != nil {
		return 0, err
	}
	return version, nil
}
