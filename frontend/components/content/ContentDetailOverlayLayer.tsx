"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useTranslations } from "next-intl";
import { AlertCircle, FileQuestion, ShieldOff, Timer } from "lucide-react";
import Link from "next/link";
import { api, ApiRequestError } from "@/lib/api";
import {
  normalizeContentDetailResponse,
  normalizeContentList,
  type NormalizedContentDetailResponse,
} from "@/lib/content";
import type { ContentCardData } from "@/components/content/ContentCard";
import { ContentDetail } from "@/components/content/ContentDetail";
import {
  ContentSidebar,
  type RelatedCardEntry,
} from "@/components/content/ContentSidebar";
import { EmptyState } from "@/components/ui/empty-state";
import { SkeletonDetail } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { FollowButton } from "@/components/social/FollowButton";
import { CommentSection } from "@/components/social/CommentSection";
import { OverlayVariantLayout } from "@/components/content/OverlayVariantLayout";
import { OverlayRelatedBlock } from "@/components/content/OverlayRelatedBlock";
import { RelatedContents } from "@/components/content/RelatedContents";
import { DeferredMount } from "@/components/content/DeferredMount";
import { useOverlayMedia } from "@/lib/overlay-media";

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
  onPush: (entry: OverlayEntry, trigger: HTMLElement | null) => void;
  /** #89 连续浏览：媒体集最后一项继续上滑时请求切换到上下文列表下一篇。 */
  onSwitchNext?: (entry: OverlayEntry) => void;
  onTitleChange: (title: string) => void;
  /** 层数据落定（含错误态）后通知浮层：入场转场可测量封面几何并启动。 */
  onMotionReady?: () => void;
  /** #409 F1 首帧保持：入场未落定期封面以卡片封面 src 渲染（防动效期间换图）。 */
  motionHoldSrc?: string | null;
  /** #409 F1 入场落定标记：true 表示首帧保持与几何冻结可以解除。 */
  motionSettled?: boolean;
  /** D2 #859：本层是否为顶层活动层（键盘翻页/媒体挂起矩阵只作用于顶层）。 */
  active?: boolean;
}

type LayerStatus = "loading" | "default" | "forbidden" | "not-found" | "error" | "rate-limited";

