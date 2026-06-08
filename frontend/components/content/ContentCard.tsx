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
  description?: string;
  ip?: {
    id?: number;
    name?: string;
    slug?: string;
  };
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
  const typeLabel = contentType === "sheet_music" ? t('home.sheetMusic') : contentType === "prompt" ? t('home.aiPrompt') : contentType === "mod" ? t('home.mod') : contentType === "video" ? t('home.video') : contentType === "audio" ? t('home.audio') : contentType === "image" ? t('home.image') : t('home.text');

  return (
    <Link
      href={getCardHref(data)}
      className={cn(
        "group block overflow-hidden bg-card transition-all duration-200",
        isOriginal
          ? "rounded-lg hover:bg-muted/10 active:bg-muted/20"
          : "rounded-lg border border-border hover:border-accent/20 hover:bg-accent-subtle/5 active:bg-accent-subtle/10",
        className
      )}
    >
      {/* Cover with type badge */}
      <div className="relative w-full bg-muted">
        {/* Aspect ratio for cover — use natural image or default */}
        <div className={cn(contentType === "video" ? "aspect-[16/9]" : "aspect-[3/4]")}>
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

        {/* Type badge — fanwork only */}
        {!isOriginal && (
          <span className="absolute top-2 left-2 rounded-md bg-background px-2 py-0.5 text-[10.5px] font-semibold text-foreground/70 border border-border/30">
            {typeLabel}
          </span>
        )}
      </div>

      <div className="flex flex-col gap-1.5 px-2.5 pb-3 pt-2">
        {/* Source IP line — fanwork only */}
        {!isOriginal && data.ip?.name && (
          <div className="flex items-center gap-1 text-[11.5px] text-muted-foreground">
            <span className="h-3.5 w-3.5 flex-shrink-0 rounded-sm bg-muted flex items-center justify-center text-[8px] overflow-hidden">
              {data.ip.name.slice(0, 1)}
            </span>
            <span>{t('content.basedOnIp', { name: data.ip.name })}</span>
          </div>
        )}

        {/* Title */}
        <h3 className="line-clamp-2 text-[13.5px] font-semibold leading-snug text-foreground">
          {displayTitle}
        </h3>

        {/* Description — fanwork only */}
        {!isOriginal && data.description && (
          <p className="line-clamp-2 text-[12px] leading-relaxed text-muted-foreground">
            {data.description}
          </p>
        )}

        {/* Author + time */}
        <div className="flex items-center gap-1.5 text-[11.5px] text-muted-foreground">
          <span className="flex h-[18px] w-[18px] items-center justify-center rounded-full bg-accent-subtle text-[9px] font-semibold text-accent-emphasis flex-shrink-0">
            {(authorName || "?").slice(0, 1).toUpperCase()}
          </span>
          <span className="font-medium text-foreground/70 truncate">
            {authorName || t('common.userLabel', { id: authorId ?? "-" })}
          </span>
        </div>

        {/* Stats row + tags */}
        <div className="flex items-center justify-between pt-1.5 border-t border-border/50">
          <div className="flex items-center gap-2.5 text-[11.5px] text-muted-foreground">
            <span className="inline-flex items-center gap-0.5">
              <Heart className="h-3 w-3" />
              {data.like_count ?? 0}
            </span>
            {!isOriginal && (
              <span className="inline-flex items-center gap-0.5">
                <svg className="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/></svg>
                {data.comment_count ?? 0}
              </span>
            )}
          </div>
          {!isOriginal && tags.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {tags.slice(0, 3).map((tag) => (
                <Badge key={tag} variant="secondary" className="text-[10px] rounded-full px-2 py-0">
                  {tag}
                </Badge>
              ))}
            </div>
          )}
        </div>
      </div>
    </Link>
  );
}
