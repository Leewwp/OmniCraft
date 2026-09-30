"use client";

import { useTranslations } from "next-intl";
import type { AgentCitation } from "@/lib/agent";
import { ipCategoryLabelKey } from "@/lib/ip-categories";
import { cn } from "@/lib/utils";

interface AgentCitationCardProps {
  citation: AgentCitation;
  index: number;
  onOpen: (citation: AgentCitation, trigger: HTMLElement) => void;
  /** 行内引用 [n] 锚定目标：短暂高亮（A-06 纯展示层映射）。 */
  highlighted?: boolean;
}

/**
 * 站内有效引用卡片（ui-spec `## Page: /agent` 关键交互）：展示序号、标题、
 * 所属分区（原文/同人）与摘录；可聚焦按钮打开共享 ContentDetailOverlay。
 * zone="ip"（SP-19 G2-1，Q16）：「IP」徽标 + 11 类分类徽标（G1-3 词表单源）
 * + 简介摘录，无统计字段；点击跳 /ip/[id] 详情页（Q5，分流在 Workspace）。
 * id=agent-citation-{index} 供行内 [n] 角标滚动定位与高亮。
 * 输入必须来自 lib/agent.ts normalizer，畸形引用在边界被拒绝，本组件不做兜底渲染。
 */
export function AgentCitationCard({ citation, index, onOpen, highlighted = false }: AgentCitationCardProps) {
  const t = useTranslations();
  /* FT-5 (#697)：id 与目标查找用轮内全局编号（与正文角标点击命中同一
     体系；历史行/旧轮次无 number 时回退位置序）。
     #719：卡片「展示序号」改为可见列表位置的连续展示号（01..N，无空洞），
     与正文角标展示号共用「位置即展示号」的同一映射；原编号仍负责锚点 id。 */
  const number = citation.number ?? index + 1;
  const displayNumber = index + 1;

  return (
    <button
      type="button"
      id={`agent-citation-${number}`}
      aria-label={`${t("agent.citations.title")} ${displayNumber}：${citation.title}`}
      onClick={(event) => onOpen(citation, event.currentTarget)}
      className={cn(
        "flex h-auto w-full min-w-0 flex-col items-start gap-0.5 rounded-md border-[1.5px] bg-canvas-default px-3 py-2 text-left transition-colors duration-150 hover:bg-canvas-subtle focus:outline-none focus-visible:ring-1 focus:ring-ring",
        highlighted ? "border-accent-emphasis bg-accent-subtle/40" : "border-border-strong",
      )}
    >
      {/* #751 截断链：标题行 = 序号（定宽）+ 标题（min-w-0 + flex-1 才能真正
          truncate）+ 单个分区徽标（ml-auto shrink-0）。IP 双徽标下移元信息行
          （见下），不与标题争宽；aria-label 保留完整标题为可达名称。 */}
      <span className="flex w-full min-w-0 items-center gap-2 text-sm font-medium text-accent-emphasis">
        <span className="shrink-0 text-xs text-fg-muted">{String(displayNumber).padStart(2, "0")}</span>
        <span className="min-w-0 flex-1 truncate">{citation.title}</span>
        {citation.zone !== "ip" && (
          <span className="ml-auto shrink-0 rounded border border-border-default px-1.5 py-0.5 text-xs font-normal text-fg-muted">
            {citation.zone === "original"
              ? t("agent.citations.zoneOriginal")
              : t("agent.citations.zoneFanwork")}
          </span>
        )}
      </span>
      {citation.zone === "ip" && (
        /* IP 徽标独立元信息行：可换行（窄宽度/英文长分类不压标题）。 */
        <span className="flex w-full flex-wrap items-center gap-1 pl-6">
          <span className="rounded border border-border-default px-1.5 py-0.5 text-xs font-normal text-fg-muted">
            {t("agent.citations.zoneIP")}
          </span>
          {citation.category && (
            <span className="rounded border border-border-default px-1.5 py-0.5 text-xs font-normal text-fg-muted">
              {t(ipCategoryLabelKey(citation.category))}
            </span>
          )}
        </span>
      )}
      {citation.excerpt && (
        <span className="line-clamp-2 w-full pl-6 text-xs break-words text-fg-muted">{citation.excerpt}</span>
      )}
    </button>
  );
}
