"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { AlertCircle } from "lucide-react";
import { api } from "@/lib/api";
import { normalizeContentList } from "@/lib/content";
import { ContentCard, type ContentCardData } from "@/components/content/ContentCard";
import { RelatedFanworks } from "@/components/content/RelatedFanworks";
import { Button } from "@/components/ui/button";

/** 关联原创/二创行插槽（复用 RelatedFanworks 组件，ui-spec:2697 合同）。 */
export interface RelatedContentsFanworksSlot {
  sourceContentId: number;
  sourceZone: "original" | "fanwork";
  titleKey: string;
  createHref?: string;
  viewAllHref?: string;
}

export interface RelatedContentsProps {
  className?: string;
  contentId: number;
  zone: "original" | "fanwork";
  contentType: string;
  category?: string;
  ipId?: number;
  /** #96 合同：关联行已加载的 id/zone 摘要（相似内容去重，可选；RF 行 onData 兜底）。 */
  relatedFanworks?: Array<{ id: number; title: string; zone: "original" | "fanwork" }>;
  /** 关联原创/二创行插槽；不传则不渲染关联行（仅相似行 + 到底提示）。 */
  relatedFanworksSlot?: RelatedContentsFanworksSlot;
  /** 浮层栈内打开卡片（source=zone-page 压栈）；不传保持整卡 Link 跳转。 */
  onOpenDetail?: (data: ContentCardData, trigger: HTMLElement) => void;
}

type SimilarStatus = "loading" | "ready" | "error";

/** 相似内容行最大展示条数（去重后，ui-spec:2761 Key Constraints）。 */
const SIMILAR_MAX_ITEMS = 8;
/** 相似内容请求固定 page_size（spec §显示层：sort=hot&page_size=12）。 */
const SIMILAR_PAGE_SIZE = 12;

/** 桌面/web 相关内容块（ui-spec:2761 #80/#90 权威）：正文与评论区之后展示，
    关联原创/二创行（复用 RelatedFanworks）+ 相似内容行（固定复用列表 API 合同，
    不新增临时 similar endpoint）+ 「已经到底了」提示；不做自动加载下一篇。
    移动端不渲染本块（移动连续浏览由 Overlay 承担）。 */
