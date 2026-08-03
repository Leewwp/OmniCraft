"use client";

import { useTranslations } from "next-intl";
import Link from "next/link";
import Image from "next/image";
import { Heart, MessageCircle } from "lucide-react";
import { TagBadge } from "@/components/ui/TagBadge";
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
  const tags = rawTags.slice(0, 2);
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
      aria-label={displayTitle}
      className={cn(
        "group block overflow-hidden bg-card transition-[border-color,box-shadow,background-color] duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background motion-reduce:transition-none",
        isOriginal
          ? "rounded-lg shadow-none hover:shadow-[var(--elevation-2)]"
          : "rounded-lg border border-border shadow-[var(--elevation-1)] hover:border-[var(--border-strong)] hover:shadow-[var(--elevation-2)]",
        className
      )}
    >
      {/* Cover with type badge */}
      <div className="relative w-full bg-muted">
        {/* Aspect ratio for cover — use natural image or default */}
        <div className={cn("relative overflow-hidden", contentType === "video" ? "aspect-[16/9]" : "aspect-[3/4]")}>
          {coverUrl ? (
            <Image
              src={coverUrl}
              alt={displayTitle}
              fill
              className={cn(
                "object-cover transition-transform duration-300 motion-reduce:transform-none",
                isOriginal ? "group-hover:scale-105" : "group-hover:scale-[1.03]",
              )}
              sizes="(max-width: 450px) 100vw, (max-width: 700px) 50vw, (max-width: 1100px) 33vw, 25vw"
            />
          ) : (
            <img
              src={placeholderSrc}
              alt={displayTitle}
              className={cn(
                "h-full w-full object-cover transition-transform duration-300 motion-reduce:transform-none",
                isOriginal ? "group-hover:scale-105" : "group-hover:scale-[1.03]",
              )}
            />
          )}
        </div>

        {isOriginal && (
          <div className="pointer-events-none absolute inset-0 bg-black/10 opacity-0 transition-opacity duration-150 group-hover:opacity-100 motion-reduce:transition-none" />
        )}

        {/* Type badge — fanwork only */}
        {!isOriginal && (
          <span className="absolute left-2 top-2 rounded-md border border-border/30 bg-background px-2 py-0.5 text-xs font-semibold text-foreground/70">
            {typeLabel}
          </span>
        )}
      </div>

      <div className={cn("flex flex-col", isOriginal ? "gap-1 p-2" : "gap-1.5 p-3")}>
        {/* Source IP line — fanwork only */}
        {!isOriginal && data.ip?.name && (
          <div className="flex items-center gap-1 text-xs text-muted-foreground">
            <span className="flex h-4 w-4 flex-shrink-0 items-center justify-center overflow-hidden rounded-sm bg-muted text-xs">
              {data.ip.name.slice(0, 1)}
            </span>
            <span>{t('content.basedOnIp', { name: data.ip.name })}</span>
          </div>
        )}

        {/* Title */}
        <h3 className="line-clamp-2 text-sm font-semibold leading-snug text-foreground">
          {displayTitle}
        </h3>

        {/* Description — fanwork only */}
        {!isOriginal && data.description && (
          <p className="line-clamp-2 text-xs leading-relaxed text-muted-foreground">
            {data.description}
          </p>
        )}

        {/* Author + time */}
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <span className="flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full bg-accent-subtle text-xs font-semibold text-accent-emphasis">
            {(authorName || "?").slice(0, 1).toUpperCase()}
          </span>
          <span className="font-medium text-foreground/70 truncate">
            {authorName || t('common.userLabel', { id: authorId ?? "-" })}
          </span>
        </div>

        {/* Stats row + tags */}
        <div className={cn("flex items-center justify-between", isOriginal ? "pt-1" : "border-t border-border/50 pt-1.5")}>
          <div className="flex items-center gap-2.5 text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-0.5">
              <Heart className="h-3 w-3" />
              {data.like_count ?? 0}
            </span>
            {!isOriginal && (
              <span className="inline-flex items-center gap-0.5">
                <MessageCircle className="h-3 w-3" aria-hidden="true" />
                {data.comment_count ?? 0}
              </span>
            )}
          </div>
          {!isOriginal && tags.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {tags.map((tag) => (
                <TagBadge key={tag}>
                  {tag}
                </TagBadge>
              ))}
            </div>
          )}
        </div>
      </div>
    </Link>
  );
}
