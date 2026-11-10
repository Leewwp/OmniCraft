/**
 * 详情浮窗相邻内容 idle 预取管线（E4 #956，spec「真实媒体延迟」①：相邻内容
 * JSON+封面 idle 预取）。api 客户端（lib/api request）无缓存层——每次
 * api.get 都是裸 fetch，故此处自建会话级内存 Map（SWR 风格去重，key=
 * contentId）存「预取 promise」，正式打开命中时由 Layer 认领（claim）直接
 * 消费，未命中走原有拉取路径不受阻（预取是加速器不是就绪门）。
 *
 * 调度契约（票面验收）：
 * - idle 门控：requestIdleCallback + timeout 兜底（无 ric 环境 setTimeout）；
 * - 并发上限（≤2）：跨内容一批至多前后两项，计数信号量通用兜底；
 * - 失败静默：预取 reject/abort 一律吞为 null、不留缓存、不重试（一次预取
 *   一轮；正式打开时走常规拉取，不是预取重试）；
 * - 取消在途：浮窗关闭（调用方 effect cleanup）/页面隐藏时 abort 未被认领
 *   的在途请求；已被认领（正在被打开消费）的除外；
 * - TTL：与 OSS 展示签名 URL 有效期（300s）对齐的软过期（240s），过期条目
 *   视为未命中，防陈旧签名 URL 进浮窗媒体链；
 * - 已浏览跳过：正式打开成功的内容登记为 viewed，后续相邻预取不再回捞
 *   （向前浏览场景 prev 常是刚看过的内容）。
 * 单元可测性：fetcher/warmer 注入 + __resetOverlayPrefetchForTests。
 */

export const OVERLAY_ADJACENT_PREFETCH = {
  /** requestIdleCallback 的 timeout：空闲迟迟不来时强制执行。 */
  idleTimeoutMs: 3000,
  /** 无 requestIdleCallback 环境的兜底延时。 */
  fallbackDelayMs: 300,
  /** 预取详情缓存软过期（OSS 展示签名 300s 有效期，留 60s 余量）。 */
  detailTtlMs: 240_000,
  /** 相邻预取并发上限（前后各 1，通用信号量兜底）。 */
  maxConcurrent: 2,
} as const;

/** 空闲调度（requestIdleCallback + timeout 兜底）；返回取消函数。 */
export function scheduleIdleTask(task: () => void): () => void {
  if (typeof window === "undefined") return () => undefined;
  if (typeof window.requestIdleCallback === "function") {
    const handle = window.requestIdleCallback(task, {
      timeout: OVERLAY_ADJACENT_PREFETCH.idleTimeoutMs,
    });
    return () => window.cancelIdleCallback?.(handle);
  }
  const timer = window.setTimeout(task, OVERLAY_ADJACENT_PREFETCH.fallbackDelayMs);
  return () => window.clearTimeout(timer);
}

/**
 * 上下文列表相邻项（前后各 1，next 优先——连续浏览/顺序下滑更可能打开
 * 下一项）；去重自身；index 缺失/越界（无法判定相邻）返回空。
 */
export function adjacentContextNeighborIds(
  list: Array<{ id: number }>,
  index: number | undefined,
): number[] {
  if (index === undefined || index < 0 || index >= list.length) return [];
  const self = list[index].id;
  const neighbors: number[] = [];
  const next = list[index + 1];
  if (next && next.id !== self) neighbors.push(next.id);
  const prev = list[index - 1];
  if (prev && prev.id !== self) neighbors.push(prev.id);
  return neighbors;
}

interface DetailCacheEntry {
  /** 解析为原始详情响应；失败/中止一律 null（调用方静默走常规拉取）。 */
  promise: Promise<unknown>;
  /** 预取已落定（成功或失败）：硬取消只中止在途，不驱逐已落定的热缓存
      （关浮窗后再点开相邻卡 = 本管线的主消费路径）。 */
  settled: boolean;
  controller: AbortController | null;
  createdAt: number;
}

const detailCache = new Map<number, DetailCacheEntry>();
const viewedContentIds = new Set<number>();

let inFlight = 0;
const slotWaiters: Array<() => void> = [];

/** 计数信号量：并发上限内立即起跑，超限排队（前序释放后依序放行）。 */
function runWithSlot(start: () => Promise<unknown>): Promise<unknown> {
  const attempt = (): Promise<unknown> => {
    inFlight += 1;
    return start().finally(() => {
      inFlight -= 1;
      slotWaiters.shift()?.();
    });
  };
  if (inFlight < OVERLAY_ADJACENT_PREFETCH.maxConcurrent) return attempt();
  return new Promise<unknown>((resolve) => {
    slotWaiters.push(() => resolve(attempt()));
  });
}

