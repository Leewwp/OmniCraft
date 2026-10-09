import test from "node:test";
import assert from "node:assert/strict";
import {
  MAX_OVERLAY_STACK_DEPTH,
  OVERLAY_HISTORY_STATE_KEY,
  __resetOverlayHistoryForTests,
  abortOverlayHistorySession,
  beginOverlayHistorySession,
  canonicalOverlayPath,
  endOverlayHistorySession,
  installOverlayHistoryController,
  pushOverlayHistoryLayer,
  readOverlayHistoryRecord,
  registerOverlayHistoryNavigator,
  replaceOverlayHistoryTop,
  requestOverlayHistoryBack,
  requestOverlayHistoryExit,
  subscribeOverlayHistory,
  type OverlayHistoryEvent,
  type OverlayHistoryRecord,
} from "@/lib/overlay-history";
import { installDom } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * D3 #860 详情弹窗 URL 深链 history 状态机（lib/overlay-history）单测：
 * - 规范路径映射：fanwork=/content/<id>、original=/original/<id>；
 * - 会话记录（namespaced key）：会话 id / 深度 / contentId / zone / 来源完整
 *   URL（含 query+hash），不写 Next 私有字段（__NA 等由 Next 包装层自补）；
 * - 转移表：开/压层 push、连续换篇 replace、五层封顶不写第六条；
 * - back/Esc 经 history.back 归并同一 popstate：sync 按目标深度绝对对齐；
 * - ×/背板全退到来源记录；forward 落死会话记录 → 导航器接管规范宿主页；
 * - StrictMode effect 双跑：abort 后立即同签名 begin 复活同一会话（不双 push）。
 * jsdom History 提供 pushState/replaceState/back/go + 异步 popstate（上移
 * 已实测），真 Chromium 的 App Router 包装行为由浏览器验收覆盖。
 * ──────────────────────────────────────────────────────────────────────────── */

function setup() {
  installDom();
  __resetOverlayHistoryForTests();
  installOverlayHistoryController();
}

function nextPopState(): Promise<void> {
  return new Promise((resolve) => {
    window.addEventListener("popstate", () => resolve(), { once: true });
  });
}

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
}

function collect(): { events: OverlayHistoryEvent[]; unsubscribe: () => void } {
  const events: OverlayHistoryEvent[] = [];
  const unsubscribe = subscribeOverlayHistory((event) => events.push(event));
  return { events, unsubscribe };
}

function readRecord(): OverlayHistoryRecord | null {
  return readOverlayHistoryRecord(window.history.state);
}

function goTo(href: string) {
  window.history.replaceState(null, "", href);
}

test("canonicalOverlayPath maps zones to the canonical detail routes", () => {
  assert.equal(canonicalOverlayPath("fanwork", 7), "/content/7");
  assert.equal(canonicalOverlayPath("original", 9), "/original/9");
});

test("beginOverlayHistorySession pushes a namespaced depth-1 record with the canonical URL and origin href", () => {
  setup();
  goTo("http://localhost/feed?tab=fanworks#section-2");
  const lengthBefore = window.history.length;

  const sessionId = beginOverlayHistorySession({ contentId: 7, zone: "fanwork", source: "recommendation" });

  assert.ok(sessionId.length > 0);
  assert.equal(window.location.pathname, "/content/7");
  assert.equal(window.history.length, lengthBefore + 1);

  const state = window.history.state as Record<string, unknown>;
  assert.deepEqual(Object.keys(state), [OVERLAY_HISTORY_STATE_KEY], "jsdom 无 Next 包装时 state 只含本会话命名空间");
  const record = readRecord();
  assert.ok(record, "depth-1 record written");
  assert.equal(record?.v, 1);
  assert.equal(record?.session, sessionId);
  assert.equal(record?.depth, 1);
  assert.equal(record?.contentId, 7);
  assert.equal(record?.zone, "fanwork");
  assert.equal(record?.source, "recommendation");
  assert.equal(record?.originHref, "http://localhost/feed?tab=fanworks#section-2");
  assert.equal(state.__NA, undefined, "绝不伪造 Next 私有字段");
  assert.equal(state.__PRIVATE_NEXTJS_INTERNALS_TREE, undefined, "绝不伪造 Next 私有字段");

  endOverlayHistorySession();
});

