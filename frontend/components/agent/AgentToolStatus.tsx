"use client";

import { useTranslations } from "next-intl";
import type { AgentStreamTool } from "@/lib/agent-stream";

interface AgentToolStatusProps {
  tools: AgentStreamTool[];
}

const TOOL_RESULT_KEYS: Record<string, Partial<Record<AgentStreamTool["status"], string>>> = {
  search_content: {
    running: "agent.tools.searchContentRunning",
    success: "agent.tools.searchContentSuccess",
    failed: "agent.tools.searchContentFailed",
    error: "agent.tools.searchContentFailed",
    skipped: "agent.tools.searchContentSkipped",
  },
  get_content_detail: {
    running: "agent.tools.getContentDetailRunning",
    success: "agent.tools.getContentDetailSuccess",
    failed: "agent.tools.getContentDetailFailed",
    error: "agent.tools.getContentDetailFailed",
    skipped: "agent.tools.getContentDetailSkipped",
  },
  get_usage_guide: {
    running: "agent.tools.getUsageGuideRunning",
    success: "agent.tools.getUsageGuideSuccess",
    failed: "agent.tools.getUsageGuideFailed",
    error: "agent.tools.getUsageGuideFailed",
    skipped: "agent.tools.getUsageGuideSkipped",
  },
  suggest_publish_metadata: {
    running: "agent.tools.suggestPublishMetadataRunning",
    success: "agent.tools.suggestPublishMetadataSuccess",
    failed: "agent.tools.suggestPublishMetadataFailed",
    error: "agent.tools.suggestPublishMetadataFailed",
    skipped: "agent.tools.suggestPublishMetadataSkipped",
  },
};

const FALLBACK_RESULT_KEYS: Record<AgentStreamTool["status"], string> = {
  running: "agent.tools.unknownRunning",
  success: "agent.tools.unknownSuccess",
  failed: "agent.tools.unknownFailed",
  error: "agent.tools.unknownFailed",
  skipped: "agent.tools.unknownSkipped",
};

/**
 * 工具状态摘要（ui-spec `## Page: /agent` 关键交互）：只展示用户向短文案与
 * 耗时摘要（≥1s 时显示），不暴露 raw args、system prompt 或内部推理。
 */
export function AgentToolStatus({ tools }: AgentToolStatusProps) {
  const t = useTranslations();
  if (tools.length === 0) return null;

  return (
    <ul aria-label={t("agent.tools.title")} className="flex flex-wrap gap-2">
      {tools.map((tool, index) => {
        const labelKey =
          TOOL_RESULT_KEYS[tool.name]?.[tool.status] ?? FALLBACK_RESULT_KEYS[tool.status];
        const seconds =
          tool.duration_ms !== undefined && tool.duration_ms >= 1000
            ? Math.round(tool.duration_ms / 1000)
            : null;
        return (
          <li
            key={`${tool.name}-${index}`}
            className="inline-flex items-center gap-1.5 rounded border border-border-default bg-canvas-default px-2 py-1 text-xs text-fg-muted"
          >
            <span className="text-fg-default">{t(labelKey)}</span>
            {seconds !== null && <span>{t("agent.tools.duration", { seconds })}</span>}
          </li>
        );
      })}
    </ul>
  );
}
