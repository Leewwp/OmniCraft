"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { ArrowRight, FileText, GitBranchPlus } from "lucide-react";
import { Button } from "@/components/ui/button";

interface AuthorInfo {
  id?: number;
  username?: string;
  avatar_url?: string | null;
  bio?: string;
}

interface ContentSidebarProps {
  author?: AuthorInfo;
  authorStats?: {
    contents: number;
    followers: number;
    likes: number;
  };
  isFollowing?: boolean;
  ip?: { id?: number; name?: string; slug?: string };
  ipContentCount?: number;
  sourceOriginal?: { id: number; title: string } | null;
  relatedFanworksCount?: number;
  originalId?: number;
  zone?: string;
}

export function ContentSidebar({
  author,
  authorStats,
  isFollowing,
  ip,
  ipContentCount,
  sourceOriginal,
  relatedFanworksCount,
  originalId,
  zone,
}: ContentSidebarProps) {
  const t = useTranslations();
  const isFanwork = zone === "fanwork";
  const isOriginal = zone === "original";

  return (
    <aside className="w-[280px] flex-shrink-0 hidden lg:block">
      <div className="sticky top-[68px] space-y-4">

        {/* Author Card */}
        {author && (
          <div className="rounded-lg border border-border/60 bg-card p-5 shadow-[var(--elevation-1)] transition-[border-color,box-shadow] hover:border-border hover:shadow-[var(--elevation-2)]">
            <div className="flex justify-center">
              <div className="flex h-14 w-14 items-center justify-center rounded-full bg-[var(--accent-subtle)] text-lg font-bold text-[var(--accent-emphasis)]">
                {(author.username || "?").slice(0, 2).toUpperCase()}
              </div>
            </div>
            <Link
              href={author.id ? `/user/${author.id}` : "#"}
              className="mt-3 block text-center text-sm font-bold text-foreground transition-colors hover:text-[var(--accent-emphasis)]"
            >
              {author.username || t("common.unknown")}
            </Link>
            {author.bio && (
              <p className="mt-1.5 line-clamp-2 text-center text-xs leading-relaxed text-muted-foreground">
                {author.bio}
              </p>
            )}
            {authorStats && (
              <div className="mt-3 flex justify-center gap-5 border-t border-border/50 pt-3">
                <div className="text-center">
                  <div className="text-sm font-bold text-foreground">{authorStats.contents}</div>
                  <div className="text-xs text-muted-foreground">{t('home.contentCountLabel')}</div>
                </div>
                <div className="text-center">
                  <div className="text-sm font-bold text-foreground">{authorStats.followers.toLocaleString()}</div>
                  <div className="text-xs text-muted-foreground">{t('studio.overview.followers')}</div>
                </div>
                <div className="text-center">
                  <div className="text-sm font-bold text-foreground">{authorStats.likes}</div>
                  <div className="text-xs text-muted-foreground">{t('studio.overview.totalLikes')}</div>
                </div>
              </div>
            )}
            <Button
              variant={isFollowing ? "outline" : "default"}
              size="sm"
              className="mt-3 w-full rounded-full"
            >
              {isFollowing ? t('social.following') : t('social.follow')}
            </Button>
          </div>
        )}

        {/* IP Card — fanwork only (primary) */}
        {isFanwork && ip?.name && (
          <div className="rounded-lg border border-border/60 bg-card p-5 shadow-[var(--elevation-1)] transition-[border-color,box-shadow] hover:border-border hover:shadow-[var(--elevation-2)]">
            <div className="mb-3 text-xs font-bold uppercase tracking-wider text-muted-foreground">
              {t('publish.linkIp')}
            </div>
            <div className="flex items-center gap-3">
              <div className="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-lg bg-muted text-base font-bold text-muted-foreground">
                {ip.name.slice(0, 2)}
              </div>
              <div className="min-w-0">
                <div className="truncate text-sm font-semibold text-foreground">{ip.name}</div>
                {ipContentCount !== undefined && (
                  <div className="text-xs text-muted-foreground">
                    {t('ip.contentCount', { count: ipContentCount })}
                  </div>
                )}
              </div>
            </div>
            {ip.id && (
              <Link
                href={`/ip/${ip.id}`}
                className="mt-3 inline-flex items-center gap-1.5 text-xs font-medium text-[var(--accent-emphasis)] transition-colors hover:text-accent-hover motion-reduce:transition-none"
              >
                {t('ip.enterDetail')} <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            )}

            {/* Source original — subtle secondary link */}
            {sourceOriginal && sourceOriginal.title && (
              <div className="mt-3 border-t border-border/50 pt-3">
                <Link
                  href={`/original/${sourceOriginal.id}`}
                  className="inline-flex items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-accent-emphasis"
                >
                  <FileText className="h-3.5 w-3.5" aria-hidden="true" />
                  {t('content.sourceOriginalLabel')}：{sourceOriginal.title.slice(0, 20)}{sourceOriginal.title.length > 20 ? "…" : ""}
                </Link>
              </div>
            )}
          </div>
        )}

        {/* Related Fanworks — original only */}
        {isOriginal && originalId && (relatedFanworksCount ?? 0) > 0 && (
          <div className="rounded-lg border border-border/60 bg-card p-5 shadow-[var(--elevation-1)] transition-[border-color,box-shadow] hover:border-border hover:shadow-[var(--elevation-2)]">
            <div className="mb-3 text-xs font-bold uppercase tracking-wider text-muted-foreground">
              {t('content.relatedFanworks')}
            </div>
            <div className="mb-3 text-sm font-medium text-accent-emphasis">
              {relatedFanworksCount} {t('content.relatedFanworks')}
            </div>
            <div className="flex flex-col gap-2">
              <Link
                href={`/original/${originalId}/fanworks`}
                className="inline-flex items-center justify-center gap-1.5 rounded-full border border-border bg-accent-subtle px-4 py-2 text-xs font-medium text-accent-emphasis transition-colors hover:border-border-strong hover:bg-muted"
              >
                {t('common.clickToView')} <ArrowRight className="h-3.5 w-3.5" />
              </Link>
              <Link
                href={`/studio/publish/fanwork?source_original_id=${originalId}`}
                className="inline-flex items-center justify-center gap-1.5 rounded-full bg-primary px-4 py-2 text-xs font-medium text-primary-foreground transition-all hover:bg-primary/90"
              >
                <GitBranchPlus className="h-3.5 w-3.5" />
                {t('content.createFanwork')}
              </Link>
            </div>
          </div>
        )}
      </div>
    </aside>
  );
}
