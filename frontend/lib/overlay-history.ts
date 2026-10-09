"use client";

/* D3 #860 详情弹窗 URL 深链 history 状态机（spec §3.4 / §9.4）。
 *
 * 职责边界：本模块是「浏览器 history 会话」的唯一权威；React 栈（滚动、焦点、
 * 动效）仍归 ContentDetailOverlay，两者通过事件（sync/exit）在 popstate 上
 * 对齐。设计要点：
 *
 * - Next 16.4 App Router 会包装 window.history.pushState/replaceState（app-router.js
 *   copyNextJsInternalHistoryState）：经包装层写入的记录自动携带 __NA 与
 *   __PRIVATE_NEXTJS_INTERNALS_TREE（取自当前记录），并同步 router canonicalUrl
 *   （usePathname/useSearchParams 立即反映新路径、渲染树不变、背景列表不卸载）。
 *   本模块只写自有命名空间 `omnicraftContentOverlay`，绝不手工伪造 Next 私有
 *   字段——伪造会导致包装层短路直通（__NA 短路分支），router 不同步、popstate
 *   语义退化。
 * - popstate 落点判定：属于当前会话 → sync（按目标深度绝对对齐，支持
 *   history.go(-n)、快速连按、动画重入）；无记录（来源或外部记录）→ exit；
 *   死会话记录（浮层已关闭后 forward/back 命中）→ 由导航器打开该记录规范
 *   URL 的宿主详情页（spec §9.4 前进/失活记录的最小一致行为）。
 * - 转移表：打开/压层 pushState、连续换篇 replaceState、按钮/Esc 统一走
 *   history.back（与浏览器返回归并同一 popstate）、×/背板 history.go(-depth)
 *   退回来源记录；最大五层，达五层不写第六条记录。
 * - StrictMode（dev effect 双跑）：abort 后短时间内同签名的 begin 复活同一
 *   会话，不重复 push depth-1 记录。
 */

export const OVERLAY_HISTORY_STATE_KEY = "omnicraftContentOverlay";

/** 与浮层 React 栈共用的深度上限（spec §9.4：达五层不得写第六条记录）。 */
export const MAX_OVERLAY_STACK_DEPTH = 5;

export type OverlayHistoryZone = "original" | "fanwork";
export type OverlayHistorySource = "recommendation" | "zone-page" | "ip-page" | "agent-citation";

/** 写入 history state 的浮层记录（自有命名空间内）。 */
export interface OverlayHistoryRecord {
  v: 1;
  /** 浮层会话标识：一次打开→关闭为一个会话，popstate 只处理属于当前会话的记录。 */
  session: string;
  /** 1 基深度（栈底 = 1）。 */
  depth: number;
  contentId: number;
  zone: OverlayHistoryZone;
  /** 打开浮层时来源页完整 URL（含 query/hash），诊断与一致性校验用。 */
  originHref: string;
  source: OverlayHistorySource;
  /** #89 连续浏览上下文（移动端），重建该层时恢复连续浏览语义。 */
  contextList?: Array<{ id: number; zone: OverlayHistoryZone }>;
  contextIndex?: number;
}

export interface OverlayHistoryEntryRef {
  contentId: number;
  zone: OverlayHistoryZone;
  source: OverlayHistorySource;
  contextList?: Array<{ id: number; zone: OverlayHistoryZone }>;
  contextIndex?: number;
}

export type OverlayHistoryEvent =
  | { type: "exit" }
  | { type: "sync"; records: OverlayHistoryRecord[] };

interface OverlaySessionState {
  id: string;
  originHref: string;
  /** trail[i] = 深度 i+1 的记录；forward 重建层与全退步数都从这里取。 */
  trail: OverlayHistoryRecord[];
  first: { contentId: number; zone: OverlayHistoryZone };
  beganAt: number;
}

/** StrictMode effect 双跑的复活窗口（两次跑之间为同步相邻，远小于该值）。 */
const SESSION_REVIVE_WINDOW_MS = 150;

let session: OverlaySessionState | null = null;
let recentlyAborted: (OverlaySessionState & { abortedAt: number }) | null = null;
let traversalPending = false;
let fullExitRequested = false;
let controllerInstalled = false;
let sessionCounter = 0;

type OverlayHistoryListener = (event: OverlayHistoryEvent) => void;
const listeners = new Set<OverlayHistoryListener>();
let deadRecordNavigator: ((href: string) => void) | null = null;

function isBrowser(): boolean {
  return typeof window !== "undefined" && typeof window.history === "object";
}

function emit(event: OverlayHistoryEvent) {
  for (const listener of listeners) listener(event);
}

/** 规范路径：fanwork=/content/<id>、original=/original/<id>（不加任何签名参数）。 */
export function canonicalOverlayPath(zone: OverlayHistoryZone, contentId: number): string {
  return zone === "fanwork" ? `/content/${contentId}` : `/original/${contentId}`;
}

