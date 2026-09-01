"use client";
/**
 * 【原型专用，随时可删】IP 详情页社区枢纽原型 — 共享小件。
 * 视觉遵循 design/design-system.md + 2026-09-01 精修方向：
 * 筛选选中态用 pill（IP 库基线），操作控件为 8px 矩形，同排等高（min-h-9）。
 */
import { useState } from "react";
import { Check, ArrowRight, SearchX } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { t } from "./copy";
import type { DiffSeg } from "./mock-data";

/** 筛选药丸（IP 库基线：选中 = accent-subtle 底 + accent-emphasis 描边 + Check）。 */
export function FilterPill({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={cn(
        "inline-flex min-h-9 shrink-0 items-center gap-1 whitespace-nowrap rounded-full border px-3 text-xs font-medium transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        active
          ? "border-accent-emphasis bg-accent-subtle font-semibold text-accent-emphasis"
          : "border-transparent text-muted-foreground hover:bg-muted hover:text-foreground",
      )}
    >
      {active && <Check className="size-3.5" aria-hidden="true" />}
      {children}
    </button>
  );
}

/** 头部统计项（关注 / 讨论 / 作品）。 */
export function StatItem({ value, label, onClick }: { value: number; label: string; onClick?: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={!onClick}
      className={cn(
        "flex items-baseline gap-1.5 rounded-md px-1.5 py-1 text-left",
        onClick && "transition-colors duration-150 hover:bg-canvas-subtle",
      )}
    >
      <span className="text-sm font-semibold tabular-nums">{value.toLocaleString()}</span>
      <span className="text-xs text-muted-foreground">{label}</span>
    </button>
  );
}

/** 搜索无结果空态（tab 内过滤后为空时使用）。 */
export function SearchEmpty({ q, onClear }: { q: string; onClear: () => void }) {
  return (
    <EmptyState
      icon={SearchX}
      title={t("hub.search.emptyTitle", { q })}
      description={t("hub.search.emptyDesc")}
      action={
        <Button variant="outline" onClick={onClear}>
          {t("hub.search.clear")}
        </Button>
      }
    />
  );
}

/** 提案状态徽章。 */
export function ProposalStatusBadge({ status }: { status: "open" | "adopted" | "rejected" }) {
  if (status === "open") {
    return (
      <span className="inline-flex items-center gap-1.5 rounded-full bg-[var(--tag-orange-bg)] px-2 py-0.5 text-xs font-medium text-[var(--tag-orange-fg)]">
        <span className="size-1.5 animate-pulse rounded-full bg-current motion-reduce:animate-none" aria-hidden="true" />
        {t("proposal.status.open")}
      </span>
    );
  }
  const adopted = status === "adopted";
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium",
        adopted
          ? "bg-[var(--tag-green-bg)] text-[var(--tag-green-fg)]"
          : "bg-[var(--tag-rose-bg)] text-[var(--tag-rose-fg)]",
      )}
    >
      {adopted ? t("proposal.status.adopted") : t("proposal.status.rejected")}
    </span>
  );
}

/** 简介行内 diff：删除线红字 + 下划线绿字；长文默认只展开前 8 行。 */
export function IntroDiff({ segments }: { segments: DiffSeg[] }) {
  const [expanded, setExpanded] = useState(false);
  const hasDel = segments.some((s) => s.kind === "del");
  const hasIns = segments.some((s) => s.kind === "ins");
  return (
    <div>
      <p
        className={cn(
          "text-sm leading-relaxed",
          !expanded && "line-clamp-8",
        )}
      >
        {segments.map((seg, i) => {
          if (seg.kind === "del") {
            return (
              <span
                key={i}
                className="text-destructive line-through decoration-destructive/70 bg-destructive/5"
              >
                {seg.text}
              </span>
            );
          }
          if (seg.kind === "ins") {
            return (
              <span
                key={i}
                className="text-[var(--tag-green-fg)] underline decoration-[var(--tag-green-fg)]/60 underline-offset-2 bg-[var(--tag-green-bg)]"
              >
                {seg.text}
              </span>
            );
          }
          return <span key={i}>{seg.text}</span>;
        })}
      </p>
      {(hasDel || hasIns) && (
        <div className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
          {hasDel && (
            <span className="inline-flex items-center gap-1">
              <span className="inline-block h-0.5 w-3 bg-destructive" aria-hidden="true" />
              {t("proposal.form.diffOld")}
            </span>
          )}
          {hasIns && (
            <span className="inline-flex items-center gap-1">
              <span className="inline-block h-0.5 w-3 bg-[var(--tag-green-fg)]" aria-hidden="true" />
              {t("proposal.form.diffNew")}
            </span>
          )}
          <button
            type="button"
            onClick={() => setExpanded((v) => !v)}
            className="ml-auto text-accent-emphasis transition-colors duration-150 hover:text-accent-hover"
          >
            {expanded ? t("common.collapse") : t("common.expand")}
          </button>
        </div>
      )}
    </div>
  );
}

/** 封面 diff：旧图 → 新图并排小卡（各 160px 宽）。 */
export function CoverDiff({ oldStyle, newStyle }: { oldStyle: React.CSSProperties; newStyle: React.CSSProperties }) {
  return (
    <div className="flex items-center gap-3">
      <figure className="w-40">
        <div className="h-28 w-40 rounded-lg border border-border" style={oldStyle} aria-hidden="true" />
        <figcaption className="mt-1 text-xs text-muted-foreground">{t("proposal.form.coverOld")}</figcaption>
      </figure>
      <ArrowRight className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      <figure className="w-40">
        <div className="h-28 w-40 rounded-lg border border-border" style={newStyle} aria-hidden="true" />
        <figcaption className="mt-1 text-xs text-muted-foreground">{t("proposal.form.coverNew")}</figcaption>
      </figure>
    </div>
  );
}

/** 标签 diff：+新标签 绿 / −移除标签 红 chips。 */
export function TagDiff({ mode, tag }: { mode: "add" | "remove"; tag: string }) {
  const add = mode === "add";
  return (
    <span
      className={cn(
        "inline-flex h-5 items-center gap-1 rounded-full px-2 text-xs font-medium",
        add
          ? "bg-[var(--tag-green-bg)] text-[var(--tag-green-fg)]"
          : "bg-[var(--tag-rose-bg)] text-[var(--tag-rose-fg)]",
      )}
    >
      <span aria-hidden="true">{add ? "+" : "−"}</span>
      <span className={add ? undefined : "line-through decoration-current/70"}>{tag}</span>
      <span className="opacity-70">{add ? t("common.tagAdd") : t("common.tagRemove")}</span>
    </span>
  );
}
