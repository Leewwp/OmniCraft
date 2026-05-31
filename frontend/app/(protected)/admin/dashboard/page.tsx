"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { AdminMetricCard } from "@/components/admin/AdminMetricCard";
import { Flag, AlertTriangle, MessageSquare, ListOrdered } from "lucide-react";

interface ReportStats {
  pending_count: number;
  resolved_count: number;
  total_count: number;
}

interface QueueStats {
  topics?: { name: string; depth: number; lag: number; failure_count: number }[];
}

interface FeedbackListResponse {
  total: number;
}

interface AppealListResponse {
  total: number;
}

export default function AdminDashboardPage() {
  const t = useTranslations();
  const [reportStats, setReportStats] = useState<ReportStats | null>(null);
  const [queueStats, setQueueStats] = useState<QueueStats | null>(null);
  const [openFeedback, setOpenFeedback] = useState<number | null>(null);
  const [pendingAppeals, setPendingAppeals] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);

  const loadDashboard = useCallback(async () => {
    setLoading(true);
    try {
      const [reports, queue, feedback, appeals] = await Promise.all([
        api.get<ReportStats>("/api/v1/admin/reports/stats").catch(() => null),
        api.get<QueueStats>("/api/v1/admin/queue/stats").catch(() => null),
        api.get<FeedbackListResponse>("/api/v1/admin/feedback?status=open&page=1&page_size=1").catch(() => null),
        api.get<AppealListResponse>("/api/v1/admin/appeals?status=pending&page=1&page_size=1").catch(() => null),
      ]);
      if (reports) setReportStats(reports);
      if (queue) setQueueStats(queue);
      if (feedback) setOpenFeedback(feedback.total);
      if (appeals) setPendingAppeals(appeals.total);
    } catch (e) {
      silentError(e, { component: "AdminDashboardPage", action: "loadDashboard" });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadDashboard();
  }, [loadDashboard]);

  const totalQueueFailures = queueStats?.topics?.reduce((sum, t) => sum + t.failure_count, 0) ?? null;

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("admin.dashboard.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("admin.dashboard.subtitle")}</p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <AdminMetricCard
          label={t("admin.dashboard.pendingReports")}
          value={reportStats?.pending_count ?? "-"}
          icon={Flag}
          variant={(reportStats?.pending_count ?? 0) > 0 ? "warning" : "default"}
          loading={loading}
        />
        <AdminMetricCard
          label={t("admin.dashboard.openFeedback")}
          value={openFeedback ?? "-"}
          icon={MessageSquare}
          loading={loading}
        />
        <AdminMetricCard
          label={t("admin.dashboard.pendingAppeals")}
          value={pendingAppeals ?? "-"}
          icon={AlertTriangle}
          variant={(pendingAppeals ?? 0) > 0 ? "warning" : "default"}
          loading={loading}
        />
        <AdminMetricCard
          label={t("admin.dashboard.queueFailures")}
          value={totalQueueFailures ?? "-"}
          icon={ListOrdered}
          variant={(totalQueueFailures ?? 0) > 0 ? "danger" : "default"}
          loading={loading}
        />
      </div>

      <div className="rounded-md border border-border bg-card p-4">
        <h3 className="text-sm font-semibold">{t("admin.dashboard.quickLinks")}</h3>
        <div className="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
          <a href="/admin/reports" className="rounded-md border border-border p-3 text-sm transition-colors hover:bg-muted/50">
            {t("admin.dashboard.goReports")}
          </a>
          <a href="/admin/feedback" className="rounded-md border border-border p-3 text-sm transition-colors hover:bg-muted/50">
            {t("admin.dashboard.goFeedback")}
          </a>
          <a href="/admin/appeal" className="rounded-md border border-border p-3 text-sm transition-colors hover:bg-muted/50">
            {t("admin.dashboard.goAppeals")}
          </a>
          <a href="/admin/queue" className="rounded-md border border-border p-3 text-sm transition-colors hover:bg-muted/50">
            {t("admin.dashboard.goQueue")}
          </a>
        </div>
      </div>
    </div>
  );
}
