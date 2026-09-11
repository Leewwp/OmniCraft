"use client";

import { useTranslations } from "next-intl";
import type { AgentCitation } from "@/lib/agent";
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
 * id=agent-citation-{index} 供行内 [n] 角标滚动定位与高亮。
 * 输入必须来自 lib/agent.ts normalizer，畸形引用在边界被拒绝，本组件不做兜底渲染。
 */
export function AgentCitationCard({ citation, index, onOpen, highlighted = false }: AgentCitationCardProps) {
  const t = useTranslations();

  return (
    <button
      type="button"
      id={`agent-citation-${index}`}
      onClick={(event) => onOpen(citation, event.currentTarget)}
      className={cn(
        "flex h-auto w-full flex-col items-start gap-0.5 rounded-md border bg-card px-3 py-2 text-left transition-colors duration-150 hover:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-ring",
        highlighted ? "border-ring ring-2 ring-ring" : "border-border-default",
      )}
    >
      <span className="flex w-full items-center gap-2 text-sm font-medium text-accent-emphasis">
        <span className="text-xs text-fg-muted">{String(index + 1).padStart(2, "0")}</span>
        <span className="truncate">{citation.title}</span>
        <span className="ml-auto shrink-0 rounded border border-border-default px-1.5 py-0.5 text-xs font-normal text-fg-muted">
          {citation.zone === "original"
            ? t("agent.citations.zoneOriginal")
            : t("agent.citations.zoneFanwork")}
        </span>
      </span>
      {citation.excerpt && (
        <span className="line-clamp-2 w-full pl-6 text-xs text-fg-muted">{citation.excerpt}</span>
      )}
    </button>
  );
}
