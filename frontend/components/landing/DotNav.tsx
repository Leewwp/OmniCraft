"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";

// 右缘磨砂胶囊圆点导航（R5 Q22-A）：5 点、hover 章节名标签、
// IntersectionObserver 跟踪当前章（rootMargin 中线判定）；
// 窄屏（≤820px）由 CSS 隐藏且内容自然可达；无滚动事件劫持。

export const LANDING_SECTIONS = ["hero", "surfaces", "mechanics", "showcase", "cta"] as const;

export type LandingSectionId = (typeof LANDING_SECTIONS)[number];

const SECTION_LABEL_KEYS: Record<LandingSectionId, string> = {
  hero: "dots.hero",
  surfaces: "dots.surfaces",
  mechanics: "dots.mechanics",
  showcase: "dots.showcase",
  cta: "dots.cta",
};

export function DotNav() {
  const t = useTranslations("landing");
  const [activeId, setActiveId] = useState<LandingSectionId>("hero");

  useEffect(() => {
    if (typeof IntersectionObserver === "undefined") return;
    const visible = new Map<string, boolean>();
    const io = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          visible.set(entry.target.id, entry.isIntersecting);
        }
        const current = LANDING_SECTIONS.find((id) => visible.get(id));
        if (current) setActiveId(current);
      },
      { rootMargin: "-50% 0px -50% 0px" },
    );
    for (const id of LANDING_SECTIONS) {
      const el = document.getElementById(id);
      if (el) io.observe(el);
    }
    return () => io.disconnect();
  }, []);

  return (
    <nav className="dots" data-testid="landing-dots" aria-label={t("showcase.title")}>
      {LANDING_SECTIONS.map((id) => (
        <a
          key={id}
          href={`#${id}`}
          data-l={t(SECTION_LABEL_KEYS[id])}
          className={activeId === id ? "on" : ""}
          aria-label={t(SECTION_LABEL_KEYS[id])}
          aria-current={activeId === id ? "true" : undefined}
        />
      ))}
    </nav>
  );
}
