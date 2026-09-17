"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { FlaskConical, Trash2, RefreshCw } from "lucide-react";

interface RetrievalHeadline {
  retrieval_evaluated: number;
  context_recall_at_10?: number;
  hit_rate_at_5?: number;
  hit_rate_at_10?: number;
  mrr?: number;
  over_refusal_proxy_rate?: number;
  mean_latency_ms?: number;
}

interface EvalRunRow {
  id: number;
  created_at: string;
  run_key: string;
  dataset_checksum: string;
  retriever_version: string;
  metrics: { retrieval_headline?: RetrievalHeadline } | null;
  environment: {
    switches?: Record<string, boolean>;
    axes?: Record<string, unknown>;
    split?: string;
    label?: string;
  } | null;
}

interface GoldenDraft {
  case_key: string;
  created_at: string;
  query: string;
  query_language: string;
  source_trace_id: string;
}

// Higher-is-better headline metrics diffed against the baseline run; the
// over-refusal proxy is the one lower-is-better cell. Latency is reference
// only (not gated).
const TREND_METRICS = [
  { key: "context_recall_at_10", higherIsBetter: true },
  { key: "hit_rate_at_5", higherIsBetter: true },
  { key: "hit_rate_at_10", higherIsBetter: true },
  { key: "mrr", higherIsBetter: true },
  { key: "over_refusal_proxy_rate", higherIsBetter: false },
] as const;

function kindOf(runKey: string): string {
  if (runKey.startsWith("grid-")) return "grid";
  if (runKey.startsWith("a04-")) return "a04";
  if (runKey.startsWith("rag-baseline-")) return "baseline";
  return "other";
}

function axesSummary(run: EvalRunRow): string {
  const axes = run.environment?.axes;
  if (!axes) return "—";
  const parts: string[] = [];
  if (typeof axes.rrf_k === "number") parts.push(`rrf=${axes.rrf_k}`);
  if (typeof axes.bm25_topk === "number" && typeof axes.vector_topk === "number") {
    parts.push(`cand=${axes.bm25_topk}/${axes.vector_topk}`);
  }
  if (typeof axes.final_topk === "number") parts.push(`final=${axes.final_topk}`);
  if (run.environment?.switches?.rerank) parts.push("rerank=on");
  return parts.join(" ") || "—";
}

function cellClass(delta: number | null): string {
  if (delta === null) return "";
  if (delta < -0.005) return "text-destructive font-medium";
  if (delta > 0.005) return "text-emerald-600 dark:text-emerald-400";
  return "";
}

