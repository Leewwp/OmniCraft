import test from "node:test";
import assert from "node:assert/strict";
import {
  OVERLAY_ADJACENT_PREFETCH,
  __resetOverlayPrefetchForTests,
  adjacentContextNeighborIds,
  claimPrefetchedDetail,
  markContentViewed,
  prefetchAdjacentContentDetails,
} from "@/lib/overlay-prefetch";
import { installDom } from "./runtime-test-helpers";

installDom();

test.beforeEach(() => {
  __resetOverlayPrefetchForTests();
  /* jsdom 文档默认 prerender（hidden=true），真浏览器可见页为 false——预取
     门控按真实语义补桩（隐藏用例内自行覆写并恢复）。 */
  Object.defineProperty(document, "hidden", { configurable: true, get: () => false });
});
test.afterEach(() => __resetOverlayPrefetchForTests());

/** rIC 桩（runtime-test-helpers 的 setTimeout(0) 版）：await 一拍即落定。 */
const idleFlush = () => new Promise((resolve) => setTimeout(resolve, 10));

function deferred() {
  let resolve!: (value: unknown) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<unknown>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

test("E4 相邻预取 idle 门控：调度期不发请求，空闲落定后逐 id 拉取", async () => {
  const fetched: number[] = [];
  prefetchAdjacentContentDetails({
    ids: [11, 12],
    fetchDetail: (id) => {
      fetched.push(id);
      return Promise.resolve({ id });
    },
  });
  assert.deepEqual(fetched, [], "scheduling must not fetch synchronously");
  await idleFlush();
  assert.deepEqual(fetched, [11, 12], "idle flush runs each id once");
});

test("E4 相邻预取去重：同 id 重复调度只拉一次；已浏览内容跳过", async () => {
  const fetched: number[] = [];
  const run = () =>
    prefetchAdjacentContentDetails({
      ids: [21],
      fetchDetail: (id) => {
        fetched.push(id);
        return Promise.resolve({ id });
      },
    });
  const first = run();
  const second = run();
  await idleFlush();
  first.cancel();
  second.cancel();
  assert.equal(fetched.filter((id) => id === 21).length, 1, "in-flight entry dedupes the second schedule");
});

test("E4 已浏览跳过：markContentViewed 后不再回捞刚看过的内容", async () => {
  markContentViewed(31);
  const fetched: number[] = [];
  prefetchAdjacentContentDetails({
    ids: [31, 32],
    fetchDetail: (id) => {
      fetched.push(id);
      return Promise.resolve({ id });
    },
  });
  await idleFlush();
  assert.deepEqual(fetched, [32]);
});

test("E4 失败静默：reject 吞为 null、清缓存、claim 返回 null、无未处理拒绝", async () => {
  const gate = deferred();
  const handle = prefetchAdjacentContentDetails({
    ids: [41],
    fetchDetail: () => gate.promise,
  });
  await idleFlush();
  const claimed = claimPrefetchedDetail(41);
  assert.ok(claimed, "in-flight entry is claimable");
  let observed: unknown = "unset";
  void claimed.then((raw) => {
    observed = raw;
  });
  gate.reject(new Error("network down"));
  await idleFlush();
  assert.equal(observed, null, "rejection resolves to null (silent)");
  assert.equal(claimPrefetchedDetail(41), null, "failed entry leaves no cache");
  handle.cancel();
});

test("E4 并发上限：至多 2 个在途，前序释放后排队者放行", async () => {
  const gates = [deferred(), deferred(), deferred(), deferred()];
  const starts: number[] = [];
  const handle = prefetchAdjacentContentDetails({
    ids: [51, 52, 53, 54],
    fetchDetail: (id) => {
      starts.push(id);
      return gates[id - 51].promise;
    },
  });
  await idleFlush();
  assert.deepEqual(starts, [51, 52], "at most maxConcurrent=2 in flight");
  gates[0].resolve({ id: 51 });
  await idleFlush();
  assert.deepEqual(starts, [51, 52, 53], "released slot admits the next waiter");
  gates[1].resolve({ id: 52 });
  gates[2].resolve({ id: 53 });
  gates[3].resolve({ id: 54 });
  await idleFlush();
  handle.cancel();
});

test("E4 cancel：空闲期取消不发请求；在途取消 abort 未认领 signal", async () => {
  /* 空闲期取消：任务不执行。 */
  const fetched: number[] = [];
  const early = prefetchAdjacentContentDetails({
    ids: [61],
    fetchDetail: (id) => {
      fetched.push(id);
      return Promise.resolve({ id });
    },
  });
  early.cancel();
  await idleFlush();
  /* expected 侧显式 number[]：asserts-narrow 推断为 never[] 会污染后续使用。 */
  assert.deepEqual(fetched, [] as number[], "cancelled idle task never runs");

  /* 在途取消：未认领条目收到 abort 且出缓存。 */
  const gate = deferred();
  const signals: AbortSignal[] = [];
  const late = prefetchAdjacentContentDetails({
    ids: [62],
    fetchDetail: (id, signal) => {
      signals.push(signal);
      fetched.push(id);
      return gate.promise;
    },
  });
  await idleFlush();
  assert.equal(fetched.includes(62), true);
  late.cancel();
  assert.equal(signals[0].aborted, true, "unclaimed in-flight fetch is aborted");
  assert.equal(claimPrefetchedDetail(62), null, "aborted entry leaves no cache");
  /* 吞掉 rejection，避免未处理拒绝（模块层 .catch 已吞，此处兜底断言）。 */
  gate.reject(new Error("aborted"));
  await idleFlush();
});

test("E4 claim 可重复消费（StrictMode 双挂载同享缓存）：两次认领同一 promise", async () => {
  const gate = deferred();
  const handle = prefetchAdjacentContentDetails({
    ids: [71],
    fetchDetail: () => gate.promise,
  });
  await idleFlush();
  const first = claimPrefetchedDetail(71);
  const second = claimPrefetchedDetail(71);
  assert.ok(first && second, "repeat claims share the warm entry within the TTL");
  gate.resolve({ content: { id: 71 } });
  assert.deepEqual(await first, { content: { id: 71 } }, "claim resolves the raw detail");
  assert.deepEqual(await second, { content: { id: 71 } }, "second claim resolves the same entry");
  handle.cancel();
});

test("E4 TTL 软过期：超时条目视为未命中（防陈旧签名 URL）", async () => {
  const gate = deferred();
  const handle = prefetchAdjacentContentDetails({
    ids: [81],
    fetchDetail: () => gate.promise,
  });
  await idleFlush();
  gate.resolve({ content: { id: 81 } });
  await idleFlush();
  const realNow = Date.now;
  Date.now = () => realNow() + OVERLAY_ADJACENT_PREFETCH.detailTtlMs + 1;
  try {
    assert.equal(claimPrefetchedDetail(81), null, "expired entry is a miss");
  } finally {
    Date.now = realNow;
  }
  handle.cancel();
});

test("E4 页面隐藏不预取（不挤占后台带宽）", async () => {
  const original = Object.getOwnPropertyDescriptor(document, "hidden");
  Object.defineProperty(document, "hidden", { configurable: true, get: () => true });
  const fetched: number[] = [];
  const handle = prefetchAdjacentContentDetails({
    ids: [91],
    fetchDetail: (id) => {
      fetched.push(id);
      return Promise.resolve({ id });
    },
  });
  await idleFlush();
  assert.deepEqual(fetched, [], "hidden page never starts a prefetch");
  handle.cancel();
  const holder = document as Partial<Document> & { hidden?: boolean };
  if (original) Object.defineProperty(document, "hidden", original);
  else delete holder.hidden;
});

test("E4 warmDetail 预热钩子：详情落定即回调（失败请求不回调）", async () => {
  const warmed: Array<{ id: number; raw: unknown }> = [];
  const handle = prefetchAdjacentContentDetails({
    ids: [101],
    fetchDetail: () => Promise.resolve({ content: { id: 101, cover_image_url: "u" } }),
    warmDetail: (id, raw) => warmed.push({ id, raw }),
  });
  await idleFlush();
  await idleFlush();
  assert.deepEqual(
    warmed.map((entry) => entry.id),
    [101],
  );
  handle.cancel();
});

test("E4 release（层内换篇）：取消未起跑调度、保留在途预取供新层认领", async () => {
  /* 在途条目：release 后仍可认领（换篇场景，abort 掉反而重复拉取）。 */
  const gate = deferred();
  const inFlightHandle = prefetchAdjacentContentDetails({
    ids: [111],
    fetchDetail: () => gate.promise,
  });
  await idleFlush();
  inFlightHandle.release();
  const claimed = claimPrefetchedDetail(111);
  assert.ok(claimed, "in-flight entry survives release for the next layer to claim");
  gate.resolve({ id: 111 });
  assert.deepEqual(await claimed, { id: 111 });
  inFlightHandle.cancel();

  /* 未起跑调度：release 即取消，不再发请求。 */
  const fetched: number[] = [];
  const pendingHandle = prefetchAdjacentContentDetails({
    ids: [112],
    fetchDetail: (id) => {
      fetched.push(id);
      return Promise.resolve({ id });
    },
  });
  pendingHandle.release();
  await idleFlush();
  assert.deepEqual(fetched, [] as number[], "released idle task never runs");
  pendingHandle.cancel();
});

test("E4 已落定热缓存不被硬取消驱逐（关窗后再点开 = 主消费路径）", async () => {
  const handle = prefetchAdjacentContentDetails({
    ids: [121],
    fetchDetail: () => Promise.resolve({ content: { id: 121 } }),
  });
  await idleFlush();
  await idleFlush();
  handle.cancel();
  const claimed = claimPrefetchedDetail(121);
  assert.ok(claimed, "settled warm entry survives overlay close (cancel aborts in-flight only)");
  assert.deepEqual(await claimed, { content: { id: 121 } });
});

test("E4 adjacentContextNeighborIds：next 优先、边界单项、index 缺失为空、自引用去重", () => {
  const list = [{ id: 1 }, { id: 2 }, { id: 3 }];
  assert.deepEqual(adjacentContextNeighborIds(list, 1), [3, 1], "middle yields next first then prev");
  assert.deepEqual(adjacentContextNeighborIds(list, 0), [2], "head has next only");
  assert.deepEqual(adjacentContextNeighborIds(list, 2), [2], "tail has prev only");
  assert.deepEqual(adjacentContextNeighborIds(list, undefined), []);
  assert.deepEqual(adjacentContextNeighborIds(list, 9), []);
  assert.deepEqual(adjacentContextNeighborIds(list, -1), []);
  assert.deepEqual(adjacentContextNeighborIds([{ id: 5 }, { id: 5 }], 0), [], "self id never prefetched");
});
