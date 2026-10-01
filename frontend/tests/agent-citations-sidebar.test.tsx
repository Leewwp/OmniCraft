import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { cleanup, installDom, render, waitFor, fireEvent } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * FT-4（#696）参考来源侧栏：原内联折叠列表（AgentCitationList）退役；
 * 侧栏显示所点回答的引用卡片，双通道关闭（X），卡片点击回调透传。
 *
 * #746（#717 What-to-build 第 3 项补齐）：移动抽屉手写 pointer 三路关闭
 * （把手拖拽过半 / 快滑 >0.5px/ms / 内容滚动到顶接管）与关闭清理的
 * 常驻回归——测试方式与组件实现一致：stub matchMedia 走 mobile 分支，
 * fireEvent pointer 事件携带 pointerType:"touch"。
 * ──────────────────────────────────────────────────────────────────────────── */

type SidebarModule = typeof import("@/components/agent/AgentCitationsSidebar");
let AgentCitationsSidebar: SidebarModule["AgentCitationsSidebar"];

test.before(async () => {
  const module = await import("@/components/agent/AgentCitationsSidebar");
  AgentCitationsSidebar = module.AgentCitationsSidebar;
});

test.afterEach(() => cleanup());

function citation(index: number) {
  return {
    contentId: 100 + index,
    title: `Reference ${index + 1}`,
    zone: "original" as const,
    excerpt: `excerpt ${index + 1}`,
  };
}

function renderSidebar(overrides: Partial<React.ComponentProps<typeof AgentCitationsSidebar>> = {}) {
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <AgentCitationsSidebar
        open
        onClose={() => undefined}
        citations={[citation(0), citation(1), citation(2), citation(3), citation(4), citation(5)]}
        onOpen={() => undefined}
        {...overrides}
      />
    </IntlProvider>,
  );
}

test("FT-4 sidebar renders every citation card under the unified reference-sources title", () => {
  installDom();
  const view = renderSidebar();
  assert.ok(view.getByText("Reference sources"), "unified naming (was the old inline list title)");
  assert.ok(view.getByText("6"), "count is projected");
  for (let i = 0; i < 6; i += 1) {
    assert.ok(view.getByRole("button", { name: new RegExp(`Reference ${i + 1}`) }), `card ${i + 1} renders`);
  }
});

test("FT-4 sidebar hides when closed or empty (no dead panel)", () => {
  installDom();
  const closed = renderSidebar({ open: false });
  assert.equal(closed.container.firstChild, null);
  const empty = renderSidebar({ citations: [] });
  assert.equal(empty.container.firstChild, null);
});

test("FT-4 close affordance exists (X channel)", () => {
  installDom();
  const view = renderSidebar();
  assert.ok(view.getAllByRole("button", { name: "Close reference sources" }).length >= 1);
});

/* ─────────────────────────── #746 mobile drawer coverage ─────────────────── */

/** 关闭阈值 = innerHeight×0.7×DRAG_CLOSE_RATIO(0.5)；jsdom 默认 768 → 268.8。 */
const CLOSE_THRESHOLD = window.innerHeight * 0.7 * 0.5;

/**
 * 移动抽屉测试环境：matchMedia 命中 (max-width: 767px) 走 mobile 分支；
 * jsdom 无 Pointer Capture API，补 setPointerCapture no-op（组件 beginDrag
 * 会调用）。返回还原函数，避免 stub 泄漏到后续桌面用例。
 */
function installMobileDrawerEnv() {
  installDom();
  const originalMatchMedia = window.matchMedia;
  window.matchMedia = ((query: string) => ({
    matches: query === "(max-width: 767px)",
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  })) as typeof window.matchMedia;
  const proto = window.Element.prototype as Element & { setPointerCapture?: (id: number) => void };
  const hadCapture = typeof proto.setPointerCapture === "function";
  if (!hadCapture) proto.setPointerCapture = () => undefined;
  return () => {
    if (originalMatchMedia) {
      window.matchMedia = originalMatchMedia;
    } else {
      Reflect.deleteProperty(window, "matchMedia");
    }
    if (!hadCapture) Reflect.deleteProperty(proto, "setPointerCapture");
  };
}

