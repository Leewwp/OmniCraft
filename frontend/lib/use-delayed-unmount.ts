"use client";

import { useEffect, useState } from "react";

/**
 * #721 延迟卸载：open 翻 false 后保留挂载 exitMs 毫秒，供退出动画
 * （关闭方向滑出/遮罩淡出）走完再真正卸载。快速反复开关安全——关闭期间
 * 重新置 open 会取消卸载计时（同一元素过渡，无残影）。
 */
export function useDelayedUnmount(open: boolean, exitMs = 200): boolean {
  const [mounted, setMounted] = useState(open);

  useEffect(() => {
    if (open) {
      setMounted(true);
      return;
    }
    const timer = setTimeout(() => setMounted(false), exitMs);
    return () => clearTimeout(timer);
  }, [open, exitMs]);

  return mounted;
}
