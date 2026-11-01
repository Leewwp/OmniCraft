import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import React from "react";
import { act, cleanup, installDom, render } from "./runtime-test-helpers";
import {
  PC_TIER_QUERY,
  TWO_COLUMN_QUERY,
  overlayViewportServerSnapshot,
  useOverlayViewport,
} from "@/components/content/use-overlay-viewport";

/* C2 #921 视口缝单测：useOverlayViewport = uSES + 模块级 MQL 缓存
   （按 query 缓存、按 matchMedia 函数身份失效）。覆盖：订阅/清理、
   无 matchMedia 默认 {false,false}、同 query 复用同一 MQL（缓存命中）、
   matchMedia 桩更换自动失效、getServerSnapshot=false、两断点值；
   另钉 source 契约——content/ 族 959 / min-width: 960 查询字面量仅存于
   视口模块（RelatedContents 参数化缝与 MarkdownRenderer hover 查询不在此面）。 */

const root = path.resolve(process.cwd());

type ChangeListener = () => void;

/** 可变 MQL 桩：matches 可翻转，change 监听可触发/核销。 */
function createMqlStub(initialMatches: boolean) {
  const listeners = new Set<ChangeListener>();
  const mql = {
    matches: initialMatches,
    media: "",
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: (_type: string, listener: ChangeListener) => {
      listeners.add(listener);
    },
    removeEventListener: (_type: string, listener: ChangeListener) => {
      listeners.delete(listener);
    },
    dispatchEvent: () => false,
    activeListeners: () => listeners.size,
    trigger: () => {
      for (const listener of [...listeners]) listener();
    },
  };
  return mql;
}

/** 计数 matchMedia 桩：按 query 分发可变 MQL 并记录每次调用。 */
function createCountingMatchMedia(matchesByQuery: (query: string) => boolean) {
  const calls: string[] = [];
  const mqls = new Map<string, ReturnType<typeof createMqlStub>>();
  const matchMedia = ((query: string) => {
    calls.push(query);
    let mql = mqls.get(query);
    if (!mql) {
      mql = createMqlStub(matchesByQuery(query));
      mql.media = query;
      mqls.set(query, mql);
    }
    return mql as unknown as MediaQueryList;
  }) as typeof window.matchMedia;
  return { matchMedia, calls, mqls };
}

function ViewportProbe({ label }: { label: string }) {
  const { twoColumn, pcTier } = useOverlayViewport();
  return (
    <div
      data-testid={label}
      data-two-column={String(twoColumn)}
      data-pc-tier={String(pcTier)}
    />
  );
}

function deleteMatchMedia() {
  delete (window as { matchMedia?: typeof window.matchMedia }).matchMedia;
}

test.afterEach(() => {
  cleanup();
  deleteMatchMedia();
});

test("useOverlayViewport: twoColumn/pcTier track their queries; change events re-render; unmount cleans listeners", () => {
  installDom();
  const { matchMedia, mqls } = createCountingMatchMedia((query) => query === TWO_COLUMN_QUERY);
  window.matchMedia = matchMedia;

  const view = render(<ViewportProbe label="p1" />);

  const probe = view.getByTestId("p1");
  assert.equal(probe.getAttribute("data-two-column"), "true");
  assert.equal(probe.getAttribute("data-pc-tier"), "false");

  /* 订阅生效：翻转 960 matches → change 通知 → 重渲染取新值。 */
  const twoColumnMql = mqls.get(TWO_COLUMN_QUERY)!;
  assert.ok(twoColumnMql, "960 query resolved through matchMedia");
  twoColumnMql.matches = false;
  act(() => {
    twoColumnMql.trigger();
  });
  assert.equal(probe.getAttribute("data-two-column"), "false");

  /* 清理生效：两断点监听随卸载回收。 */
  const pcTierMql = mqls.get(PC_TIER_QUERY)!;
  assert.equal(pcTierMql.activeListeners(), 1);
  view.unmount();
  assert.equal(twoColumnMql.activeListeners(), 0, "twoColumn listener removed on unmount");
  assert.equal(pcTierMql.activeListeners(), 0, "pcTier listener removed on unmount");
});

