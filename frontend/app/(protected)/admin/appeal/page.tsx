"use client";

import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";

interface AppealItem {
  id: number;
  user_id: number;
  target_type: string;
  target_id: number;
  reason: string;
  status: string;
  admin_response: string;
  created_at: string;
}

export default function AdminAppealPage() {
  const t = useTranslations();
  const [appeals, setAppeals] = useState<AppealItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmAction, setConfirmAction] = useState<{
    appealId: number;
    action: "approved" | "rejected";
    targetInfo: string;
  } | null>(null);

  const pageSize = 20;
  // T31（FIX-27）：status 筛选（默认 pending 保持历史行为；all 全量）。
  const [statusFilter, setStatusFilter] = useState<"pending" | "approved" | "rejected" | "all">("pending");

  const loadAppeals = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await api.get<{ appeals: AppealItem[]; total: number }>(
        `/api/v1/admin/appeals?page=${page}&page_size=${pageSize}&status=${statusFilter}`
      );
      setAppeals(data.appeals || []);
      setTotal(data.total || 0);
    } catch (e) {
      silentError(e, { component: 'AdminAppealPage', action: 'loadAppeals' });
      setError(t(getUserFacingErrorKey(e, "admin.appeals.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [page, statusFilter, t]);

  useEffect(() => {
    void loadAppeals();
  }, [loadAppeals]);

  async function resolveAppeal(id: number, status: string, response: string) {
    setBusy(true);
    setError("");
    try {
      await api.post(`/api/v1/admin/appeals/${id}`, {
        status,
        admin_response: response,
      });
      // T31（FIX-27）：status 筛选下处理后重拉列表（pending 视图行自然消失，
      // approved/rejected 视图行刷新为已处理态）。
      await loadAppeals();
    } catch (e) {
      silentError(e, { component: 'AdminAppealPage', action: 'resolveAppeal' });
      setError(t(getUserFacingErrorKey(e, "admin.appeals.processFailed")));
    } finally {
      setBusy(false);
    }
  }

  const totalPages = Math.ceil(total / pageSize);

  if (loading) {
    return (
      <div className="space-y-4 p-6">
        <div className="space-y-3 rounded-md border border-border bg-card p-6 ">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-8 w-full animate-pulse rounded bg-muted" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4 ">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('admin.appeals.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t('admin.appeals.subtitle', { total })}
          </p>
        </div>
        <select
          value={statusFilter}
          onChange={(e) => {
            setPage(1);
            setStatusFilter(e.target.value as typeof statusFilter);
          }}
          aria-label={t('admin.appeals.statusFilter')}
          className="rounded-md border border-border bg-background px-3 py-2 text-sm"
        >
          <option value="pending">{t('admin.appeals.statusPending')}</option>
          <option value="approved">{t('admin.appeals.statusApproved')}</option>
          <option value="rejected">{t('admin.appeals.statusRejected')}</option>
          <option value="all">{t('admin.appeals.statusAll')}</option>
        </select>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {appeals.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center ">
          <p className="text-sm text-muted-foreground">{t('admin.appeals.noAppeals')}</p>
        </div>
      ) : (
        <>
          <div className="overflow-x-auto rounded-md border border-border bg-card ">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border bg-muted/30 text-xs text-muted-foreground">
                <tr>
                  <th className="px-4 py-3 font-medium">ID</th>
                  <th className="px-4 py-3 font-medium">{t('admin.appeals.colApplicant')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.appeals.colTargetType')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.appeals.colTargetId')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.appeals.colReason')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.appeals.colStatus')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.appeals.colActions')}</th>
                </tr>
              </thead>
              <tbody>
                {appeals.map((a) => (
                  <tr key={a.id} className="border-b border-border hover:bg-muted/20">
                    <td className="px-4 py-3 text-xs text-muted-foreground">{a.id}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{a.user_id}</td>
                    <td className="px-4 py-3 text-xs">
                      <span
                        className={`rounded-full px-2 py-0.5 text-xs ${
                          a.target_type === "content"
                            ? "bg-blue-50 text-blue-700"
                            : "bg-purple-50 text-purple-700"
                        }`}
                      >
                        {a.target_type}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">
                      {a.target_id}
                      {/* T31（FIX-27）：content 申诉行内直达预览。 */}
                      {a.target_type === "content" && (
                        <Link
                          href={`/content/${a.target_id}`}
                          className="ml-2 text-primary hover:underline"
                        >
                          {t('admin.appeals.viewContent')}
                        </Link>
                      )}
                    </td>
                    <td className="max-w-[200px] truncate px-4 py-3 text-xs text-muted-foreground">
                      {a.reason}
                    </td>
                    <td className="px-4 py-3">
                      <span className="rounded-full bg-amber-50 px-2 py-0.5 text-xs text-amber-700">
                        {a.status}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex gap-2">
                        <Button
                          size="sm"
                          variant="outline"
                          className="text-emerald-600 hover:bg-emerald-50"
                          disabled={busy}
                          onClick={() => {
                            setConfirmAction({
                              appealId: a.id,
                              action: "approved",
                              targetInfo: `${a.target_type} #${a.target_id}`,
                            });
                            setConfirmOpen(true);
                          }}
                        >
                          {t('admin.appeals.approve')}
                        </Button>
                        <Button
                          size="sm"
                          variant="destructive"
                          disabled={busy}
                          onClick={() => {
                            setConfirmAction({
                              appealId: a.id,
                              action: "rejected",
                              targetInfo: `${a.target_type} #${a.target_id}`,
                            });
                            setConfirmOpen(true);
                          }}
                        >
                          {t('admin.appeals.reject')}
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-between">
              <span className="text-xs text-muted-foreground">
                {t('common.page', { current: page, total: totalPages })}
              </span>
              <div className="flex gap-2">
                <Button size="sm" variant="outline" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                  {t('common.previous')}
                </Button>
                <Button size="sm" variant="outline" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
                  {t('common.next')}
                </Button>
              </div>
            </div>
          )}
        </>
      )}

      <ConfirmModal
        open={confirmOpen}
        onOpenChange={(v) => { setConfirmOpen(v); if (!v) setConfirmAction(null); }}
        title={confirmAction?.action === "approved" ? t('admin.appeals.approveTitle') : t('admin.appeals.rejectTitle')}
        description={
          confirmAction
            ? confirmAction.action === "approved"
              ? t('admin.appeals.approveConfirm', { name: confirmAction.targetInfo })
              : t('admin.appeals.rejectConfirm', { name: confirmAction.targetInfo })
            : ""
        }
        confirmLabel={confirmAction?.action === "approved" ? t('admin.appeals.confirmApprove') : t('admin.appeals.confirmReject')}
        confirmVariant={confirmAction?.action === "approved" ? "default" : "destructive"}
        requireReason
        reasonLabel={t('admin.appeals.opinion')}
        onConfirm={async (reason) => {
          if (confirmAction) {
            await resolveAppeal(confirmAction.appealId, confirmAction.action, reason);
          }
        }}
      />
    </div>
  );
}