test("pushOverlayHistoryLayer writes per-layer records and rewrites the URL", () => {
  setup();
  goTo("http://localhost/original");
  beginOverlayHistorySession({ contentId: 1, zone: "original", source: "zone-page" });
  const lengthAfterBegin = window.history.length;

  assert.equal(pushOverlayHistoryLayer({ contentId: 2, zone: "fanwork", source: "zone-page" }), true);
  assert.equal(window.location.pathname, "/content/2");
  assert.equal(window.history.length, lengthAfterBegin + 1);
  assert.equal(readRecord()?.depth, 2);
  assert.equal(readRecord()?.contentId, 2);
  assert.equal(readRecord()?.originHref, "http://localhost/original");

  assert.equal(pushOverlayHistoryLayer({ contentId: 3, zone: "original", source: "zone-page" }), true);
  assert.equal(window.location.pathname, "/original/3");
  assert.equal(readRecord()?.depth, 3);

  endOverlayHistorySession();
});

test("replaceOverlayHistoryTop swaps the top record and URL in place", () => {
  setup();
  goTo("http://localhost/original");
  beginOverlayHistorySession({ contentId: 1, zone: "original", source: "zone-page" });
  pushOverlayHistoryLayer({ contentId: 2, zone: "fanwork", source: "zone-page" });
  const lengthBefore = window.history.length;

  replaceOverlayHistoryTop({ contentId: 4, zone: "fanwork", source: "zone-page", contextIndex: 1 });

  assert.equal(window.history.length, lengthBefore, "连续换篇不新增记录");
  assert.equal(window.location.pathname, "/content/4");
  const record = readRecord();
  assert.equal(record?.depth, 2, "深度不变");
  assert.equal(record?.contentId, 4);
  assert.equal(record?.contextIndex, 1);

  endOverlayHistorySession();
});

test("push refuses the sixth record at the five-layer cap", () => {
  setup();
  goTo("http://localhost/recommend");
  beginOverlayHistorySession({ contentId: 1, zone: "original", source: "recommendation" });
  for (let depth = 2; depth <= MAX_OVERLAY_STACK_DEPTH; depth += 1) {
    assert.equal(pushOverlayHistoryLayer({ contentId: depth, zone: "fanwork", source: "zone-page" }), true);
  }
  assert.equal(readRecord()?.depth, MAX_OVERLAY_STACK_DEPTH);
  const lengthAtCap = window.history.length;
  const urlAtCap = window.location.pathname;

  assert.equal(
    pushOverlayHistoryLayer({ contentId: 99, zone: "original", source: "zone-page" }),
    false,
    "达五层不得写第六条记录",
  );
  assert.equal(window.history.length, lengthAtCap);
  assert.equal(window.location.pathname, urlAtCap);
  assert.equal(readRecord()?.contentId, MAX_OVERLAY_STACK_DEPTH);

  endOverlayHistorySession();
});

test("history.back from depth 2 emits sync with the absolute target trail", async () => {
  setup();
  goTo("http://localhost/recommend");
  beginOverlayHistorySession({ contentId: 1, zone: "original", source: "recommendation" });
  pushOverlayHistoryLayer({ contentId: 2, zone: "fanwork", source: "zone-page" });
  const { events, unsubscribe } = collect();

  requestOverlayHistoryBack();
  await nextPopState();

  assert.equal(window.location.pathname, "/original/1");
  const sync = events.find((event) => event.type === "sync");
  assert.ok(sync && sync.type === "sync", "返回归并到同一 popstate 的 sync 事件");
  assert.equal(sync.records.length, 1);
  assert.equal(sync.records[0].contentId, 1);
  assert.equal(readRecord()?.depth, 1);

  unsubscribe();
  endOverlayHistorySession();
});

