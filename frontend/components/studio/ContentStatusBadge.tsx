"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

// 内容状态徽标（FIX-14 / ui-spec `Component: ContentStatusBadge`）：
// 仅非 published 状态显示——已发布是常态无需标注。药丸信息标签复用 Badge
// 原语（rounded-full、12px medium、20px 高），语义 token 着色，不引入任意色。

const STATUS_STYLES: Record<string, string> = {
  draft: "border-border bg-canvas-subtle text-fg-muted",
  pending: "border-primary/30 bg-primary/10 text-primary",
  under_review: "border-accent-emphasis/40 bg-accent-subtle text-accent-emphasis",
  banned: "border-border-destructive/30 bg-destructive/10 text-destructive",
};

const STATUS_KEYS: Record<string, string> = {
  draft: "studio.contents.statusDraft",
  pending: "studio.contents.statusPending",
  under_review: "studio.contents.statusUnderReview",
  banned: "studio.contents.statusBanned",
};

export function ContentStatusBadge({ status, className }: { status: string; className?: string }) {
  const t = useTranslations();
  const style = STATUS_STYLES[status];
  const labelKey = STATUS_KEYS[status];
  if (!style || !labelKey) return null;
  return (
    <Badge variant="outline" className={cn("text-[10px]", style, className)}>
      {t(labelKey)}
    </Badge>
  );
}
