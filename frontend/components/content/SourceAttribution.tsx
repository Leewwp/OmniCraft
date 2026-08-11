"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { GitBranch } from "lucide-react";

export interface SourceSummary {
  id: number;
  title: string;
  zone: "original" | "fanwork";
}

interface SourceAttributionProps {
  zone: "original" | "fanwork";
  sourceOriginalId?: number;
  sourceOriginal?: SourceSummary;
  sourceFanworkId?: number;
  sourceFanwork?: SourceSummary;
}

/** 灵感来源归因（ui-spec:2635）：仅 fanwork 且存在内容级来源时渲染一行小字链接；
    仅 IP 来源不渲染；来源 ID 存在但摘要缺失（内容已下架/删除）时渲染灰色不可点击文本。 */
export function SourceAttribution({
  zone,
  sourceOriginalId,
  sourceOriginal,
  sourceFanworkId,
  sourceFanwork,
}: SourceAttributionProps) {
  const t = useTranslations();

  if (zone !== "fanwork") {
    return null;
  }

  const originalSummary =
    sourceOriginal && sourceOriginal.zone === "original" && sourceOriginal.title.trim() !== ""
      ? sourceOriginal
      : undefined;
  const fanworkSummary =
    sourceFanwork && sourceFanwork.zone === "fanwork" && sourceFanwork.title.trim() !== ""
      ? sourceFanwork
      : undefined;

  if (originalSummary) {
    return (
      <div
        data-slot="source-attribution"
        className="inline-flex items-center gap-1 py-2 text-xs text-fg-muted"
      >
        <GitBranch className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        <Link
          href={`/original/${originalSummary.id}`}
          className="line-clamp-1 underline-offset-2 hover:underline focus-visible:underline focus-visible:outline-none"
          aria-label={t("sourceAttribution.a11y.original", { title: originalSummary.title })}
        >
          {t("sourceAttribution.original", { title: originalSummary.title })}
        </Link>
      </div>
    );
  }

  if (fanworkSummary) {
    return (
      <div
        data-slot="source-attribution"
        className="inline-flex items-center gap-1 py-2 text-xs text-fg-muted"
      >
        <GitBranch className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        <Link
          href={`/content/${fanworkSummary.id}`}
          className="line-clamp-1 underline-offset-2 hover:underline focus-visible:underline focus-visible:outline-none"
          aria-label={t("sourceAttribution.a11y.fanwork", { title: fanworkSummary.title })}
        >
          {t("sourceAttribution.fanwork", { title: fanworkSummary.title })}
        </Link>
      </div>
    );
  }

  const hasSourceId = sourceOriginalId != null || sourceFanworkId != null;
  if (!hasSourceId) {
    return null;
  }

  return (
    <div
      data-slot="source-attribution"
      aria-disabled="true"
      className="inline-flex items-center gap-1 py-2 text-xs text-fg-muted"
    >
      <GitBranch className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
      <span className="line-clamp-1">{t("sourceAttribution.unavailable")}</span>
    </div>
  );
}
