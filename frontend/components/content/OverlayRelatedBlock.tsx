"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import { useTranslations } from "next-intl";
import type { RelatedCardEntry } from "@/components/content/ContentSidebar";
import type { SeriesMembership, SourceSummary } from "@/lib/content";

interface OverlayRelatedBlockProps {
  /** ① 二创关联的原创（仅 fanwork 且存在内容级来源时传入；点击浮窗内压栈打开）。 */
  sourceOriginal?: SourceSummary | null;
  /** ② 同系列跳转：取第一个系列（系列名 + 第 X/Y 篇 + 上一章/下一章，边界禁用）。 */
  series?: SeriesMembership[];
  /** ③ 衍生二创/相关二创列表（浮层层已拉取的 related-fanworks 合同）。 */
  related: RelatedCardEntry[];
  relatedLabelKey: string;
  onOpenRelated: (entry: { id: number; zone?: string }, trigger: HTMLElement) => void;
  onNavigateSeries: (contentId: number) => void;
}

function isValidMembership(value: SeriesMembership | undefined): value is SeriesMembership {
  return Boolean(
    value &&
      Number.isInteger(value.series_id) &&
      value.series_id > 0 &&
      value.series_title.trim() &&
      Number.isInteger(value.current_index) &&
      value.current_index > 0 &&
      Number.isInteger(value.total) &&
      value.total >= value.current_index,
  );
}

function isValidTarget(value: SeriesMembership["previous"] | undefined): boolean {
  return Boolean(value && Number.isInteger(value.id) && value.id > 0 && value.title.trim());
}

/**
 * 竖屏集新版布局右栏的关联内容块（#397，胜者记录 §7 布局钉死）：
 * 位置 = 内容详情之后、评论之前；内部顺序 = ①关联的原创 → ②同系列跳转（第一个系列）
 * → ③衍生二创列表；无任何关联时整块不渲染。
 */
export function OverlayRelatedBlock({
  sourceOriginal,
  series,
  related,
  relatedLabelKey,
  onOpenRelated,
  onNavigateSeries,
}: OverlayRelatedBlockProps) {
  const t = useTranslations();
  const firstSeries = series?.[0];
  const seriesValid = isValidMembership(firstSeries);
  const hasSourceOriginal = Boolean(sourceOriginal?.id && sourceOriginal.title.trim());

  if (!hasSourceOriginal && !seriesValid && related.length === 0) return null;

  const previous =
    seriesValid && firstSeries.current_index > 1 && isValidTarget(firstSeries.previous)
      ? firstSeries.previous
      : undefined;
  const next =
    seriesValid && firstSeries.current_index < firstSeries.total && isValidTarget(firstSeries.next)
      ? firstSeries.next
      : undefined;

  return (
    <section
      data-slot="overlay-related-block"
      aria-label={t("overlayVariant.relatedTitle")}
      className="rounded-md border border-border bg-card p-4"
    >
      <h2 className="text-sm font-semibold text-foreground">{t("overlayVariant.relatedTitle")}</h2>
      <div className="mt-2 space-y-2">
        {hasSourceOriginal && sourceOriginal && (
          <button
            type="button"
            data-slot="related-source-btn"
            onClick={(event) => onOpenRelated({ id: sourceOriginal.id, zone: "original" }, event.currentTarget)}
            aria-label={t("contentDetailOverlay.openRelated", { title: sourceOriginal.title })}
            className="flex w-full items-center gap-2 rounded-md border border-border px-2.5 py-2 text-left transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <span className="shrink-0 rounded-full bg-primary/10 px-2 py-0.5 text-[11px] font-medium text-primary">
              {t("overlayVariant.originalBadge")}
            </span>
            <span className="min-w-0 flex-1 truncate text-sm text-foreground">{sourceOriginal.title}</span>
            <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          </button>
        )}

        {seriesValid && firstSeries && (
          <div className="flex items-center gap-2 rounded-md border border-border bg-muted/30 px-2.5 py-2">
            <div className="min-w-0 flex-1">
              <p className="truncate text-xs font-medium text-foreground">
                {t("overlayVariant.seriesLabel", { title: firstSeries.series_title })}
              </p>
              <p className="text-[11px] text-muted-foreground">
                {t("overlayVariant.seriesPosition", {
                  current: firstSeries.current_index,
                  total: firstSeries.total,
                })}
              </p>
            </div>
            <button
              type="button"
              disabled={!previous}
              onClick={() => previous && onNavigateSeries(previous.id)}
              aria-label={
                previous
                  ? t("overlayVariant.previousChapterA11y", { title: previous.title })
                  : t("overlayVariant.previousChapter")
              }
              className="inline-flex h-7 shrink-0 items-center gap-0.5 rounded-md border border-border px-2 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-40"
            >
              <ChevronLeft className="h-3.5 w-3.5" aria-hidden="true" />
              {t("overlayVariant.previousChapter")}
            </button>
            <button
              type="button"
              disabled={!next}
              onClick={() => next && onNavigateSeries(next.id)}
              aria-label={
                next ? t("overlayVariant.nextChapterA11y", { title: next.title }) : t("overlayVariant.nextChapter")
              }
              className="inline-flex h-7 shrink-0 items-center gap-0.5 rounded-md border border-border px-2 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-40"
            >
              {t("overlayVariant.nextChapter")}
              <ChevronRight className="h-3.5 w-3.5" aria-hidden="true" />
            </button>
          </div>
        )}

        {related.length > 0 && (
          <ul data-slot="related-list" aria-label={t(relatedLabelKey)} className="space-y-1">
            {related.map((entry) => (
              <li key={entry.id}>
                <button
                  type="button"
                  onClick={(event) => onOpenRelated({ id: entry.id, zone: entry.zone }, event.currentTarget)}
                  aria-label={t("contentDetailOverlay.openRelated", { title: entry.title })}
                  className="flex w-full items-center gap-2.5 rounded-md p-1.5 text-left transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  <span className="h-10 w-10 shrink-0 overflow-hidden rounded-md border border-border bg-muted">
                    {entry.coverUrl ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img src={entry.coverUrl} alt="" loading="lazy" className="h-full w-full object-cover" />
                    ) : (
                      <span className="flex h-full w-full items-center justify-center text-xs font-semibold text-muted-foreground">
                        {entry.title.slice(0, 1)}
                      </span>
                    )}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm text-foreground">{entry.title}</span>
                    {entry.meta && (
                      <span className="block truncate text-xs text-muted-foreground">{entry.meta}</span>
                    )}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
