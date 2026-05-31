"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { ScrollText, ChevronDown, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";

interface AuditLog {
  id: number;
  admin_user_id: number;
  action: string;
  target_type: string;
  target_id: string;
  trace_id: string;
  metadata: Record<string, unknown>;
  result: string;
  created_at: string;
}

export default function AdminAuditLogsPage() {
  const t = useTranslations();
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [actionFilter, setActionFilter] = useState("");
  const [expandedId, setExpandedId] = useState<number | null>(null);

  const pageSize = 20;

  const loadLogs = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const params = new URLSearchParams();
      params.set("page", String(page));
      params.set("page_size", String(pageSize));
      if (actionFilter) params.set("action", actionFilter);
      const data = await api.get<{ items: AuditLog[]; total: number }>(
        `/api/v1/admin/audit-logs?${params.toString()}`
      );
      setLogs(data.items || []);
      setTotal(data.total || 0);
    } catch (e) {
      silentError(e, { component: "AdminAuditLogsPage", action: "loadLogs" });
      setError(e instanceof ApiRequestError ? e.message : t("admin.auditLogs.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [page, actionFilter, t]);

  useEffect(() => {
    void loadLogs();
  }, [loadLogs]);

  const totalPages = Math.ceil(total / pageSize);

  if (loading && logs.length === 0) {
    return (
      <div className="space-y-4 p-6">
        <div className="space-y-3 rounded-md border border-border bg-card p-6">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-8 w-full animate-pulse rounded bg-muted" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight">{t("admin.auditLogs.title")}</h1>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="flex items-center gap-3">
        <select
          className="rounded-md border border-border bg-background px-3 py-1.5 text-sm"
          value={actionFilter}
          onChange={(e) => { setActionFilter(e.target.value); setPage(1); }}
        >
          <option value="">{t("admin.auditLogs.allActions")}</option>
          <option value="content_ban">content_ban</option>
          <option value="content_restore">content_restore</option>
          <option value="user_ban">user_ban</option>
          <option value="user_unban">user_unban</option>
          <option value="ip_approve">ip_approve</option>
          <option value="ip_reject">ip_reject</option>
          <option value="appeal_resolve">appeal_resolve</option>
          <option value="report_resolve">report_resolve</option>
          <option value="config_patch">config_patch</option>
          <option value="category_create">category_create</option>
          <option value="category_update">category_update</option>
          <option value="category_delete">category_delete</option>
          <option value="feedback_reply">feedback_reply</option>
          <option value="feedback_close">feedback_close</option>
          <option value="feedback_reopen">feedback_reopen</option>
        </select>
      </div>

      {logs.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-8 text-center">
          <ScrollText className="mx-auto h-8 w-8 text-muted-foreground" />
          <p className="mt-2 text-sm text-muted-foreground">{t("admin.auditLogs.noLogs")}</p>
        </div>
      ) : (
        <div className="space-y-2">
          {logs.map((log) => (
            <div key={log.id} className="rounded-md border border-border bg-card">
              <button
                type="button"
                className="w-full p-3 text-left transition-colors hover:bg-muted/50"
                onClick={() => setExpandedId(expandedId === log.id ? null : log.id)}
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0 flex items-center gap-2">
                    {expandedId === log.id ? (
                      <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
                    ) : (
                      <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
                    )}
                    <div>
                      <p className="text-sm font-medium">{log.action}</p>
                      <p className="mt-0.5 text-xs text-muted-foreground">
                        admin #{log.admin_user_id} · {log.target_type} {log.target_id} · {new Date(log.created_at).toLocaleString()}
                      </p>
                    </div>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <span className={cn(
                      "inline-flex rounded px-1.5 py-0.5 text-[10px] font-medium",
                      log.result === "success" ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400" : "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
                    )}>
                      {log.result}
                    </span>
                  </div>
                </div>
              </button>
              {expandedId === log.id && (
                <div className="border-t border-border px-3 py-3">
                  {log.trace_id && (
                    <p className="text-xs text-muted-foreground mb-2">
                      trace: {log.trace_id}
                    </p>
                  )}
                  {log.metadata && Object.keys(log.metadata).length > 0 ? (
                    <pre className="rounded-md border border-border bg-background p-2 text-xs overflow-x-auto">
                      {JSON.stringify(log.metadata, null, 2)}
                    </pre>
                  ) : (
                    <p className="text-xs text-muted-foreground">{t("admin.auditLogs.noMetadata")}</p>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      )}

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
