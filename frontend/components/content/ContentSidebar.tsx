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

export interface RelatedContentEntry {
  id: number;
  zone: "original" | "fanwork";
}

/** 浮层内相关二创/关联内容的可点击行数据（点击压入浮层内部导航栈）。 */
export interface RelatedCardEntry extends RelatedContentEntry {
  title: string;
  meta?: string;
  coverUrl?: string;
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
  onOpenRelated?: (entry: RelatedContentEntry, trigger: HTMLElement) => void;
  /** 浮层模式：相关内容行列表（与 onOpenRelatedItem 配合压栈）。 */
  relatedItems?: RelatedCardEntry[];
  onOpenRelatedItem?: (entry: RelatedCardEntry, trigger: HTMLElement) => void;
  /** 浮层模式：相关卡片标题键（fanwork 用「衍生二创」，original 用「相关二创」）。 */
  relatedItemsLabelKey?: string;
  /** 浮层模式：行列表下方的扩展动作（如「查看全部」）。 */
  relatedFooterAction?: React.ReactNode;
  /** 浮层模式：创作者卡片内的关注动作（FollowButton），缺省渲染静态按钮。 */
  followAction?: React.ReactNode;
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
  onOpenRelated,
  relatedItems,
  onOpenRelatedItem,
  relatedItemsLabelKey,
  relatedFooterAction,
  followAction,
}: ContentSidebarProps) {
  const t = useTranslations();
  const isFanwork = zone === "fanwork";
  const isOriginal = zone === "original";
  const relatedLabelKey = relatedItemsLabelKey ?? "content.relatedFanworks";

  const relatedItemRows = relatedItems && relatedItems.length > 0 ? (
    <div className="space-y-1.5">
      {relatedItems.map((item) => (
        <button
          key={item.id}
          type="button"
          onClick={(event) => onOpenRelatedItem?.(item, event.currentTarget)}
          aria-label={t("contentDetailOverlay.openRelated", { title: item.title })}
          className="flex w-full items-center gap-2.5 rounded-md p-2 text-left transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        >
          {item.coverUrl ? (
            <span className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-md border border-border bg-muted">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={item.coverUrl} alt="" loading="lazy" className="h-full w-full object-cover" />
            </span>
          ) : (
            <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-accent-subtle text-xs font-semibold text-accent-emphasis">
              {item.title.slice(0, 1)}
            </span>
          )}
          <span className="min-w-0">
            <strong className="block truncate text-sm font-medium text-foreground">{item.title}</strong>
            {item.meta && <small className="block truncate text-xs text-muted-foreground">{item.meta}</small>}
          </span>
        </button>
      ))}
    </div>
  ) : null;

  const sourceOriginalTrigger =
    sourceOriginal && sourceOriginal.title ? (
      <div className="mt-3 border-t border-border/50 pt-3">
        {onOpenRelated ? (
          <button
            type="button"
            onClick={(event) =>
              onOpenRelated({ id: sourceOriginal.id, zone: "original" }, event.currentTarget)
            }
            className="inline-flex max-w-full items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-accent-emphasis focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            <FileText className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
            <span className="truncate">
              {t('content.sourceOriginalLabel')}：{sourceOriginal.title.slice(0, 20)}
              {sourceOriginal.title.length > 20 ? "…" : ""}
            </span>
          </button>
        ) : (
          <Link
            href={`/original/${sourceOriginal.id}`}
            className="inline-flex items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-accent-emphasis"
          >
            <FileText className="h-3.5 w-3.5" aria-hidden="true" />
            {t('content.sourceOriginalLabel')}：{sourceOriginal.title.slice(0, 20)}{sourceOriginal.title.length > 20 ? "…" : ""}
          </Link>
        )}
      </div>
    ) : null;

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
            {followAction ? (
              <div className="mt-3">{followAction}</div>
            ) : (
              <Button
                variant={isFollowing ? "outline" : "default"}
                size="sm"
                className="mt-3 w-full rounded-full"
              >
                {isFollowing ? t('social.following') : t('social.follow')}
              </Button>
            )}
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

            {/* Source original — subtle secondary link; overlay host turns it into a floating-detail trigger */}
            {sourceOriginalTrigger}
          </div>
        )}

        {/* Source original outside the IP card — rendered standalone when the API does not
            provide a nested `ip` object (ip_id only), keeping the overlay trigger reachable. */}
        {sourceOriginalTrigger && !(isFanwork && ip?.name) && (
          <div className="rounded-lg border border-border/60 bg-card p-5 shadow-[var(--elevation-1)] transition-[border-color,box-shadow] hover:border-border hover:shadow-[var(--elevation-2)]">
            <div className="mb-3 text-xs font-bold uppercase tracking-wider text-muted-foreground">
              {t('content.sourceOriginalLabel')}
            </div>
            {sourceOriginalTrigger}
          </div>
        )}

        {/* Related Fanworks — original only（浮层模式下二创/原创均可由 relatedItems 驱动） */}
        {(relatedItems && relatedItems.length > 0) ||
        (isOriginal && originalId && (relatedFanworksCount ?? 0) > 0) ? (
          <div className="rounded-lg border border-border/60 bg-card p-5 shadow-[var(--elevation-1)] transition-[border-color,box-shadow] hover:border-border hover:shadow-[var(--elevation-2)]">
            <div className="mb-3 text-xs font-bold uppercase tracking-wider text-muted-foreground">
              {t(relatedLabelKey)}
            </div>
            {relatedItemRows ? (
              <>
                {relatedItemRows}
                {relatedFooterAction}
              </>
            ) : (
              <>
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
              </>
            )}
          </div>
        ) : null}
      </div>
    </aside>
  );
}