/** #89 连续浏览视口判定：与 ui-spec 全局三档一致（PC > 1100px），桌面不出现连续浏览交互。 */
function isMobileViewport(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return true;
  return !window.matchMedia("(min-width: 1100px)").matches;
}

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
  onPush,
  onSwitchNext,
  onTitleChange,
  onMotionReady,
  motionHoldSrc,
  motionSettled,
  active = true,
}: ContentDetailOverlayLayerProps) {
  const t = useTranslations();
  const [status, setStatus] = useState<LayerStatus>("loading");
  const [detail, setDetail] = useState<NormalizedContentDetailResponse | null>(null);
  const [related, setRelated] = useState<ContentCardData[]>([]);
  const [relatedTotal, setRelatedTotal] = useState(0);
  /* 扇出收敛（2026-09-09）：层内关联行拉取落定标记——就绪后向 ContentDetail 的
     relatedFanworks 插槽直供数据（RelatedFanworks 不再自拉同一接口）。 */
  const [relatedLoaded, setRelatedLoaded] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [coverReady, setCoverReady] = useState<boolean | undefined>(undefined);
  /* D1 #858 统一版式视口判定（≥960 与浮层壳层断点同源）：桌面全状态双栏，
     移动 <960 单列。 */
  const [isDesktop, setIsDesktop] = useState(false);
  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") return;
    const mediaQuery = window.matchMedia("(min-width: 960px)");
    const update = () => setIsDesktop(mediaQuery.matches);
    update();
    mediaQuery.addEventListener("change", update);
    return () => mediaQuery.removeEventListener("change", update);
  }, []);

  const onTitleChangeRef = useRef(onTitleChange);
  useEffect(() => {
    onTitleChangeRef.current = onTitleChange;
  }, [onTitleChange]);

  /* #397 全类型媒体链（真实媒体集 → 内容封面 → 自动文字封面）：D2 #859 类型
     分流——图片缺尺寸走 Image 规范变体探针、视频走 loadedmetadata 实测（失败
     保留视频 URL）；实测在共享有界预算内结清（先于浮层 2s 入场保险），超时按
     稳定 fallback 放行，迟到元数据由冻结窗口挡在入场动画外。 */
  const { media: probedMedia, ready: chainReady } = useOverlayMedia(
    status === "default" ? detail : null,
  );

  /* D2 #859：挂载视频 loadedmetadata 回填（探针失败/超时后的迟到修正）——
     按附件 id 覆盖尺寸，入场落定后随 240ms 过渡平滑应用；换内容复位。 */
  const [dimOverrides, setDimOverrides] = useState<Record<number, { width: number; height: number }>>({});
  const chainMedia = useMemo(() => {
    if (Object.keys(dimOverrides).length === 0) return probedMedia;
    return probedMedia.map((item) => {
      const override = dimOverrides[item.id];
      return override ? { ...item, width: override.width, height: override.height } : item;
    });
  }, [probedMedia, dimOverrides]);
  const handleMediaDimensions = useCallback((id: number, width: number, height: number) => {
    setDimOverrides((prev) =>
      prev[id]?.width === width && prev[id]?.height === height
        ? prev
        : { ...prev, [id]: { width, height } },
    );
  }, []);

  /* #409 F1 起跑前几何冻结（统一版式媒体列）：媒体列宽度由 ResizeObserver
     实测驱动（挂载期从百分比兜底到实测像素）；转场起跑须等首次实测到达
     （几何稳定后测量），否则目标矩形在动画中段跳变。 */
  const [paneMeasured, setPaneMeasured] = useState(false);
  const handlePaneMeasured = useCallback(() => setPaneMeasured(true), []);

  const onMotionReadyRef = useRef(onMotionReady);
  useEffect(() => {
    onMotionReadyRef.current = onMotionReady;
  }, [onMotionReady]);

  /* 状态离开 loading（default/forbidden/not-found/error）后触发一次入场转场；
     错误态没有封面几何，浮层会走居中缩淡降级。#397：default 态等媒体几何实测
     就绪（chainReady）再触发；#409 F1：统一版式再等媒体列首次实测
     （paneMeasured）。网络悬挂由浮层 2s 保险定时器兜底。
     D2：motionStarted 标记入场转场起跑——冻结窗口 = 起跑→落定，起跑前壳层
     opacity 0，媒体比例修正即时生效不可见；窗口内锁存起跑比例（零跳宽）。 */
  const [motionStarted, setMotionStarted] = useState(false);
  const motionGate =
    status === "loading"
      ? false
      : status !== "default"
        ? true
        : chainReady && (!isDesktop || paneMeasured);
  const motionFiredRef = useRef(false);
  useEffect(() => {
    if (!motionGate || motionFiredRef.current) return;
    motionFiredRef.current = true;
    setMotionStarted(true);
    onMotionReadyRef.current?.();
  }, [motionGate]);

  useEffect(() => {
    let cancelled = false;
    setStatus("loading");
    setDetail(null);
    setRelated([]);
    setRelatedTotal(0);
    setRelatedLoaded(false);
    setCoverReady(undefined);
    setDimOverrides({});

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
        } else if (error instanceof ApiRequestError && error.status === 429) {
          /* #400：429 曾被误渲染成网络错误——限流有自己的语义（稍后重试即可），
             详情打开时一次爆发 10+ 请求很容易触顶（dev StrictMode 翻倍）。 */
          setStatus("rate-limited");
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
        setRelatedLoaded(true);
      })
      .catch(() => {
        if (!cancelled) {
          setRelated([]);
          setRelatedTotal(0);
          setRelatedLoaded(true);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [entry.contentId, attempt]);

  /* #89 连续浏览：媒体集最后一项继续上滑 → 切换上下文列表下一篇（仅移动端）；
     上下文列表到底时不再切换，显示「已经到底」提示。浮层内关联内容等无
     contextList 的入口不参与连续浏览。 */
  const atContextEnd = Boolean(
    entry.contextList?.length &&
      entry.contextIndex !== undefined &&
      entry.contextIndex >= entry.contextList.length - 1,
  );

  const handleReachEnd = useCallback(() => {
    if (!isMobileViewport()) return;
    const list = entry.contextList;
    const index = entry.contextIndex;
    if (!list || list.length === 0 || index === undefined) return;
    if (index >= list.length - 1) return;
    const nextItem = list[index + 1];
    onSwitchNext?.({
      contentId: nextItem.id,
      zone: nextItem.zone,
      source: entry.source,
      contextList: list,
      contextIndex: index + 1,
    });
  }, [entry, onSwitchNext]);

  /* #69 浮层内系列导航：上一章/下一章/目录选择都压入同一导航栈（与关联卡片同模型），
     不整页跳转；zone 沿用当前层（系列单一 zone，与 membership.series_zone 一致）。 */
  const handleNavigateInOverlay = useCallback(
    (contentId: number, trigger?: HTMLElement | null) => {
      if (contentId === entry.contentId) return;
      onPush({ contentId, zone: entry.zone, source: entry.source }, trigger ?? null);
    },
    [entry, onPush],
  );

  /* 状态体（D1 #858）：loading 骨架 / 各错误态 EmptyState / default 完整详情。
     桌面 ≥960 时所有状态由统一双栏外壳承载（本函数末尾 OverlayVariantLayout）；
     移动 <960 直接渲染状态体（单列纵滚）。 */
  let statusBody: ReactNode = null;
  if (status === "loading") {
    statusBody = (
      <div aria-busy="true" aria-label={t("contentDetailOverlay.title")}>
        <SkeletonDetail />
      </div>
    );
  } else if (status === "not-found") {
    statusBody = (
      <EmptyState
        icon={FileQuestion}
        title={t("contentDetailOverlay.notFoundTitle")}
        description={t("contentDetailOverlay.notFoundDescription")}
      />
    );
  } else if (status === "forbidden") {
    statusBody = (
      <EmptyState
        icon={ShieldOff}
        title={t("contentDetailOverlay.forbiddenTitle")}
        description={t("contentDetailOverlay.forbiddenDescription")}
      />
    );
  } else if (status === "rate-limited" || status === "error") {
    /* #400：429 曾被误渲染成网络错误——限流有自己的语义（稍后重试即可）。 */
    statusBody = (
      <EmptyState
        icon={status === "error" ? AlertCircle : Timer}
        title={t(
          status === "error"
            ? "contentDetailOverlay.loadFailedTitle"
            : "contentDetailOverlay.rateLimitedTitle",
        )}
        description={t(
          status === "error"
            ? "contentDetailOverlay.loadFailedDescription"
            : "contentDetailOverlay.rateLimitedDescription",
        )}
        action={
          <Button variant="outline" size="sm" onClick={() => setAttempt((value) => value + 1)}>
            {t("common.retry")}
          </Button>
        }
      />
    );
  } else if (detail?.content) {
    const content = detail.content;
    const isFanwork = content.zone === "fanwork";
    const sourceOriginal = detail.sourceOriginal;
    const relatedLabelKey = isFanwork ? "contentDetailOverlay.derivatives" : "content.relatedFanworks";

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

    /* #90 相关内容块卡片：浮层栈内打开（source=zone-page）；ContentCardData 的
       zone 为可选字符串，此处统一归一化为 original/fanwork。 */
    const handleOpenEntry = (
      relatedEntry: { id: number; zone?: string },
      trigger: HTMLElement,
    ) => {
      onPush(
        {
          contentId: relatedEntry.id,
          zone: relatedEntry.zone === "original" ? "original" : "fanwork",
          source: "zone-page",
        },
        trigger,
      );
    };

    /* #90 相关内容块：关联行插槽（复用 RelatedFanworks 组件）+ 相似内容去重摘要
       （layer 已为侧栏拉取 related-fanworks 合同，直接复用该数据源）。 */
    const relatedFanworksSlot = {
      sourceContentId: content.id,
      sourceZone: isFanwork ? ("fanwork" as const) : ("original" as const),
      titleKey: isFanwork ? "relatedFanworks.derivatives.title" : "relatedFanworks.original.title",
      createHref: isFanwork
        ? `/studio/publish/fanwork?source_fanwork_id=${content.id}`
        : `/studio/publish/fanwork?source_original_id=${content.id}`,
      viewAllHref: !isFanwork ? `/original/${content.id}/fanworks` : undefined,
      initialData: relatedLoaded ? { items: related, total: relatedTotal } : undefined,
    };
    const relatedFanworksSummary = related.map((item) => ({
      id: item.id,
      title: item.title,
      zone: item.zone === "original" ? ("original" as const) : ("fanwork" as const),
    }));

    /* 创作者栏/相关列表（ui-spec:2402 相关推荐 + :2438 创作者栏）：仅移动单列
       渲染（CSS 自隐藏于 <lg 视口，DOM 契约保持）；桌面统一右栏由 ContentDetail
       头部创作者行 + 推荐区承担，ContentSidebar 退出桌面路径（D1 #858）。 */
    /* #430 挂载分帧：侧栏延后一拍（双 rAF + 低优先级），降低动画期主线程拥塞。 */
    const sidebar = (
      <DeferredMount>
        <ContentSidebar
        author={
          content.author?.id
            ? {
                id: content.author.id,
                username: content.author.username,
                avatar_url: content.author.avatar_url,
              }
            : undefined
        }
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
              className="mt-2 inline-flex items-center gap-1 text-xs font-medium text-accent-emphasis transition-colors hover:text-accent-hover focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring "
            >
              {t("contentDetailOverlay.viewAll")}
            </Link>
          ) : undefined
        }
        onOpenRelated={handleOpenEntry}
        onOpenRelatedItem={handleOpenEntry}
        followAction={
          content.author?.id ? (
            <FollowButton
              targetType="user"
              targetId={content.author.id}
              initialFollowing={content.author.is_following ?? false}
            />
          ) : undefined
        }
        />
      </DeferredMount>
    );

    if (isDesktop) {
      /* D1 #858 桌面统一右栏（全类型一致，钉死块序）：ContentDetail（creatorFirst
         = 创作者行置顶 + mediaSlot="variant" 隐藏行内媒体）→ 关联块（⓪关联 IP →
         ①关联的原创 → ②系列导航）→ 评论区 → 推荐区（衍生列表 + 相似推荐，
         RelatedContents 复用层内 related 数据、视口门下浮到 960）。
         SP-17/T4 契约保持：内联关注按钮 = 唯一直接入口（不传
         inlineFollowClassName），ContentSidebar 不进右栏。 */
      statusBody = (
        <ContentDetail
          data={{ ...content, attachments: detail.attachments, tags: detail.tags }}
          coverSync
          mediaSlot="variant"
          coverReady={coverReady}
          coverHoldSrc={motionHoldSrc}
          creatorFirst
          sourceOriginal={isFanwork ? detail.sourceOriginal : undefined}
          sourceFanwork={isFanwork ? detail.sourceFanwork : undefined}
          variantTail={
            /* #430 挂载分帧：关联块/评论区/推荐区延后一拍（双 rAF + 低优先级），
               降低入场动画期主线程拥塞。 */
            <DeferredMount>
              <OverlayRelatedBlock
                /* #846：统一右栏不挂 ContentSidebar，关联 IP 行补在关联块首位
                   （与移动侧栏同一数据源/门槛）。 */
                ip={isFanwork && content.ip?.id && content.ip.name ? content.ip : undefined}
                sourceOriginal={isFanwork ? detail.sourceOriginal ?? null : null}
                series={content.series_memberships ?? []}
                onOpenRelated={handleOpenEntry}
                onNavigateSeries={handleNavigateInOverlay}
              />
              <section className="rounded-md border border-border bg-card p-4">
                <CommentSection contentId={content.id} />
              </section>
              <RelatedContents
                contentId={content.id}
                zone={isFanwork ? "fanwork" : "original"}
                contentType={content.content_type ?? "other"}
                category={content.category}
                ipId={isFanwork ? content.ip?.id ?? content.ip_id : undefined}
                relatedFanworks={relatedFanworksSummary}
                relatedFanworksSlot={{ ...relatedFanworksSlot, titleKey: "media.related.relatedTitle" }}
                onOpenDetail={handleOpenEntry}
                /* D1 #858：浮层统一右栏推荐区 960 起可见（旧 1100 门只约束
                   独立详情页等其余调用方）。 */
                minViewportPx={960}
              />
            </DeferredMount>
          }
        />
      );
    } else {
      /* 移动 <960 单列（既有契约不回归）：行内画廊/CoverImage + header 工具栏 +
         ContentSidebar（CSS <lg 自隐藏）+ 连续浏览钩子与到底提示。 */
      statusBody = (
        <div className="mx-auto flex w-full max-w-[1280px] gap-6">
          <div className="min-w-0 flex-1">
            <ContentDetail
              data={{ ...content, attachments: detail.attachments, tags: detail.tags }}
              coverSync
              deferTail
              coverHoldSrc={motionHoldSrc}
              inlineFollowClassName="lg:hidden"
              sourceOriginal={isFanwork ? detail.sourceOriginal : undefined}
              sourceFanwork={isFanwork ? detail.sourceFanwork : undefined}
              onGalleryReachEnd={handleReachEnd}
              galleryEndHint={atContextEnd}
              onOpenRelatedDetail={handleOpenEntry}
              onNavigateInOverlay={handleNavigateInOverlay}
            />
          </div>

          {sidebar}
        </div>
      );
    }
  }

  if (isDesktop) {
    return (
      <OverlayVariantLayout
        media={chainMedia}
        onFirstMediaSettled={(state) => setCoverReady(state === "ready")}
        holdSrc={motionHoldSrc}
        /* D2 #859 冻结窗口 = 入场转场起跑（motionStarted）→落定（motionSettled）：
           起跑前比例修正即时生效（壳层 opacity 0 不可见），窗口内锁存起跑比例
           零跳宽，落定后恢复 240ms 逐张过渡。 */
        freezeGeometry={motionStarted && !motionSettled}
        onPaneMeasured={handlePaneMeasured}
        active={active}
        onMediaDimensions={handleMediaDimensions}
      >
        {statusBody}
      </OverlayVariantLayout>
    );
  }
  return statusBody;
}
