"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useTranslations } from "next-intl";
import {
  HIT_PAD_PX,
  resolveHitPane,
  resolvePaneAction,
  type PaneHitInput,
} from "@/lib/landing-interactions";

// 三大页面舞台（R5 Q23′-A + R5-② 动态命中区）：
// - 默认三窗等分定格；桌面 hover=放大播放预览（动态命中区：配图+配文实际
//   视觉边界包围盒，空白不触发也不复位=激活粘性、进入他页才切换、离舞台复位）；
// - 桌面整卡点击=进入路由；触屏轻点=放大（再点收起、预激活防抖）+「进入」按钮；
// - 键盘 focus=预览，Enter/空格=无条件导航（修正 R5 原型 primary() 把键盘
//   Enter 变成预览切换的缺陷）；导航统一走 goto 单出口。
// 视觉 class 与 R5 原型同名，样式见 app/(landing)/landing.css（.landing 域内）。

export const SURFACE_PANES = [
  { id: "original", href: "/original" },
  { id: "fanworks", href: "/fanworks" },
  { id: "agent", href: "/agent" },
] as const;

export type SurfacePaneId = (typeof SURFACE_PANES)[number]["id"];

// matchMedia 查询结果缓存：MQL 对象按 matchMedia 函数身份缓存一次，
// `.matches` 本身仍是实时值（媒体状态变化自动反映），mousemove 高频路径
// 不再逐事件重新解析查询串；测试更换 matchMedia 桩（函数身份变化）自动失效。
let cachedMql: MediaQueryList | null = null;
let cachedMatchMedia: unknown = null;

export function isFinePointerDevice(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return false;
  }
  if (cachedMatchMedia !== window.matchMedia || !cachedMql) {
    cachedMatchMedia = window.matchMedia;
    cachedMql = window.matchMedia("(hover: hover) and (pointer: fine)");
  }
  return cachedMql.matches;
}

