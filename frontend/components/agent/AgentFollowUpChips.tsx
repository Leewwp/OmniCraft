"use client";

import { useTranslations } from "next-intl";

/**
 * 推荐追问药丸（ui-spec `## Component: AgentFollowUpChips`，SP-15 B #435）。
 * 动作药丸而非筛选控件：无 aria-pressed、无选中态；点击只把追问文本填入
 * composer 并聚焦，发送仍由用户回车确认。仅 grounded 轮 done 事件携带
 * follow_ups 时渲染（v1 不落库，历史回放不出现）。
 */
interface AgentFollowUpChipsProps {
  followUps: string[];
  /** 填入 composer（不发送）；工作台负责聚焦输入框。 */
  onFill: (query: string) => void;
}

export function AgentFollowUpChips({ followUps, onFill }: AgentFollowUpChipsProps) {
  const t = useTranslations("agent.workspace");
  if (followUps.length === 0) return null;
  return (
    <div
      role="group"
      aria-label={t("followUpsLabel")}
      className="flex flex-wrap gap-2"
    >
      {followUps.map((followUp) => (
        <button
          key={followUp}
          type="button"
          onClick={() => onFill(followUp)}
          aria-label={`${followUp} — ${t("followUpFill")}`}
          className="inline-flex items-center rounded-full border border-border-default bg-canvas-default px-3.5 py-1.5 text-sm text-fg-default transition-colors duration-150 hover:border-accent-emphasis hover:bg-accent-subtle hover:text-accent-emphasis focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {followUp}
        </button>
      ))}
    </div>
  );
}
