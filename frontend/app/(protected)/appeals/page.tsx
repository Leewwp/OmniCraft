"use client";

import { Suspense, useEffect, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { FileText, Flag } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DataList } from "@/components/ui/data-list";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useToast } from "@/components/ui/Toast";

interface Appeal {
  id: number;
  target_type: string;
  target_id: number;
  reason: string;
  status: string;
  created_at: string;
}

interface MyReport {
  id: number;
  target_type: string;
  target_id: number;
  reason: string;
  detail?: string;
  status: string;
  action_taken?: string;
  created_at: string;
}

export default function AppealsPage() {
  return (
    <Suspense fallback={null}>
      <AppealsPageContent />
    </Suspense>
  );
}

function AppealsPageContent() {
  const t = useTranslations();
  const { toast } = useToast();
  const locale = useLocale();
  const { user } = useAuth();
  const searchParams = useSearchParams();
  // 申诉入口预填（FIX-14）：/appeals?target_type=content&target_id=123 自动
  // 展开表单并带出目标，免手填数字 id。
  const presetType = searchParams.get("target_type") ?? "";
  const presetId = searchParams.get("target_id") ?? "";
  const [appeals, setAppeals] = useState<Appeal[]>([]);
  const [reports, setReports] = useState<MyReport[]>([]);
  const [loading, setLoading] = useState(true);
  const [reportsLoading, setReportsLoading] = useState(true);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ target_type: "content", target_id: "", reason: "" });
  const [submitting, setSubmitting] = useState(false);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [reportsPage, setReportsPage] = useState(1);
  const [reportsHasMore, setReportsHasMore] = useState(false);
  const [reportsLoadingMore, setReportsLoadingMore] = useState(false);
  const [tab, setTab] = useState(
    // ?tab=reports 深链（FIX-31b：举报结果通知归位到「我的举报」）。
    searchParams.get("tab") === "reports" ? "reports" : "appeals"
  );

  useEffect(() => {
    if (!user) return;
    void loadAppeals(1, false);
    void loadReports(1, false);
  }, [user]);

  useEffect(() => {
    // T29（FIX-15）：account 申诉（封禁出路）无 target_id，预填 target_type 即展开。
    if (presetType === "account" || (presetType && presetId)) {
      setShowForm(true);
      setForm((f) => ({ ...f, target_type: presetType, target_id: presetId }));
    }
  }, [presetType, presetId]);

  async function loadAppeals(nextPage = 1, append = false) {
    setError("");
    setPage(nextPage);
    if (append) setLoadingMore(true); else setLoading(true);
    try {
      const data = await api.get<{ appeals?: Appeal[]; total?: number; page_size?: number }>(`/api/v1/appeals/me?page=${nextPage}&page_size=20`);
      const incoming = data.appeals || [];
      setAppeals((current) => append ? [...current, ...incoming.filter((item) => !current.some((existing) => existing.id === item.id))] : incoming);
      setPage(nextPage);
      const pageSize = data.page_size ?? 20;
      setHasMore((data.total ?? incoming.length) > nextPage * pageSize);
    } catch (e) {
      silentError(e, { component: 'AppealsPage', action: 'loadAppeals' });
      const message = t(getUserFacingErrorKey(e, "common.loadFailed"));
      setError(message);
      toast("error", message);
    } finally {
      setLoadingMore(false);
      setLoading(false);
    }
  }

  // 我的举报（FIX-28a）：admin 处理结果与处置说明对举报者可见。
  async function loadReports(nextPage = 1, append = false) {
    setReportsPage(nextPage);
    if (append) setReportsLoadingMore(true); else setReportsLoading(true);
    try {
      const data = await api.get<{ reports?: MyReport[]; total?: number; page_size?: number }>(`/api/v1/social/reports/me?page=${nextPage}&page_size=20`);
      const incoming = data.reports || [];
      setReports((current) => append ? [...current, ...incoming.filter((item) => !current.some((existing) => existing.id === item.id))] : incoming);
      const pageSize = data.page_size ?? 20;
      setReportsHasMore((data.total ?? incoming.length) > nextPage * pageSize);
    } catch (e) {
      silentError(e, { component: 'AppealsPage', action: 'loadReports' });
      const message = t(getUserFacingErrorKey(e, "common.loadFailed"));
      toast("error", message);
    } finally {
      setReportsLoadingMore(false);
      setReportsLoading(false);
    }
  }

  async function submitAppeal() {
    // T29（FIX-15）：account 申诉免填 target_id（服务端强制为本人）。
    const isAccount = form.target_type === "account";
    if (!form.reason || (!isAccount && !form.target_id)) return;
    setSubmitting(true);
    try {
      await api.post(
        "/api/v1/appeals",
        isAccount
          ? { target_type: form.target_type, reason: form.reason }
          : {
              target_type: form.target_type,
              target_id: parseInt(form.target_id),
              reason: form.reason,
            }
      );
      setShowForm(false);
      setForm({ target_type: "content", target_id: "", reason: "" });
      void loadAppeals();
    } catch (e) {
      silentError(e, { component: 'AppealsPage', action: 'submitAppeal' });
      const message = t(getUserFacingErrorKey(e, "appeals.submitFailed"));
      setError(message);
      toast("error", message);
    } finally {
      setSubmitting(false);
    }
  }

  function getStatusLabel(s: string) {
    switch (s) {
      case "pending": return t('appeals.pending');
      case "approved": return t('appeals.approved');
      case "rejected": return t('appeals.rejected');
      default: return s;
    }
  }

  function getReportStatusLabel(s: string) {
    switch (s) {
      case "pending": return t('appeals.reportPending');
      case "resolved": return t('appeals.reportResolved');
      case "dismissed": return t('appeals.reportDismissed');
      default: return s;
    }
  }

  return (
    <div className="mx-auto w-full max-w-2xl space-y-4 px-4 py-6">
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4 ">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('appeals.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t('appeals.subtitle')}</p>
        </div>
        <Button size="sm" onClick={() => setShowForm(true)} disabled={showForm}>
          {t('appeals.newAppeal')}
        </Button>
      </div>

      <Tabs value={tab} onValueChange={(value) => setTab(value === "reports" ? "reports" : "appeals")}>
        <TabsList aria-label={t('appeals.tabLabel')}>
          <TabsTrigger value="appeals">{t('appeals.appealsTab')}</TabsTrigger>
          <TabsTrigger value="reports">{t('appeals.reportsTab')}</TabsTrigger>
        </TabsList>

        <TabsContent value="appeals" className="mt-4 space-y-4">
          {showForm && (
            <div className="space-y-3 rounded-md border border-border bg-card p-4 ">
              <h3 className="text-sm font-semibold">{t('appeals.newAppeal')}</h3>
              <select
                value={form.target_type}
                onChange={(e) => setForm((f) => ({ ...f, target_type: e.target.value }))}
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
              >
                <option value="content">{t('appeals.typeContent')}</option>
                <option value="comment">{t('appeals.typeComment')}</option>
                <option value="account">{t('appeals.typeAccount')}</option>
              </select>
              {form.target_type !== "account" && (
                <input
                  type="number"
                  placeholder={t('appeals.targetId')}
                  value={form.target_id}
                  onChange={(e) => setForm((f) => ({ ...f, target_id: e.target.value }))}
                  className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
                />
              )}
              {form.target_type === "account" && (
                <p className="text-xs text-muted-foreground">{t('appeals.accountHint')}</p>
              )}
              <textarea
                placeholder={t('appeals.reason')}
                value={form.reason}
                onChange={(e) => setForm((f) => ({ ...f, reason: e.target.value }))}
                rows={3}
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
              />
              <div className="flex gap-2">
                <Button size="sm" disabled={submitting} onClick={() => void submitAppeal()}>
                  {submitting ? t('appeals.submitting') : t('appeals.submit')}
                </Button>
                <Button size="sm" variant="outline" onClick={() => setShowForm(false)}>
                  {t('appeals.cancel')}
                </Button>
              </div>
            </div>
          )}

          <DataList
            items={appeals}
            loading={loading}
            error={showForm ? undefined : error}
            onRetry={() => void loadAppeals(page, page > 1)}
            hasMore={hasMore}
            loadingMore={loadingMore}
            onLoadMore={() => loadAppeals(page + 1, true)}
            empty={<EmptyState icon={FileText} title={t('appeals.noAppeals')} />}
            loadingState={<div className="space-y-3"><Skeleton className="h-20 w-full" /><Skeleton className="h-20 w-full" /><Skeleton className="h-20 w-full" /></div>}
            getKey={(appeal) => appeal.id}
            renderItem={(a) => (
              <div key={a.id} className="rounded-md border border-border bg-card p-4 ">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">{a.target_type} #{a.target_id}</span>
                  <span className={`rounded px-2 py-0.5 text-xs ${
                    a.status === "approved" ? "bg-emerald-50 text-emerald-700" :
                    a.status === "rejected" ? "bg-red-50 text-red-700" :
                    "bg-amber-50 text-amber-700"
                  }`}>{getStatusLabel(a.status)}</span>
                </div>
                <p className="mt-2 text-sm text-muted-foreground">{a.reason}</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {new Date(a.created_at).toLocaleString(locale === "en" ? "en-US" : "zh-CN")}
                </p>
              </div>
            )}
          />
        </TabsContent>

        <TabsContent value="reports" className="mt-4 space-y-4">
          <DataList
            items={reports}
            loading={reportsLoading}
            onRetry={() => void loadReports(reportsPage, reportsPage > 1)}
            hasMore={reportsHasMore}
            loadingMore={reportsLoadingMore}
            onLoadMore={() => loadReports(reportsPage + 1, true)}
            empty={<EmptyState icon={Flag} title={t('appeals.noReports')} />}
            loadingState={<div className="space-y-3"><Skeleton className="h-20 w-full" /><Skeleton className="h-20 w-full" /><Skeleton className="h-20 w-full" /></div>}
            getKey={(report) => report.id}
            renderItem={(r) => (
              <div key={r.id} className="rounded-md border border-border bg-card p-4 ">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">{r.target_type} #{r.target_id}</span>
                  <span className={`rounded px-2 py-0.5 text-xs ${
                    r.status === "resolved" ? "bg-emerald-50 text-emerald-700" :
                    r.status === "dismissed" ? "bg-zinc-100 text-zinc-600" :
                    "bg-amber-50 text-amber-700"
                  }`}>{getReportStatusLabel(r.status)}</span>
                </div>
                <p className="mt-2 text-sm text-muted-foreground">{r.reason}</p>
                {r.status !== "pending" && (
                  <p className="mt-2 rounded-md border border-border bg-background p-2 text-sm">
                    <span className="text-muted-foreground">{t('appeals.actionTaken')}：</span>
                    {r.action_taken || t('appeals.reportFallbackAction')}
                  </p>
                )}
                <p className="mt-1 text-xs text-muted-foreground">
                  {new Date(r.created_at).toLocaleString(locale === "en" ? "en-US" : "zh-CN")}
                </p>
              </div>
            )}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
}
