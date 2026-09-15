"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { ArrowUp } from "lucide-react";

/**
 * #537：内容三区（二创/原创/推荐）共用的“回到顶部”按钮。
 * window 自然滚动超过一屏后出现，回顶后隐藏；仅悬浮于视口右下角，
 * 不与内容卡交互冲突。
 */
export function BackToTopButton() {
  const t = useTranslations("common");
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const onScroll = () => setVisible(window.scrollY > window.innerHeight);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  if (!visible) return null;

  return (
    <button
      type="button"
      aria-label={t("backToTop")}
      onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}
      className="fixed bottom-6 right-6 z-40 inline-flex size-11 items-center justify-center rounded-full border border-border-default bg-card text-fg-muted shadow-md transition-colors duration-150 hover:bg-canvas-subtle hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
    >
      <ArrowUp className="size-5" aria-hidden="true" />
    </button>
  );
}