function renderMobileDrawer(overrides: Partial<React.ComponentProps<typeof AgentCitationsSidebar>> = {}) {
  let closeCalls = 0;
  const view = renderSidebar({ onClose: () => { closeCalls += 1; }, ...overrides });
  const handle = view.container.querySelector<HTMLElement>(".cursor-grab");
  const content = view.container.querySelector<HTMLElement>(".touch-pan-y");
  const sheet = view.container.querySelector<HTMLElement>(".rounded-t-xl");
  const overlay = view.getAllByRole("button", { name: "Close reference sources" })[0];
  assert.ok(handle, "mobile drag handle rendered");
  assert.ok(content, "mobile scrollable content rendered");
  assert.ok(sheet, "mobile bottom sheet rendered");
  return { view, handle: handle!, content: content!, sheet: sheet!, overlay, closeCalls: () => closeCalls };
}

/** Date.now 桩：beginDrag 记 startTs、endDrag 算 duration，钉死时长避免真时间抖动。 */
function stubDateNow(startMs = 1_700_000_000_000) {
  const realNow = Date.now;
  let current = startMs;
  Date.now = () => current;
  return {
    advanceTo(ms: number) { current = ms; },
    restore() { Date.now = realNow; },
  };
}

const TOUCH = { pointerType: "touch" as const, pointerId: 1, bubbles: true };

test("#746 handle drag past half height closes the drawer (touch)", async () => {
  const restoreEnv = installMobileDrawerEnv();
  try {
    const drawer = renderMobileDrawer();
    fireEvent.pointerDown(drawer.handle, { ...TOUCH, clientY: 300 });
    fireEvent.pointerMove(drawer.handle, { ...TOUCH, clientY: 300 + CLOSE_THRESHOLD + 90 });
    await waitFor(() => {
      assert.equal(drawer.sheet.style.transform, `translateY(${CLOSE_THRESHOLD + 90}px)`, "sheet follows the finger while dragging");
    });
    fireEvent.pointerUp(drawer.handle, { ...TOUCH, clientY: 300 + CLOSE_THRESHOLD + 90 });
    assert.equal(drawer.closeCalls(), 1, "crossing the half-height threshold closes");
  } finally {
    restoreEnv();
  }
});

test("#746 fast fling below the distance threshold still closes (>0.5px/ms)", () => {
  const restoreEnv = installMobileDrawerEnv();
  const clock = stubDateNow();
  try {
    const drawer = renderMobileDrawer();
    const dy = 60; // < CLOSE_THRESHOLD：只有快滑分支可触发关闭
    fireEvent.pointerDown(drawer.handle, { ...TOUCH, clientY: 300 });
    clock.advanceTo(1_700_000_000_000 + 10); // 60px / 10ms = 6px/ms
    fireEvent.pointerMove(drawer.handle, { ...TOUCH, clientY: 300 + dy });
    fireEvent.pointerUp(drawer.handle, { ...TOUCH, clientY: 300 + dy });
    assert.equal(drawer.closeCalls(), 1, "fling velocity closes despite short distance");
  } finally {
    clock.restore();
    restoreEnv();
  }
});

test("#746 slow short drag rebounds without closing", () => {
  const restoreEnv = installMobileDrawerEnv();
  const clock = stubDateNow();
  try {
    const drawer = renderMobileDrawer();
    const dy = 30; // 30px / 100ms = 0.3px/ms < 0.5，且远低于距离阈值
    fireEvent.pointerDown(drawer.handle, { ...TOUCH, clientY: 300 });
    fireEvent.pointerMove(drawer.handle, { ...TOUCH, clientY: 300 + dy });
    clock.advanceTo(1_700_000_000_000 + 100);
    fireEvent.pointerUp(drawer.handle, { ...TOUCH, clientY: 300 + dy });
    assert.equal(drawer.closeCalls(), 0, "below both thresholds keeps the drawer open");
    assert.equal(drawer.sheet.style.transform, "", "offset resets on release");
  } finally {
    clock.restore();
    restoreEnv();
  }
});

