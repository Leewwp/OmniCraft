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
import { Select } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useToast } from "@/components/ui/Toast";

interface Appeal {
  id: number;
  target_type: string;
  target_id: number;
  reason: string;
  status: string;
  admin_response?: string;
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

// #845/A3：「选择近期事件」弹窗行——content 为被下架内容（detail=下架原因），
// comment 为被隐藏评论（detail=关联父对象标题）。
interface PickerEvent {
  id: number;
  title: string;
  detail?: string;
  detailKind: "ban" | "parent";
  date: string;
}

interface MyContentRow {
  id: number;
  title: string;
  status: string;
  ban_reason?: string;
  created_at: string;
}

interface MyHiddenCommentRow {
  id: number;
  body: string;
  content_title: string;
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
  // #845/A3：target_id 不再手输——content/comment 经「选择近期事件」弹窗
  // 填入（深链预填除外），account 由服务端强制为本人。
  const [form, setForm] = useState<{ target_type: string; target_id: number | null; reason: string }>({
    target_type: "content",
    target_id: null,
    reason: "",
  });
  const [selectedEvent, setSelectedEvent] = useState<PickerEvent | null>(null);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [pickerItems, setPickerItems] = useState<PickerEvent[]>([]);
  const [pickerLoading, setPickerLoading] = useState(false);
  const [pickerError, setPickerError] = useState("");
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
    // #845/A3：深链预填行为不变（studio「去申诉」入口），仅目标从手输改为
    // 直接带出——content/comment 带 target_id，account 只带 target_type。
    if (presetType === "account" || (presetType && presetId)) {
      const parsedId = presetId ? Number(presetId) : null;
      const validId = parsedId !== null && Number.isFinite(parsedId) && parsedId > 0 ? parsedId : null;
      setShowForm(true);
      setForm((f) => ({ ...f, target_type: presetType, target_id: presetType === "account" ? null : validId }));
      setSelectedEvent(null);
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

  // #845/A3：「选择近期事件」数据源——content 复用 GET /users/me/contents
  // （含全状态，客户端过滤 banned；翻页拉全以防被下架项不在首页），
  // comment 走新只读端点 GET /users/me/comments?status=hidden。
  async function loadPickerEvents() {
    setPickerLoading(true);
    setPickerError("");
    try {
      let events: PickerEvent[] = [];
      if (form.target_type === "content") {
        const collected: MyContentRow[] = [];
        const maxPages = 10; // 安全上限：10×100 行已覆盖演示语料规模
        for (let page = 1; page <= maxPages; page++) {
          const data = await api.get<{ contents?: MyContentRow[]; total?: number }>(
            `/api/v1/users/me/contents?page=${page}&page_size=100`
          );
          const incoming = data.contents ?? [];
          collected.push(...incoming);
          const total = data.total ?? collected.length;
          if (collected.length >= total || incoming.length === 0) break;
        }
        events = collected
          .filter((c) => c.status === "banned")
          .map((c) => ({
            id: c.id,
            title: c.title,
            detail: c.ban_reason,
            detailKind: "ban" as const,
            date: c.created_at,
          }));
      } else if (form.target_type === "comment") {
        const data = await api.get<{ comments?: MyHiddenCommentRow[] }>(
          `/api/v1/users/me/comments?status=hidden&page=1&page_size=100`
        );
        events = (data.comments ?? []).map((cm) => ({
          id: cm.id,
          title: cm.body,
          detail: cm.content_title,
          detailKind: "parent" as const,
          date: cm.created_at,
        }));
      }
      setPickerItems(events);
    } catch (e) {
      silentError(e, { component: 'AppealsPage', action: 'loadPickerEvents' });
      setPickerError(t(getUserFacingErrorKey(e, "appeals.loadFailed")));
    } finally {
      setPickerLoading(false);
    }
  }

  function openPicker() {
    setPickerOpen(true);
    void loadPickerEvents();
  }

  function chooseEvent(event: PickerEvent) {
    setForm((f) => ({ ...f, target_id: event.id }));
    setSelectedEvent(event);
    setPickerOpen(false);
  }

  function changeTargetType(value: string) {
    // 类型切换后旧 target_id 跨类型无意义，清空重选；account 免填。
    setForm((f) => ({ ...f, target_type: value, target_id: value === "account" ? null : f.target_type === value ? f.target_id : null }));
    if (value !== form.target_type) setSelectedEvent(null);
  }

  async function submitAppeal() {
    // T29（FIX-15）：account 申诉免填 target_id（服务端强制为本人）。
    // #845/A3：content/comment 的 target_id 只来自事件弹窗或深链预填，
    // 无任何手输路径（后端只认本人资产，手输是 403/404 死路）。
    const isAccount = form.target_type === "account";
    if (!form.reason || (!isAccount && form.target_id == null)) return;
    setSubmitting(true);
    try {
      await api.post(
        "/api/v1/appeals",
        isAccount
          ? { target_type: form.target_type, reason: form.reason }
          : {
              target_type: form.target_type,
              target_id: form.target_id,
              reason: form.reason,
            }
      );
      setShowForm(false);
      setForm({ target_type: "content", target_id: null, reason: "" });
      setSelectedEvent(null);
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
              {/* #845/A3：裸 select 换共享 Select（appearance-none + 统一箭头）。 */}
              <Select
                value={form.target_type}
                onChange={(e) => changeTargetType(e.target.value)}
                aria-label={t('appeals.typeLabel')}
              >
                <option value="content">{t('appeals.typeContent')}</option>
                <option value="comment">{t('appeals.typeComment')}</option>
                <option value="account">{t('appeals.typeAccount')}</option>
              </Select>
              {form.target_type !== "account" && (
                <div className="space-y-2">
                  {selectedEvent ? (
                    <div className="rounded-md border border-border bg-background p-3">
                      <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0">
                          <p className="line-clamp-2 text-sm font-medium">{selectedEvent.title}</p>
                          {selectedEvent.detail && (
                            <p className={`mt-1 line-clamp-2 text-xs ${selectedEvent.detailKind === "ban" ? "text-destructive" : "text-muted-foreground"}`}>
                              {selectedEvent.detailKind === "ban"
                                ? `${t('appeals.banReasonLabel')}：${selectedEvent.detail}`
                                : `${t('appeals.commentOn')}：${selectedEvent.detail}`}
                            </p>
                          )}
                          <p className="mt-1 text-xs text-muted-foreground">
                            {new Date(selectedEvent.date).toLocaleString(locale === "en" ? "en-US" : "zh-CN")}
                          </p>
                        </div>
                        <Button variant="outline" size="sm" onClick={openPicker}>
                          {t('appeals.pickerChange')}
                        </Button>
                      </div>
                    </div>
                  ) : form.target_id != null ? (
                    // 深链预填（studio「去申诉」）：直接带出目标 ID，可重选。
                    <div className="flex items-center justify-between gap-2 rounded-md border border-border bg-background p-3">
                      <p className="text-sm font-medium">
                        {form.target_type === "content" ? t('appeals.typeContent') : t('appeals.typeComment')} #{form.target_id}
                      </p>
                      <Button variant="outline" size="sm" onClick={openPicker}>
                        {t('appeals.pickerChange')}
                      </Button>
                    </div>
                  ) : (
                    <Button variant="outline" size="sm" className="w-full" onClick={openPicker}>
                      {t('appeals.pickerOpen')}
                    </Button>
                  )}
                </div>
              )}
              {form.target_type === "account" && (
                <p className="text-xs text-muted-foreground">{t('appeals.accountHint')}</p>
              )}
              <textarea
                placeholder={t('appeals.reason')}
                value={form.reason}
                onChange={(e) => setForm((f) => ({ ...f, reason: e.target.value }))}
                rows={3}
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:border-border-strong focus:outline-none"
              />
              <div className="flex gap-2">
                <Button
                  size="sm"
                  disabled={submitting || !form.reason.trim() || (form.target_type !== "account" && form.target_id == null)}
                  onClick={() => void submitAppeal()}
                >
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
                {/* T31（FIX-27）：admin 处理意见对申诉人可见。 */}
                {a.status !== "pending" && a.admin_response && (
                  <p className="mt-2 rounded-md border border-border bg-background p-2 text-sm">
                    <span className="text-muted-foreground">{t('appeals.adminResponse')}：</span>
                    {a.admin_response}
                  </p>
                )}
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

      {/* #845/A3：「选择近期事件」弹窗——content 列被下架内容、comment 列被隐藏
          评论，选中即填 target_type/target_id，无手输路径。 */}
      {pickerOpen && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label={t('appeals.pickerTitle')}
          className="fixed inset-0 z-50 flex items-center justify-center bg-foreground/40 p-4"
          onClick={(e) => { if (e.target === e.currentTarget) setPickerOpen(false); }}
        >
          <div className="flex max-h-[80vh] w-full max-w-md flex-col rounded-lg border border-border bg-card p-5 shadow-md">
            <h2 className="text-base font-semibold text-foreground">{t('appeals.pickerTitle')}</h2>
            <p className="mt-1 text-xs text-muted-foreground">{t('appeals.pickerHint')}</p>
            <div className="mt-3 min-h-0 flex-1 space-y-2 overflow-y-auto">
              {pickerLoading ? (
                <div className="space-y-2">
                  {[1, 2, 3].map((i) => <Skeleton key={i} className="h-16 w-full" />)}
                </div>
              ) : pickerError ? (
                <div className="py-8 text-center">
                  <p className="text-sm text-muted-foreground">{pickerError}</p>
                  <Button variant="outline" size="sm" className="mt-2" onClick={() => void loadPickerEvents()}>
                    {t('common.retry')}
                  </Button>
                </div>
              ) : pickerItems.length === 0 ? (
                <EmptyState
                  icon={FileText}
                  title={t('appeals.pickerEmpty')}
                  description={t('appeals.pickerEmptyHint')}
                />
              ) : (
                pickerItems.map((event) => (
                  <button
                    type="button"
                    key={event.id}
                    onClick={() => chooseEvent(event)}
                    className="w-full rounded-md border border-border bg-background p-3 text-left transition-colors hover:border-border-strong focus:border-border-strong focus:outline-none"
                  >
                    <span className="line-clamp-2 text-sm font-medium text-foreground">{event.title}</span>
                    {event.detail && (
                      <span className={`mt-1 line-clamp-2 block text-xs ${event.detailKind === "ban" ? "text-destructive" : "text-muted-foreground"}`}>
                        {event.detailKind === "ban"
                          ? `${t('appeals.banReasonLabel')}：${event.detail}`
                          : `${t('appeals.commentOn')}：${event.detail}`}
                      </span>
                    )}
                    <span className="mt-1 block text-xs text-muted-foreground">
                      {new Date(event.date).toLocaleString(locale === "en" ? "en-US" : "zh-CN")}
                    </span>
                  </button>
                ))
              )}
            </div>
            <div className="mt-4 flex justify-end">
              <Button variant="outline" size="sm" onClick={() => setPickerOpen(false)}>
                {t('appeals.cancel')}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
