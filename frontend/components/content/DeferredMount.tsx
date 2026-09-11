"use client";

import { useEffect, useState, startTransition, type ReactNode } from "react";

/**
 * 挂载分帧（#430）：浮窗重子树（评论区/关联内容块/侧栏）延后一拍挂载——
 * 双 rAF 越过入场动效的首帧提交，再经 startTransition 低优先级渲染，
 * 降低动画期主线程拥塞（图片延迟落地/加载动画卡顿的来源之一）。
 * 首帧渲染 null（SSR/客户端首绘一致，无 hydration 差异）；占位由调用方
 * 块级流式布局天然承担，延后挂载不产生跳动。
 */
export function DeferredMount({ defer = true, children }: { defer?: boolean; children: ReactNode }) {
  const [mounted, setMounted] = useState(!defer);

  useEffect(() => {
    if (!defer) return;
    let active = true;
    const frame = window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => {
        if (!active) return;
        startTransition(() => setMounted(true));
      });
    });
    return () => {
      active = false;
      window.cancelAnimationFrame(frame);
    };
  }, [defer]);

  if (!mounted) return null;
  return <>{children}</>;
}
