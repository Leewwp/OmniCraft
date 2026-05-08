"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { MessageCircle, Clock } from "lucide-react";
import { cn } from "@/lib/utils";

interface DiscussionCardData {
  id: number;
  title: string;
  ip_id?: number;
  ip_name?: string;
  author?: { id?: number; username?: string };
  reply_count?: number;
  last_active_at?: string;
  created_at?: string;
}

interface DiscussionCardProps {
  data: DiscussionCardData;
  className?: string;
}

export function DiscussionCard({ data, className }: DiscussionCardProps) {
  const t = useTranslations();
  const cardContent = (
    <>
      <h3 className="text-sm font-semibold line-clamp-1">{data.title}</h3>
      <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
        {data.author?.username && (
          <span>{data.author.username}</span>
        )}
        <span className="inline-flex items-center gap-1">
          <MessageCircle className="h-3 w-3" />
          {t("discussion.replyCount", { count: data.reply_count ?? 0 })}
        </span>
        {data.last_active_at && (
          <span className="inline-flex items-center gap-1">
            <Clock className="h-3 w-3" />
            {new Date(data.last_active_at).toLocaleDateString()}
          </span>
        )}
      </div>
    </>
  );

  const cardClass = cn(
    "block rounded-md border border-border bg-card p-4 hover:bg-muted/20 transition-colors",
    className,
  );

  if (data.ip_id) {
    return (
      <Link href={`/ip/${data.ip_id}/discussions/${data.id}`} className={cardClass}>
        {cardContent}
      </Link>
    );
  }
  return <div className={cardClass}>{cardContent}</div>;
}