/** 校验并读出 history state 里的浮层记录；任何外形不符都视作非浮层记录。 */
export function readOverlayHistoryRecord(historyState: unknown): OverlayHistoryRecord | null {
  if (typeof historyState !== "object" || historyState === null) return null;
  const candidate = (historyState as Record<string, unknown>)[OVERLAY_HISTORY_STATE_KEY];
  if (typeof candidate !== "object" || candidate === null) return null;
  const record = candidate as Partial<OverlayHistoryRecord>;
  if (record.v !== 1) return null;
  if (typeof record.session !== "string" || record.session.length === 0) return null;
  if (typeof record.depth !== "number" || record.depth < 1 || record.depth > MAX_OVERLAY_STACK_DEPTH) return null;
  if (typeof record.contentId !== "number" || !Number.isFinite(record.contentId)) return null;
  if (record.zone !== "original" && record.zone !== "fanwork") return null;
  if (typeof record.originHref !== "string") return null;
  if (
    record.source !== "recommendation" &&
    record.source !== "zone-page" &&
    record.source !== "ip-page" &&
    record.source !== "agent-citation"
  ) {
    return null;
  }
  return candidate as OverlayHistoryRecord;
}

export function subscribeOverlayHistory(listener: OverlayHistoryListener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/** 浮层关闭后命中死会话记录时的宿主页接管者（根布局 bridge 注册 router.replace）。 */
export function registerOverlayHistoryNavigator(navigate: (href: string) => void): () => void {
  deadRecordNavigator = navigate;
  return () => {
    if (deadRecordNavigator === navigate) deadRecordNavigator = null;
  };
}

function navigateDeadRecordToCanonical(record: OverlayHistoryRecord) {
  const href = canonicalOverlayPath(record.zone, record.contentId);
  /* 必须延后一个宏任务：popstate 分发内 App Router 自己的 listener 也派发了
     traverse→ACTION_RESTORE（恢复该记录压栈时拷贝的旧页树）。若在此分发内同步
     router.replace，其 ACTION_NAVIGATE 会先入队、RESTORE 后处理并覆盖——最终态
     = 旧页树 + 新地址（地址与可见内容不一致，spec §9.4 禁止）。延后到 popstate
     处理与 restore transition 落定之后再导航，navigate 即队列中的最终动作。 */
  window.setTimeout(() => {
    if (deadRecordNavigator) {
      deadRecordNavigator(href);
      return;
    }
    window.location.replace(href);
  }, 0);
}

function clearSessionState() {
  session = null;
  traversalPending = false;
  fullExitRequested = false;
}

function openDeadRecord(record: OverlayHistoryRecord) {
  clearSessionState();
  emit({ type: "exit" });
  navigateDeadRecordToCanonical(record);
}

function handlePopState() {
  const record = readOverlayHistoryRecord(window.history.state);

  if (record && session && record.session === session.id) {
    traversalPending = false;
    if (fullExitRequested) {
      /* back 途经中途层后仍要全退：从当前层一次 go 到来源记录。 */
      fullExitRequested = false;
      beginTraversal(-record.depth);
      return;
    }
    emit({ type: "sync", records: session.trail.slice(0, record.depth) });
    return;
  }

  if (!record) {
    if (session) {
      /* 落回来源记录（或任何无浮层记录的入口）：会话终结，浮层执行关闭动效。 */
      clearSessionState();
      emit({ type: "exit" });
      return;
    }
    return;
  }

  /* 记录存在但不属于当前会话：死会话记录（forward/back 命中旧详情）。
     活跃会话一并退出，落点统一交给规范宿主页，保证地址与可见内容一致。 */
  openDeadRecord(record);
}

/** 安装全局 popstate 控制器（幂等）。根布局 bridge 挂载时调用，会话开始前
 *  兜底安装，保证「浮层关闭后 forward 到死记录」也有人接管。 */
export function installOverlayHistoryController(): void {
  if (controllerInstalled || !isBrowser()) return;
  controllerInstalled = true;
  window.addEventListener("popstate", handlePopState);
}

function makeRecord(
  sessionId: string,
  depth: number,
  entry: OverlayHistoryEntryRef,
  originHref: string,
): OverlayHistoryRecord {
  return {
    v: 1,
    session: sessionId,
    depth,
    contentId: entry.contentId,
    zone: entry.zone,
    originHref,
    source: entry.source,
    ...(entry.contextList ? { contextList: entry.contextList } : {}),
    ...(entry.contextIndex !== undefined ? { contextIndex: entry.contextIndex } : {}),
  };
}

function beginTraversal(delta: number) {
  traversalPending = true;
  window.history.go(delta);
}

/** 打开浮层：记录会话并 pushState 到首层规范路径。返回会话 id。 */
export function beginOverlayHistorySession(entry: OverlayHistoryEntryRef): string {
  installOverlayHistoryController();
  if (!isBrowser()) return "";

  const now = Date.now();
  if (
    recentlyAborted &&
    now - recentlyAborted.abortedAt < SESSION_REVIVE_WINDOW_MS &&
    recentlyAborted.first.contentId === entry.contentId &&
    recentlyAborted.first.zone === entry.zone
  ) {
    const currentRecord = readOverlayHistoryRecord(window.history.state);
    if (currentRecord && currentRecord.session === recentlyAborted.id && currentRecord.depth === 1) {
      /* StrictMode 卸载→重挂：当前记录仍是本会话的 depth-1，复活会话，
         不再 push 第二条记录（trail 以当前记录为基，自愈任何漂移）。 */
      session = { ...recentlyAborted, trail: [currentRecord], beganAt: now };
      recentlyAborted = null;
      traversalPending = false;
      fullExitRequested = false;
      return session.id;
    }
    /* 当前记录已不属于该会话（真实跳转后重开）：复活窗口作废，走全新会话。 */
    recentlyAborted = null;
  } else {
    recentlyAborted = null;
  }

  sessionCounter += 1;
  const originHref = window.location.href;
  const next: OverlaySessionState = {
    id: `overlay-${now.toString(36)}-${sessionCounter}`,
    originHref,
    trail: [],
    first: { contentId: entry.contentId, zone: entry.zone },
    beganAt: now,
  };
  session = next;
  traversalPending = false;
  fullExitRequested = false;
  const record = makeRecord(next.id, 1, entry, originHref);
  next.trail.push(record);
  /* 经 Next 包装层 pushState：内部字段由包装层自补，router canonicalUrl 同步。 */
  window.history.pushState({ [OVERLAY_HISTORY_STATE_KEY]: record }, "", canonicalOverlayPath(entry.zone, entry.contentId));
  return next.id;
}

/** 压层：pushState 到新层规范路径；达五层返回 false（不写第六条记录）。 */
export function pushOverlayHistoryLayer(entry: OverlayHistoryEntryRef): boolean {
  if (!isBrowser() || !session) return false;
  if (session.trail.length >= MAX_OVERLAY_STACK_DEPTH) return false;
  const record = makeRecord(session.id, session.trail.length + 1, entry, session.originHref);
  session.trail.push(record);
  window.history.pushState(
    { [OVERLAY_HISTORY_STATE_KEY]: record },
    "",
    canonicalOverlayPath(entry.zone, entry.contentId),
  );
  return true;
}

/** 连续换篇（#89）：replaceState 原地换顶层记录与 URL，不新增历史。 */
export function replaceOverlayHistoryTop(entry: OverlayHistoryEntryRef): void {
  if (!isBrowser() || !session || session.trail.length === 0) return;
  const record = makeRecord(session.id, session.trail.length, entry, session.originHref);
  session.trail[session.trail.length - 1] = record;
  window.history.replaceState(
    { [OVERLAY_HISTORY_STATE_KEY]: record },
    "",
    canonicalOverlayPath(entry.zone, entry.contentId),
  );
}

/** 返回按钮 / Esc：与浏览器返回归并——经 history.back 触发同一个 popstate。 */
export function requestOverlayHistoryBack(): void {
  if (!isBrowser() || !session || session.trail.length === 0) return;
  if (traversalPending) return;
  beginTraversal(-1);
}

/** ×/背板全退：一次 go 回到会话来源记录；back 在途时先记全退意图，
 *  popstate 到达中途层后补齐剩余步数（Esc 紧跟 × 不越过来源）。
 *  步数按当前记录深度计（部分返回后 × 不得按 trail 全长过冲越过来来源）。 */
export function requestOverlayHistoryExit(): void {
  if (!isBrowser() || !session || session.trail.length === 0) return;
  if (traversalPending) {
    fullExitRequested = true;
    return;
  }
  const current = readOverlayHistoryRecord(window.history.state);
  const steps =
    current && current.session === session.id ? current.depth : session.trail.length;
  beginTraversal(-steps);
}

/** 正常关闭收尾（finalizeClose）：会话终结，不留可复活状态。 */
export function endOverlayHistorySession(): void {
  recentlyAborted = null;
  if (!isBrowser()) return;
  clearSessionState();
}

/** 浮层卸载（未经关闭动效，如宿主页真实跳转）：清会话；StrictMode 双跑窗口
 *  内的同签名 begin 会复活同一会话，避免 dev 下双写 depth-1 记录。 */
export function abortOverlayHistorySession(sessionId: string): void {
  if (!isBrowser()) return;
  if (!session || session.id !== sessionId) return;
  recentlyAborted = { ...session, abortedAt: Date.now() };
  clearSessionState();
}

/** 测试专用：清空全部模块状态（history 栈本身由 installDom 复位 URL）。 */
export function __resetOverlayHistoryForTests(): void {
  session = null;
  recentlyAborted = null;
  traversalPending = false;
  fullExitRequested = false;
  deadRecordNavigator = null;
  listeners.clear();
}