export function SurfaceStage() {
  const t = useTranslations("landing");
  const router = useRouter();
  const [activeId, setActiveId] = useState<SurfacePaneId | null>(null);
  const paneRefs = useRef<Partial<Record<SurfacePaneId, HTMLElement | null>>>({});
  const preActivatedAtRef = useRef(0);
  // mousemove 命中测试合并：每帧最多跑一次几何计算（rAF），点坐标只存 ref。
  const hitTestFrameRef = useRef<number | null>(null);
  const pendingPointRef = useRef<{ x: number; y: number } | null>(null);
  const activeIdRef = useRef<SurfacePaneId | null>(null);

  useEffect(() => {
    activeIdRef.current = activeId;
  }, [activeId]);

  useEffect(
    () => () => {
      if (hitTestFrameRef.current !== null && typeof cancelAnimationFrame === "function") {
        cancelAnimationFrame(hitTestFrameRef.current);
      }
    },
    [],
  );

  // 导航唯一出口：站内路由（原型 window.__goto 的产品实现）。
  const goto = useCallback(
    (href: string) => {
      router.push(href);
    },
    [router],
  );

  const activate = useCallback((id: SurfacePaneId) => {
    preActivatedAtRef.current = Date.now();
    setActiveId(id);
  }, []);

  const resetAll = useCallback(() => setActiveId(null), []);

  // 整窗等比缩放：跟随 pane 实际尺寸（ResizeObserver 在 flex 动画期间逐帧回调）。
  useEffect(() => {
    if (typeof ResizeObserver === "undefined") return;
    const fit = () => {
      for (const def of SURFACE_PANES) {
        const pane = paneRefs.current[def.id];
        const frame = pane?.querySelector<HTMLElement>(".pframe");
        if (!pane || !frame) continue;
        const box = pane.getBoundingClientRect();
        // 推导：.pframe 布局尺寸 560×404 + 1px 边框 ×2 = 572/416（含留量 10/8）；
        // 分母高度扣 124 = 窗顶 chrome(28) + 配图/配文间隙与标签行高的估计值。
        // 下限 0.07 防极端窄容器下帧内容翻转过小。
        const s = Math.min((box.width - 10) / 572, (box.height - 124) / 416);
        frame.style.setProperty("--s", Math.max(0.07, s).toFixed(4));
      }
    };
    const ro = new ResizeObserver(fit);
    for (const def of SURFACE_PANES) {
      const pane = paneRefs.current[def.id];
      if (pane) ro.observe(pane);
    }
    fit();
    return () => ro.disconnect();
  }, []);

  const runHitTest = useCallback(() => {
    hitTestFrameRef.current = null;
    const point = pendingPointRef.current;
    if (!point) return;
    if (!isFinePointerDevice()) return;
    const inputs: PaneHitInput[] = [];
    for (const def of SURFACE_PANES) {
      const pane = paneRefs.current[def.id];
      const frame = pane?.querySelector<HTMLElement>(".pframe");
      const label = pane?.querySelector<HTMLElement>(".plabel");
      if (!frame || !label) continue;
      inputs.push({
        id: def.id,
        frameRect: frame.getBoundingClientRect(),
        labelRect: label.getBoundingClientRect(),
        isOn: def.id === activeIdRef.current,
      });
    }
    const hit = resolveHitPane(inputs, point.x, point.y, HIT_PAD_PX);
    // 空白（hit=null）不触发也不复位：激活粘性；进入他页视觉才切换。
    if (hit && hit !== activeIdRef.current) {
      setActiveId(hit as SurfacePaneId);
    }
  }, []);

  const handleStageMouseMove = useCallback(
    (event: React.MouseEvent) => {
      if (!isFinePointerDevice()) return;
      pendingPointRef.current = { x: event.clientX, y: event.clientY };
      if (hitTestFrameRef.current === null && typeof requestAnimationFrame === "function") {
        hitTestFrameRef.current = requestAnimationFrame(runHitTest);
      }
    },
    [runHitTest],
  );

  const handlePaneClick = useCallback(
    (def: (typeof SURFACE_PANES)[number]) => {
      const action = resolvePaneAction({
        finePointer: isFinePointerDevice(),
        isOn: activeId === def.id,
        msSincePreActivated: Date.now() - preActivatedAtRef.current,
      });
      if (action === "navigate") goto(def.href);
      else if (action === "activate") activate(def.id);
      else if (action === "collapse") resetAll();
    },
    [activeId, activate, goto, resetAll],
  );

  // 键盘 Enter/空格始终导航，不随 pointer 媒体查询变成预览切换（§12.2 红线）。
  const handlePaneKeyDown = useCallback(
    (event: React.KeyboardEvent, def: (typeof SURFACE_PANES)[number]) => {
      if (event.key !== "Enter" && event.key !== " ") return;
      event.preventDefault();
      goto(def.href);
    },
    [goto],
  );

  return (
    <div
      className={activeId ? "stage live" : "stage"}
      data-testid="surface-stage"
      onMouseMove={handleStageMouseMove}
      onMouseLeave={resetAll}
    >
      {SURFACE_PANES.map((def) => {
        const on = activeId === def.id;
        return (
          <article
            key={def.id}
            ref={(el) => {
              paneRefs.current[def.id] = el;
            }}
            className={on ? "pane on" : "pane"}
            tabIndex={0}
            role="link"
            aria-label={t(`panes.${def.id}.name`)}
            data-testid={`surface-pane-${def.id}`}
            data-goto={def.href}
            onClick={() => handlePaneClick(def)}
            onFocus={() => {
              if (!on) activate(def.id);
            }}
            onBlur={() => {
              if (isFinePointerDevice()) resetAll();
            }}
            onKeyDown={(event) => handlePaneKeyDown(event, def)}
          >
            <div className="pframe">
              <div className="mchrome">
                <b />
                <b />
                <b />
                <span className="murl">{def.href}</span>
              </div>
              <div className="mscreen">
                {def.id === "original" && <OriginalMock />}
                {def.id === "fanworks" && <FanworksMock />}
                {def.id === "agent" && <AgentMock chathead={t("panes.agent.chathead")} input={t("panes.agent.input")} send={t("panes.agent.send")} />}
              </div>
            </div>
            <div className="plabel">
              <b>{t(`panes.${def.id}.name`)}</b>
              <div className="pdesc">
                <span>{t(`panes.${def.id}.desc`)}</span>{" "}
                <Link
                  href={def.href}
                  data-testid={`surface-link-${def.id}`}
                  onClick={(event) => {
                    // 简介尾部「→」链接：触屏/键盘主通道，直接导航不冒泡。
                    event.stopPropagation();
                  }}
                >
                  {t(`panes.${def.id}.go`)}
                </Link>
              </div>
              <button
                type="button"
                className="penter"
                data-testid={`surface-enter-${def.id}`}
                onClick={(event) => {
                  event.stopPropagation();
                  goto(def.href);
                }}
              >
                {t("panes.enter")}
              </button>
            </div>
          </article>
        );
      })}
    </div>
  );
}

