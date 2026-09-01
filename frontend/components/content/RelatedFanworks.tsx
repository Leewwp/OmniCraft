"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { AlertCircle, ArrowRight } from "lucide-react";
import { api } from "@/lib/api";
import { normalizeContentList } from "@/lib/content";
import { ContentCard, type ContentCardData } from "@/components/content/ContentCard";
import { Button } from "@/components/ui/button";

interface RelatedFanworksProps {
  sourceContentId: number;
  sourceZone: "original" | "fanwork";
  titleKey: string;
  createHref?: string;
  viewAllHref?: string;
  /** #90 浮层栈内打开卡片（source=zone-page 压栈）；不传保持整卡 Link 跳转。 */
  onOpenDetail?: (data: ContentCardData, trigger: HTMLElement) => void;
  /** #90 行数据落定后回报（RelatedContents 相似内容去重用）。 */
  onData?: (items: ContentCardData[]) => void;
  /** #90 嵌入模式：不渲染自身 bordered 容器（由宿主 RelatedContents 提供外层单容器）。 */
  embedded?: boolean;
}

type RowStatus = "loading" | "ready" | "error";

/** 相关二创/衍生作品行（ui-spec:2697）：正文后、评论区上方；横向滚动卡片行最多 8 张；
    total=0 隐藏；「查看全部」仅 total>8 且提供 viewAllHref 时显示。 */
export function RelatedFanworks({
  sourceContentId,
  sourceZone,
  titleKey,
  createHref,
  viewAllHref,
  onOpenDetail,
  onData,
  embedded = false,
}: RelatedFanworksProps) {
  const t = useTranslations();
  const [items, setItems] = useState<ContentCardData[]>([]);
  const [total, setTotal] = useState(0);
  const [status, setStatus] = useState<RowStatus>("loading");
  const [attempt, setAttempt] = useState(0);

  const onDataRef = useRef(onData);
  useEffect(() => {
    onDataRef.current = onData;
  }, [onData]);

  useEffect(() => {
    let cancelled = false;
    setStatus("loading");
    api
      .get<{ contents?: unknown[]; total?: number }>(
        `/api/v1/contents/${sourceContentId}/related-fanworks?page=1&page_size=8`,
      )
      .then((raw) => {
        if (cancelled) return;
        const normalized = normalizeContentList(raw.contents);
        setItems(normalized);
        setTotal(raw.total ?? 0);
        setStatus("ready");
        onDataRef.current?.(normalized);
      })
      .catch(() => {
        if (!cancelled) setStatus("error");
      });
    return () => {
      cancelled = true;
    };
  }, [sourceContentId, attempt]);

  const containerClass = embedded
    ? undefined
    : "rounded-lg border border-border-default bg-card p-4";

  if (status === "loading") {
    return (
      <div
        data-slot="related-fanworks-loading"
        aria-hidden="true"
        className={containerClass}
      >
        <div className="h-4 w-40 animate-pulse rounded bg-muted" />
        <div className="mt-3 flex gap-3 overflow-hidden">
          {[0, 1, 2, 3].map((index) => (
            <div key={index} className="h-44 w-[148px] shrink-0 animate-pulse rounded-lg bg-muted" />
          ))}
        </div>
      </div>
    );
  }

  if (status === "error") {
    return (
      <section
        data-slot="related-fanworks"
        aria-label={t(titleKey)}
        className={containerClass}
      >
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h2 className="text-sm font-semibold text-foreground">{t(titleKey)}</h2>
          <Button variant="outline" size="sm" onClick={() => setAttempt((value) => value + 1)}>
            {t("relatedFanworks.error.retry")}
          </Button>
        </div>
        <p className="mt-2 inline-flex items-center gap-1.5 text-xs text-muted-foreground">
          <AlertCircle className="h-3.5 w-3.5" aria-hidden="true" />
          {t("relatedFanworks.error.loadFailed")}
        </p>
      </section>
    );
  }

  if (total === 0) {
    return null;
  }

  const showViewAll = total > 8 && Boolean(viewAllHref);

  return (
    <section
      data-slot="related-fanworks"
      data-source-zone={sourceZone}
      aria-label={t(titleKey)}
      className={containerClass}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2
          data-slot="related-fanworks-title"
          className="text-sm font-semibold text-foreground"
        >
          {t(titleKey)}
          <span className="ml-1.5 text-xs font-normal text-muted-foreground">{total}</span>
        </h2>
        <div className="flex flex-wrap items-center gap-2">
          {showViewAll && viewAllHref && (
            <Link
              href={viewAllHref}
              className="inline-flex items-center gap-1 text-xs font-medium text-accent-emphasis transition-colors hover:text-accent-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
            >
              {t("relatedFanworks.actions.viewAll")}
              <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
            </Link>
          )}
          {createHref && (
            <Link
              href={createHref}
              className="inline-flex items-center justify-center gap-1.5 rounded-full bg-primary px-4 py-2 text-xs font-medium text-primary-foreground transition-all hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
            >
              {t("relatedFanworks.actions.create")}
            </Link>
          )}
        </div>
      </div>

      <div className="mt-3 flex gap-3 overflow-x-auto pb-1">
        {items.map((item) => (
          <ContentCard
            key={item.id}
            data={item}
            onOpenDetail={onOpenDetail}
            className="w-[148px] shrink-0 min-[701px]:w-[160px] min-[1101px]:w-[180px]"
          />
        ))}
      </div>
    </section>
  );
}
