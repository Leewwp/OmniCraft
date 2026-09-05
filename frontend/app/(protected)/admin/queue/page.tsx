"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { ListOrdered, AlertTriangle } from "lucide-react";

interface QueueStats {
  topics: QueueTopic[];
}

interface QueueTopic {
  name: string;
  depth: number;
  lag: number;
  failure_count: number;
}

// T28（FIX-35 / F-113）：字段契约与后端 worker.DLQEntry 对齐（dlq_worker.go）——
// 此前期望 {topic,retry_count,trace_id} 而后端返回 {original_topic,attempts,...}，
// 卡片 topic 与重试数恒空白。
interface DLQEntry {
  id: string;
  original_topic: string;
  original_id: string;
  consumer_group?: string;
  payload: string;
  attempts: number;
  error: string;
  failed_at: string;
}

export default function AdminQueuePage() {
  const t = useTranslations();
  const [stats, setStats] = useState<QueueStats | null>(null);
  const [dlqEntries, setDlqEntries] = useState<DLQEntry[]>([]);
  const [loading, setLoading] = useState(true);
  // T28（FIX-35）：stats/dlq 分区错误态——此前 .catch(()=>null) 把失败伪装成
  // 空态，排障时无法区分「队列空」与「加载失败」。
  const [statsError, setStatsError] = useState(false);
  const [dlqError, setDlqError] = useState(false);
  const [replayTarget, setReplayTarget] = useState<DLQEntry | null>(null);
  const [replayBusy, setReplayBusy] = useState(false);
  const [replayResult, setReplayResult] = useState<{ ok: boolean; id: string } | null>(null);

  const loadStats = useCallback(async () => {
    setStatsError(false);
    try {
      setStats(await api.get<QueueStats>("/api/v1/admin/queue/stats"));
    } catch (e) {
      silentError(e, { component: "AdminQueuePage", action: "loadStats" });
      setStatsError(true);
    }
  }, []);

  const loadDlq = useCallback(async () => {
    setDlqError(false);
    try {
      const data = await api.get<{ entries: DLQEntry[] }>("/api/v1/admin/queue/dlq");
      setDlqEntries(data.entries || []);
    } catch (e) {
      silentError(e, { component: "AdminQueuePage", action: "loadDlq" });
      setDlqError(true);
    }
  }, []);

  const loadData = useCallback(async () => {
    setLoading(true);
    await Promise.all([loadStats(), loadDlq()]);
    setLoading(false);
  }, [loadStats, loadDlq]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  // Replay 语义（FIX-35）：重投回原 topic，但**不删除** DLQ 原条目——成功后
  // 刷新列表并给出行内反馈，条目不会消失。
  async function replayEntry(entry: DLQEntry) {
    setReplayBusy(true);
    try {
      await api.post(`/api/v1/admin/queue/dlq/${encodeURIComponent(entry.id)}/replay`, {});
      setReplayResult({ ok: true, id: entry.id });
      await loadDlq();
    } catch (e) {
      silentError(e, { component: "AdminQueuePage", action: "replayEntry" });
      setReplayResult({ ok: false, id: entry.id });
    } finally {
      setReplayBusy(false);
    }
  }

  if (loading) {
    return (
      <div className="space-y-4 p-6">
        <div className="space-y-3 rounded-md border border-border bg-card p-6">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="h-8 w-full animate-pulse rounded bg-muted" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 p-6">
      <h1 className="text-2xl font-bold tracking-tight">{t("admin.queue.title")}</h1>

      <div className="rounded-md border border-border bg-card">
        <div className="border-b border-border px-4 py-3">
          <h3 className="text-sm font-semibold">{t("admin.queue.stats")}</h3>
        </div>
        <div className="p-4">
          {statsError ? (
            <div className="flex flex-col items-center gap-2 py-6">
              <AlertTriangle className="h-8 w-8 text-destructive" />
              <p role="alert" className="text-sm text-destructive">{t("admin.queue.loadFailed")}</p>
              <Button variant="outline" size="sm" onClick={() => void loadStats()}>{t("common.retry")}</Button>
            </div>
          ) : !stats || !stats.topics || stats.topics.length === 0 ? (
            <div className="text-center py-6">
              <ListOrdered className="mx-auto h-8 w-8 text-muted-foreground" />
              <p className="mt-2 text-sm text-muted-foreground">{t("admin.queue.noStats")}</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border">
                    <th className="pb-2 text-left font-medium text-muted-foreground">{t("admin.queue.topic")}</th>
                    <th className="pb-2 text-right font-medium text-muted-foreground">{t("admin.queue.depth")}</th>
                    <th className="pb-2 text-right font-medium text-muted-foreground">{t("admin.queue.lag")}</th>
                    <th className="pb-2 text-right font-medium text-muted-foreground">{t("admin.queue.failures")}</th>
                  </tr>
                </thead>
                <tbody>
                  {stats.topics.map((topic) => (
                    <tr key={topic.name} className="border-b border-border last:border-b-0">
                      <td className="py-2 font-mono text-xs">{topic.name}</td>
                      <td className="py-2 text-right">{topic.depth}</td>
                      <td className="py-2 text-right">{topic.lag}</td>
                      <td className="py-2 text-right">
                        {topic.failure_count > 0 ? (
                          <span className="text-red-600 font-medium">{topic.failure_count}</span>
                        ) : (
                          <span className="text-muted-foreground">0</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>

      <div className="rounded-md border border-border bg-card">
        <div className="border-b border-border px-4 py-3">
          <h3 className="text-sm font-semibold">{t("admin.queue.dlq")}</h3>
        </div>
        <div className="p-4">
          {replayResult && (
            <p
              role="status"
              className={`mb-3 text-sm ${replayResult.ok ? "text-emerald-600" : "text-destructive"}`}
            >
              {replayResult.ok
                ? t("admin.queue.replaySuccess", { id: replayResult.id })
                : t("admin.queue.replayFailed", { id: replayResult.id })}
            </p>
          )}
          {dlqError ? (
            <div className="flex flex-col items-center gap-2 py-6">
              <AlertTriangle className="h-8 w-8 text-destructive" />
              <p role="alert" className="text-sm text-destructive">{t("admin.queue.loadFailed")}</p>
              <Button variant="outline" size="sm" onClick={() => void loadDlq()}>{t("common.retry")}</Button>
            </div>
          ) : dlqEntries.length === 0 ? (
            <div className="text-center py-6">
              <AlertTriangle className="mx-auto h-8 w-8 text-muted-foreground" />
              <p className="mt-2 text-sm text-muted-foreground">{t("admin.queue.noDLQ")}</p>
            </div>
          ) : (
            <div className="space-y-3">
              {dlqEntries.map((entry, index) => (
                // F-A004: DLQ 条目 id 可能缺失/重复——复合兜底保 key 唯一。
                <div key={entry.id || `${entry.original_topic}:${entry.failed_at}:${index}`} className="rounded-md border border-red-200 bg-red-50 p-3 dark:border-red-800 dark:bg-red-900/20">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-xs font-mono font-medium">{entry.original_topic}</span>
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-muted-foreground">{new Date(entry.failed_at).toLocaleString()}</span>
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={replayBusy}
                        onClick={() => setReplayTarget(entry)}
                      >
                        {t("admin.queue.replay")}
                      </Button>
                    </div>
                  </div>
                  <p className="mt-1 text-xs text-red-700 dark:text-red-400">{entry.error}</p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {t("admin.queue.retries", { count: entry.attempts })}
                    {entry.consumer_group ? ` · ${entry.consumer_group}` : ""}
                  </p>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <p className="text-xs text-muted-foreground">{t("admin.queue.readOnlyNote")}</p>

      <ConfirmModal
        open={replayTarget !== null}
        onOpenChange={(open) => { if (!open) setReplayTarget(null); }}
        title={t("admin.queue.replayTitle")}
        description={
          replayTarget
            ? t("admin.queue.replayConfirm", { topic: replayTarget.original_topic, id: replayTarget.id })
            : ""
        }
        confirmLabel={t("admin.queue.replay")}
        onConfirm={() => (replayTarget ? replayEntry(replayTarget) : Promise.resolve())}
      />
    </div>
  );
}
