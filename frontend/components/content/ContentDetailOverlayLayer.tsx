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
  onPush: (entry: OverlayEntry, trigger: HTMLElement | null) => void;
  onTitleChange: (title: string) => void;
  /** 层数据落定（含错误态）后通知浮层：入场转场可测量封面几何并启动。 */
  onMotionReady?: () => void;
}

type LayerStatus = "loading" | "default" | "forbidden" | "not-found" | "error";

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
  onTitleChange,
  onMotionReady,
}: ContentDetailOverlayLayerProps) {
  const t = useTranslations();
  const [status, setStatus] = useState<LayerStatus>("loading");
  const [detail, setDetail] = useState<NormalizedContentDetailResponse | null>(null);
  const [related, setRelated] = useState<ContentCardData[]>([]);
  const [relatedTotal, setRelatedTotal] = useState(0);
  const [attempt, setAttempt] = useState(0);

  const onTitleChangeRef = useRef(onTitleChange);
  useEffect(() => {
    onTitleChangeRef.current = onTitleChange;
  }, [onTitleChange]);

  const onMotionReadyRef = useRef(onMotionReady);
  useEffect(() => {
    onMotionReadyRef.current = onMotionReady;
  }, [onMotionReady]);

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

  return (
    <div className="mx-auto flex w-full max-w-[1280px] gap-6">
      <div className="min-w-0 flex-1">
        <ContentDetail
          data={{ ...content, attachments: detail.attachments, tags: detail.tags }}
          coverSync
        />
      </div>

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
    </div>
  );
}
