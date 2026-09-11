"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { AdminMetricCard } from "@/components/admin/AdminMetricCard";
import { Button } from "@/components/ui/button";
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

// F-A003: 部分统计缺失时 reduce/后端都可能产出 NaN；`?? "-"` 只拦
// nullish 不拦 NaN，会把 NaN 直接喂给 React children 触发告警。
function formatMetricValue(value: number | null | undefined): number | "-" {
  return Number.isFinite(value) ? (value as number) : "-";
}

export default function AdminDashboardPage() {
  const t = useTranslations();
  const [reportStats, setReportStats] = useState<ReportStats | null>(null);
  const [queueStats, setQueueStats] = useState<QueueStats | null>(null);
  const [openFeedback, setOpenFeedback] = useState<number | null>(null);
  const [pendingAppeals, setPendingAppeals] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  // T28（FIX-35 / F-117）：部分加载失败可见——此前 .catch(()=>null) 把失败
  // 伪装成空态（"-"}, 无法区分「无数据」与「加载失败」。
  const [loadFailed, setLoadFailed] = useState(false);

  const loadDashboard = useCallback(async () => {
    setLoading(true);
    setLoadFailed(false);
    const results = await Promise.allSettled([
      api.get<ReportStats>("/api/v1/admin/reports/stats"),
      api.get<QueueStats>("/api/v1/admin/queue/stats"),
      api.get<FeedbackListResponse>("/api/v1/admin/feedback?status=open&page=1&page_size=1"),
      api.get<AppealListResponse>("/api/v1/admin/appeals?status=pending&page=1&page_size=1"),
    ]);
    let failures = 0;
    for (const r of results) {
      if (r.status === "rejected") {
        failures++;
        silentError(r.reason, { component: "AdminDashboardPage", action: "loadDashboard" });
      }
    }
    if (failures < results.length) {
      if (results[0].status === "fulfilled") setReportStats(results[0].value);
      if (results[1].status === "fulfilled") setQueueStats(results[1].value);
      if (results[2].status === "fulfilled") setOpenFeedback(results[2].value.total);
      if (results[3].status === "fulfilled") setPendingAppeals(results[3].value.total);
    }
    if (failures > 0) setLoadFailed(true);
    setLoading(false);
  }, []);

  useEffect(() => {
    void loadDashboard();
  }, [loadDashboard]);

  const totalQueueFailures =
    queueStats?.topics?.reduce((sum, topic) => sum + (Number.isFinite(topic.failure_count) ? topic.failure_count : 0), 0) ?? null;

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("admin.dashboard.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("admin.dashboard.subtitle")}</p>
      </div>

      {loadFailed && !loading && (
        <div className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border bg-card p-3">
          <p role="alert" className="text-sm text-destructive">{t("admin.dashboard.partialLoadFailed")}</p>
          <Button variant="outline" size="sm" onClick={() => void loadDashboard()}>
            {t("common.retry")}
          </Button>
        </div>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <AdminMetricCard
          label={t("admin.dashboard.pendingReports")}
          value={formatMetricValue(reportStats?.pending_count)}
          icon={Flag}
          variant={(reportStats?.pending_count ?? 0) > 0 ? "warning" : "default"}
          loading={loading}
        />
        <AdminMetricCard
          label={t("admin.dashboard.openFeedback")}
          value={formatMetricValue(openFeedback)}
          icon={MessageSquare}
          loading={loading}
        />
        <AdminMetricCard
          label={t("admin.dashboard.pendingAppeals")}
          value={formatMetricValue(pendingAppeals)}
          icon={AlertTriangle}
          variant={(pendingAppeals ?? 0) > 0 ? "warning" : "default"}
          loading={loading}
        />
        <AdminMetricCard
          label={t("admin.dashboard.queueFailures")}
          value={formatMetricValue(totalQueueFailures)}
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
