"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Activity, ArrowLeft, Zap, Timer, GitBranch, FlaskConical } from "lucide-react";
import { cn } from "@/lib/utils";

interface TraceNode {
  id: number;
  trace_id: string;
  node_key: string;
  parent_node_key: string | null;
  depth: number;
  node_type: string;
  node_name: string;
  status: string;
  error_code: string;
  error_message: string;
  started_at: string;
  ended_at: string | null;
  duration_ms: number | null;
  model: string;
  prompt_digest: string;
  completion_digest: string;
  tokens_in: number | null;
  tokens_out: number | null;
  cost_estimate: number | null;
  extra: Record<string, unknown> | string | null;
}

interface TraceRun {
  id: number;
  trace_id: string;
  conversation_id: number | null;
  message_id: number | null;
  user_id: number | null;
  surface: string;
  status: string;
  error_code: string;
  started_at: string;
  ended_at: string | null;
  duration_ms: number | null;
  ttft_ms: number | null;
  model: string;
  answer_kind: string;
  routing_events: Array<{ from?: string; to?: string; reason?: string; err?: string }> | string | null;
  prompt_name: string;
  prompt_version: number | null;
}

interface WaterfallRow {
  node: TraceNode;
  offsetMs: number;
  leftPct: number;
  barWidthPct: number;
}