test("rapid multi-step back lands once and syncs to the target depth", async () => {
  setup();
  goTo("http://localhost/recommend");
  beginOverlayHistorySession({ contentId: 1, zone: "original", source: "recommendation" });
  pushOverlayHistoryLayer({ contentId: 2, zone: "fanwork", source: "zone-page" });
  pushOverlayHistoryLayer({ contentId: 3, zone: "original", source: "zone-page" });
  const { events, unsubscribe } = collect();

  window.history.go(-2);
  await nextPopState();
  await flush();

  assert.equal(window.location.pathname, "/original/1");
  const syncs = events.filter((event) => event.type === "sync");
  assert.equal(syncs.length, 1, "history.go(-n) 只触发一次 popstate");
  assert.ok(syncs[0].type === "sync" && syncs[0].records.length === 1);

  unsubscribe();
  endOverlayHistorySession();
});

test("back at depth 1 exits the session at the origin entry", async () => {
  setup();
  goTo("http://localhost/original?tab=all#grid");
  beginOverlayHistorySession({ contentId: 5, zone: "original", source: "zone-page" });
  const { events, unsubscribe } = collect();

  requestOverlayHistoryBack();
  await nextPopState();

  assert.equal(window.location.href, "http://localhost/original?tab=all#grid", "来源 query/hash 原样保留");
  assert.ok(events.some((event) => event.type === "exit"));
  assert.equal(readRecord(), null, "来源记录无浮层 state");

  unsubscribe();
});

test("exit unwinds all records to the origin and forward to a dead record opens the canonical host page", async () => {
  setup();
  goTo("http://localhost/recommend?sort=hot");
  beginOverlayHistorySession({ contentId: 1, zone: "original", source: "recommendation" });
  pushOverlayHistoryLayer({ contentId: 2, zone: "fanwork", source: "zone-page" });
  pushOverlayHistoryLayer({ contentId: 3, zone: "original", source: "zone-page" });

  const navigated: string[] = [];
  const unregister = registerOverlayHistoryNavigator((href) => navigated.push(href));
  const { events, unsubscribe } = collect();

  requestOverlayHistoryExit();
  await nextPopState();
  await flush();

  assert.equal(window.location.href, "http://localhost/recommend?sort=hot", "×/背板退回来源记录");
  assert.ok(events.some((event) => event.type === "exit"));
  assert.deepEqual(navigated, []);

  /* 退出整个浮层后浏览器 forward 到旧详情记录 → 打开该记录规范 URL 的完整详情页。 */
  window.history.go(3);
  await nextPopState();

  /* P2-1 回归：死记录导航必须延后一个宏任务（popstate 分发内同步 replace 会被
     App Router 随后处理的 ACTION_RESTORE 覆盖，地址与内容不一致）。 */
  assert.deepEqual(navigated, [], "popstate 分发内不得同步导航");
  await flush();
  assert.deepEqual(navigated, ["/original/3"], "死会话记录由导航器打开规范宿主页");

  unregister();
  unsubscribe();
});

test("landing on a foreign dead record while a session is active exits and opens the canonical page", async () => {
  setup();
  goTo("http://localhost/recommend");
  beginOverlayHistorySession({ contentId: 1, zone: "original", source: "recommendation" });
  pushOverlayHistoryLayer({ contentId: 2, zone: "fanwork", source: "zone-page" });
  endOverlayHistorySession();

  const { events, unsubscribe } = collect();
  const navigated: string[] = [];
  const unregister = registerOverlayHistoryNavigator((href) => navigated.push(href));
  beginOverlayHistorySession({ contentId: 8, zone: "original", source: "agent-citation" });

  window.history.back();
  await nextPopState();
  await flush();

  assert.ok(events.some((event) => event.type === "exit"), "外来会话记录使活跃会话退出");
  assert.deepEqual(navigated, ["/content/2"], "落点内容与地址一致性由规范宿主页承接");

  unregister();
  unsubscribe();
});

