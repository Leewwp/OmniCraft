import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { Home, Compass } from "lucide-react";

import { Sidebar } from "@/components/layout/Sidebar";
import {
  SECTION_HEADER_HEIGHT,
  SIDEBAR_ROW_HEIGHT,
  SidebarSectionHeader,
  SIDEBAR_TOOLTIP_CLASS,
} from "@/components/layout/sidebar-shell";
import { PUBLIC_SIDEBAR_STORAGE_KEY } from "@/lib/use-sidebar-collapse";

import { cleanup, installDom, renderWithIntl } from "./runtime-test-helpers";

/* #412 F4+F5：侧边栏垂直节奏不变性（组件级结构断言）。
   几何真值（getBoundingClientRect().top 两态不变）由
   e2e/sidebar-vertical-rhythm.mock.spec.ts 在真浏览器中做机器验收；
   本文件锁定达成该不变性的结构/类不变式与共享单源。 */

const SECTIONS = [
  {
    label: "IP Categories",
    items: [
      { icon: <Home className="h-4 w-4" />, label: "Home", href: "/" },
      { icon: <Compass className="h-4 w-4" />, label: "Explore", href: "/explore", count: 12 },
    ],
  },
];

const TRENDING = {
  title: "Trending IPs · This Week",
  entries: [{ rank: 1, name: "Alpha", stat: "1.2k shares", href: "/ip/alpha" }],
};

function renderSidebar(collapsed: boolean) {
  window.localStorage.setItem(PUBLIC_SIDEBAR_STORAGE_KEY, String(collapsed));
  return renderWithIntl(
    <Sidebar sections={SECTIONS} trending={TRENDING} />,
  );
}

test.afterEach(() => {
  cleanup();
  window.localStorage.clear();
});

test("vertical rhythm constants are single-sourced and two-state shared", () => {
  installDom();
  assert.equal(SECTION_HEADER_HEIGHT, 36);
  assert.equal(SIDEBAR_ROW_HEIGHT, 44);
});

test("section header keeps equal-height placeholder in both states (same component, no hidden)", async () => {
  installDom();
  for (const collapsed of [false, true]) {
    const view = renderSidebar(collapsed);
    await new Promise((r) => setTimeout(r, 60));
    void view;

    const headerEls = document.querySelectorAll('[data-sidebar-anchor="section-header"]');
    assert.equal(headerEls.length, 1, `section header must render in ${collapsed ? "collapsed" : "expanded"} state`);

    const header = headerEls[0] as HTMLElement;
    assert.equal(
      header.style.height,
      `${SECTION_HEADER_HEIGHT}px`,
      "both states must share the SECTION_HEADER_HEIGHT single source",
    );
    assert.doesNotMatch(header.className, /hidden/, "collapsed state uses equal-height placeholder, not display:none");

    const divider = header.querySelector("span[aria-hidden='true']");
    assert.ok(divider, "decorative divider must exist");
    assert.match(divider.className, /w-4/, "divider is 16px wide");
    assert.match(divider.className, /h-px/, "divider is 1px tall");
    assert.match(divider.className, /bg-border-default/, "divider uses the semantic border token");
    assert.match(divider.className, /left-1\/2/, "divider is horizontally centered");

    const label = header.lastElementChild as HTMLElement;
    if (collapsed) {
      assert.match(label.className, /opacity-0/, "collapsed label text is invisible");
      assert.match(divider.className, /opacity-100/, "collapsed divider is visible");
    } else {
      assert.match(label.className, /opacity-100/, "expanded label text is visible");
      assert.match(divider.className, /opacity-0/, "expanded divider is faded out");
    }
    // 文字与分隔线同一布局槽位叠放：均为 absolute，不上下堆叠。
    assert.match(label.className, /absolute/, "label overlays the same layout slot");
    assert.match(divider.className, /absolute/, "divider overlays the same layout slot");
    cleanup();
  }
});

test("nav rows keep 44px height class and stay in the DOM across both states", async () => {
  installDom();
  for (const collapsed of [false, true]) {
    const view = renderSidebar(collapsed);
    await new Promise((r) => setTimeout(r, 60));
    void view;

    const rows = document.querySelectorAll('[data-sidebar-anchor="item"]');
    assert.equal(rows.length, 2, `nav rows must exist in ${collapsed ? "collapsed" : "expanded"} state`);
    for (const row of rows) {
      const link = row.firstElementChild as HTMLElement;
      assert.match(link.className, /min-h-\[44px\]/, "row height is the shared 44px contract");
    }
    cleanup();
  }
});