test("useOverlayViewport: no matchMedia environment defaults to { twoColumn: false, pcTier: false }", () => {
  installDom();
  deleteMatchMedia();

  const view = render(<ViewportProbe label="p" />);
  const probe = view.getByTestId("p");
  assert.equal(probe.getAttribute("data-two-column"), "false");
  assert.equal(probe.getAttribute("data-pc-tier"), "false");
});

test("useOverlayViewport: pcTier resolves the 1100 breakpoint independently of twoColumn", () => {
  installDom();
  const { matchMedia } = createCountingMatchMedia((query) => query === PC_TIER_QUERY);
  window.matchMedia = matchMedia;

  const view = render(<ViewportProbe label="p" />);
  const probe = view.getByTestId("p");
  assert.equal(probe.getAttribute("data-two-column"), "false");
  assert.equal(probe.getAttribute("data-pc-tier"), "true");
});

test("useOverlayViewport: same query reuses the cached MQL; a new matchMedia identity invalidates the cache", () => {
  installDom();
  const first = createCountingMatchMedia(() => false);
  window.matchMedia = first.matchMedia;

  render(
    <>
      <ViewportProbe label="a" />
      <ViewportProbe label="b" />
    </>,
  );

  /* 缓存命中：两断点各只解析一次，第二个组件复用同一 MQL。 */
  assert.equal(first.calls.filter((q) => q === TWO_COLUMN_QUERY).length, 1);
  assert.equal(first.calls.filter((q) => q === PC_TIER_QUERY).length, 1);
  cleanup();

  /* 桩更换（函数身份变化）自动失效：重新解析两断点。 */
  const second = createCountingMatchMedia(() => false);
  window.matchMedia = second.matchMedia;
  render(<ViewportProbe label="c" />);
  assert.equal(second.calls.length, 2, "new matchMedia identity rebuilds the MQL cache");
});

test("useOverlayViewport: server snapshot is fixed false (SSR/hydration first frame)", () => {
  installDom();
  assert.equal(overlayViewportServerSnapshot(), false);
  /* 服务端快照固定 false 的同时，模块内 uSES 三参接线保持（防水合分裂）。 */
  const source = fs.readFileSync(
    path.join(root, "components/content/use-overlay-viewport.ts"),
    "utf8",
  );
  assert.match(source, /useSyncExternalStore\(\s*subscribeTwoColumn,\s*getTwoColumn,\s*overlayViewportServerSnapshot,/);
  assert.match(source, /useSyncExternalStore\(\s*subscribePcTier,\s*getPcTier,\s*overlayViewportServerSnapshot,/);
});

test("C2 #921 source contract: 959 / min-width: 960 matchMedia query literals live only in use-overlay-viewport", () => {
  const viewportPath = path.join(root, "components/content/use-overlay-viewport.ts");
  const viewportSource = fs.readFileSync(viewportPath, "utf8");
  assert.match(viewportSource, /min-width: 960px/);
  assert.match(viewportSource, /min-width: 1100px/);

  const contentDir = path.join(root, "components/content");
  for (const file of fs.readdirSync(contentDir)) {
    if (!/\.(ts|tsx)$/.test(file)) continue;
    if (file === "use-overlay-viewport.ts") continue;
    const source = fs.readFileSync(path.join(contentDir, file), "utf8");
    for (const match of source.matchAll(/matchMedia\(\s*(`[^`]*`|"[^"]*"|'[^']*')\s*\)/g)) {
      const query = match[1];
      assert.ok(
        !query.includes("959") && !query.includes("min-width: 960"),
        `${file} must read 960/959 viewport semantics via useOverlayViewport (found ${query})`,
      );
    }
  }
});