test("#746 mouse pointers never engage the drawer gesture (desktop guard)", () => {
  const restoreEnv = installMobileDrawerEnv();
  try {
    const drawer = renderMobileDrawer();
    const mouse = { pointerType: "mouse" as const, pointerId: 1, bubbles: true };
    fireEvent.pointerDown(drawer.handle, { ...mouse, clientY: 300 });
    fireEvent.pointerMove(drawer.handle, { ...mouse, clientY: 300 + CLOSE_THRESHOLD + 90 });
    fireEvent.pointerUp(drawer.handle, { ...mouse, clientY: 300 + CLOSE_THRESHOLD + 90 });
    assert.equal(drawer.closeCalls(), 0, "beginDrag ignores mouse pointers");
  } finally {
    restoreEnv();
  }
});

test("#746 content area takes over the pull-down only when scrolled to top", () => {
  const restoreEnv = installMobileDrawerEnv();
  try {
    const drawer = renderMobileDrawer();

    // 滚动到顶（scrollTop=0）：内容区接管下拉，过半关闭。
    assert.equal(drawer.content.scrollTop, 0);
    fireEvent.pointerDown(drawer.content, { ...TOUCH, clientY: 300 });
    fireEvent.pointerMove(drawer.content, { ...TOUCH, clientY: 300 + CLOSE_THRESHOLD + 90 });
    fireEvent.pointerUp(drawer.content, { ...TOUCH, clientY: 300 + CLOSE_THRESHOLD + 90 });
    assert.equal(drawer.closeCalls(), 1, "scrolled-to-top pull-down closes");

    // 未滚到顶（scrollTop>0）：内容区不起拖，松手不关闭。
    drawer.content.scrollTop = 120;
    fireEvent.pointerDown(drawer.content, { ...TOUCH, clientY: 300 });
    fireEvent.pointerMove(drawer.content, { ...TOUCH, clientY: 300 + CLOSE_THRESHOLD + 90 });
    fireEvent.pointerUp(drawer.content, { ...TOUCH, clientY: 300 + CLOSE_THRESHOLD + 90 });
    assert.equal(drawer.closeCalls(), 1, "mid-scroll drag leaves gesture to native scrolling");
  } finally {
    restoreEnv();
  }
});

test("#746 overlay tap and Escape both close the drawer", () => {
  const restoreEnv = installMobileDrawerEnv();
  try {
    let closeCalls = 0;
    const view = renderSidebar({ onClose: () => { closeCalls += 1; } });
    fireEvent.click(view.getAllByRole("button", { name: "Close reference sources" })[0]);
    assert.equal(closeCalls, 1, "overlay tap closes");
    const dialog = view.getByRole("dialog");
    fireEvent.keyDown(dialog, { key: "Escape", bubbles: true });
    assert.equal(closeCalls, 2, "Escape closes");
  } finally {
    restoreEnv();
  }
});

