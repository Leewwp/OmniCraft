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

  const isOriginal = data.zone === "original";

  return (
    <Link
      href={getCardHref(data)}
      className={cn(
        "group block overflow-hidden bg-card transition-all duration-200",
        isOriginal
          ? "rounded-lg hover:-translate-y-0.5"
          : "rounded-lg border border-border hover:border-border/80",
        className
      )}
    >
      <div className="relative w-full bg-muted">
        {coverUrl ? (
          <Image
            src={coverUrl}
            alt={displayTitle}
            fill
            className="object-cover transition-transform duration-300 group-hover:scale-[1.03]"
            sizes="(max-width: 450px) 100vw, (max-width: 700px) 50vw, (max-width: 1100px) 33vw, 25vw"
          />
        ) : (
          <img
            src={placeholderSrc}
            alt={displayTitle}
            className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-[1.03]"
          />
        )}
      </div>

      <div className="flex flex-col gap-2 px-2.5 pb-3 pt-2">
        <h3 className="line-clamp-2 text-[13.5px] font-semibold leading-snug text-foreground">
          {displayTitle}
        </h3>

        {!isOriginal && tags.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {tags.map((tag) => (
              <Badge key={tag} variant="secondary" className="text-[10px]">
                {tag}
              </Badge>
            ))}
          </div>
        )}

        <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
          <span className="truncate">
            {authorName || t('common.userLabel', { id: authorId ?? "-" })}
          </span>
          <span className="inline-flex items-center gap-1 flex-shrink-0">
            <Heart className="h-3 w-3" />
            {data.like_count ?? 0}
          </span>
        </div>
      </div>
    </Link>
  );
}