test("text labels slide via opacity/translateX overlay and never sit in the icon flex flow", async () => {
  installDom();
  const view = renderSidebar(false);
  await new Promise((r) => setTimeout(r, 60));
  const firstLabel = document.querySelectorAll('[data-sidebar-anchor="item"] a [class*="transition-[opacity,transform]"]')[0] as HTMLElement;
  assert.match(firstLabel.className, /absolute/, "label container is an overlay, not flex flow");
  assert.match(firstLabel.className, /opacity-100/);

  cleanup();
  const view2 = renderSidebar(true);
  await new Promise((r) => setTimeout(r, 60));
  void view2;
  const collapsedLabel = document.querySelectorAll('[data-sidebar-anchor="item"] a [class*="transition-[opacity,transform]"]')[0] as HTMLElement;
  assert.match(collapsedLabel.className, /opacity-0/, "collapsed label fades out");
  assert.match(collapsedLabel.className, /-translate-x-1/, "collapsed label slides toward the icon");
  assert.match(collapsedLabel.className, /pointer-events-none/);
});

test("collapsed rows expose instant CSS tooltips with no transition delay", async () => {
  installDom();
  const view = renderSidebar(true);
  await new Promise((r) => setTimeout(r, 60));
  void view;

  const tooltips = document.querySelectorAll(`[class*="group-hover:opacity-100"]`);
  assert.ok(tooltips.length >= 3, "collapsed rows + toggle expose tooltips");
  for (const tip of tooltips) {
    assert.equal(tip.getAttribute("aria-hidden"), "true", "tooltip copy is decorative");
    assert.doesNotMatch(tip.className, /delay-\d/, "tooltip must be instant (no delay-*)");
    assert.match(tip.className, /left-full/, "tooltip escapes the rail via left-full");
  }

  // 行的可访问名由 aria-label 承担（图标-only 链接）。
  const firstLink = document.querySelectorAll('[data-sidebar-anchor="item"] a')[0] as HTMLElement;
  assert.equal(firstLink.getAttribute("aria-label"), "Home");
});

test("trending block is fully removed when collapsed (not participating in layout)", async () => {
  installDom();
  const expanded = renderSidebar(false);
  await new Promise((r) => setTimeout(r, 60));
  assert.ok(document.body.textContent?.includes("Trending IPs"));
  cleanup();

  const collapsedView = renderSidebar(true);
  await new Promise((r) => setTimeout(r, 60));
  void collapsedView;
  assert.ok(!document.body.textContent?.includes("Trending IPs"), "collapsed trending is unmounted");
});

test("toggle is a single fixed button across states (icon + label swap only)", async () => {
  installDom();
  const view = renderSidebar(false);
  await new Promise((r) => setTimeout(r, 60));
  const toggle = view.getByRole("button", { name: "Collapse sidebar" });
  assert.equal(toggle.tagName, "BUTTON");
  assert.doesNotMatch(toggle.className, /delay-\d/);
  assert.doesNotMatch(toggle.className, /translate-y/, "no vertical movement classes on the toggle");
  cleanup();

  const view2 = renderSidebar(true);
  await new Promise((r) => setTimeout(r, 60));
  const toggle2 = view2.getByRole("button", { name: "Expand sidebar" });
  assert.equal(toggle2.tagName, "BUTTON", "same single element swaps icon/label between states");
});

test("animation classes stay within the width/opacity/translateX whitelist", async () => {
  installDom();
  for (const collapsed of [false, true]) {
    const view = renderSidebar(collapsed);
    await new Promise((r) => setTimeout(r, 60));
    void view;

    const animated = document.querySelectorAll(
      'aside [class*="transition-"], aside [class*="group-hover"]',
    );
    assert.ok(animated.length > 0);
    for (const el of animated) {
      const cls = (el as HTMLElement).className;
      // 白名单外禁用的几何属性：垂直位移/盒模型尺寸动画。
      assert.doesNotMatch(cls, /transition-\[(?![^]]*color)[^]]*(top|height|margin|padding)/, `forbidden geometry transition: ${cls}`);
    }
    // transform 参与动画的元素（transition-[…,transform]）禁带 Y 位移分量；
    // 静态定位 transform（如分隔线 -translate-y-1/2 挂 transition-opacity）不在动画路径，不在此列。
    const transformAnimated = document.querySelectorAll(
      'aside [class*="transition-[opacity,transform]"], aside [class*="transition-transform"]',
    );
    assert.ok(transformAnimated.length > 0, "text overlays animate via transform");
    for (const el of transformAnimated) {
      const cls = (el as HTMLElement).className;
      assert.doesNotMatch(cls, /translate-y-/, `no vertical translate on transform-animated elements: ${cls}`);
    }
    cleanup();
  }
});

test("tooltip class contract is exported and delay-free", () => {
  installDom();
  assert.doesNotMatch(SIDEBAR_TOOLTIP_CLASS, /delay-\d/);
  assert.match(SIDEBAR_TOOLTIP_CLASS, /group-hover:opacity-100/);
});

test("section header renders standalone for studio/admin reuse", async () => {
  installDom();
  const view = renderWithIntl(
    <>
      <SidebarSectionHeader label="Management" collapsed={false} />
      <SidebarSectionHeader label="Management" collapsed />
    </>,
  );
  await new Promise((r) => setTimeout(r, 60));
  void view;
  const headers = document.querySelectorAll('[data-sidebar-anchor="section-header"]');
  assert.equal(headers.length, 2);
  for (const h of headers) {
    assert.equal((h as HTMLElement).style.height, "36px");
  }
});