export default function AdminEvalsPage() {
  const t = useTranslations();
  const [runs, setRuns] = useState<EvalRunRow[]>([]);
  const [drafts, setDrafts] = useState<GoldenDraft[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busyKey, setBusyKey] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [runsRes, draftsRes] = await Promise.all([
        api.get<{ runs: EvalRunRow[] }>("/api/v1/admin/evals/runs"),
        api.get<{ drafts: GoldenDraft[] }>("/api/v1/admin/evals/drafts"),
      ]);
      setRuns(runsRes.runs ?? []);
      setDrafts(draftsRes.drafts ?? []);
    } catch (e) {
      silentError(e, { component: "AdminEvalsPage", action: "load" });
      setError(t(getUserFacingErrorKey(e, "admin.evals.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  const deleteDraft = useCallback(
    async (caseKey: string) => {
      setBusyKey(caseKey);
      try {
        await api.delete(`/api/v1/admin/evals/drafts/${encodeURIComponent(caseKey)}`);
        setDrafts((prev) => prev.filter((d) => d.case_key !== caseKey));
      } catch (e) {
        silentError(e, { component: "AdminEvalsPage", action: "deleteDraft" });
        setError(t(getUserFacingErrorKey(e, "admin.evals.deleteFailed")));
      } finally {
        setBusyKey("");
      }
    },
    [t],
  );

  // Baseline = the oldest run carrying a retrieval headline (g0-baseline for
  // grid batches); every later headline run diffs against it cell by cell.
  const headlineRuns = runs.filter((r) => r.metrics?.retrieval_headline);
  const baseline = headlineRuns.length > 0 ? headlineRuns[headlineRuns.length - 1] : null;

  return (
    <div className="space-y-4 p-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold">{t("admin.evals.title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("admin.evals.subtitle")}</p>
        </div>
        <button
          type="button"
          onClick={() => void load()}
          className="inline-flex h-8 items-center gap-1.5 rounded-md border border-border px-3 text-sm hover:bg-canvas-subtle"
        >
          <RefreshCw className="h-3.5 w-3.5" />
          {t("admin.evals.refresh")}
        </button>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {loading && runs.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-8 text-center">
          <FlaskConical className="mx-auto h-8 w-8 animate-pulse text-muted-foreground" />
          <p className="mt-2 text-sm text-muted-foreground">{t("admin.evals.loading")}</p>
        </div>
      ) : (
        <>
          <div className="overflow-x-auto rounded-md border border-border bg-card">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs text-muted-foreground">
                  <th className="px-3 py-2 font-medium">{t("admin.evals.colRun")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.evals.colKind")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.evals.colAxes")}</th>
                  {TREND_METRICS.map((m) => (
                    <th key={m.key} className="px-3 py-2 font-medium">
                      {t(`admin.evals.metric_${m.key}`)}
                    </th>
                  ))}
                  <th className="px-3 py-2 font-medium">{t("admin.evals.colEvaluated")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.evals.colTime")}</th>
                </tr>
              </thead>
              <tbody>
                {runs.map((run) => {
                  const h = run.metrics?.retrieval_headline;
                  const bh = baseline?.metrics?.retrieval_headline;
                  return (
                    <tr
                      key={run.run_key}
                      className="border-b border-border last:border-0 hover:bg-muted/50"
                    >
                      <td className="whitespace-nowrap px-3 py-2 font-mono text-xs">{run.run_key}</td>
                      <td className="whitespace-nowrap px-3 py-2">
                        <span className="rounded-full bg-muted px-2 py-0.5 text-xs">{kindOf(run.run_key)}</span>
                      </td>
                      <td className="whitespace-nowrap px-3 py-2 font-mono text-xs text-muted-foreground">
                        {axesSummary(run)}
                      </td>
                      {TREND_METRICS.map((m) => {
                        const v = h?.[m.key];
                        const bv = bh?.[m.key];
                        if (v === undefined || v === null) {
                          return (
                            <td key={m.key} className="whitespace-nowrap px-3 py-2 text-xs text-muted-foreground">
                              —
                            </td>
                          );
                        }
                        const isBaseline = baseline?.run_key === run.run_key;
                        const delta = isBaseline || bv === undefined || bv === null ? null : (v - bv) * (m.higherIsBetter ? 1 : -1);
                        return (
                          <td
                            key={m.key}
                            className={`whitespace-nowrap px-3 py-2 tabular-nums ${cellClass(delta)}`}
                            title={delta === null ? undefined : `${delta >= 0 ? "+" : ""}${delta.toFixed(4)} vs ${baseline?.run_key}`}
                          >
                            {v.toFixed(4)}
                          </td>
                        );
                      })}
                      <td className="whitespace-nowrap px-3 py-2 tabular-nums text-muted-foreground">
                        {h ? h.retrieval_evaluated : "—"}
                      </td>
                      <td className="whitespace-nowrap px-3 py-2 text-xs text-muted-foreground">
                        {new Date(run.created_at).toLocaleString()}
                      </td>
                    </tr>
                  );
                })}
                {runs.length === 0 && (
                  <tr>
                    <td colSpan={9} className="px-3 py-8 text-center text-sm text-muted-foreground">
                      {t("admin.evals.noRuns")}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          <div className="rounded-md border border-border bg-card">
            <div className="border-b border-border px-4 py-3">
              <h2 className="text-sm font-semibold">{t("admin.evals.draftsTitle")}</h2>
              <p className="mt-0.5 text-xs text-muted-foreground">{t("admin.evals.draftsHint")}</p>
            </div>
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs text-muted-foreground">
                  <th className="px-3 py-2 font-medium">{t("admin.evals.colCaseKey")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.evals.colQuery")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.evals.colTrace")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.evals.colTime")}</th>
                  <th className="px-3 py-2 font-medium" aria-label={t("admin.evals.colActions")} />
                </tr>
              </thead>
              <tbody>
                {drafts.map((d) => (
                  <tr key={d.case_key} className="border-b border-border last:border-0 hover:bg-muted/50">
                    <td className="max-w-56 truncate px-3 py-2 font-mono text-xs">{d.case_key}</td>
                    <td className="max-w-96 truncate px-3 py-2">{d.query}</td>
                    <td className="px-3 py-2">
                      {d.source_trace_id ? (
                        <a
                          href={`/admin/traces/${d.source_trace_id}`}
                          className="font-mono text-xs text-primary underline-offset-2 hover:underline"
                        >
                          {d.source_trace_id.slice(0, 12)}…
                        </a>
                      ) : (
                        "—"
                      )}
                    </td>
                    <td className="whitespace-nowrap px-3 py-2 text-xs text-muted-foreground">
                      {new Date(d.created_at).toLocaleString()}
                    </td>
                    <td className="px-3 py-2 text-right">
                      <button
                        type="button"
                        disabled={busyKey === d.case_key}
                        onClick={() => void deleteDraft(d.case_key)}
                        aria-label={t("admin.evals.deleteDraft")}
                        className="inline-flex size-8 items-center justify-center rounded-md text-muted-foreground hover:bg-destructive/10 hover:text-destructive disabled:opacity-50"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </td>
                  </tr>
                ))}
                {drafts.length === 0 && (
                  <tr>
                    <td colSpan={5} className="px-3 py-8 text-center text-sm text-muted-foreground">
                      {t("admin.evals.noDrafts")}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}
