"use client";

import Link from "next/link";
import Image from "next/image";
import { useTranslations } from "next-intl";
import { MessageSquareText, Users } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { FollowButton } from "@/components/social/FollowButton";
import { DiscussionBoard } from "@/components/social/DiscussionBoard";
import { TagBadge } from "@/components/ui/TagBadge";

interface IPItem {
  id: number;
  name: string;
  description?: string;
  category?: string;
  cover_url?: string;
  tags?: string[];
  follower_count?: number;
  is_following?: boolean;
}

// TagBadge 6-color cycle from ui-spec.md TagBadge color mapping.
const TAG_COLOR_CYCLE = ["blue", "green", "purple", "orange", "rose", "sky"] as const;

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
            {ip.tags && ip.tags.length > 0 && (
              <div className="flex flex-wrap gap-1.5" aria-label={t('ip.tagsLabel')}>
                {ip.tags.map((tag, index) => (
                  <TagBadge key={tag} color={TAG_COLOR_CYCLE[index % TAG_COLOR_CYCLE.length]}>
                    {tag}
                  </TagBadge>
                ))}
              </div>
            )}
            {/* Hero 操作行：同排同高（紧凑档 28px），操作控件统一 8px 矩形（ui-spec §/ip/[ipId] Hero 区控件规格） */}
            <div className="flex flex-wrap items-center gap-3">
              <FollowButton
                targetType="ip"
                targetId={ip.id}
                initialFollowing={ip.is_following ?? false}
              />
              <Link
                href={`/ip/${ip.id}/discussions`}
                className={buttonVariants({ variant: "outline", size: "sm", className: "gap-1.5" })}
              >
                <MessageSquareText className="h-3.5 w-3.5" aria-hidden="true" />
                {t('ip.enterDiscussion')}
              </Link>
              {ip.follower_count != null && ip.follower_count > 0 && (
                <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                  <Users className="h-3.5 w-3.5" />
                  {t('ip.followerCount', { count: ip.follower_count })}
                </span>
              )}
            </div>
          </div>
        </div>
      </div>

      <DiscussionBoard ipId={ip.id} compact />
    </section>
  );
}
