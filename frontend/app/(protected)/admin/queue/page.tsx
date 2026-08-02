"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
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

interface DLQEntry {
  id: string;
  topic: string;
  payload: string;
  error: string;
  trace_id: string;
  failed_at: string;
  retry_count: number;
}

export default function AdminQueuePage() {
  const t = useTranslations();
  const [stats, setStats] = useState<QueueStats | null>(null);
  const [dlqEntries, setDlqEntries] = useState<DLQEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const loadData = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [statsData, dlqData] = await Promise.all([
        api.get<QueueStats>("/api/v1/admin/queue/stats").catch(() => null),
        api.get<{ entries: DLQEntry[] }>("/api/v1/admin/queue/dlq").catch(() => null),
      ]);
      if (statsData) setStats(statsData);
      if (dlqData) setDlqEntries(dlqData.entries || []);
    } catch (e) {
      silentError(e, { component: "AdminQueuePage", action: "loadData" });
      setError(t(getUserFacingErrorKey(e, "admin.queue.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

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

      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}

      <div className="rounded-md border border-border bg-card">
        <div className="border-b border-border px-4 py-3">
          <h3 className="text-sm font-semibold">{t("admin.queue.stats")}</h3>
        </div>
        <div className="p-4">
          {!stats || !stats.topics || stats.topics.length === 0 ? (
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
          {dlqEntries.length === 0 ? (
            <div className="text-center py-6">
              <AlertTriangle className="mx-auto h-8 w-8 text-muted-foreground" />
              <p className="mt-2 text-sm text-muted-foreground">{t("admin.queue.noDLQ")}</p>
            </div>
          ) : (
            <div className="space-y-3">
              {dlqEntries.map((entry) => (
                <div key={entry.id} className="rounded-md border border-red-200 bg-red-50 p-3 dark:border-red-800 dark:bg-red-900/20">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-xs font-mono font-medium">{entry.topic}</span>
                    <span className="text-xs text-muted-foreground">{new Date(entry.failed_at).toLocaleString()}</span>
                  </div>
                  <p className="mt-1 text-xs text-red-700 dark:text-red-400">{entry.error}</p>
                  {entry.trace_id && (
                    <p className="mt-1 text-xs text-muted-foreground">
                      trace: {entry.trace_id} · retries: {entry.retry_count}
                    </p>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <p className="text-xs text-muted-foreground">{t("admin.queue.readOnlyNote")}</p>
    </div>
  );
}
