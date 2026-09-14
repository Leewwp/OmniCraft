"use client";

import { useTranslations } from "next-intl";
import type { ReactNode } from "react";

/* 个人主页头部信息卡（#508，SP-18 修复轮）：与 UserHoverCard 信息同步——
   真实头像（avatar_url，缺省首字母兜底）+ 三项统计（内容/获赞/粉丝）。
   纯展示组件（服务端 page 渲染），stats/meta 由调用方组装。 */

interface ProfileSummaryCardProps {
  displayName: string;
  avatarUrl?: string;
  bio: string;
  meta: ReactNode;
  stats: { contents: number; likes: number; followers: number };
}

export function ProfileSummaryCard({ displayName, avatarUrl, bio, meta, stats }: ProfileSummaryCardProps) {
  const t = useTranslations();

  return (
    <div className="rounded-md border border-border bg-card p-6">
      <div className="flex items-start gap-4">
        {avatarUrl ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={avatarUrl}
            alt=""
            className="h-16 w-16 shrink-0 rounded-full bg-muted object-cover"
          />
        ) : (
          <div className="flex h-16 w-16 shrink-0 items-center justify-center rounded-full bg-muted text-xl font-bold text-muted-foreground">
            {displayName.slice(0, 1)}
          </div>
        )}
        <div className="min-w-0 space-y-1">
          <h1 className="text-2xl font-bold tracking-tight">{displayName}</h1>
          <p className="text-sm text-muted-foreground">{meta}</p>
          {bio && <p className="text-sm text-foreground/80">{bio}</p>}
          <p className="flex items-center gap-4 pt-1 text-xs text-muted-foreground">
            <span>
              <strong className="font-semibold text-foreground">{stats.contents}</strong>{" "}
              {t("user.statContents")}
            </span>
            <span>
              <strong className="font-semibold text-foreground">{stats.likes}</strong>{" "}
              {t("user.statLikes")}
            </span>
            <span>
              <strong className="font-semibold text-foreground">{stats.followers}</strong>{" "}
              {t("user.statFollowers")}
            </span>
          </p>
        </div>
      </div>
    </div>
  );
}
