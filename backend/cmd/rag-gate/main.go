// Command rag-gate is the PR-level zero-cost evaluation gate (SP-22 E3, #562).
//
// It reads a frozen snapshot of golden labels plus recorded retrieval/answer
// outcomes (backend/testdata/rag_pr_gate_snapshot.jsonl) and asserts the
// derived retrieval-layer ID metrics and refusal confusion matrix against
// evals/thresholds.yaml. It makes no provider calls, needs no database and
// finishes in milliseconds, so it is safe on every pull request.
//
// Exit codes: 0 = all assertions hold, 1 = at least one violation,
// 2 = usage or artifact error.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	rageval "omnicraft/backend/internal/service/rag_eval"
)

func main() {
	snapshotPath := flag.String("snapshot", "testdata/rag_pr_gate_snapshot.jsonl", "frozen gate snapshot")
	thresholdsPath := flag.String("thresholds", "../evals/thresholds.yaml", "threshold contract")
	asJSON := flag.Bool("json", false, "emit metrics and violations as JSON")
	verbose := flag.Bool("verbose", true, "print the metric table")
	flag.Parse()

	snap, err := rageval.LoadPRGateSnapshot(*snapshotPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rag-gate: %v\n", err)
		os.Exit(2)
	}
	thresholds, err := rageval.LoadPRGateThresholds(*thresholdsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rag-gate: %v\n", err)
		os.Exit(2)
	}

	metrics := rageval.ComputePRGateMetrics(snap)
	violations := rageval.EvaluatePRGate(metrics, thresholds)

	if *asJSON {
		out := map[string]any{
			"snapshot": map[string]any{
				"label":            snap.SourceLabel,
				"dataset_checksum": snap.DatasetChecksum,
				"cases":            len(snap.Cases),
			},
			"metrics":    metrics,
			"violations": violations,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "rag-gate: encode json: %v\n", err)
			os.Exit(2)
		}
	} else if *verbose {
		fmt.Printf("PR eval gate — snapshot %q (%d cases, %s)\n\n",
			snap.SourceLabel, len(snap.Cases), shortChecksum(snap.DatasetChecksum))
		fmt.Println("retrieval layer (ID-based)")
		fmt.Printf("  context recall @10 : %.4f  (evaluated %d, skipped %d)\n",
			metrics.ContextRecallAt10, metrics.RetrievalEvaluated, metrics.RetrievalNotEvaluated)
		fmt.Printf("  hit rate @5 / @10  : %.4f / %.4f\n", metrics.HitRateAt5, metrics.HitRateAt10)
		fmt.Printf("  mrr                : %.4f\n", metrics.MRR)
		fmt.Println("refusal confusion matrix")
		fmt.Printf("  answerable   %3d  answered %3d  refused %3d\n",
			metrics.Answerable, metrics.AnsweredAnswerable, metrics.RefusedAnswerable)
		fmt.Printf("  unanswerable %3d  answered %3d  refused %3d\n",
			metrics.Unanswerable, metrics.AnsweredUnanswerable, metrics.RefusedUnanswerable)
		fmt.Printf("  correct answer %.4f  over-refusal %.4f\n",
			metrics.CorrectAnswerRate, metrics.OverRefusalRate)
		fmt.Printf("  hallucination  %.4f  correct refusal %.4f\n",
			metrics.HallucinationRate, metrics.CorrectRefusalRate)
	}

	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "\nFAIL: %d threshold violation(s)\n", len(violations))
		for _, v := range violations {
			fmt.Fprintf(os.Stderr, "  - %s [%s]\n", v.String(), v.Kind)
		}
		os.Exit(1)
	}
	fmt.Println("\nPASS: all PR eval gate thresholds hold")
}

func shortChecksum(sum string) string {
	if len(sum) > 24 {
		return sum[:24] + "…"
	}
	return sum
}
