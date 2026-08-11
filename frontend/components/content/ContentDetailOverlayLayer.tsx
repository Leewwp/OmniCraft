"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { AlertCircle, FileQuestion, ShieldOff } from "lucide-react";
import Link from "next/link";
import { api, ApiRequestError } from "@/lib/api";
import {
  normalizeContentDetailResponse,
  normalizeContentList,
  type NormalizedContentDetailResponse,
} from "@/lib/content";
import type { ContentCardData } from "@/components/content/ContentCard";
import { ContentDetail } from "@/components/content/ContentDetail";
import { MediaGallery, selectMediaItems } from "@/components/content/MediaGallery";
import {
  ContentSidebar,
  type RelatedCardEntry,
} from "@/components/content/ContentSidebar";
import { EmptyState } from "@/components/ui/empty-state";
import { SkeletonDetail } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { FollowButton } from "@/components/social/FollowButton";

export type OverlaySource = "recommendation" | "zone-page" | "ip-page" | "agent-citation";

export interface OverlayEntry {
  contentId: number;
  zone: "original" | "fanwork";
  source: OverlaySource;
  /** #89 连续浏览：触发上下文列表与当前索引（移动端从卡片网格进入时传入）。 */
  contextList?: Array<{ id: number; zone: "original" | "fanwork" }>;
  contextIndex?: number;
}

interface ContentDetailOverlayLayerProps {
  entry: OverlayEntry;
  /** 层在浮层导航栈中的下标（0 起），用于向浮层回报布局归属。 */
  layerIndex: number;
  /** #88 布局回报：image/video 媒体集内容 = "split-media"（桌面双栏），其余 "single"。 */
  onLayoutChange: (index: number, layout: "single" | "split-media") => void;
  onPush: (entry: OverlayEntry, trigger: HTMLElement | null) => void;
  onTitleChange: (title: string) => void;
  /** 层数据落定（含错误态）后通知浮层：入场转场可测量封面几何并启动。 */
  onMotionReady?: () => void;
}

type LayerStatus = "loading" | "default" | "forbidden" | "not-found" | "error";

/** 双栏媒体列控件区（翻页/指示点行）近似高度（px）：min-h-11 按钮 + py-2 + border-t。
    左栏 aspect 盒据此预留，保证媒体 contain 区不被控件裁切。 */
const SPLIT_MEDIA_CONTROLS_HEIGHT = 64;

/** 媒体集首项几何缺失时的防御性默认比例（与 MediaGallery DEFAULT_ASPECT_RATIO 一致）。 */
const SPLIT_DEFAULT_RATIO = 3 / 4;

const TYPE_LABEL_KEYS: Record<string, string> = {
  article: "home.text",
  image: "home.image",
  video: "home.video",
  audio: "home.audio",
  mod: "home.mod",
  prompt: "home.aiPrompt",
  sheet_music: "home.sheetMusic",
  template: "home.template",
};

