"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import Link from "next/link";

interface ContentItem {
  ID: number;
  Title: string;
  ContentType: string;
  Status: string;
  ViewCount: number;
  LikeCount: number;
  CreatedAt: string;
}

export default function DashboardContentsPage() {
  const t = useTranslations();
  const { user } = useAuth();
  const [contents, setContents] = useState<ContentItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!user) return;
    void loadContents();
  }, [user]);

  async function loadContents() {
    setError("");
    setLoading(true);
    try {
      const data = await api.get<{ contents?: ContentItem[] }>(
        `/api/v1/contents?author_id=${user!.id}&page=1&page_size=50&sort=newest&time_range=all`
      );
      setContents(data.contents || []);
    } catch (e) {
      setError(e instanceof ApiRequestError ? `${e.code}: ${e.message}` : t('dashboard.content.loadFailed'));
    } finally {
      setLoading(false);
    }
  }

  async function deleteContent(id: number) {
    if (!window.confirm(t('dashboard.content.deleteConfirm'))) return;
    setBusy(true);
    try {
      await api.delete(`/api/v1/contents/${id}`);
      setContents((prev) => prev.filter((c) => c.ID !== id));
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t('dashboard.content.deleteFailed'));
    } finally {
      setBusy(false);
    }
  }

  if (loading) {
    return <div className="mx-auto w-full max-w-6xl px-4 py-6 text-sm text-muted-foreground">{t('common.loading')}</div>;
  }

  return (
    <div className="mx-auto w-full max-w-6xl space-y-6 px-4 py-6">
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4 ">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('dashboard.content.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t('dashboard.content.count', { count: contents.length })}</p>
        </div>
        <Link href="/publish">
          <Button size="sm">{t('dashboard.content.publishNew')}</Button>
        </Link>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {contents.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center ">
          <p className="text-sm text-muted-foreground">{t('dashboard.content.noContent')}</p>
          <Link href="/publish" className="mt-3 inline-block text-sm text-accent underline underline-offset-4">
            {t('dashboard.content.publishFirst')}
          </Link>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border bg-card ">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border bg-muted/30 text-xs text-muted-foreground">
              <tr>
                <th className="px-4 py-3 font-medium">{t('dashboard.content.colTitle')}</th>
                <th className="px-4 py-3 font-medium">{t('dashboard.content.colType')}</th>
                <th className="px-4 py-3 font-medium">{t('dashboard.content.colStatus')}</th>
                <th className="px-4 py-3 font-medium">{t('dashboard.content.colViews')}</th>
                <th className="px-4 py-3 font-medium">{t('dashboard.content.colLikes')}</th>
                <th className="px-4 py-3 font-medium">{t('dashboard.content.colActions')}</th>
              </tr>
            </thead>
            <tbody>
              {contents.map((c) => (
                <tr key={c.ID} className="border-b border-border hover:bg-muted/20">
                  <td className="px-4 py-3 font-medium">{c.Title}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{c.ContentType}</td>
                  <td className="px-4 py-3">
                    <span className={`rounded px-2 py-0.5 text-xs ${
                      c.Status === "published" ? "bg-emerald-50 text-emerald-700" :
                      c.Status === "under_review" ? "bg-amber-50 text-amber-700" :
                      "bg-muted text-muted-foreground"
                    }`}>{c.Status}</span>
                  </td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{c.ViewCount}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{c.LikeCount}</td>
                  <td className="px-4 py-3">
                    <div className="flex gap-2">
                      <Link href={`/content/${c.ID}`}>
                        <Button size="sm" variant="outline">{t('common.view')}</Button>
                      </Link>
                      <Button size="sm" variant="outline" disabled={busy} onClick={() => void deleteContent(c.ID)}>
                        {t('dashboard.content.delete')}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
