"use client";

import { useTranslations } from "next-intl";
import Link from "next/link";
import Image from "next/image";
import { Eye, Heart, MessageCircle } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { getCoverPlaceholder } from "@/lib/coverPlaceholder";

export interface ContentCardData {
  id: number;
  title: string;
  author_id?: number;
  author?: {
    id?: number;
    username?: string;
  };
  zone?: string;
  content_type?: string;
  cover_image_url?: string;
  view_count?: number;
  like_count?: number;
  comment_count?: number;
  tags?: string[];
  category?: string;
}

interface ContentCardProps {
  data: ContentCardData;
  className?: string;
}

function getCardHref(data: ContentCardData): string {
  if (data.zone === "original") {
    return `/original/${data.id}`;
  }
  return `/content/${data.id}`;
}

export function ContentCard({ data, className }: ContentCardProps) {
  const t = useTranslations();
  const contentType = data.content_type || "other";
  const rawTags = data.tags ?? [];
  const tags = rawTags.slice(0, 3);
  const coverUrl = data.cover_image_url;
  const displayTitle = data.title;
  const authorName = data.author?.username ?? "";
  const authorId = data.author_id;
  const placeholderSrc = getCoverPlaceholder(contentType, displayTitle);

  return (
    <Link
      href={getCardHref(data)}
      className={cn(
        "group block overflow-hidden rounded-md border border-border bg-card transition-colors hover:bg-muted/30",
        className
      )}
    >
      <div className="aspect-[3/4] w-full border-b border-border bg-muted/40 relative">
        {coverUrl ? (
          <Image
            src={coverUrl}
            alt={displayTitle}
            fill
            className="object-cover"
            sizes="(max-width: 450px) 100vw, (max-width: 700px) 50vw, (max-width: 1100px) 33vw, 25vw"
          />
        ) : (
          <img
            src={placeholderSrc}
            alt={displayTitle}
            className="h-full w-full object-cover"
          />
        )}
      </div>

      <div className="flex flex-col gap-3 p-3">
        <div className="space-y-1">
          <h3 className="line-clamp-2 text-sm font-semibold text-foreground">
            {displayTitle}
          </h3>
          <p className="text-xs text-muted-foreground">
            {authorName || t('common.userLabel', { id: authorId ?? "-" })}
          </p>
        </div>

        {tags.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {tags.map((tag) => (
              <Badge key={tag} variant="secondary" className="text-[10px]">
                {tag}
              </Badge>
            ))}
          </div>
        )}

        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          <span className="inline-flex items-center gap-1">
            <Heart className="h-3.5 w-3.5" />
            {data.like_count ?? 0}
          </span>
          <span className="inline-flex items-center gap-1">
            <MessageCircle className="h-3.5 w-3.5" />
            {data.comment_count ?? 0}
          </span>
          <span className="inline-flex items-center gap-1">
            <Eye className="h-3.5 w-3.5" />
            {data.view_count ?? 0}
          </span>
        </div>
      </div>
    </Link>
  );
}
