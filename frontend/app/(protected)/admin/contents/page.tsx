"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { ShieldCheck } from "lucide-react";
import Link from "next/link";

interface ContentItem {
  id: number;
  title: string;
  content_type: string;
  zone: string;
  author_id: number;
  status: string;
  report_count?: number;
  view_count: number;
  created_at: string;
}

export default function AdminContentsPage() {
  const t = useTranslations();
  const [contents, setContents] = useState<ContentItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmTarget, setConfirmTarget] = useState<ContentItem | null>(null);

  const pageSize = 20;

  const loadContents = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await api.get<{ contents: ContentItem[]; total: number }>(
        `/api/v1/admin/contents?page=${page}&page_size=${pageSize}`
      );
      setContents(data.contents || []);
      setTotal(data.total || 0);
    } catch (e) {
      silentError(e, { component: 'AdminContentsPage', action: 'loadContents' });
      setError(t(getUserFacingErrorKey(e, "admin.contents.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [page, t]);

  useEffect(() => {
    void loadContents();
  }, [loadContents]);

  async function banContent(id: number) {
    setBusy(true);
    setError("");
    try {
      await api.post(`/api/v1/admin/contents/${id}/ban`, {});
      setContents((prev) => prev.filter((c) => c.id !== id));
      setTotal((t) => t - 1);
    } catch (e) {
      silentError(e, { component: 'AdminContentsPage', action: 'banContent' });
      setError(t(getUserFacingErrorKey(e, "admin.contents.banFailed")));
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
            <Skeleton key={i} className="h-8 w-full" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4 ">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('admin.contents.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t('admin.contents.subtitle', { total })}
          </p>
        </div>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {contents.length === 0 ? (
        <EmptyState
          icon={ShieldCheck}
          title={t("admin.contents.noContents")}
          description={t("admin.contents.noContentsHint")}
          className="p-12"
        />
      ) : (
        <>
          <div className="overflow-x-auto rounded-md border border-border bg-card ">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border bg-muted/30 text-xs text-muted-foreground">
                <tr>
                  <th className="px-4 py-3 font-medium">{t('admin.contents.colTitle')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.contents.colType')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.contents.colZone')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.contents.colAuthor')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.contents.colViews')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.contents.colStatus')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.contents.colActions')}</th>
                </tr>
              </thead>
              <tbody>
                {contents.map((c) => (
                  <tr key={c.id} className="border-b border-border hover:bg-muted/20">
                    <td className="max-w-[200px] truncate px-4 py-3 font-medium">{c.title}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{c.content_type}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{c.zone}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{c.author_id}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{c.view_count}</td>
                    <td className="px-4 py-3">
                      <span className="rounded bg-amber-50 px-2 py-0.5 text-xs text-amber-700">
                        {c.status}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex gap-2">
                        <Link href={`/content/${c.id}`} target="_blank">
                          <Button size="sm" variant="outline">{t('common.view')}</Button>
                        </Link>
                        <Button
                          size="sm"
                          variant="destructive"
                          disabled={busy}
                          onClick={() => {
                            setConfirmTarget(c);
                            setConfirmOpen(true);
                          }}
                        >
                          {t('admin.contents.ban')}
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
        onOpenChange={(v) => { setConfirmOpen(v); if (!v) setConfirmTarget(null); }}
        title={t('admin.contents.banTitle')}
        description={confirmTarget ? t('admin.contents.banConfirm', { title: confirmTarget.title }) : ""}
        confirmLabel={t('admin.contents.confirmBan')}
        confirmVariant="destructive"
        requireReason
        reasonLabel={t('admin.contents.banReason')}
        onConfirm={async (_reason) => {
          if (confirmTarget) {
            await banContent(confirmTarget.id);
          }
        }}
      />
    </div>
  );
}
