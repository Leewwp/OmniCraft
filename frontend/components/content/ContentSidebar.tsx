"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { ArrowRight, GitBranchPlus, User } from "lucide-react";
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
          <div className="rounded-xl border border-border/60 bg-card p-5 transition-colors hover:border-border">
            <div className="flex justify-center">
              <div className="flex h-14 w-14 items-center justify-center rounded-full bg-[var(--accent-subtle)] text-lg font-bold text-[var(--accent-emphasis)]">
                {(author.username || "?").slice(0, 2).toUpperCase()}
              </div>
            </div>
            <Link
              href={author.id ? `/user/${author.id}` : "#"}
              className="mt-3 block text-center text-[15px] font-bold text-foreground hover:text-[var(--accent-emphasis)] transition-colors"
            >
              {author.username || t("common.unknown")}
            </Link>
            {author.bio && (
              <p className="mt-1.5 text-center text-[12.5px] leading-relaxed text-muted-foreground line-clamp-2">
                {author.bio}
              </p>
            )}
            {authorStats && (
              <div className="mt-3 flex justify-center gap-5 border-t border-border/50 pt-3">
                <div className="text-center">
                  <div className="text-[15px] font-bold text-foreground">{authorStats.contents}</div>
                  <div className="text-[11px] text-muted-foreground">内容</div>
                </div>
                <div className="text-center">
                  <div className="text-[15px] font-bold text-foreground">{authorStats.followers.toLocaleString()}</div>
                  <div className="text-[11px] text-muted-foreground">粉丝</div>
                </div>
                <div className="text-center">
                  <div className="text-[15px] font-bold text-foreground">{authorStats.likes}</div>
                  <div className="text-[11px] text-muted-foreground">获赞</div>
                </div>
              </div>
            )}
            <Button
              variant={isFollowing ? "outline" : "default"}
              size="sm"
              className="mt-3 w-full rounded-full"
            >
              {isFollowing ? "已关注" : "关注作者"}
            </Button>
          </div>
        )}

        {/* Source Original Card — fanwork only */}
        {isFanwork && sourceOriginal && (
          <div className="rounded-xl border border-border/60 bg-card p-5 transition-colors hover:border-border">
            <div className="mb-3 text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
              来源原创
            </div>
            <Link
              href={`/original/${sourceOriginal.id}`}
              className="group flex items-center gap-3 rounded-lg border border-sky-100 bg-sky-50/50 p-3 transition-all hover:border-sky-200 hover:bg-sky-50"
            >
              <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-sky-100 to-blue-100 text-sky-600">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/></svg>
              </div>
              <div className="min-w-0 flex-1">
                <div className="truncate text-[13px] font-semibold text-foreground">{sourceOriginal.title}</div>
                <div className="text-[11px] text-muted-foreground">查看原创内容</div>
              </div>
              <ArrowRight className="h-4 w-4 flex-shrink-0 text-sky-500 transition-transform group-hover:translate-x-0.5" />
            </Link>
          </div>
        )}

        {/* IP Card — fanwork only */}
        {isFanwork && ip?.name && (
          <div className="rounded-xl border border-border/60 bg-card p-5 transition-colors hover:border-border">
            <div className="mb-3 text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
              关联 IP
            </div>
            <div className="flex items-center gap-3">
              <div className="flex h-11 h-11 w-11 flex-shrink-0 items-center justify-center rounded-xl bg-muted text-base font-bold text-muted-foreground">
                {ip.name.slice(0, 2)}
              </div>
              <div className="min-w-0">
                <div className="truncate text-[14px] font-semibold text-foreground">{ip.name}</div>
                {ipContentCount !== undefined && (
                  <div className="text-[12px] text-muted-foreground">
                    {ipContentCount.toLocaleString()} 内容
                  </div>
                )}
              </div>
            </div>
            {ip.id && (
              <Link
                href={`/ip/${ip.id}`}
                className="mt-3 inline-flex items-center gap-1.5 text-[12.5px] font-medium text-[var(--accent-emphasis)] hover:gap-2 transition-all"
              >
                进入 IP 详情 <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            )}
          </div>
        )}

        {/* Related Fanworks — original only */}
        {isOriginal && originalId && (relatedFanworksCount ?? 0) > 0 && (
          <div className="rounded-xl border border-border/60 bg-card p-5 transition-colors hover:border-border">
            <div className="mb-3 text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
              关联二创
            </div>
            <div className="mb-3 text-[13px] font-medium text-violet-600">
              {relatedFanworksCount} 个相关二创
            </div>
            <div className="flex flex-col gap-2">
              <Link
                href={`/original/${originalId}/fanworks`}
                className="inline-flex items-center justify-center gap-1.5 rounded-full border border-violet-200 bg-violet-50 px-4 py-2 text-[12.5px] font-medium text-violet-700 transition-all hover:bg-violet-100"
              >
                查看全部 <ArrowRight className="h-3.5 w-3.5" />
              </Link>
              <Link
                href={`/studio/publish/fanwork?source_original_id=${originalId}`}
                className="inline-flex items-center justify-center gap-1.5 rounded-full bg-violet-600 px-4 py-2 text-[12.5px] font-medium text-white transition-all hover:bg-violet-700"
              >
                <GitBranchPlus className="h-3.5 w-3.5" />
                基于此文创作二创
              </Link>
            </div>
          </div>
        )}
      </div>
    </aside>
  );
}