test("abort then immediate same-signature begin revives the session without a second push (StrictMode)", () => {
  setup();
  goTo("http://localhost/recommend");

  const first = beginOverlayHistorySession({ contentId: 1, zone: "original", source: "recommendation" });
  const lengthAfterFirst = window.history.length;
  abortOverlayHistorySession(first);
  const second = beginOverlayHistorySession({ contentId: 1, zone: "original", source: "recommendation" });

  assert.equal(second, first, "StrictMode effect 双跑复活同一会话");
  assert.equal(window.history.length, lengthAfterFirst, "不双写 depth-1 记录");
  assert.equal(window.location.pathname, "/original/1");
  assert.equal(readRecord()?.depth, 1);

  endOverlayHistorySession();
});

test("abort without revive leaves the session dead (real unmount)", async () => {
  setup();
  goTo("http://localhost/recommend");
  const lengthBefore = window.history.length;
  const sessionId = beginOverlayHistorySession({ contentId: 1, zone: "original", source: "recommendation" });
  abortOverlayHistorySession(sessionId);
  await flush();

  const { events, unsubscribe } = collect();
  window.history.back();
  await nextPopState();

  assert.equal(window.history.length, lengthBefore + 1);
  assert.equal(events.length, 0, "死会话 popstate 不再驱动已卸载的浮层");
  assert.equal(readRecord(), null);

  unsubscribe();
});

test("full exit requested while a back traversal is pending still lands on the origin", async () => {
  setup();
  goTo("http://localhost/recommend");
  beginOverlayHistorySession({ contentId: 1, zone: "original", source: "recommendation" });
  pushOverlayHistoryLayer({ contentId: 2, zone: "fanwork", source: "zone-page" });
  pushOverlayHistoryLayer({ contentId: 3, zone: "original", source: "zone-page" });
  const { events, unsubscribe } = collect();

  requestOverlayHistoryBack();
  requestOverlayHistoryExit();
  await nextPopState();
  await flush();
  await flush();

  assert.equal(window.location.href, "http://localhost/recommend", "Esc 后紧跟 × 不越过来源记录");
  assert.ok(events.some((event) => event.type === "exit"));

  unsubscribe();
});

test("exit after partial back unwinds from the current record depth, not the full trail", async () => {
  setup();
  goTo("http://localhost/recommend");
  beginOverlayHistorySession({ contentId: 1, zone: "original", source: "recommendation" });
  pushOverlayHistoryLayer({ contentId: 2, zone: "fanwork", source: "zone-page" });
  pushOverlayHistoryLayer({ contentId: 3, zone: "original", source: "zone-page" });
  /* 先多步回到 depth-1，再 ×：步数按当前记录深度（1），不得按 trail 全长（3）
     过冲越过来来源。 */
  window.history.go(-2);
  await nextPopState();

  requestOverlayHistoryExit();
  await nextPopState();
  await flush();

  assert.equal(window.location.href, "http://localhost/recommend", "× 从当前层一步退到来源");
  assert.equal(readOverlayHistoryRecord(window.history.state), null);
});

test("readOverlayHistoryRecord rejects malformed or foreign state", () => {
  assert.equal(readOverlayHistoryRecord(null), null);
  assert.equal(readOverlayHistoryRecord({}), null);
  assert.equal(readOverlayHistoryRecord({ [OVERLAY_HISTORY_STATE_KEY]: { v: 2 } }), null);
  assert.equal(
    readOverlayHistoryRecord({ [OVERLAY_HISTORY_STATE_KEY]: { v: 1, session: "s", depth: 9, contentId: "x" } }),
    null,
    "深度越界/字段类型不符一律视作非浮层记录",
  );
});