export function RelatedContents({
  className,
  contentId,
  zone,
  contentType,
  category,
  ipId,
  relatedFanworks,
  relatedFanworksSlot,
  onOpenDetail,
}: RelatedContentsProps) {
  const t = useTranslations();
  const [isDesktop, setIsDesktop] = useState(false);
  const [similarItems, setSimilarItems] = useState<ContentCardData[]>([]);
  const [similarStatus, setSimilarStatus] = useState<SimilarStatus>("loading");
  const [similarAttempt, setSimilarAttempt] = useState(0);
  const [relatedIds, setRelatedIds] = useState<Set<number>>(() => new Set());
  /** 关联行是否已实际加载出数据（RF 组件 total=0 时自行隐藏）。 */
  const [relatedLoaded, setRelatedLoaded] = useState(false);
  /** 关联行请求是否已落定（成功回调 onData 触发；错误态保持 false，走 RF 行内错误 UI）。 */
  const [relatedSettled, setRelatedSettled] = useState(false);

  /* 桌面/web 视口判定（与 #88/#89 的 min-width: 1100px 全局三档一致）；
     SSR/移动端不渲染、不发起任何请求。 */
  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") return;
    const mediaQuery = window.matchMedia("(min-width: 1100px)");
    const update = () => setIsDesktop(mediaQuery.matches);
    update();
    mediaQuery.addEventListener("change", update);
    return () => mediaQuery.removeEventListener("change", update);
  }, []);

  /* 相似内容：固定复用列表 API（同 zone、同 content_type、同 category，fanwork
     有 IP 时再带 ip_id，sort=hot&page_size=12）。 */
  useEffect(() => {
    if (!isDesktop) return;
    let cancelled = false;
    setSimilarStatus("loading");
    const params = new URLSearchParams();
    params.set("zone", zone);
    params.set("content_type", contentType);
    if (category) params.set("category", category);
    if (zone === "fanwork" && ipId) params.set("ip_id", String(ipId));
    params.set("sort", "hot");
    params.set("page_size", String(SIMILAR_PAGE_SIZE));
    api
      .get<{ contents?: unknown[]; total?: number }>(`/api/v1/contents?${params.toString()}`)
      .then((raw) => {
        if (cancelled) return;
        setSimilarItems(normalizeContentList(raw.contents));
        setSimilarStatus("ready");
      })
      .catch(() => {
        if (!cancelled) setSimilarStatus("error");
      });
    return () => {
      cancelled = true;
    };
  }, [isDesktop, contentId, zone, contentType, category, ipId, similarAttempt]);

  const handleRelatedData = (items: ContentCardData[]) => {
    setRelatedIds((prev) => {
      const next = new Set(prev);
      for (const item of items) next.add(item.id);
      return next;
    });
    setRelatedSettled(true);
    if (items.length > 0) setRelatedLoaded(true);
  };

  /* 客户端去重：排除当前内容与关联行（prop 摘要 + RF 行实际加载）重复项。 */
  const relatedIdSet = useMemo(() => {
    const set = new Set(relatedIds);
    for (const item of relatedFanworks ?? []) set.add(item.id);
    return set;
  }, [relatedIds, relatedFanworks]);

  const visibleSimilar = useMemo(
    () =>
      similarItems
        .filter((item) => item.id !== contentId && !relatedIdSet.has(item.id))
        .slice(0, SIMILAR_MAX_ITEMS),
    [similarItems, contentId, relatedIdSet],
  );

  if (!isDesktop) return null;

  const relatedRow = relatedFanworksSlot ? (
    <RelatedFanworks
      {...relatedFanworksSlot}
      embedded
      onOpenDetail={onOpenDetail}
      onData={handleRelatedData}
    />
  ) : null;

  /* 空分支：两行都无数据时不渲染空块标题，仅保留到底提示（ui-spec empty 态）。
     关联行须已落定（relatedSettled）才可判定为空，避免加载中闪切 hint-only。 */
  const blockEmpty =
    similarStatus === "ready" &&
    visibleSimilar.length === 0 &&
    (!relatedFanworksSlot || relatedSettled) &&
    !relatedLoaded;

  if (blockEmpty) {
    return (
      <div data-slot="related-contents-end" className={className}>
        <p className="py-3 text-center text-xs text-muted-foreground">
          {t("media.related.endReached")}
        </p>
      </div>
    );
  }

  return (
    <section
      data-slot="related-contents"
      aria-label={t("media.related.relatedTitle")}
      className={className}
    >
      <div
        data-slot="related-contents-box"
        className="rounded-lg border border-border-default bg-canvas-default p-4"
      >
        {relatedRow}

        {similarStatus === "loading" ? (
          <div data-slot="related-contents-similar-loading" aria-hidden="true">
            <div className="h-4 w-40 animate-pulse rounded bg-muted" />
            <div className="mt-3 flex gap-3 overflow-hidden">
              {[0, 1, 2, 3].map((index) => (
                <div key={index} className="h-44 w-[148px] shrink-0 animate-pulse rounded-lg bg-muted" />
              ))}
            </div>
          </div>
        ) : similarStatus === "error" ? (
          <div
            data-slot="related-contents-similar-error"
            className={relatedRow ? "mt-6" : undefined}
          >
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h2 className="text-sm font-semibold text-foreground">
                {t("media.related.similarTitle")}
              </h2>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setSimilarAttempt((value) => value + 1)}
              >
                {t("media.related.error.retry")}
              </Button>
            </div>
            <p className="mt-2 inline-flex items-center gap-1.5 text-xs text-muted-foreground">
              <AlertCircle className="h-3.5 w-3.5" aria-hidden="true" />
              {t("media.related.error.loadFailed")}
            </p>
          </div>
        ) : visibleSimilar.length > 0 ? (
          <div className={relatedRow ? "mt-6" : undefined}>
            <h2
              data-slot="related-contents-similar-title"
              className="text-sm font-semibold text-foreground"
            >
              {t("media.related.similarTitle")}
            </h2>
            <div
              data-slot="related-contents-similar"
              aria-label={t("media.related.a11y.similarList")}
              className="mt-3 flex gap-3 overflow-x-auto pb-1"
            >
              {visibleSimilar.map((item) => (
                <ContentCard
                  key={item.id}
                  data={item}
                  onOpenDetail={onOpenDetail}
                  className="w-[148px] shrink-0 min-[701px]:w-[160px] min-[1101px]:w-[180px]"
                />
              ))}
            </div>
          </div>
        ) : null}

        <div data-slot="related-contents-end">
          <p className="py-3 text-center text-xs text-muted-foreground">
            {t("media.related.endReached")}
          </p>
        </div>
      </div>
    </section>
  );
}
