"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Brain, ChevronDown, Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

interface AgentThinkingBlockProps {
  content: string;
  /** 流式中：自动展开并持续跟随；翻转为 false 时自动折叠（DeepSeek 形态）。 */
  streaming: boolean;
}

/**
 * 思考折叠区（A-06 三层生成形态第一层）：think_delta 流式展开 → 完成自动折叠；
 * 仅展示层，不参与引用复验（R2-Q7）。历史回放（phase="think" 行）复用本组件，
 * 默认折叠可手动展开。
 */
export function AgentThinkingBlock({ content, streaming }: AgentThinkingBlockProps) {
  const t = useTranslations();
  const [open, setOpen] = useState(streaming);
  const bodyRef = useRef<HTMLDivElement>(null);

  /* 流式结束自动折叠；用户手动展开不被覆盖（只在 streaming 边沿收起一次）。 */
  const wasStreaming = useRef(streaming);
  useEffect(() => {
    if (wasStreaming.current && !streaming) setOpen(false);
    wasStreaming.current = streaming;
  }, [streaming]);

  if (content.trim() === "") return null;

  return (
    <div className="rounded-md border border-border-default bg-card">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        className="flex w-full items-center gap-2 px-3 py-2 text-xs text-fg-muted transition-colors hover:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-inset focus:ring-ring"
      >
        {streaming ? (
          <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin" aria-hidden="true" />
        ) : (
          <Brain className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        )}
        <span className="font-medium">
          {streaming ? t("agent.thinking.streamingLabel") : t("agent.thinking.label")}
        </span>
        <ChevronDown
          className={cn(
            "ml-auto h-3.5 w-3.5 shrink-0 transition-transform duration-150",
            open && "rotate-180",
          )}
          aria-hidden="true"
        />
      </button>
      {open && (
        <div
          ref={bodyRef}
          className="max-h-64 overflow-y-auto whitespace-pre-wrap border-t border-border-default px-3 py-2 text-xs leading-5 text-fg-muted"
        >
          {content}
        </div>
      )}
    </div>
  );
}
