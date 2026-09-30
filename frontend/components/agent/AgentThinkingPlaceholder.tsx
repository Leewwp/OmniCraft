"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";

/**
 * #727 发送后「正在思考…」占位：覆盖发送到首个可见内容之间的空窗
 * （TTFT 常态 1-3s，深度思考更长）。
 *
 * 合同：
 * - 计时纯客户端（起点 = 轮发起时间），不依赖服务端事件；秒数递增对
 *   读屏静默（aria-hidden），占位自身不挂 role=status/aria-live——
 *   transcript 容器已 aria-live，避免双重播报。
 * - 卸载即清理计时器；渲染条件（shouldShowThinkingPlaceholder）由调用方
 *   掌握，首个可见内容同帧移除。
 * - prefers-reduced-motion 下呼吸动画停用（motion-safe 变体）。
 */
export function AgentThinkingPlaceholder({ startedAt }: { startedAt: number }) {
  const t = useTranslations();
  const [elapsedSec, setElapsedSec] = useState(() => Math.max(0, Math.floor((Date.now() - startedAt) / 1000)));

  useEffect(() => {
    setElapsedSec(Math.max(0, Math.floor((Date.now() - startedAt) / 1000)));
    const timer = setInterval(() => {
      setElapsedSec(Math.max(0, Math.floor((Date.now() - startedAt) / 1000)));
    }, 1000);
    return () => clearInterval(timer);
  }, [startedAt]);

  return (
    <div
      className="flex items-center gap-2 text-sm text-muted-foreground"
      data-testid="agent-thinking-placeholder"
    >
      <span className="flex items-center gap-0.5" aria-hidden="true">
        <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/70 motion-safe:animate-pulse" />
        <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/70 motion-safe:animate-pulse [animation-delay:200ms]" />
        <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/70 motion-safe:animate-pulse [animation-delay:400ms]" />
      </span>
      <span>{t("agent.thinkingLabel")}</span>
      <span className="tabular-nums" aria-hidden="true" data-testid="agent-thinking-seconds">
        {elapsedSec}s
      </span>
    </div>
  );
}