export function ContentDetailOverlayLayer({
  entry,
  layerIndex,
  onLayoutChange,
  onPush,
  onTitleChange,
  onMotionReady,
}: ContentDetailOverlayLayerProps) {
  const t = useTranslations();
  const [status, setStatus] = useState<LayerStatus>("loading");
  const [detail, setDetail] = useState<NormalizedContentDetailResponse | null>(null);
  const [related, setRelated] = useState<ContentCardData[]>([]);
  const [relatedTotal, setRelatedTotal] = useState(0);
  const [attempt, setAttempt] = useState(0);
  const [coverReady, setCoverReady] = useState<boolean | undefined>(undefined);

  const onTitleChangeRef = useRef(onTitleChange);
  useEffect(() => {
    onTitleChangeRef.current = onTitleChange;
  }, [onTitleChange]);

  const onMotionReadyRef = useRef(onMotionReady);
  useEffect(() => {
    onMotionReadyRef.current = onMotionReady;
  }, [onMotionReady]);

  const onLayoutChangeRef = useRef(onLayoutChange);
  useEffect(() => {
    onLayoutChangeRef.current = onLayoutChange;
  }, [onLayoutChange]);

  /* 状态离开 loading（default/forbidden/not-found/error）后触发一次入场转场；
     错误态没有封面几何，浮层会走居中缩淡降级。 */
  const motionFiredRef = useRef(false);
  useEffect(() => {
    if (status === "loading" || motionFiredRef.current) return;
    motionFiredRef.current = true;
    onMotionReadyRef.current?.();
  }, [status]);

  useEffect(() => {
    let cancelled = false;
    setStatus("loading");
    setDetail(null);
    setRelated([]);
    setRelatedTotal(0);
    setCoverReady(undefined);

    api
      .get(`/api/v1/contents/${entry.contentId}`)
      .then((raw) => {
        if (cancelled) return;
        const normalized = normalizeContentDetailResponse(raw);
        if (!normalized.content) {
          setStatus("not-found");
          return;
        }
        if (normalized.content.status === "banned") {
          setStatus("forbidden");
          return;
        }
        setDetail(normalized);
        onTitleChangeRef.current(normalized.content.title);
        setStatus("default");
      })
      .catch((error) => {
        if (cancelled) return;
        if (error instanceof ApiRequestError && error.status === 404) {
          setStatus("not-found");
        } else if (error instanceof ApiRequestError && error.status === 403) {
          setStatus("forbidden");
        } else {
          setStatus("error");
        }
      });

    api
      .get<{ contents?: unknown[]; total?: number }>(
        `/api/v1/contents/${entry.contentId}/related-fanworks?page=1&page_size=8`,
      )
      .then((raw) => {
        if (cancelled) return;
        setRelated(normalizeContentList(raw.contents));
        setRelatedTotal(raw.total ?? 0);
      })
      .catch(() => {
        if (!cancelled) setRelated([]);
      });

    return () => {
      cancelled = true;
    };
  }, [entry.contentId, attempt]);

  /* #88 布局回报：数据落定后按内容类型与媒体集是否非空决定双栏归属。
     浮层据此切换唯一滚动容器（overlay-scroller ↔ 层内 layer-scroller）。 */
  useEffect(() => {
    if (status !== "default" || !detail?.content) return;
    const { media: mediaItems } = selectMediaItems(
      detail.attachments ?? [],
      detail.content.content_type,
      detail.content.content_type === "video" ? detail.content.cover_image_url : undefined,
    );
    const isSplit =
      (detail.content.content_type === "image" || detail.content.content_type === "video") &&
      mediaItems.length > 0;
    onLayoutChangeRef.current?.(layerIndex, isSplit ? "split-media" : "single");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status, detail, layerIndex]);

  if (status === "loading") {
    return (
      <div aria-busy="true" aria-label={t("contentDetailOverlay.title")}>
        <SkeletonDetail />
      </div>
    );
  }

  if (status === "not-found") {
    return (
      <EmptyState
        icon={FileQuestion}
        title={t("contentDetailOverlay.notFoundTitle")}
        description={t("contentDetailOverlay.notFoundDescription")}
      />
    );
  }

  if (status === "forbidden") {
    return (
      <EmptyState
        icon={ShieldOff}
        title={t("contentDetailOverlay.forbiddenTitle")}
        description={t("contentDetailOverlay.forbiddenDescription")}
      />
    );
  }

  if (status === "error") {
    return (
      <EmptyState
        icon={AlertCircle}
        title={t("contentDetailOverlay.loadFailedTitle")}
        description={t("contentDetailOverlay.loadFailedDescription")}
        action={
          <Button variant="outline" size="sm" onClick={() => setAttempt((value) => value + 1)}>
            {t("common.retry")}
          </Button>
        }
      />
    );
  }

  if (!detail?.content) return null;

  const content = detail.content;
  const isFanwork = content.zone === "fanwork";
  const sourceOriginal = detail.sourceOriginal;
  const relatedLabelKey = isFanwork ? "contentDetailOverlay.derivatives" : "content.relatedFanworks";

  /* #88 桌面双栏：仅 image/video 且媒体集非空（有 MediaGallery 可渲染）时生效；
     历史 image/video 内容无媒体集（行内 CoverImage）维持单栏。 */
  const { media: mediaItems } = selectMediaItems(
    detail.attachments ?? [],
    content.content_type,
    content.content_type === "video" ? content.cover_image_url : undefined,
  );
  const isSplitMedia =
    (content.content_type === "image" || content.content_type === "video") &&
    mediaItems.length > 0;
  const splitFirst = mediaItems[0];
  const splitRatio =
    splitFirst && splitFirst.width && splitFirst.height && splitFirst.width > 0 && splitFirst.height > 0
      ? splitFirst.width / splitFirst.height
      : SPLIT_DEFAULT_RATIO;
  const splitHasControls = mediaItems.length > 1;

  const relatedEntries: RelatedCardEntry[] = related.map((item) => {
    const typeLabelKey = TYPE_LABEL_KEYS[item.content_type ?? "other"] ?? "home.other";
    return {
      id: item.id,
      zone: item.zone === "original" ? "original" : "fanwork",
      title: item.title,
      meta: `${t(typeLabelKey)} · @${item.author?.username ?? t("common.userLabel", { id: item.author_id ?? "-" })}`,
      coverUrl: item.cover_image_url,
    };
  });

  const handleOpenEntry = (entry: { id: number; zone: "original" | "fanwork" }, trigger: HTMLElement) => {
    onPush({ contentId: entry.id, zone: entry.zone, source: "zone-page" }, trigger);
  };

  /* 创作者栏/相关列表（ui-spec:2402 相关推荐 + :2438 创作者栏）：单列时是右侧栏；
     双栏时下置于信息列末尾，维持关联入口（≥1100 显示，<1100 依旧隐藏）。 */
  const sidebar = (
    <ContentSidebar
      author={content.author?.id ? { id: content.author.id, username: content.author.username } : undefined}
      zone={isFanwork ? "fanwork" : "original"}
      ip={isFanwork && content.ip?.id && content.ip.name ? content.ip : undefined}
      sourceOriginal={isFanwork && sourceOriginal ? sourceOriginal : null}
      originalId={!isFanwork ? content.id : undefined}
      relatedFanworksCount={relatedTotal}
      relatedItems={relatedEntries}
      relatedItemsLabelKey={relatedLabelKey}
      relatedFooterAction={
        !isFanwork && relatedTotal > 8 ? (
          <Link
            href={`/original/${content.id}/fanworks`}
            className="mt-2 inline-flex items-center gap-1 text-xs font-medium text-accent-emphasis transition-colors hover:text-accent-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            {t("contentDetailOverlay.viewAll")}
          </Link>
        ) : undefined
      }
      onOpenRelated={handleOpenEntry}
      onOpenRelatedItem={handleOpenEntry}
      followAction={
        content.author?.id ? (
          <FollowButton targetType="user" targetId={content.author.id} />
        ) : undefined
      }
    />
  );

  if (isSplitMedia) {
    return (
      <div className="mx-auto w-full max-w-[1280px] min-[1100px]:grid min-[1100px]:h-full min-[1100px]:min-h-0 min-[1100px]:grid-cols-[minmax(0,3fr)_minmax(0,2fr)] min-[1100px]:items-stretch min-[1100px]:gap-6">
        {/* 左媒体列（#88）：仅 ≥1100px 显示；aspect 盒高度 = 视口可用高（留控件位），
            宽度按首项媒体比例自适应（aspect-ratio 传递尺寸），列内居中不裁切。 */}
        <div className="hidden min-[1100px]:grid min-[1100px]:h-full min-[1100px]:min-h-0 min-[1100px]:place-items-center min-[1100px]:overflow-hidden">
          <div
            className="min-[1100px]:h-full min-[1100px]:max-w-full"
            style={{
              aspectRatio: String(splitRatio),
              maxHeight: splitHasControls
                ? `calc(100% - ${SPLIT_MEDIA_CONTROLS_HEIGHT}px)`
                : "100%",
            }}
          >
            <MediaGallery
              items={mediaItems}
              onFirstMediaSettled={(state) => setCoverReady(state === "ready")}
            />
          </div>
        </div>

        {/* 右信息列（#88）：≥1100px 时是浮层内唯一滚动容器（layer-scroller）；
            <1100px 时回到 overlay-scroller 单列滚动，本列不滚动。 */}
        <div
          data-slot="layer-scroller"
          className="min-w-0 min-[1100px]:h-full min-[1100px]:min-h-0 min-[1100px]:overflow-y-auto min-[1100px]:overscroll-contain"
        >
          <ContentDetail
            data={{ ...content, attachments: detail.attachments, tags: detail.tags }}
            coverSync
            mediaSlot="split"
            coverReady={coverReady}
          />
          {sidebar}
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-[1280px] gap-6">
      <div className="min-w-0 flex-1">
        <ContentDetail
          data={{ ...content, attachments: detail.attachments, tags: detail.tags }}
          coverSync
        />
      </div>

      {sidebar}
    </div>
  );
}
