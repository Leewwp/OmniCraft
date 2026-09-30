"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Brain } from "lucide-react";

import { AgentMetaPill } from "@/components/agent/AgentMetaPill";

interface AgentThinkingBlockProps {
  content: string;
  /** 流式中：自动展开并持续跟随；翻转为 false 时自动折叠（DeepSeek 形态）。 */
  streaming: boolean;
}

/**
 * 思考折叠区（A-06 三层生成形态第一层）：think_delta 流式展开 → 完成自动折叠；
 * 仅展示层，不参与引用复验（R2-Q7）。历史回放（phase="think" 行）复用本组件，
 * 默认折叠可手动展开。#719：折叠头统一为 AgentMetaPill 胶囊（仅外观，
 * 流式位置与自动折叠语义不变）。
 */
export function AgentThinkingBlock({ content, streaming }: AgentThinkingBlockProps) {
  const t = useTranslations();
  const [open, setOpen] = useState(streaming);
  const bodyRef = useRef<HTMLDivElement | null>(null);

  /* 流式结束自动折叠；用户手动展开不被覆盖（只在 streaming 边沿收起一次）。 */
  const wasStreaming = useRef(streaming);
  useEffect(() => {
    if (wasStreaming.current && !streaming) setOpen(false);
    wasStreaming.current = streaming;
  }, [streaming]);

  if (content.trim() === "") return null;

  return (
    <div className="max-w-[85%] space-y-1.5">
      <AgentMetaPill
        icon={Brain}
        label={streaming ? t("agent.thinking.streamingLabel") : t("agent.thinking.label")}
        expanded={open}
        streaming={streaming}
        onClick={() => setOpen((value) => !value)}
      />
      {open && (
        <div
          ref={bodyRef}
          className="max-h-64 overflow-y-auto whitespace-pre-wrap rounded-md border border-border-default bg-card px-3 py-2 text-xs leading-5 text-fg-muted"
        >
          {content}
        </div>
      )}
    </div>
  );
}
