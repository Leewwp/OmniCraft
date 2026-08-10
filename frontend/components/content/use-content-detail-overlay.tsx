"use client";

import { useCallback, useRef, useState } from "react";
import { ContentDetailOverlay } from "@/components/content/ContentDetailOverlay";
import type { OverlaySource } from "@/components/content/ContentDetailOverlayLayer";

export interface OverlayOpenEntry {
  contentId: number;
  zone: "original" | "fanwork";
}

export interface ContentOverlayContextItem {
  id: number;
  zone: "original" | "fanwork";
}

export interface UseContentDetailOverlayOptions {
  /** 来源参数：决定返回文案语义（推荐流 / 分区页 / IP 页 / Agent 引用）。 */
  source: OverlaySource;
  /** #89 连续浏览：触发上下文列表与当前索引（移动端从卡片网格进入时传入）。 */
  contextList?: ContentOverlayContextItem[];
  contextIndex?: number;
}

/**
 * 全站共享浮层入口控制器（#64 C2+C6 / #68 权威）：推荐流、分区页、IP 页、
 * Agent 引用与独立详情页宿主共用同一份 open/close/focus/history 状态机，
 * 入口只保留来源参数、触发 ref 与上下文列表差异。
 */
export function useContentDetailOverlay(options: UseContentDetailOverlayOptions) {
  const [entry, setEntry] = useState<OverlayOpenEntry | null>(null);
  const returnFocusRef = useRef<HTMLElement | null>(null);

  const open = useCallback((next: OverlayOpenEntry, trigger: HTMLElement | null) => {
    returnFocusRef.current = trigger;
    setEntry(next);
  }, []);

  const handleOpenChange = useCallback((open: boolean) => {
    if (!open) setEntry(null);
  }, []);

  const overlayElement = entry ? (
    <ContentDetailOverlay
      key={`${entry.zone}:${entry.contentId}`}
      contentId={entry.contentId}
      zone={entry.zone}
      source={options.source}
      contextList={options.contextList}
      contextIndex={options.contextIndex}
      open
      onOpenChange={handleOpenChange}
      returnFocusRef={returnFocusRef}
    />
  ) : null;

  return { open, overlayElement };
}
