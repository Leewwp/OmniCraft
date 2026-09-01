"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { AdminFilterBar } from "@/components/admin/AdminFilterBar";
import { Flag, Eye, ArrowLeft, CheckCircle, XCircle } from "lucide-react";
import { cn } from "@/lib/utils";

interface Report {
  id: number;
  target_type: string;
  target_id: number;
  reporter_id: number;
  reason: string;
  status: string;
  action_taken?: string;
  created_at: string;
  resolved_at: string | null;
}

const STATUS_COLORS: Record<string, string> = {
  pending: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
  resolved: "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400",
  dismissed: "bg-muted text-muted-foreground",
};

export default function AdminReportsPage() {
  const t = useTranslations();
  const [reports, setReports] = useState<Report[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [selectedReport, setSelectedReport] = useState<Report | null>(null);
  const [busy, setBusy] = useState(false);

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmAction, setConfirmAction] = useState<"resolve" | "dismiss" | null>(null);

  const pageSize = 20;

  const loadReports = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const params = new URLSearchParams();
      params.set("page", String(page));
      params.set("page_size", String(pageSize));
      if (statusFilter) params.set("status", statusFilter);
      if (typeFilter) params.set("target_type", typeFilter);
      const data = await api.get<{ reports: Report[]; total: number }>(
        `/api/v1/admin/reports?${params.toString()}`
      );
      setReports(data.reports || []);
      setTotal(data.total || 0);
    } catch (e) {
      silentError(e, { component: "AdminReportsPage", action: "loadReports" });
      setError(t(getUserFacingErrorKey(e, "admin.reports.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [page, statusFilter, typeFilter, t]);

  useEffect(() => {
    void loadReports();
  }, [loadReports]);

  const totalPages = Math.ceil(total / pageSize);

  if (loading && reports.length === 0) {
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

  if (selectedReport) {
    return (
      <div className="space-y-4 p-6">
        <button
          type="button"
          className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
          onClick={() => setSelectedReport(null)}
        >
          <ArrowLeft className="h-4 w-4" />
          {t("common.back")}
        </button>
        {error && <p className="text-sm text-destructive" role="alert">{error}</p>}

        <div className="rounded-md border border-border bg-card p-4">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h2 className="text-lg font-semibold">
                {t("admin.reports.reportDetail", { id: selectedReport.id })}
              </h2>
              <p className="mt-1 text-xs text-muted-foreground">
                {t("admin.reports.reporter")}: #{selectedReport.reporter_id} · {new Date(selectedReport.created_at).toLocaleString()}
              </p>
            </div>
            <span className={cn("inline-flex rounded px-2 py-0.5 text-xs font-medium", STATUS_COLORS[selectedReport.status] || STATUS_COLORS.pending)}>
              {selectedReport.status}
            </span>
          </div>

          <div className="mt-4 space-y-3">
            <div>
              <p className="text-xs font-medium text-muted-foreground">{t("admin.reports.target")}</p>
              <p className="text-sm">{selectedReport.target_type} #{selectedReport.target_id}</p>
            </div>
            <div>
              <p className="text-xs font-medium text-muted-foreground">{t("admin.reports.reason")}</p>
              <p className="whitespace-pre-wrap text-sm">{selectedReport.reason}</p>
            </div>
            {selectedReport.action_taken && (
              <div>
                <p className="text-xs font-medium text-muted-foreground">{t("admin.reports.actionTaken")}</p>
                <p className="whitespace-pre-wrap text-sm">{selectedReport.action_taken}</p>
              </div>
            )}
          </div>
        </div>

        {selectedReport.status === "pending" && (
          <div className="flex gap-3">
            <Button
              variant="outline"
              size="sm"
              onClick={() => { setConfirmAction("resolve"); setError(""); setConfirmOpen(true); }}
              disabled={busy}
            >
              <CheckCircle className="mr-1.5 h-4 w-4 text-emerald-600" />
              {t("admin.reports.uphold")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => { setConfirmAction("dismiss"); setError(""); setConfirmOpen(true); }}
              disabled={busy}
            >
              <XCircle className="mr-1.5 h-4 w-4 text-muted-foreground" />
              {t("admin.reports.dismiss")}
            </Button>
          </div>
        )}

        <ConfirmModal
          open={confirmOpen}
          onOpenChange={setConfirmOpen}
          title={confirmAction === "resolve" ? t("admin.reports.upholdTitle") : t("admin.reports.dismissTitle")}
          description={confirmAction === "resolve" ? t("admin.reports.upholdConfirm") : t("admin.reports.dismissConfirm")}
          confirmLabel={confirmAction === "resolve" ? t("admin.reports.confirmUphold") : t("admin.reports.confirmDismiss")}
          confirmVariant="default"
          requireReason
          reasonLabel={t("admin.reports.explanation")}
          onConfirm={async (explanation) => {
            if (confirmAction) {
              try {
                await api.patch(`/api/v1/admin/reports/${selectedReport.id}`, {
                  status: confirmAction === "resolve" ? "resolved" : "dismissed",
                  action_taken: explanation,
                });
              } catch (e) {
                // surface the failure on the page; rethrow so the modal stays open for retry
                silentError(e, { component: "AdminReportsPage", action: "patchReport" });
                setError(t("admin.reports.updateFailed"));
                throw e;
              }
              setConfirmOpen(false);
              setSelectedReport(null);
              await loadReports();
            }
          }}
        />
      </div>
    );
  }

  return (
    <div className="space-y-4 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight">{t("admin.reports.title")}</h1>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <AdminFilterBar
        filters={[
          {
            key: "status",
            value: statusFilter,
            onChange: (v) => { setStatusFilter(v); setPage(1); },
            ariaLabel: t("admin.reports.statusLabel"),
            allLabel: t("admin.reports.allStatuses"),
            options: [
              { value: "pending", label: "pending" },
              { value: "resolved", label: "resolved" },
              { value: "dismissed", label: "dismissed" },
            ],
          },
          {
            key: "type",
            value: typeFilter,
            onChange: (v) => { setTypeFilter(v); setPage(1); },
            ariaLabel: t("admin.reports.typeLabel"),
            allLabel: t("admin.reports.allTypes"),
            options: [
              { value: "content", label: "content" },
              { value: "comment", label: "comment" },
            ],
          },
        ]}
      />

      {reports.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-8 text-center">
          <Flag className="mx-auto h-8 w-8 text-muted-foreground" />
          <p className="mt-2 text-sm text-muted-foreground">{t("admin.reports.noReports")}</p>
        </div>
      ) : (
        <div className="space-y-2">
          {reports.map((report) => (
            <button
              key={report.id}
              type="button"
              className="w-full rounded-md border border-border bg-card p-3 text-left transition-colors hover:bg-muted/50"
              onClick={() => setSelectedReport(report)}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate">{report.reason}</p>
                  <p className="mt-0.5 text-xs text-muted-foreground">
                    #{report.id} · {report.target_type} #{report.target_id} · {new Date(report.created_at).toLocaleDateString()}
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-1.5">
                  <span className={cn("inline-flex rounded px-1.5 py-0.5 text-[10px] font-medium", STATUS_COLORS[report.status] || STATUS_COLORS.pending)}>
                    {report.status}
                  </span>
                  <Eye className="h-3.5 w-3.5 text-muted-foreground" />
                </div>
              </div>
            </button>
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