/**
 * 正式打开时认领预取结果：命中（未过期）返回预取 promise（TTL 内可重复
 * 消费——dev StrictMode 双挂载与短时间内重复打开同享缓存），未命中/已过期
 * 返回 null（调用方走常规 api.get）。
 */
export function claimPrefetchedDetail(id: number): Promise<unknown> | null {
  const entry = detailCache.get(id);
  if (!entry) return null;
  if (Date.now() - entry.createdAt > OVERLAY_ADJACENT_PREFETCH.detailTtlMs) {
    detailCache.delete(id);
    return null;
  }
  return entry.promise;
}

/** 正式打开成功的内容登记为已浏览：相邻预取不再回捞刚看过的内容。 */
export function markContentViewed(id: number): void {
  viewedContentIds.add(id);
}

export interface AdjacentPrefetchOptions {
  /** 相邻内容 id（前后各 1）。 */
  ids: number[];
  /** 详情拉取（走既有 api 客户端：同 cookie/CSRF 语义 + abort signal）。 */
  fetchDetail: (id: number, signal: AbortSignal) => Promise<unknown>;
  /** 详情落定后的预热钩子（封面规范变体预热；不抛错、不阻塞）。 */
  warmDetail?: (id: number, raw: unknown) => void;
}

export interface AdjacentPrefetchHandle {
  /** 软释放（层内换篇）：取消未起跑的空闲调度并摘除监听；已在途的预取保留
      （紧随其后的新内容层正要认领它们，abort 掉反而制造重复拉取）。 */
  release: () => void;
  /** 硬取消（浮窗关闭/页面隐藏）：软释放 + abort 未认领的在途请求并出缓存。 */
  cancel: () => void;
}

/**
 * 相邻内容详情 idle 预取：每 id 独立空闲调度（超时兜底）+ 并发上限 + 去重
 * （缓存已有/已 viewed 跳过）；失败吞为 null 并清缓存。页面隐藏即硬取消。
 */
export function prefetchAdjacentContentDetails(options: AdjacentPrefetchOptions): AdjacentPrefetchHandle {
  const { ids, fetchDetail, warmDetail } = options;
  const idleCancellers = new Set<() => void>();
  const ownedIds = new Set<number>();
  let cancelled = false;

  const release = () => {
    for (const cancelIdle of idleCancellers) cancelIdle();
    idleCancellers.clear();
    if (typeof document !== "undefined") {
      document.removeEventListener("visibilitychange", onVisibilityChange);
    }
  };

  const handle: AdjacentPrefetchHandle = {
    release,
    cancel() {
      if (cancelled) return;
      cancelled = true;
      release();
      for (const id of ownedIds) {
        const entry = detailCache.get(id);
        /* 已落定（关窗后再点开的主消费路径）不驱逐；只中止仍在途的请求。 */
        if (!entry || entry.settled) continue;
        entry.controller?.abort();
        detailCache.delete(id);
      }
    },
  };

  function onVisibilityChange() {
    if (document.hidden) handle.cancel();
  }

  const runOne = (id: number) => {
    if (cancelled || detailCache.has(id) || viewedContentIds.has(id)) return;
    /* 页面隐藏时不启动新预取（在途取消由 visibilitychange 承担）。 */
    if (typeof document !== "undefined" && document.hidden) return;
    ownedIds.add(id);
    const controller = typeof AbortController !== "undefined" ? new AbortController() : null;
    const entry: DetailCacheEntry = {
      createdAt: Date.now(),
      settled: false,
      controller,
      promise: runWithSlot(() =>
        fetchDetail(id, controller ? controller.signal : ({} as AbortSignal))
          .then((raw) => {
            if (raw != null) {
              try {
                warmDetail?.(id, raw);
              } catch {
                /* 预热钩子失败静默 */
              }
            }
            return raw;
          })
          .catch(() => null),
      ),
    };
    detailCache.set(id, entry);
    /* 落定记账（成功/失败）；失败（null）不留缓存：后续正式打开走常规拉取，
       不算重试风暴。 */
    void entry.promise.then((raw) => {
      entry.settled = true;
      if (raw == null) detailCache.delete(id);
    });
  };

  for (const id of ids) {
    idleCancellers.add(scheduleIdleTask(() => runOne(id)));
  }
  if (typeof document !== "undefined") {
    document.addEventListener("visibilitychange", onVisibilityChange);
  }
  return handle;
}

/** 单测隔离：清空会话级缓存/已浏览集合（生产代码不调用）。 */
export function __resetOverlayPrefetchForTests(): void {
  detailCache.clear();
  viewedContentIds.clear();
  inFlight = 0;
  slotWaiters.length = 0;
}
