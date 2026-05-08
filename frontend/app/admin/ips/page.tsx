"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";

interface IPItem {
  id: number;
  name: string;
  slug: string;
  category: string;
  description: string;
  cover_image_url: string;
  status: string;
  submitter_id: number;
  created_at: string;
}

export default function AdminIPsPage() {
  const t = useTranslations();
  const [ips, setIps] = useState<IPItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmAction, setConfirmAction] = useState<{
    ipId: number;
    action: "approve" | "reject";
    title: string;
  } | null>(null);

  const pageSize = 20;

  const loadIPs = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await api.get<{ ips: IPItem[]; total: number }>(
        `/api/v1/admin/ips?page=${page}&page_size=${pageSize}`
      );
      setIps(data.ips || []);
      setTotal(data.total || 0);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t('admin.ips.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [page, t]);

  useEffect(() => {
    void loadIPs();
  }, [loadIPs]);

  async function handleAction(ipId: number, action: "approve" | "reject") {
    setError("");
    try {
      await api.post(`/api/v1/admin/ips/${ipId}/${action}`, {});
      setIps((prev) => prev.filter((ip) => ip.id !== ipId));
      setTotal((t) => t - 1);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t('common.operationFailed'));
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
          <h1 className="text-2xl font-bold tracking-tight">{t('admin.ips.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t('admin.ips.subtitle', { total })}
          </p>
        </div>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {ips.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center ">
          <p className="text-sm text-muted-foreground">{t('admin.ips.noIps')}</p>
        </div>
      ) : (
        <>
          <div className="overflow-x-auto rounded-md border border-border bg-card ">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border bg-muted/30 text-xs text-muted-foreground">
                <tr>
                  <th className="px-4 py-3 font-medium">{t('admin.ips.colName')}</th>
                  <th className="px-4 py-3 font-medium">Slug</th>
                  <th className="px-4 py-3 font-medium">{t('admin.ips.colCategory')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.ips.colSubmitter')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.ips.colStatus')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.ips.colActions')}</th>
                </tr>
              </thead>
              <tbody>
                {ips.map((ip) => (
                  <tr key={ip.id} className="border-b border-border hover:bg-muted/20">
                    <td className="px-4 py-3 font-medium">{ip.name}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{ip.slug}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{ip.category || "-"}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{ip.submitter_id}</td>
                    <td className="px-4 py-3">
                      <span className="rounded bg-amber-50 px-2 py-0.5 text-xs text-amber-700">
                        {ip.status}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex gap-2">
                        <Button
                          size="sm"
                          variant="outline"
                          className="text-emerald-600 hover:bg-emerald-50"
                          onClick={() => {
                            setConfirmAction({ ipId: ip.id, action: "approve", title: ip.name });
                            setConfirmOpen(true);
                          }}
                        >
                          {t('admin.ips.approve')}
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          className="text-destructive hover:bg-destructive/10"
                          onClick={() => {
                            setConfirmAction({ ipId: ip.id, action: "reject", title: ip.name });
                            setConfirmOpen(true);
                          }}
                        >
                          {t('admin.ips.reject')}
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
        onOpenChange={setConfirmOpen}
        title={confirmAction?.action === "approve" ? t('admin.ips.approveTitle') : t('admin.ips.rejectTitle')}
        description={
          confirmAction?.action === "approve"
            ? t('admin.ips.approveConfirm', { name: confirmAction?.title ?? "" })
            : t('admin.ips.rejectConfirm', { name: confirmAction?.title ?? "" })
        }
        confirmLabel={confirmAction?.action === "approve" ? t('admin.ips.confirmApprove') : t('admin.ips.confirmReject')}
        confirmVariant={confirmAction?.action === "approve" ? "default" : "destructive"}
        requireReason={confirmAction?.action === "reject"}
        reasonLabel={t('admin.ips.rejectReason')}
        onConfirm={async (_reason) => {
          if (confirmAction) {
            await handleAction(confirmAction.ipId, confirmAction.action);
          }
        }}
      />
    </div>
  );
}
