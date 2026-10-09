"use client";

import { useEffect, useRef } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { LandingTopBar } from "@/components/landing/LandingTopBar";
import { DotNav } from "@/components/landing/DotNav";
import { SurfaceStage } from "@/components/landing/SurfaceStage";
import { MechanicsGrid } from "@/components/landing/MechanicsGrid";
import { ShowcaseSection } from "@/components/landing/ShowcaseSection";
import { getBrowserApiBase } from "@/lib/server-api";

// 访客落地页（票 #853，R5 视觉基准 + spec §九/§十/§十一/§十二）：
// 五章 = hero → 三大页面 → 四机制 2×2 → 精选陈列 → 尾章 CTA。
// 挂载期间给 html 动态加 landing-active 类承载整屏 scroll-snap（卸载移除，
// 社区页面零残留）；「已看过」cookie 一律不做。

export function GuestLanding() {
  const t = useTranslations("landing");
  const rootRef = useRef<HTMLDivElement | null>(null);

  // 整屏 scroll-snap 只在落地页挂载期间生效（html 类动态增删）。
  useEffect(() => {
    const root = document.documentElement;
    root.classList.add("landing-active");
    return () => {
      root.classList.remove("landing-active");
    };
  }, []);

  // 章节揭示：IntersectionObserver 入场加 .in；环境无 IO（单测/老浏览器）
  // 或 prefers-reduced-motion（CSS 全降级）时内容直接可见。
  useEffect(() => {
    const rootEl = rootRef.current;
    if (!rootEl) return;
    const targets = Array.from(rootEl.querySelectorAll<HTMLElement>(".reveal"));
    if (typeof IntersectionObserver === "undefined") {
      targets.forEach((el) => el.classList.add("in"));
      return;
    }
    const io = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            (entry.target as HTMLElement).classList.add("in");
            io.unobserve(entry.target);
          }
        }
      },
      { threshold: 0.15 },
    );
    targets.forEach((el) => io.observe(el));
    return () => io.disconnect();
  }, []);

  return (
    <div className="landing" ref={rootRef} data-testid="guest-landing">
      <LandingTopBar />
      <DotNav />

      {/* 01 HERO */}
      <section id="hero" className="chapter" data-testid="landing-hero">
        <div className="bg anim" style={{ backgroundImage: "url(/landing/v1-hero-workshop.jpg)" }} />
        <div className="scrim" />
        <div className="ct">
          <div className="hero-main">
            <div className="hkicker reveal">{t("hero.kicker")}</div>
            <h1 className="reveal">
              {t("hero.titleA")}
              <br />
              <em>{t("hero.titleEm")}</em>
              {t("hero.titleB")}
            </h1>
            <p className="hsub reveal">{t("hero.sub")}</p>
            <div className="hrow reveal">
              <Link href="/recommend" className="cta" data-testid="hero-enter">
                {t("hero.enter")}
              </Link>
              <Link href="/agent" className="ghost" data-testid="hero-ai">
                {t("hero.ai")}
              </Link>
            </div>
          </div>
        </div>
      </section>

      {/* 02 三大页面 */}
      <section id="surfaces" className="chapter" data-testid="landing-surfaces">
        <div className="ct">
          <div className="shead">
            <div>
              <div className="kicker reveal">{t("surfaces.kicker")}</div>
              <h2 className="reveal">{t("surfaces.title")}</h2>
            </div>
            <div className="shint reveal">{t("surfaces.hint")}</div>
          </div>
          <div className="stage-wrap reveal">
            <SurfaceStage />
          </div>
        </div>
      </section>

      {/* 03 特色玩法 */}
      <section id="mechanics" className="chapter" data-testid="landing-mechanics-section">
        <div className="bg" style={{ backgroundImage: "url(/landing/v1-community.jpg)" }} />
        <div className="scrim" />
        <div className="ct">
          <div className="kicker reveal">{t("mechanics.kicker")}</div>
          <h2 className="reveal">{t("mechanics.title")}</h2>
          <MechanicsGrid />
        </div>
      </section>

      {/* 04 精选陈列 */}
      <section id="showcase" className="chapter" data-testid="landing-showcase-section">
        <div className="ct">
          <div className="sc-head">
            <div>
              <div className="kicker reveal">{t("showcase.kicker")}</div>
              <h2 className="reveal">{t("showcase.title")}</h2>
            </div>
            <p className="lead reveal">{t("showcase.lead")}</p>
          </div>
          <ShowcaseSection apiBase={getBrowserApiBase()} />
        </div>
      </section>

      {/* 05 CTA */}
      <section id="cta" className="chapter" data-testid="landing-cta-section">
        <div className="bg dim" style={{ backgroundImage: "url(/landing/v1-community.jpg)" }} />
        <div className="scrim" />
        <div className="ct">
          <div className="fin">
            <div className="kicker reveal">{t("cta.kicker")}</div>
            <h2 className="reveal">{t("cta.title")}</h2>
            <p className="reveal">{t("cta.sub")}</p>
            <div className="row reveal">
              <Link href="/recommend" className="cta">
                {t("cta.enter")}
              </Link>
              <Link href="/agent" className="ghost">
                {t("cta.ai")}
              </Link>
              <Link href="/register" className="ghost">
                {t("cta.register")}
              </Link>
            </div>
          </div>
          <div className="foot">{t("foot")}</div>
        </div>
      </section>
    </div>
  );
}
