"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { Heart, Eye, Loader2 } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { Skeleton } from "@/components/ui/skeleton";

const CONTENT_TYPES = [
  { key: "", labelKey: "studio.favorites.allTypes" },
  { key: "image", labelKey: "studio.favorites.image" },
  { key: "article", labelKey: "studio.favorites.article" },
  { key: "video", labelKey: "studio.favorites.video" },
  { key: "audio", labelKey: "studio.favorites.audio" },
  { key: "template", labelKey: "studio.favorites.template" },
  { key: "mod", labelKey: "studio.favorites.mod" },
  { key: "sheet_music", labelKey: "studio.favorites.sheetMusic" },
];

interface FavoriteItem {
  id: number;
  title: string;
  content_type: string;
  zone: string;
  cover_url?: string;
  like_count: number;
  view_count: number;
  author?: { username: string };
  created_at: string;
}

export default function StudioFavoritesPage() {
  const t = useTranslations();
  const { user } = useAuth();
  const [contentType, setContentType] = useState("");
  const [favorites, setFavorites] = useState<FavoriteItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const pageSize = 20;

  useEffect(() => {
    if (!user) return;
    setLoading(true);
    const params = new URLSearchParams({
      page: String(page),
      page_size: String(pageSize),
    });
    if (contentType) params.set("content_type", contentType);
    api.get(`/api/v1/users/${user.id}/favorites?${params}`)
      .then((res) => {
        const data = res as Record<string, unknown>;
        setFavorites((data.favorites as FavoriteItem[]) || []);
        setTotal((data.total as number) || 0);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [user, contentType, page]);

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div>
      <h1 className="mb-1 text-xl font-bold text-foreground">{t('studio.favorites.title')}</h1>
      <p className="mb-6 text-sm text-muted-foreground">{t('studio.favorites.subtitle')}</p>

      <div className="mb-4 flex gap-1 flex-wrap">
        {CONTENT_TYPES.map((ct) => (
          <button
            key={ct.key}
            onClick={() => { setContentType(ct.key); setPage(1); }}
            className={`rounded-md px-3 py-1 text-xs transition-colors ${
              contentType === ct.key
                ? "bg-accent/10 text-accent font-medium"
                : "text-muted-foreground hover:bg-muted hover:text-foreground"
            }`}
          >
            {t(ct.labelKey)}
          </button>
        ))}
      </div>

      {loading ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[1, 2, 3, 4, 5, 6].map((i) => (
            <Skeleton key={i} className="h-48 rounded-lg" />
          ))}
        </div>
      ) : favorites.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-12 text-center">
          <Heart className="mx-auto mb-3 h-8 w-8 text-muted-foreground/40" />
          <p className="mb-2 text-sm font-medium text-foreground">{t('studio.favorites.empty')}</p>
          <p className="text-sm text-muted-foreground">{t('studio.favorites.emptyHint')}</p>
          <Link href="/" className="mt-4 inline-block text-sm text-accent hover:underline">
            {t('studio.favorites.discover')}
          </Link>
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {favorites.map((item) => (
              <Link
                key={item.id}
                href={`/content/${item.id}`}
                className="group rounded-md border border-border bg-card transition-colors hover:bg-muted/50"
              >
                <div className="flex flex-col gap-2 p-3">
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium text-foreground line-clamp-1 group-hover:underline">
                      {item.title}
                    </span>
                    <span className="ml-2 shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground uppercase">
                      {item.content_type}
                    </span>
                  </div>
                  <div className="flex items-center gap-3 text-xs text-muted-foreground">
                    {item.author && <span>@{item.author.username}</span>}
                    <span className="flex items-center gap-1">
                      <Heart className="h-3 w-3" /> {item.like_count}
                    </span>
                    <span className="flex items-center gap-1">
                      <Eye className="h-3 w-3" /> {item.view_count}
                    </span>
                  </div>
                </div>
              </Link>
            ))}
          </div>

          {totalPages > 1 && (
            <div className="mt-6 flex items-center justify-center gap-2">
              <button
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page === 1}
                className="rounded-md border border-border px-3 py-1.5 text-sm disabled:opacity-50"
              >
                {t('common.previous')}
              </button>
              <span className="text-sm text-muted-foreground">
                {page} / {totalPages}
              </span>
              <button
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page === totalPages}
                className="rounded-md border border-border px-3 py-1.5 text-sm disabled:opacity-50"
              >
                {t('common.next')}
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}