test("#746 closing hides a11y tree + overlay first, then unmounts after the exit animation", async () => {
  const restoreEnv = installMobileDrawerEnv();
  try {
    let closeCalls = 0;
    const ui = (open: boolean) => (
      <IntlProvider locale="en" messages={enMessages}>
        <AgentCitationsSidebar
          open={open}
          onClose={() => { closeCalls += 1; }}
          citations={[citation(0), citation(1)]}
          onOpen={() => undefined}
        />
      </IntlProvider>
    );
    const view = render(ui(true));
    const sheet = view.container.querySelector<HTMLElement>(".rounded-t-xl")!;
    const overlay = view.getAllByRole("button", { name: "Close reference sources" })[0];
    const root = view.container.firstElementChild as HTMLElement;

    view.rerender(ui(false));

    // 退出帧：useDelayedUnmount(220ms) 内仍挂载，但已从 a11y 树摘除。
    assert.equal(root.getAttribute("aria-hidden"), "true", "exit frame is aria-hidden");
    assert.ok(root.hasAttribute("inert"), "exit frame is inert");
    assert.ok(overlay.className.includes("opacity-0"), "overlay fades out");
    assert.ok(sheet.className.includes("translate-y-full"), "sheet slides out");
    assert.ok(view.container.firstElementChild, "still mounted during the 220ms exit window");

    // 220ms 卸载由裸 setTimeout 的 act 外 setState 驱动：waitFor 的 act 轮询
    // 会饿死该更新（实测挂死），这里用手动有界轮询观察真实卸载。
    for (let i = 0; i < 20 && view.container.firstElementChild; i += 1) {
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    assert.equal(view.container.firstElementChild, null, "fully unmounted after the exit animation");
    assert.equal(closeCalls, 0, "cleanup assertions never trigger onClose");
  } finally {
    restoreEnv();
  }
});

/* ────────────────────────────────────────────────────────────────────────────
 * #751：桌面宽度公式 + 完整收缩链（类级契约；几何 scrollWidth ≤ clientWidth
 * 由真实浏览器验证，jsdom 无布局引擎）。
 * ──────────────────────────────────────────────────────────────────────────── */

test("#751 desktop sidebar width = min(440px,38vw) with the flex-sibling model kept", () => {
  installDom();
  const view = renderSidebar();
  const aside = view.container.querySelector('[data-testid="citations-sidebar"]');
  assert.ok(aside, "desktop aside renders");
  const cls = aside.getAttribute("class") ?? "";
  assert.ok(cls.includes("w-[min(440px,38vw)]"), "width must be min(440px,38vw)");
  assert.ok(cls.includes("shrink-0"), "shrink-0 flex sibling model kept");
  assert.ok(cls.includes("md:flex"), "desktop-only visibility kept");

  const scroller = aside.querySelector("div.overflow-y-auto");
  assert.ok(scroller, "desktop content scroller exists");
  assert.match(scroller.className, /overflow-x-hidden/, "horizontal overflow only as backstop");
  const list = scroller.querySelector("ul");
  assert.ok(list, "list renders");
  assert.match(list!.className, /grid-cols-1/, "single zero-min track (minmax(0,1fr))");
  const item = list.querySelector("li");
  assert.ok(item, "list item renders");
  assert.match(item!.className, /(^| )min-w-0( |$)/, "list items join the shrink chain");
});

test("#751 citation card truncation chain: title truncates, IP badges drop to a wrapping meta row, excerpt breaks words", () => {
  installDom();
  const longTitle = "一个特别特别特别长的中文标题".repeat(6);
  const longCategoryURL = "https://example.com/a/very/long/path/segment/that/never/breaks?query=1";
  const view = render(
    <IntlProvider locale="en" messages={enMessages}>
      <AgentCitationsSidebar
        open
        onClose={() => undefined}
        citations={[
          { contentId: 900, title: longTitle, zone: "ip" as const, category: "vtuber", excerpt: longCategoryURL },
          { contentId: 901, title: "Content title", zone: "fanwork" as const, excerpt: "short" },
        ]}
        onOpen={() => undefined}
      />
    </IntlProvider>,
  );

  const cards = view.getAllByRole("button", { name: /Reference sources/ });
  assert.equal(cards.length, 2);

  const ipCard = cards.find((c) => c.textContent.includes("IP")) as HTMLElement;
  const titleRow = ipCard.querySelector("span.flex");
  assert.ok(titleRow, "title row renders");
  const title = titleRow!.querySelector("span.truncate");
  assert.ok(title, "title keeps truncate");
  assert.match(title.className, /min-w-0/, "title needs min-w-0 to actually shrink");
  assert.match(title.className, /flex-1/, "title takes the remaining row width");
  assert.equal(ipName(ipCard), longTitle, "full title preserved as the accessible name");

  const metaRow = [...ipCard.querySelectorAll("span")].find((el) => (el.getAttribute("class") ?? "").includes("flex-wrap"));
  assert.ok(metaRow, "IP badges live on a wrapping meta row");
  assert.ok(metaRow.textContent.includes("IP"), "zone badge present");
  assert.ok(metaRow.textContent.trim().length > 0, "category badge present when set");

  const excerpt = [...ipCard.querySelectorAll("span")].find((s) => s.textContent.startsWith("https://"));
  assert.ok(excerpt, "excerpt span renders");
  assert.match(excerpt!.className, /break-words/, "URL-like excerpts break tokens");
  assert.match(excerpt!.className, /line-clamp-2/, "still clamped to two lines");

  const contentCard = cards.find((c) => c !== ipCard) as HTMLElement;
  assert.ok(contentCard.textContent.includes("Fanwork"), "single zone badge stays inline on content cards");
});

function ipName(card: HTMLElement) {
  return (card.getAttribute("aria-label") ?? "").split("：").slice(1).join("：");
}
