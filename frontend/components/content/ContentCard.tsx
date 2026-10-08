"use client";

import { useRef, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import Image from "next/image";
import { Heart, MessageCircle, Play } from "lucide-react";
import { TagBadge } from "@/components/ui/TagBadge";
import { cn } from "@/lib/utils";
import { getCoverPlaceholder } from "@/lib/coverPlaceholder";
import {
  CARD_COVER_VARIANT_WIDTH,
  coverRenderSrc,
  prefetchCoverVariant,
} from "@/lib/overlay-motion";

export interface ContentCardData {
  id: number;
  title: string;
  author_id?: number;
  author?: {
    id?: number;
    username?: string;
    /** SP-17/T1：详情响应携带（OSS 签名 URL，有有效期；列表/卡片响应可不带）。 */
    avatar_url?: string;
    /** SP-17/T1：登录视角是否已关注（详情响应；匿名 omit）。 */
    is_following?: boolean;
  };
  zone?: string;
  content_type?: string;
  cover_image_url?: string;
  cover_width?: number;
  cover_height?: number;
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
    /** #846：IP 摘要封面（后端 hydrate + handler 签名；列表/详情响应携带）。 */
    cover_url?: string;
  };
}

/** #753 信息流两档封面（小红书口径）：h/w ≤ 4/3 → 3:4；h/w > 4/3 → 9:16；
 * object-cover 中心裁切（仅显示比例，不改原图/服务端数据）；元数据缺失/
 * 加载失败回落 3:4 稳定占位。比例方向统一用 h/w（高÷宽）。 */
const COVER_TWO_TIER_PORTRAIT = "3 / 4";
const COVER_TWO_TIER_TALL = "9 / 16";
/** 两档分界：h/w = 4/3（含边界归 3:4 档）。 */
const COVER_TIER_BOUNDARY_H_OVER_W = 4 / 3;

function feedCoverAspectRatio(width?: number, height?: number): string {
  if (width && height && width > 0 && height > 0) {
    return height / width > COVER_TIER_BOUNDARY_H_OVER_W
      ? COVER_TWO_TIER_TALL
      : COVER_TWO_TIER_PORTRAIT;
  }
  return COVER_TWO_TIER_PORTRAIT;
}

interface ContentCardProps {
  data: ContentCardData;
  className?: string;
  /**
   * 浮窗模式（/recommend 等发现面）：提供后卡片主点击区改为按钮并触发
   * onOpenDetail(data, trigger)，作者身份入口独立成链接跳用户主页，不再
   * 整卡跳详情页。不传则保持整卡 Link 跳详情页的默认行为。
   */
  onOpenDetail?: (data: ContentCardData, trigger: HTMLElement) => void;
}

function getCardHref(data: ContentCardData): string {
  if (data.zone === "original") {
    return `/original/${data.id}`;
  }
  return `/content/${data.id}`;
}

