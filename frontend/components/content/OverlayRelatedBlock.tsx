"use client";

import Link from "next/link";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useTranslations } from "next-intl";
import type { SeriesMembership, SourceSummary } from "@/lib/content";

interface OverlayRelatedBlockProps {
  /** ⓪ 关联 IP 行（#846：仅 fanwork 传入；有封面显签名图、无封面显名称首两字，
   *  整行 Link 跳 /ip/{id}——IP 页不在浮层导航栈内，与侧栏 IP 卡同款跳转）。 */
  ip?: { id?: number; name?: string; cover_url?: string };
  /** ① 二创关联的原创（仅 fanwork 且存在内容级来源时传入；点击浮窗内压栈打开）。 */
  sourceOriginal?: SourceSummary | null;
  /** ② 同系列跳转：取第一个系列（系列名 + 第 X/Y 篇 + 上一章/下一章，边界禁用）。 */
  series?: SeriesMembership[];
  /** trigger 透传给浮层压栈（弹层时焦点还给触发钮，与关联列表行同一契约）。 */
  onOpenRelated: (entry: { id: number; zone?: string }, trigger: HTMLElement) => void;
  onNavigateSeries: (contentId: number, trigger?: HTMLElement | null) => void;
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
 * 统一版式右栏的关联块（D1 #858 块序钉死 + #846 IP 行；承袭 #397 胜者记录 §7）：
 * 位置 = 内容详情之后、评论区之前；内部顺序 = ⓪关联 IP → ①关联的原创 → ②同系列
 * 跳转（第一个系列）。衍生二创列表（旧 ③）已从本块拆出——随评论区之后的推荐区
 * （RelatedContents 关联行 + 相似推荐）渲染，避免推荐内容出现在创作者信息之前
 * （全横集 split 版式的历史缺陷）。无任何关联时整块不渲染。
 */
export function OverlayRelatedBlock({
  ip,
  sourceOriginal,
  series,
  onOpenRelated,
  onNavigateSeries,
}: OverlayRelatedBlockProps) {
  const t = useTranslations();
  const firstSeries = series?.[0];
  const seriesValid = isValidMembership(firstSeries);
  const hasSourceOriginal = Boolean(sourceOriginal?.id && sourceOriginal.title.trim());
  /* #846：行渲染门槛 = id + 非空名称（与侧栏 IP 卡一致）；派生局部量让
     TS 在行内完成收窄（ipName 空串只可能出现在 ipRow 未设的分支）。 */
  const ipName = ip?.name?.trim() ?? "";
  const ipRow = ip?.id && ipName ? ip : undefined;

  if (!ipRow && !hasSourceOriginal && !seriesValid) return null;

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
        {/* ⓪ 关联 IP 行（#846）：置于块首（zone 级主关联，与侧栏 IP 卡层级一致）；
            有封面显签名图、无封面显名称首两字（IPCard 同款 fallback）。 */}
        {ipRow && (
          <Link
            href={`/ip/${ipRow.id}`}
            data-slot="related-ip-link"
            aria-label={t("overlayVariant.ipRowA11y", { name: ipName })}
            className="flex w-full items-center gap-2 rounded-md border border-border px-2.5 py-2 text-left transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          >
            <span className="h-10 w-10 shrink-0 overflow-hidden rounded-md border border-border bg-muted">
              {ipRow.cover_url ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={ipRow.cover_url} alt="" loading="lazy" className="h-full w-full object-cover" />
              ) : (
                <span className="flex h-full w-full items-center justify-center text-xs font-semibold text-muted-foreground">
                  {ipName.slice(0, 2)}
                </span>
              )}
            </span>
            <span className="shrink-0 rounded-full bg-primary/10 px-2 py-0.5 text-[11px] font-medium text-primary">
              {t("overlayVariant.ipBadge")}
            </span>
            <span className="min-w-0 flex-1 truncate text-sm text-foreground">{ipName}</span>
            <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          </Link>
        )}

        {hasSourceOriginal && sourceOriginal && (
          <button
            type="button"
            data-slot="related-source-btn"
            onClick={(event) => onOpenRelated({ id: sourceOriginal.id, zone: "original" }, event.currentTarget)}
            aria-label={t("contentDetailOverlay.openRelated", { title: sourceOriginal.title })}
            className="flex w-full items-center gap-2 rounded-md border border-border px-2.5 py-2 text-left transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
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
              onClick={(event) => previous && onNavigateSeries(previous.id, event.currentTarget)}
              aria-label={
                previous
                  ? t("overlayVariant.previousChapterA11y", { title: previous.title })
                  : t("overlayVariant.previousChapter")
              }
              className="inline-flex h-7 shrink-0 items-center gap-0.5 rounded-md border border-border px-2 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-40"
            >
              <ChevronLeft className="h-3.5 w-3.5" aria-hidden="true" />
              {t("overlayVariant.previousChapter")}
            </button>
            <button
              type="button"
              disabled={!next}
              onClick={(event) => next && onNavigateSeries(next.id, event.currentTarget)}
              aria-label={
                next ? t("overlayVariant.nextChapterA11y", { title: next.title }) : t("overlayVariant.nextChapter")
              }
              className="inline-flex h-7 shrink-0 items-center gap-0.5 rounded-md border border-border px-2 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-40"
            >
              {t("overlayVariant.nextChapter")}
              <ChevronRight className="h-3.5 w-3.5" aria-hidden="true" />
            </button>
          </div>
        )}
      </div>
    </section>
  );
}