function formatMs(ms: number | null | undefined): string {
  if (ms == null) return "—";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

export default function AdminTraceDetailPage() {
  const t = useTranslations();
  const params = useParams<{ traceId: string }>();
  const traceId = params?.traceId ?? "";
  const [run, setRun] = useState<TraceRun | null>(null);
  const [nodes, setNodes] = useState<TraceNode[]>([]);
  const [selected, setSelected] = useState<TraceNode | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  // SP-22 E5 badcase feedback: turn this traced turn into a golden-set
  // draft (question/answer/citations) for curator review on /admin/evals.
  const [converting, setConverting] = useState(false);
  const [convertDone, setConvertDone] = useState("");
  const [convertError, setConvertError] = useState("");

  const createGoldenDraft = async () => {
    setConverting(true);
    setConvertDone("");
    setConvertError("");
    try {
      const res = await api.post<{ case_key: string; created: boolean }>(
        "/api/v1/admin/evals/drafts",
        { trace_id: traceId },
      );
      setConvertDone(res.created ? t("admin.evals.draftCreated") : t("admin.evals.draftExists"));
    } catch (e) {
      silentError(e, { component: "AdminTraceDetailPage", action: "createGoldenDraft" });
      setConvertError(t(getUserFacingErrorKey(e, "admin.evals.convertFailed")));
    } finally {
      setConverting(false);
    }
  };

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api
      .get<{ run: TraceRun; nodes: TraceNode[] }>(`/api/v1/admin/traces/${traceId}`)
      .then((data) => {
        if (cancelled) return;
        setRun(data.run);
        setNodes(data.nodes || []);
      })
      .catch((e) => {
        silentError(e, { component: "AdminTraceDetailPage", action: "load" });
        if (!cancelled) setError(t(getUserFacingErrorKey(e, "admin.traces.loadFailed")));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [traceId, t]);

  // DFS tree ordering by parent_node_key, then waterfall geometry relative
  // to the run start (polyu RagTraceDetailPage blueprint).
  const rows = useMemo<WaterfallRow[]>(() => {
    if (!run || nodes.length === 0) return [];
    const runStart = new Date(run.started_at).getTime();
    const byParent = new Map<string, TraceNode[]>();
    const roots: TraceNode[] = [];
    for (const n of nodes) {
      if (n.parent_node_key) {
        const list = byParent.get(n.parent_node_key) ?? [];
        list.push(n);
        byParent.set(n.parent_node_key, list);
      } else {
        roots.push(n);
      }
    }
    const ordered: TraceNode[] = [];
    const visit = (n: TraceNode) => {
      ordered.push(n);
      for (const child of byParent.get(n.node_key) ?? []) visit(child);
    };
    roots.sort((a, b) => new Date(a.started_at).getTime() - new Date(b.started_at).getTime());
    for (const root of roots) visit(root);

    const spanEnd = Math.max(
      new Date(run.ended_at ?? run.started_at).getTime(),
      ...nodes.map((n) => new Date(n.ended_at ?? n.started_at).getTime()),
    );
    const totalSpan = Math.max(1, spanEnd - runStart);
    return ordered.map((node) => {
      const offset = Math.max(0, new Date(node.started_at).getTime() - runStart);
      const dur = node.duration_ms ?? Math.max(0, new Date(node.ended_at ?? node.started_at).getTime() - new Date(node.started_at).getTime());
      return {
        node,
        offsetMs: offset,
        leftPct: Math.min(99, (offset / totalSpan) * 100),
        barWidthPct: Math.min(100, Math.max(0.5, (dur / totalSpan) * 100)),
      };
    });
  }, [run, nodes]);

  const slowest = useMemo(() => {
    if (rows.length === 0) return null;
    return rows.reduce((a, b) => ((b.node.duration_ms ?? 0) > (a.node.duration_ms ?? 0) ? b : a));
  }, [rows]);

  const routingEvents = useMemo(() => {
    if (!run?.routing_events) return [];
    if (Array.isArray(run.routing_events)) return run.routing_events;
    try {
      const parsed = JSON.parse(run.routing_events);
      return Array.isArray(parsed) ? parsed : [];
    } catch {
      return [];
    }
  }, [run]);

  if (loading) {
    return (
      <div className="rounded-md border border-border bg-card p-8 text-center">
        <Activity className="mx-auto h-8 w-8 animate-pulse text-muted-foreground" />
        <p className="mt-2 text-sm text-muted-foreground">{t("admin.traces.loading")}</p>
      </div>
    );
  }

  if (error || !run) {
    return (
      <div className="space-y-3">
        <Link href="/admin/traces" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
          <ArrowLeft className="h-4 w-4" />
          {t("admin.tracesDetail.backToList")}
        </Link>
        <div className="rounded-md border border-border bg-card p-8 text-center">
          <p className="text-sm text-muted-foreground">{error || t("admin.traces.noTraces")}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div>
        <Link href="/admin/traces" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
          <ArrowLeft className="h-4 w-4" />
          {t("admin.tracesDetail.backToList")}
        </Link>
        <div className="mt-2 flex flex-wrap items-center gap-3">
          <h1 className="break-all font-mono text-lg font-bold">{run.trace_id}</h1>
          <button
            type="button"
            onClick={() => void createGoldenDraft()}
            disabled={converting}
            className="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-md border border-border px-3 text-xs hover:bg-canvas-subtle disabled:opacity-50"
          >
            <FlaskConical className="h-3.5 w-3.5" />
            {converting ? t("admin.evals.converting") : t("admin.evals.toGoldenDraft")}
          </button>
          {convertDone && (
            <span className="text-xs text-emerald-600 dark:text-emerald-400">
              {convertDone}{" "}
              <Link href="/admin/evals" className="underline underline-offset-2">
                {t("admin.evals.viewDrafts")}
              </Link>
            </span>
          )}
          {convertError && <span className="text-xs text-destructive">{convertError}</span>}
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-4">
        <div className="rounded-md border border-border bg-card p-3">
          <p className="text-xs text-muted-foreground">{t("admin.tracesDetail.runStatus")}</p>
          <p className="mt-1 flex items-center gap-2 text-sm font-semibold">
            <span className={cn(
              "inline-flex rounded-full px-1.5 py-0.5 text-[10px] font-medium",
              run.status === "SUCCESS" && "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400",
              run.status === "ERROR" && "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
              run.status === "CANCELLED" && "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
              run.status === "RUNNING" && "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
            )}>{run.status}</span>
            <span>{run.answer_kind || "—"}</span>
          </p>
        </div>
        <div className="rounded-md border border-border bg-card p-3">
          <p className="text-xs text-muted-foreground">{t("admin.traces.colDuration")} / {t("admin.traces.colTtft")}</p>
          <p className="mt-1 flex items-center gap-3 text-sm font-semibold">
            <span className="flex items-center gap-1"><Timer className="h-3.5 w-3.5" />{formatMs(run.duration_ms)}</span>
            <span className="flex items-center gap-1"><Zap className="h-3.5 w-3.5" />{formatMs(run.ttft_ms)}</span>
          </p>
        </div>
        <div className="rounded-md border border-border bg-card p-3">
          <p className="text-xs text-muted-foreground">{t("admin.traces.colModel")}</p>
          <p className="mt-1 text-sm font-semibold">{run.model || "—"}</p>
          <p className="text-xs text-muted-foreground">
            {run.prompt_name}{run.prompt_version != null ? ` v${run.prompt_version}` : ""}
          </p>
        </div>
        <div className="rounded-md border border-border bg-card p-3">
          <p className="text-xs text-muted-foreground">{t("admin.tracesDetail.links")}</p>
          <p className="mt-1 space-y-0.5 text-sm">
            <span className="block">{t("admin.traces.colUser")}: {run.user_id ?? "—"}</span>
            <span className="block">{t("admin.traces.colConversation")}: {run.conversation_id ?? "—"}</span>
          </p>
        </div>
      </div>

      {routingEvents.length > 0 && (
        <div className="rounded-md border border-border bg-card p-3">
          <p className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
            <GitBranch className="h-3.5 w-3.5" />
            {t("admin.tracesDetail.routingEvents")}
          </p>
          <ul className="mt-1.5 space-y-1">
            {routingEvents.map((ev, i) => (
              <li key={i} className="text-xs">
                <span className="font-mono">{ev.from}</span> → <span className="font-mono">{ev.to}</span>
                <span className="ml-2 rounded-full bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-400">{ev.reason}</span>
                {ev.err && <span className="ml-2 text-muted-foreground">{ev.err}</span>}
              </li>
            ))}
          </ul>
        </div>
      )}

      {rows.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-8 text-center">
          <p className="text-sm text-muted-foreground">{t("admin.tracesDetail.noNodes")}</p>
        </div>
      ) : (
        <div className="grid gap-4 lg:grid-cols-[1fr_360px]">
          <div className="overflow-x-auto rounded-md border border-border bg-card">
            <div className="min-w-[520px] divide-y divide-border">
              {rows.map(({ node, leftPct, barWidthPct }) => {
                const isSlowest = slowest?.node.id === node.id;
                return (
                  <button
                    key={node.id}
                    type="button"
                    onClick={() => setSelected(node)}
                    className={cn(
                      "flex w-full items-center gap-3 px-3 py-1.5 text-left transition-colors hover:bg-muted/50",
                      selected?.id === node.id && "bg-muted",
                    )}
                    style={{ paddingLeft: `${12 + node.depth * 16}px` }}
                  >
                    <span className="w-44 shrink-0 truncate text-xs" title={`${node.node_type} / ${node.node_key}`}>
                      <span className={cn("font-medium", isSlowest && "text-amber-600 dark:text-amber-400")}>
                        {node.node_name || node.node_key}
                      </span>
                      <span className="ml-1 text-muted-foreground">{node.node_type}</span>
                    </span>
                    <span className="relative h-3.5 flex-1 overflow-hidden rounded-sm bg-muted/60">
                      <span
                        className={cn(
                          "absolute top-0 h-full rounded-sm",
                          node.status === "SUCCESS" && "bg-indigo-500/80",
                          node.status === "ERROR" && "bg-red-500/80",
                          node.status === "CANCELLED" && "bg-amber-500/80",
                          node.status === "RUNNING" && "bg-blue-500/80",
                          isSlowest && "ring-1 ring-amber-500",
                        )}
                        style={{ left: `${leftPct}%`, width: `${barWidthPct}%` }}
                      />
                    </span>
                    <span className={cn("w-16 shrink-0 text-right text-xs tabular-nums", isSlowest && "font-semibold text-amber-600 dark:text-amber-400")}>
                      {formatMs(node.duration_ms)}
                    </span>
                  </button>
                );
              })}
            </div>
          </div>

          <div className="rounded-md border border-border bg-card p-4">
            {selected ? (
              <div className="space-y-2 text-sm">
                <p className="break-all font-mono text-xs text-muted-foreground">{selected.node_key}</p>
                <dl className="space-y-1.5 text-xs">
                  <div className="flex gap-2"><dt className="w-24 shrink-0 text-muted-foreground">type</dt><dd className="font-medium">{selected.node_type}</dd></div>
                  {selected.model && <div className="flex gap-2"><dt className="w-24 shrink-0 text-muted-foreground">model</dt><dd className="font-mono">{selected.model}</dd></div>}
                  <div className="flex gap-2"><dt className="w-24 shrink-0 text-muted-foreground">status</dt><dd>{selected.status}{selected.error_code ? ` · ${selected.error_code}` : ""}</dd></div>
                  {(selected.tokens_in != null || selected.tokens_out != null) && (
                    <div className="flex gap-2"><dt className="w-24 shrink-0 text-muted-foreground">tokens</dt><dd>↑{selected.tokens_in ?? "—"} / ↓{selected.tokens_out ?? "—"}</dd></div>
                  )}
                  {selected.cost_estimate != null && (
                    <div className="flex gap-2"><dt className="w-24 shrink-0 text-muted-foreground">cost</dt><dd>¥{(selected.cost_estimate * 1000).toFixed(4)}‰</dd></div>
                  )}
                  {selected.prompt_digest && (
                    <div><dt className="text-muted-foreground">prompt_digest</dt><dd className="mt-0.5 whitespace-pre-wrap break-all rounded border border-border bg-background p-1.5">{selected.prompt_digest}</dd></div>
                  )}
                  {selected.completion_digest && (
                    <div><dt className="text-muted-foreground">completion_digest</dt><dd className="mt-0.5 whitespace-pre-wrap break-all rounded border border-border bg-background p-1.5">{selected.completion_digest}</dd></div>
                  )}
                  {selected.error_message && (
                    <div><dt className="text-muted-foreground">error</dt><dd className="mt-0.5 whitespace-pre-wrap break-all rounded border border-red-300 bg-red-50 p-1.5 text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-400">{selected.error_message}</dd></div>
                  )}
                  {selected.extra && (
                    <div><dt className="text-muted-foreground">extra</dt><dd className="mt-0.5 overflow-x-auto rounded border border-border bg-background p-1.5"><pre className="text-[10px]">{typeof selected.extra === "string" ? selected.extra : JSON.stringify(selected.extra, null, 2)}</pre></dd></div>
                  )}
                </dl>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">{t("admin.tracesDetail.selectNodeHint")}</p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
