"use client";

import { useTranslations } from "next-intl";
import type { AgentCitation } from "@/lib/agent";
import { AgentCitationCard } from "@/components/agent/AgentCitationCard";

interface AgentCitationListProps {
  citations: AgentCitation[];
  onOpen: (citation: AgentCitation, trigger: HTMLElement) => void;
}

/** The shared citation list keeps the Agent workspace's citation contract in one place. */
export function AgentCitationList({ citations, onOpen }: AgentCitationListProps) {
  const t = useTranslations();

  if (citations.length === 0) return null;

  return (
    <section aria-label={t("agent.citations.title")} className="mt-1">
      <div className="flex items-baseline gap-2">
        <h3 className="text-sm font-medium text-fg-default">{t("agent.citations.title")}</h3>
        <span className="text-xs text-fg-muted">
          {t("agent.citations.count", { count: citations.length })}
        </span>
      </div>
      <ul className="mt-2 grid gap-2 sm:grid-cols-2">
        {citations.map((citation, index) => (
          <li key={`${citation.contentId}-${index}`}>
            <AgentCitationCard citation={citation} index={index} onOpen={onOpen} />
          </li>
        ))}
      </ul>
    </section>
  );
}
