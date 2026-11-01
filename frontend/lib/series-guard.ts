import type { SeriesContentSummary, SeriesMembership } from "@/lib/content";

/**
 * 系列合法性守卫的单一来源：统一版式右栏关联块（OverlayRelatedBlock，membership
 * 可能缺省）与系列导航（SeriesNav，上游已 filter 保证非空）共用同一套判定，
 * 守卫规则变更只改这一份。判定内核：series_id / current_index / total 三字段
 * 均为正整数且 total ≥ current_index，系列标题 trim 后非空。
 */
export function isValidSeriesMembership(value: SeriesMembership | undefined): value is SeriesMembership {
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

/**
 * 步进/章节目标的统一形状校验：id 为正整数且标题 trim 后非空。
 * 泛型保持调用侧的具体类型（previous/next 摘要、目录章节项等结构兼容形状）。
 */
export function isValidSeriesStepTarget<T extends SeriesContentSummary>(value: T | undefined): value is T {
  return Boolean(value && Number.isInteger(value.id) && value.id > 0 && value.title.trim());
}

export type SeriesStep = "previous" | "next";

/**
 * 解析 membership 的上一章/下一章跳转目标：membership 不合法、越界
 * （current_index=1 无上一章 / =total 无下一章）或目标对象不合法时返回 undefined。
 */
export function resolveSeriesStepTarget(
  membership: SeriesMembership | undefined,
  step: SeriesStep,
): SeriesContentSummary | undefined {
  if (!isValidSeriesMembership(membership)) {
    return undefined;
  }
  const target = membership[step];
  const boundaryReached =
    step === "previous" ? membership.current_index <= 1 : membership.current_index >= membership.total;
  if (boundaryReached || !isValidSeriesStepTarget(target)) {
    return undefined;
  }
  return target;
}
