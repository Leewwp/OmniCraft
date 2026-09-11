"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { ChevronDown, ChevronUp } from "lucide-react";
import type { AgentCitation } from "@/lib/agent";
import { AgentCitationCard } from "@/components/agent/AgentCitationCard";

interface AgentCitationListProps {
  citations: AgentCitation[];
  onOpen: (citation: AgentCitation, trigger: HTMLElement) => void;
  /** 行内 [n] 锚定：当前高亮的引用序号（0 基）。 */
  highlightedIndex?: number | null;
}

/** #399 R1 引用折叠阈值：超过该条数默认收起（citation_max_count=12 配套——
    列表变长已获接受，前提是可折叠且默认不占版面）。 */
const FOLD_THRESHOLD = 4;

/**
 * The shared citation list keeps the Agent workspace's citation contract in one place.
 * #399 R1：折叠纯展示层——默认折叠由本组件内部状态承担，不触碰引用持久化/
 * 回填合并逻辑；行内 [n] 角标直开浮窗不受折叠影响。
 */
export function AgentCitationList({ citations, onOpen, highlightedIndex }: AgentCitationListProps) {
  const t = useTranslations();
  const [expanded, setExpanded] = useState(() => citations.length <= FOLD_THRESHOLD);

  if (citations.length === 0) return null;

  return (
    <section aria-label={t("agent.citations.title")} className="mt-1">
      <div className="flex items-baseline gap-2">
        <h3 className="text-sm font-medium text-fg-default">{t("agent.citations.title")}</h3>
        <span className="text-xs text-fg-muted">
          {t("agent.citations.count", { count: citations.length })}
        </span>
        <button
          type="button"
          onClick={() => setExpanded((value) => !value)}
          aria-expanded={expanded}
          className="ml-auto inline-flex min-h-7 items-center gap-1 rounded-md px-2 text-xs text-fg-muted transition-colors hover:bg-canvas-default hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
        >
          {expanded ? (
            <>
              <ChevronUp className="h-3.5 w-3.5" aria-hidden="true" />
              {t("agent.citations.collapse")}
            </>
          ) : (
            <>
              <ChevronDown className="h-3.5 w-3.5" aria-hidden="true" />
              {t("agent.citations.expand", { count: citations.length })}
            </>
          )}
        </button>
      </div>
      {expanded && (
        <ul className="mt-2 grid gap-2 sm:grid-cols-2">
          {citations.map((citation, index) => (
            <li key={`${citation.contentId}-${index}`}>
              <AgentCitationCard
                citation={citation}
                index={index}
                onOpen={onOpen}
                highlighted={highlightedIndex === index}
              />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
