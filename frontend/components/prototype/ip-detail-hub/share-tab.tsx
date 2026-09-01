"use client";
/**
 * 【原型专用，随时可删】内容分享 tab：作品瀑布流 + 作品详情浮层。
 * 真实实现应复用 OverlayMasonryGrid + ContentDetailOverlay；此处为简化假数据版。
 */
import { useState } from "react";
import {
  Clapperboard,
  Eye,
  FileText,
  Headphones,
  Image as ImageIcon,
  Package,
  Puzzle,
  Sparkles,
  Music,
  ThumbsUp,
  X,
  XCircle,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { useToast } from "@/components/ui/Toast";
import { t } from "./copy";
import type { ShareItem, ShareType } from "./mock-data";

const TYPE_ICONS: Record<ShareType, React.ComponentType<{ className?: string }>> = {
  image: ImageIcon,
  article: FileText,
  video: Clapperboard,
  audio: Headphones,
  mod: Puzzle,
  prompt: Sparkles,
  sheet_music: Music,
  other: Package,
};

export const SHARE_TYPES: ShareType[] = [
  "image",
  "article",
  "video",
  "audio",
  "mod",
  "prompt",
  "sheet_music",
  "other",
];

export function typeLabel(type: ShareType): string {
  return t(`hub.share.type.${type}`);
}

/** 作品卡瀑布流（CSS columns 简化版 masonry）。 */
export function ShareGrid({ items, onOpen }: { items: ShareItem[]; onOpen: (item: ShareItem) => void }) {
  const { toast } = useToast();
  if (items.length === 0) {
    return (
      <EmptyState
        icon={Package}
        title={t("hub.share.emptyTitle")}
        description={t("hub.share.emptyDesc")}
        action={
          <Button onClick={() => toast("info", t("common.prototypeOnly"))}>{t("hub.share.emptyAction")}</Button>
        }
      />
    );
  }
  return (
    <div className="columns-2 gap-4 md:columns-3 xl:columns-4">
      {items.map((item) => {
        const Icon = TYPE_ICONS[item.type];
        return (
          <button
            key={item.id}
            type="button"
            onClick={() => onOpen(item)}
            className="mb-4 block w-full break-inside-avoid overflow-hidden rounded-lg border border-border bg-canvas-default text-left shadow-[var(--elevation-1)] transition-[box-shadow,border-color,translate] duration-150 hover:-translate-y-0.5 hover:border-border-strong hover:shadow-[var(--elevation-2)] motion-reduce:hover:translate-y-0"
          >
            <div className="relative w-full" style={{ aspectRatio: item.aspect }}>
              <div className="absolute inset-0" style={item.coverStyle} aria-hidden="true" />
              <span className="absolute left-2 top-2 inline-flex size-6 items-center justify-center rounded-md bg-black/45 text-white">
                <Icon className="size-3.5" aria-hidden="true" />
              </span>
            </div>
            <div className="p-3">
              <h3 className="line-clamp-2 text-sm font-medium">{item.title}</h3>
              <p className="mt-1.5 text-xs text-muted-foreground">{item.author}</p>
              <p className="mt-1 flex items-center gap-2.5 text-xs text-muted-foreground">
                <span className="inline-flex items-center gap-0.5">
                  <Eye className="size-3" aria-hidden="true" />
                  {item.views.toLocaleString()}
                </span>
                <span className="inline-flex items-center gap-0.5">
                  <ThumbsUp className="size-3" aria-hidden="true" />
                  {item.likes.toLocaleString()}
                </span>
                <span className="inline-flex items-center gap-0.5 text-[var(--tag-orange-fg)]">
                  ★ {item.rating.toFixed(1)}
                </span>
              </p>
            </div>
          </button>
        );
      })}
    </div>
  );
}

/** 作品详情浮层（简化版 ContentDetailOverlay 模式）。 */
export function ShareOverlay({ item, onClose }: { item: ShareItem; onClose: () => void }) {
  const { toast } = useToast();
  const [liked, setLiked] = useState(false);
  const Icon = TYPE_ICONS[item.type];
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" role="dialog" aria-modal="true" aria-label={item.title}>
      <div className="absolute inset-0 bg-black/50" onClick={onClose} aria-hidden="true" />
      <div className="relative max-h-[85vh] w-full max-w-2xl overflow-y-auto rounded-lg border border-border bg-canvas-default shadow-[var(--elevation-3)]">
        <button
          type="button"
          onClick={onClose}
          aria-label={t("common.close")}
          className="absolute right-3 top-3 z-10 inline-flex size-8 items-center justify-center rounded-md bg-black/40 text-white transition-colors duration-150 hover:bg-black/60"
        >
          <X className="size-4" aria-hidden="true" />
        </button>
        <div className="relative aspect-video w-full" style={item.coverStyle} aria-hidden="true">
          <span className="absolute bottom-3 left-4 inline-flex items-center gap-1 rounded-full bg-black/45 px-2 py-0.5 text-xs text-white">
            <Icon className="size-3" aria-hidden="true" />
            {typeLabel(item.type)}
          </span>
        </div>
        <div className="space-y-4 p-5">
          <div>
            <h2 className="text-xl font-semibold leading-snug">{item.title}</h2>
            <p className="mt-1 text-xs text-muted-foreground">
              {item.author} · {t("hub.share.detailZone")}
            </p>
          </div>
          <p className="text-sm leading-relaxed text-foreground/90">{item.excerpt}</p>
          <div className="flex flex-wrap items-center gap-4 border-t border-border pt-4 text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-1">
              <Eye className="size-3.5" aria-hidden="true" />
              {t("hub.share.views", { count: item.views.toLocaleString() })}
            </span>
            <Button
              size="sm"
              variant={liked ? "default" : "outline"}
              className="gap-1"
              onClick={() => setLiked((v) => !v)}
            >
              <ThumbsUp className="size-3.5" aria-hidden="true" />
              {t("hub.share.likes", { count: (item.likes + (liked ? 1 : 0)).toLocaleString() })}
            </Button>
            <span className="ml-auto inline-flex items-center gap-1">
              <XCircle className="size-3.5" aria-hidden="true" />
              {t("hub.share.overlayNote")}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