// —— Mock A 原创区（CSS 演示；内文已双译 §12.2）——
function OriginalMock() {
  const d = useTranslations("landing.demo.surfaces");
  const cats = [d("catAll"), d("catIllust"), d("catText"), d("catMusic"), d("catSetting")];
  const cards = [
    { cv: "linear-gradient(140deg,#065f46,#34d399)", title: d("o1t"), meta: d("o1m") },
    { cv: "linear-gradient(140deg,#7c2d12,#fbbf24)", title: d("o2t"), meta: d("o2m") },
    { cv: "linear-gradient(140deg,#312e81,#818cf8)", title: d("o3t"), meta: d("o3m") },
    { cv: "linear-gradient(140deg,#1e293b,#64748b)", title: d("o4t"), meta: d("o4m") },
    { cv: "linear-gradient(140deg,#9d174d,#f472b6)", title: d("o5t"), meta: d("o5m") },
    { cv: "linear-gradient(140deg,#0f766e,#14b8a6)", title: d("o6t"), meta: d("o6m") },
  ];
  return (
    <div className="mo">
      <div className="mo-bar">
        <i className="mo-hi demo" />
        {cats.map((label) => (
          <span key={label}>{label}</span>
        ))}
      </div>
      <div className="mo-grid">
        {cards.map((card) => (
          <div className="mo-card demo" key={card.title}>
            <div className="cv" style={{ background: card.cv }} />
            <div className="tb">
              <div className="t">{card.title}</div>
              <div className="m">{card.meta}</div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// —— Mock B 二创区（三列瀑布流缓动；封面用 CSS 渐变，不用 IP 名义图片素材；内文已双译 §12.2）——
function FanworksMock() {
  const d = useTranslations("landing.demo.surfaces");
  const columns: Array<{ dir: "up" | "dn"; variant?: string; items: Array<[string, string, string]> }> = [
    {
      dir: "up",
      items: [
        ["linear-gradient(140deg,#b91c1c,#f97316)", d("f4t"), d("f4m")],
        ["linear-gradient(140deg,#7c2d12,#fbbf24)", d("f2t"), d("f2m")],
        ["linear-gradient(140deg,#312e81,#818cf8)", d("f1t"), d("f1m")],
      ],
    },
    {
      dir: "dn",
      items: [
        ["linear-gradient(140deg,#065f46,#34d399)", d("f8t"), d("f8m")],
        ["linear-gradient(140deg,#9d174d,#f472b6)", d("f5t"), d("f5m")],
        ["linear-gradient(140deg,#1e293b,#64748b)", d("f6t"), d("f6m")],
      ],
    },
    {
      dir: "up",
      variant: "s2",
      items: [
        ["linear-gradient(140deg,#0f766e,#14b8a6)", d("f7t"), d("f7m")],
        ["linear-gradient(140deg,#4c1d95,#a78bfa)", d("f9t"), d("f9m")],
        ["linear-gradient(140deg,#155e75,#22d3ee)", d("f3t"), d("f3m")],
      ],
    },
  ];
  const pills = [d("ipAll"), d("ip1"), d("ip2"), d("ip3"), d("ip4")];
  return (
    <div className="mf">
      <div className="mf-bar">
        <span className="hot">{pills[0]}</span>
        {pills.slice(1).map((label) => (
          <span key={label}>{label}</span>
        ))}
      </div>
      <div className="mf-cols">
        {columns.map((col, i) => (
          <div className="mf-col" key={i}>
            <div className={`mf-track ${col.dir} ${col.variant ?? ""} demo`.trim()}>
              {[...col.items, ...col.items].map(([cv, title, meta], j) => (
                <div className="mf-item" key={j}>
                  <div className="cv" style={{ background: cv }} />
                  <div className="tb">
                    <div className="t">{title}</div>
                    <div className="m">{meta}</div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// —— Mock C AI 助手（问答流循环；聊天气与引用已双译 §12.2）——
function AgentMock({ chathead, input, send }: { chathead: string; input: string; send: string }) {
  const d = useTranslations("landing.demo.surfaces");
  return (
    <div className="ma">
      <div className="ma-head">{chathead}</div>
      <div className="ma-chat">
        <div className="ma-u demo">{d("chatQ")}</div>
        <div className="ma-typing demo">
          <i />
          <i />
          <i />
        </div>
        <div className="ma-a demo">
          <span className="clip">{d("chatA")}</span>
          <span className="ma-cite demo">{d("cite1")}</span>
          <span className="ma-cite demo">{d("cite2")}</span>
        </div>
      </div>
      <div className="ma-input">
        <span>{input}</span>
        <b>{send}</b>
      </div>
    </div>
  );
}