export function ContentCard({ data, className, onOpenDetail }: ContentCardProps) {
  const t = useTranslations();
  const contentType = data.content_type || "other";
  const rawTags = data.tags ?? [];
  const tags = rawTags.slice(0, 2);
  const coverUrl = data.cover_image_url;
  const displayTitle = data.title;
  const authorName = data.author?.username ?? "";
  const authorId = data.author_id ?? data.author?.id;
  const placeholderSrc = getCoverPlaceholder(contentType, displayTitle);
  const isOriginal = data.zone === "original";

  /* #753 信息流两档封面（小红书口径）：几何优先有效 cover_width/height
     （image = 媒体集首项尺寸、video = poster 尺寸），其次当前图实测
     （#398 C2 加载自愈链保留），两档 3:4 / 9:16 + object-cover 中心裁切；
     未知/失败 3:4 稳定占位。卡片与详情不再天然同取景（详情 contain），
     转场侧按几何差异走既有居中缩淡降级（overlay-motion C2）。 */
  const [measuredCover, setMeasuredCover] = useState<{ w: number; h: number } | null>(null);
  const coverWidth = data.cover_width;
  const coverHeight = data.cover_height;
  const hasCoverSize =
    typeof coverWidth === "number" && typeof coverHeight === "number" && coverWidth > 0 && coverHeight > 0;
  const effectiveWidth = hasCoverSize ? coverWidth : measuredCover?.w;
  const effectiveHeight = hasCoverSize ? coverHeight : measuredCover?.h;
  const coverAspectRatio = feedCoverAspectRatio(effectiveWidth, effectiveHeight);
  /* 图片加载落定后（仅在元数据缺失时）回填实测比例。两条进入路径：
     onLoad 事件 + 回调 ref 的 complete 自愈——SSR 出的 <img> 在 React 水合
     挂上 onLoad 之前就可能完成加载（快网/缓存/测试桩即时响应），事件会被
     错过，next/image 内部自带同款自愈，受控 <img> 须自己补（#409）。 */
  function measureIntrinsic(img: HTMLImageElement) {
    if (hasCoverSize) return;
    if (img.naturalWidth > 0 && img.naturalHeight > 0) {
      setMeasuredCover((prev) =>
        prev?.w === img.naturalWidth && prev?.h === img.naturalHeight
          ? prev
          : { w: img.naturalWidth, h: img.naturalHeight },
      );
    }
  }
  function handleCoverLoad(event: React.SyntheticEvent<HTMLImageElement>) {
    measureIntrinsic(event.currentTarget);
  }
  function coverImgRef(img: HTMLImageElement | null) {
    if (img?.complete) measureIntrinsic(img);
  }

  const typeLabel = contentType === "sheet_music" ? t('home.sheetMusic') : contentType === "prompt" ? t('home.aiPrompt') : contentType === "mod" ? t('home.mod') : contentType === "video" ? t('home.video') : contentType === "audio" ? t('home.audio') : contentType === "image" ? t('home.image') : t('home.text');

  const cardClasses = cn(
    "group block overflow-hidden bg-card transition-[border-color,box-shadow,background-color] duration-150 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring motion-reduce:transition-none",
    isOriginal
      ? "rounded-lg shadow-none hover:shadow-[var(--elevation-2)]"
      : "rounded-lg border border-border shadow-[var(--elevation-1)] hover:border-[var(--border-strong)] hover:shadow-[var(--elevation-2)]",
    className,
  );

  const cover = (
    <div data-slot="card-cover" className="relative w-full bg-muted">
      {/* #753 两档封面：3:4 / 9:16 + object-cover 中心裁切 */}
      <div
        data-slot="card-cover-aspect"
        className="relative overflow-hidden"
        style={{ aspectRatio: coverAspectRatio }}
      >
        {coverUrl ? (
          onOpenDetail ? (
            /* #430 两变体渐进（修订 #409 F1）：信息流卡片封面用轻量快变体
               （w=420，带宽/解码更轻）——浮窗首帧保持层与该串同源，必命中缓存；
               1080 规范变体由 pointerdown/hover 预取预热、settle 后交叉淡入。
               next/image 的响应式 sizes 无法跨端钉死同一变体，故用受控 <img>。 */
            <img
              src={coverRenderSrc(coverUrl, CARD_COVER_VARIANT_WIDTH) ?? coverUrl}
              alt={displayTitle}
              loading="lazy"
              decoding="async"
              draggable={false}
              ref={coverImgRef}
              className={cn(
                "absolute inset-0 h-full w-full object-cover object-center transition-transform duration-300 motion-reduce:transform-none",
                isOriginal ? "group-hover:scale-105" : "group-hover:scale-[1.03]",
              )}
              onLoad={handleCoverLoad}
            />
          ) : (
            <Image
              src={coverUrl}
              alt={displayTitle}
              fill
              className={cn(
                "object-cover object-center transition-transform duration-300 motion-reduce:transform-none",
                isOriginal ? "group-hover:scale-105" : "group-hover:scale-[1.03]",
              )}
              sizes="(max-width: 450px) 100vw, (max-width: 700px) 50vw, (max-width: 1100px) 33vw, 25vw"
              onLoad={handleCoverLoad}
            />
          )
        ) : (
          <img
            src={placeholderSrc}
            alt={displayTitle}
            ref={coverImgRef}
            className={cn(
              "h-full w-full object-cover object-center transition-transform duration-300 motion-reduce:transform-none",
              isOriginal ? "group-hover:scale-105" : "group-hover:scale-[1.03]",
            )}
            onLoad={handleCoverLoad}
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

      {/* #753 视频右上播放角标（仅视频；不加多图数量角标） */}
      {contentType === "video" && (
        <span
          aria-hidden="true"
          className="absolute right-2 top-2 flex size-6 items-center justify-center rounded-full bg-black/45 text-white backdrop-blur"
        >
          <Play className="ml-0.5 h-3 w-3" />
        </span>
      )}
    </div>
  );

  const info = (
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
    </div>
  );

  const tagsBlock = (
    <div className="flex flex-wrap gap-1">
      {tags.map((tag) => (
        <TagBadge key={tag}>
          {tag}
        </TagBadge>
      ))}
    </div>
  );

  const author = authorId ? (
    <Link
      href={`/user/${authorId}`}
      className="flex min-w-0 items-center gap-1.5 rounded-md text-xs text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
      onClick={(event) => event.stopPropagation()}
    >
      <span className="flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full bg-accent-subtle text-xs font-semibold text-accent-emphasis">
        {(authorName || "?").slice(0, 1).toUpperCase()}
      </span>
      <span className="truncate font-medium text-foreground/70">{authorName || t('common.userLabel', { id: authorId })}</span>
    </Link>
  ) : (
    <span className="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
      <span className="flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full bg-accent-subtle text-xs font-semibold text-accent-emphasis">
        {(authorName || "?").slice(0, 1).toUpperCase()}
      </span>
      <span className="truncate font-medium text-foreground/70">{authorName || t('common.userLabel', { id: "-" })}</span>
    </span>
  );

  const stats = (
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
  );

  /* 浮窗模式：主点击区为按钮打开共享详情浮层；作者身份入口分离为独立链接。
     #430 预取时机前移（instant.page 口径）：hover 65ms 防抖 + pointerdown 即刻
     预取 1080 规范变体（settle 交叉淡入与就绪门的消费目标）；onClick 保留为
     键盘激活路径兜底（prefetchCoverVariant 内部去重，重复调用零成本）。
     #398 C1/C4：浮窗封面挂载时大概率已解码，转场不再等 MB 级原图。 */
  const hoverPrefetchTimer = useRef<number | null>(null);
  const prefetchTarget = coverUrl ?? placeholderSrc;
  const scheduleHoverPrefetch = () => {
    if (hoverPrefetchTimer.current !== null) return;
    hoverPrefetchTimer.current = window.setTimeout(() => {
      hoverPrefetchTimer.current = null;
      prefetchCoverVariant(prefetchTarget);
    }, 65);
  };
  const cancelHoverPrefetch = () => {
    if (hoverPrefetchTimer.current === null) return;
    window.clearTimeout(hoverPrefetchTimer.current);
    hoverPrefetchTimer.current = null;
  };
  const prefetchNow = () => {
    cancelHoverPrefetch();
    prefetchCoverVariant(prefetchTarget);
  };

  if (onOpenDetail) {
    return (
      <article className={cardClasses}>
        <button
          type="button"
          aria-label={displayTitle}
          onPointerDown={prefetchNow}
          onMouseEnter={scheduleHoverPrefetch}
          onMouseLeave={cancelHoverPrefetch}
          onClick={(event) => {
            prefetchCoverVariant(coverUrl ?? placeholderSrc);
            onOpenDetail(data, event.currentTarget);
          }}
          className="block w-full cursor-pointer text-left focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
        >
          {cover}
          {info}
          {/* Tags — fanwork only */}
          {!isOriginal && tags.length > 0 && (
            <div className={cn(isOriginal ? "px-2" : "px-3", "pb-1")}>{tagsBlock}</div>
          )}
        </button>
        <div className={cn("flex items-center justify-between gap-2", isOriginal ? "px-2 pb-2" : "border-t border-border/50 px-3 pb-2.5 pt-2")}>
          {author}
          {stats}
        </div>
      </article>
    );
  }

  return (
    <Link
      href={getCardHref(data)}
      aria-label={displayTitle}
      className={cardClasses}
    >
      {cover}
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
          {stats}
          {!isOriginal && tags.length > 0 && tagsBlock}
        </div>
      </div>
    </Link>
  );
}
