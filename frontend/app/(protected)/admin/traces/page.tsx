"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { AdminMetricCard } from "@/components/admin/AdminMetricCard";
import { Activity, CheckCircle2, Timer, Zap, GitBranch, Search } from "lucide-react";
import Link from "next/link";
import { cn } from "@/lib/utils";

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
  duration_ms: number | null;
  ttft_ms: number | null;
  model: string;
  answer_kind: string;
  routing_events: Array<Record<string, unknown>> | string | null;
  prompt_name: string;
  prompt_version: number | null;
}

interface TraceStats {
  total_runs: number;
  terminal_runs: number;
  success_rate: number;
  avg_duration_ms: number;
  p95_duration_ms: number;
  avg_ttft_ms: number;
  routing_rate: number;
}

const STATUS_OPTIONS = ["RUNNING", "SUCCESS", "ERROR", "CANCELLED"];
const ANSWER_KIND_OPTIONS = ["grounded_content", "no_evidence", "conversational", "publish_suggestion"];
const TIME_WINDOWS = [
  { key: "1h", hours: 1 },
  { key: "24h", hours: 24 },
  { key: "7d", hours: 24 * 7 },
  { key: "all", hours: 0 },
] as const;

function formatMs(ms: number | null | undefined): string {
  if (ms == null) return "—";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

function formatRate(rate: number): string {
  return `${(rate * 100).toFixed(1)}%`;
}

export default function AdminTracesPage() {
  const t = useTranslations();
  const [runs, setRuns] = useState<TraceRun[]>([]);
  const [stats, setStats] = useState<TraceStats | null>(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [traceId, setTraceId] = useState("");
  const [conversationId, setConversationId] = useState("");
  const [userId, setUserId] = useState("");
  const [model, setModel] = useState("");
  const [status, setStatus] = useState("");
  const [answerKind, setAnswerKind] = useState("");
  const [windowKey, setWindowKey] = useState<(typeof TIME_WINDOWS)[number]["key"]>("24h");
  const [applied, setApplied] = useState({
    traceId: "",
    conversationId: "",
    userId: "",
    model: "",
    status: "",
    answerKind: "",
    windowKey: "24h" as (typeof TIME_WINDOWS)[number]["key"],
  });

  const pageSize = 20;

  const windowStart = useCallback((key: (typeof TIME_WINDOWS)[number]["key"]): string => {
    const win = TIME_WINDOWS.find((w) => w.key === key);
    if (!win || win.hours === 0) return "";
    const from = new Date(Date.now() - win.hours * 3600 * 1000);
    return from.toISOString();
  }, []);

  const loadStats = useCallback(async () => {
    try {
      const params = new URLSearchParams();
      const from = windowStart(applied.windowKey);
      if (from) params.set("from", from);
      setStats(await api.get<TraceStats>(`/api/v1/admin/traces/stats?${params.toString()}`));
    } catch (e) {
      silentError(e, { component: "AdminTracesPage", action: "loadStats" });
    }
  }, [applied.windowKey, windowStart]);

  const loadRuns = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const params = new URLSearchParams();
      params.set("page", String(page));
      params.set("page_size", String(pageSize));
      if (applied.traceId) params.set("trace_id", applied.traceId);
      if (applied.conversationId) params.set("conversation_id", applied.conversationId);
      if (applied.userId) params.set("user_id", applied.userId);
      if (applied.model) params.set("model", applied.model);
      if (applied.status) params.set("status", applied.status);
      if (applied.answerKind) params.set("answer_kind", applied.answerKind);
      const from = windowStart(applied.windowKey);
      if (from) params.set("from", from);
      const data = await api.get<{ items: TraceRun[]; total: number }>(
        `/api/v1/admin/traces?${params.toString()}`
      );
      setRuns(data.items || []);
      setTotal(data.total || 0);
    } catch (e) {
      silentError(e, { component: "AdminTracesPage", action: "loadRuns" });
      setError(t(getUserFacingErrorKey(e, "admin.traces.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [page, applied, t, windowStart]);

  useEffect(() => {
    void loadRuns();
  }, [loadRuns]);
  useEffect(() => {
    void loadStats();
  }, [loadStats]);

  const applyFilters = () => {
    setApplied({ traceId: traceId.trim(), conversationId: conversationId.trim(), userId: userId.trim(), model: model.trim(), status, answerKind, windowKey });
    setPage(1);
  };
  const resetFilters = () => {
    setTraceId("");
    setConversationId("");
    setUserId("");
    setModel("");
    setStatus("");
    setAnswerKind("");
    setWindowKey("24h");
    setApplied({ traceId: "", conversationId: "", userId: "", model: "", status: "", answerKind: "", windowKey: "24h" });
    setPage(1);
  };

  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  // routing_events arrives as parsed JSON (array) or raw string depending on
  // the transport; only a non-empty array means a real failover happened.
  const hasRouting = (events: TraceRun["routing_events"]): boolean => {
    if (!events) return false;
    if (Array.isArray(events)) return events.length > 0;
    return events !== "[]" && events.trim() !== "";
  };

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-xl font-bold">{t("admin.traces.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("admin.traces.subtitle")}</p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <AdminMetricCard
          label={t("admin.traces.statsSuccessRate")}
          value={stats ? formatRate(stats.success_rate) : "—"}
          icon={CheckCircle2}
          variant={stats && stats.success_rate < 0.9 ? "warning" : "default"}
          loading={loading && !stats}
        />
        <AdminMetricCard
          label={t("admin.traces.statsDuration")}
          value={stats ? `${formatMs(stats.avg_duration_ms)} / ${formatMs(stats.p95_duration_ms)}` : "—"}
          icon={Timer}
          loading={loading && !stats}
        />
        <AdminMetricCard
          label={t("admin.traces.statsTtft")}
          value={stats ? formatMs(stats.avg_ttft_ms) : "—"}
          icon={Zap}
          loading={loading && !stats}
        />
        <AdminMetricCard
          label={t("admin.traces.statsRoutingRate")}
          value={stats ? formatRate(stats.routing_rate) : "—"}
          icon={GitBranch}
          variant={stats && stats.routing_rate > 0.2 ? "warning" : "default"}
          loading={loading && !stats}
        />
      </div>

      <div className="rounded-md border border-border bg-card p-4">
        <div className="flex flex-wrap items-end gap-3">
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted-foreground" htmlFor="trace-id">{t("admin.traces.filterTraceId")}</label>
            <input
              id="trace-id"
              className="h-8 w-64 rounded-md border border-border bg-background px-2 text-sm"
              value={traceId}
              placeholder={t("admin.traces.filterTraceIdPlaceholder")}
              onChange={(e) => setTraceId(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && applyFilters()}
            />
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted-foreground" htmlFor="conversation-id">{t("admin.traces.filterConversation")}</label>
            <input
              id="conversation-id"
              className="h-8 w-28 rounded-md border border-border bg-background px-2 text-sm"
              value={conversationId}
              onChange={(e) => setConversationId(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && applyFilters()}
            />
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted-foreground" htmlFor="user-id">{t("admin.traces.filterUser")}</label>
            <input
              id="user-id"
              className="h-8 w-24 rounded-md border border-border bg-background px-2 text-sm"
              value={userId}
              onChange={(e) => setUserId(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && applyFilters()}
            />
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted-foreground" htmlFor="model">{t("admin.traces.filterModel")}</label>
            <input
              id="model"
              className="h-8 w-36 rounded-md border border-border bg-background px-2 text-sm"
              value={model}
              onChange={(e) => setModel(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && applyFilters()}
            />
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted-foreground" htmlFor="status">{t("admin.traces.filterStatus")}</label>
            <Select
              id="status"
              aria-label={t("admin.traces.filterStatus")}
              className="h-8 px-2 py-0 text-sm"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
            >
              <option value="">{t("admin.traces.allStatuses")}</option>
              {STATUS_OPTIONS.map((s) => <option key={s} value={s}>{s}</option>)}
            </Select>
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted-foreground" htmlFor="answer-kind">{t("admin.traces.filterAnswerKind")}</label>
            <Select
              id="answer-kind"
              aria-label={t("admin.traces.filterAnswerKind")}
              className="h-8 px-2 py-0 text-sm"
              value={answerKind}
              onChange={(e) => setAnswerKind(e.target.value)}
            >
              <option value="">{t("admin.traces.allAnswerKinds")}</option>
              {ANSWER_KIND_OPTIONS.map((k) => <option key={k} value={k}>{k}</option>)}
            </Select>
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted-foreground" htmlFor="time-window">{t("admin.traces.filterWindow")}</label>
            <Select
              id="time-window"
              aria-label={t("admin.traces.filterWindow")}
              className="h-8 px-2 py-0 text-sm"
              value={windowKey}
              onChange={(e) => setWindowKey(e.target.value as (typeof TIME_WINDOWS)[number]["key"])}
            >
              {TIME_WINDOWS.map((w) => (
                <option key={w.key} value={w.key}>{t(`admin.traces.window_${w.key}`)}</option>
              ))}
            </Select>
          </div>
          <Button size="sm" onClick={applyFilters} className="h-8">
            <Search className="mr-1 h-3.5 w-3.5" />
            {t("admin.traces.apply")}
          </Button>
          <Button size="sm" variant="outline" onClick={resetFilters} className="h-8">
            {t("admin.traces.reset")}
          </Button>
        </div>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {loading ? (
        <div className="rounded-md border border-border bg-card p-8 text-center">
          <Activity className="mx-auto h-8 w-8 animate-pulse text-muted-foreground" />
          <p className="mt-2 text-sm text-muted-foreground">{t("admin.traces.loading")}</p>
        </div>
      ) : runs.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-8 text-center">
          <Activity className="mx-auto h-8 w-8 text-muted-foreground" />
          <p className="mt-2 text-sm text-muted-foreground">{t("admin.traces.noTraces")}</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border bg-card">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left text-xs text-muted-foreground">
                <th className="px-3 py-2 font-medium">{t("admin.traces.colStartedAt")}</th>
                <th className="px-3 py-2 font-medium">{t("admin.traces.colTraceId")}</th>
                <th className="px-3 py-2 font-medium">{t("admin.traces.colStatus")}</th>
                <th className="px-3 py-2 font-medium">{t("admin.traces.colAnswerKind")}</th>
                <th className="px-3 py-2 font-medium">{t("admin.traces.colModel")}</th>
                <th className="px-3 py-2 font-medium">{t("admin.traces.colDuration")}</th>
                <th className="px-3 py-2 font-medium">{t("admin.traces.colTtft")}</th>
                <th className="px-3 py-2 font-medium">{t("admin.traces.colUser")}</th>
                <th className="px-3 py-2 font-medium">{t("admin.traces.colConversation")}</th>
              </tr>
            </thead>
            <tbody>
              {runs.map((run) => (
                <tr key={run.id} className="border-b border-border last:border-0 hover:bg-muted/50">
                  <td className="whitespace-nowrap px-3 py-2 text-xs text-muted-foreground">
                    {new Date(run.started_at).toLocaleString()}
                  </td>
                  <td className="px-3 py-2">
                    <Link
                      href={`/admin/traces/${run.trace_id}`}
                      className="font-mono text-xs text-indigo-600 underline-offset-2 hover:underline dark:text-indigo-400"
                      title={run.trace_id}
                    >
                      {run.trace_id.slice(0, 12)}…
                    </Link>
                  </td>
                  <td className="px-3 py-2">
                    <span
                      className={cn(
                        "inline-flex rounded-full px-1.5 py-0.5 text-[10px] font-medium",
                        run.status === "SUCCESS" && "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400",
                        run.status === "ERROR" && "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
                        run.status === "CANCELLED" && "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
                        run.status === "RUNNING" && "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
                      )}
                    >
                      {run.status}
                    </span>
                  </td>
                  <td className="px-3 py-2 text-xs">{run.answer_kind || "—"}</td>
                  <td className="px-3 py-2 text-xs">
                    {run.model || "—"}
                    {hasRouting(run.routing_events) && (
                      <span className="ml-1 inline-flex items-center rounded-full bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-400" title={JSON.stringify(run.routing_events)}>
                        <GitBranch className="mr-0.5 h-3 w-3" />
                        routed
                      </span>
                    )}
                  </td>
                  <td className="px-3 py-2 text-xs">{formatMs(run.duration_ms)}</td>
                  <td className="px-3 py-2 text-xs">{formatMs(run.ttft_ms)}</td>
                  <td className="px-3 py-2 text-xs">{run.user_id ?? "—"}</td>
                  <td className="px-3 py-2 text-xs">{run.conversation_id ?? "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>{t("admin.traces.totalCount", { count: total })}</span>
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2 pt-2">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
            {t("common.previous")}
          </Button>
          <span className="text-sm text-muted-foreground">{page} / {totalPages}</span>
          <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
            {t("common.next")}
          </Button>
        </div>
      )}
    </div>
  );
}
