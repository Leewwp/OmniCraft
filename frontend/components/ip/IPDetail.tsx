"use client";

import Link from "next/link";
import Image from "next/image";
import { useTranslations } from "next-intl";
import { MessageSquareText, Users } from "lucide-react";
import { IPCategoryTabs } from "@/components/ip/IPCategoryTabs";
import { FollowButton } from "@/components/social/FollowButton";
import { DiscussionBoard } from "@/components/social/DiscussionBoard";

interface IPItem {
  id: number;
  name: string;
  description?: string;
  category?: string;
  cover_url?: string;
  follower_count?: number;
  is_following?: boolean;
}

interface IPDetailProps {
  ip: IPItem;
}

function getIPCategoryLabel(category?: string): string {
  switch (category) {
    case "text": return "home.text";
    case "image": return "home.image";
    case "video": return "home.video";
    case "audio": return "home.audio";
    case "mod": return "home.mod";
    case "prompt": return "home.aiPrompt";
    case "sheet_music": return "home.sheetMusic";
    case "other": return "home.other";
    default: return "";
  }
}

export function IPDetail({ ip }: IPDetailProps) {
  const t = useTranslations();
  const categoryKey = getIPCategoryLabel(ip.category);
  const categoryLabel = categoryKey ? t(categoryKey) : (ip.category || t('ip.uncategorized'));
  return (
    <section className="space-y-4">
      <div className="rounded-md border border-border bg-card p-4">
        <div className="flex flex-col gap-4 md:flex-row">
          <div className="flex h-36 w-full items-center justify-center rounded-md border border-border bg-muted/40 md:w-52 relative overflow-hidden">
            {ip.cover_url ? (
              <Image src={ip.cover_url} alt={ip.name} fill className="rounded-md object-cover" sizes="208px" />
            ) : (
              <span className="text-sm text-muted-foreground">{t('ip.cover')}</span>
            )}
          </div>

          <div className="flex flex-1 flex-col gap-2">
            <h1 className="text-2xl font-bold tracking-tight">{ip.name}</h1>
            <p className="text-sm text-muted-foreground">{t('ip.category', { category: categoryLabel })}</p>
            <p className="text-sm leading-relaxed text-foreground/90">
              {ip.description || t('ip.noDescription')}
            </p>
            <div className="flex flex-wrap items-center gap-3">
              <FollowButton
                targetType="ip"
                targetId={ip.id}
                initialFollowing={ip.is_following ?? false}
              />
              {ip.follower_count != null && ip.follower_count > 0 && (
                <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                  <Users className="h-3.5 w-3.5" />
                  {t('ip.followerCount', { count: ip.follower_count })}
                </span>
              )}
            </div>
            <div>
              <Link
                href={`/ip/${ip.id}/discussions`}
                className="inline-flex items-center gap-2 rounded-md border border-border px-3 py-2 text-xs transition-all duration-150 hover:bg-muted hover:border-accent/20 active:scale-95"
              >
                <MessageSquareText className="h-3.5 w-3.5" />
                {t('ip.enterDiscussion')}
              </Link>
            </div>
          </div>
        </div>

        <IPCategoryTabs ipId={String(ip.id)} activeCategory="all" />
      </div>

      <DiscussionBoard ipId={ip.id} compact />
    </section>
  );
}
