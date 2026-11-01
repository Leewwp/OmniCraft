import test from "node:test";
import assert from "node:assert/strict";
import type { MouseEvent } from "react";

import {
  SWIPE_THRESHOLD,
  VIDEO_CONTROLS_STRIP,
  attemptPlay,
  clampIndex,
  toggleVideoSlideClick,
} from "@/lib/media-interaction";
import { installDom } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * C1 #920 媒体交互共享原语（lib/media-interaction）纯函数面：
 * - clampIndex / attemptPlay / 常量：三宿主（轮播位 / 放大态 / 画廊）此前
 *   各自复制的微工具收口后的唯一来源；
 * - toggleVideoSlideClick：视频点击切换共享处理函数的差异面——返回值 =
 *   是否消费点击、controls 条/原生全屏豁免、preventDefault 防双触发、
 *   Map 形态视频元素获取、userPaused 记账先于播放/暂停的既有顺序。
 * ──────────────────────────────────────────────────────────────────────────── */

test.beforeEach(() => installDom());

test("共享常量与既有三宿主取值一致（controls 条 44 / 滑动阈值 40）", () => {
  assert.equal(VIDEO_CONTROLS_STRIP, 44);
  assert.equal(SWIPE_THRESHOLD, 40);
});

test("clampIndex：非有限值/空序列归 0，负数归首、越界归末（不循环）", () => {
  assert.equal(clampIndex(0, 3), 0);
  assert.equal(clampIndex(2, 3), 2);
  assert.equal(clampIndex(9, 3), 2, "over-range clamps to the last index");
  assert.equal(clampIndex(-4, 3), 0, "negative clamps to the first index");
  assert.equal(clampIndex(1.7, 3), 1, "fractional floors before clamping");
  assert.equal(clampIndex(Number.NaN, 3), 0, "NaN falls back to 0");
  assert.equal(clampIndex(Number.POSITIVE_INFINITY, 3), 0, "non-finite values fall back to 0");
  assert.equal(clampIndex(5, 0), 0, "empty sequence stays at 0");
});

test("attemptPlay：play() 被策略拒绝时静默（不产生未处理 rejection）", async () => {
  let rejected = false;
  const video = {
    paused: false,
    play: () =>
      new Promise<void>((_, reject) => {
        rejected = true;
        reject(new Error("NotAllowedError"));
      }),
    pause: () => {
      throw new Error("pause must not be called by attemptPlay");
    },
  } as unknown as HTMLVideoElement;
  attemptPlay(video);
  assert.ok(rejected, "play() was attempted");
  /* 静默失败 = catch 已挂接：给微任务队列一个空转窗口，未处理 rejection
     会让 node:test 进程报错（测试即失败）。 */
  await new Promise((resolve) => setImmediate(resolve));
});

test("attemptPlay：jsdom 形态 play() 返回 undefined 同样防御（不抛错）", () => {
  const video = {
    paused: false,
    play: () => undefined,
    pause: () => {
      throw new Error("pause must not be called");
    },
  } as unknown as HTMLVideoElement;
  attemptPlay(video);
});

/* ── toggleVideoSlideClick 差异面 ─────────────────────────────────────────── */

interface FakeEventInit {
  clientY: number;
  rect: { height: number; bottom: number };
  fullscreen?: boolean;
}

/** 结构化桩事件：只满足共享函数消费的面（currentTarget 矩形 + clientY +
 *  preventDefault），不用真实 React 合成事件。 */
function fakeEvent({ clientY, rect, fullscreen }: FakeEventInit) {
  const anchor = document.createElement("div");
  anchor.getBoundingClientRect = () =>
    ({ height: rect.height, top: 0, bottom: rect.bottom, width: 300, left: 0, right: 300, x: 0, y: 0, toJSON: () => {} }) as DOMRect;
  let defaultPrevented = false;
  let fullscreenActive = Boolean(fullscreen);
  Object.defineProperty(document, "fullscreenElement", {
    configurable: true,
    get: () => (fullscreenActive ? document.body : null),
  });
  const event = {
    clientY,
    currentTarget: anchor,
    preventDefault: () => {
      defaultPrevented = true;
    },
    get defaultPrevented() {
      return defaultPrevented;
    },
    __setFullscreen(value: boolean) {
      fullscreenActive = value;
    },
  };
  return event as unknown as MouseEvent<HTMLElement> & { defaultPrevented: boolean };
}

interface FakeVideoInit {
  paused: boolean;
  log: string[];
  tag: string;
}

/** 桩视频：记录 play/pause 调用序（记账顺序断言用）。 */
function fakeVideo({ paused, log, tag }: FakeVideoInit): HTMLVideoElement {
  let isPaused = paused;
  return {
    get paused() {
      return isPaused;
    },
    play: () => {
      log.push(`play:${tag}`);
      isPaused = false;
      return Promise.resolve();
    },
    pause: () => {
      log.push(`pause:${tag}`);
      isPaused = true;
    },
  } as unknown as HTMLVideoElement;
}

const BODY_RECT = { height: 500, bottom: 500 };

test("toggle：播放中视频点击 = 记账「用户暂停」先于 pause()，返回已消费", () => {
  const log: string[] = [];
  const video = fakeVideo({ paused: false, log, tag: "v1" });
  const event = fakeEvent({ clientY: 100, rect: BODY_RECT });
  const consumed = toggleVideoSlideClick({
    event,
    getVideo: () => video,
    onUserPausedChange: (paused) => log.push(`record:${paused}`),
  });
  assert.equal(consumed, true, "main-body click is consumed");
  assert.equal(event.defaultPrevented, true, "WebKit default toggle is suppressed");
  assert.deepEqual(log, ["record:true", "pause:v1"], "bookkeeping happens before the pause call");
});

test("toggle：暂停中视频点击 = 记账「非用户暂停」先于 play()，返回已消费", () => {
  const log: string[] = [];
  const video = fakeVideo({ paused: true, log, tag: "v2" });
  const event = fakeEvent({ clientY: 100, rect: BODY_RECT });
  const consumed = toggleVideoSlideClick({
    event,
    getVideo: () => video,
    onUserPausedChange: (paused) => log.push(`record:${paused}`),
  });
  assert.equal(consumed, true);
  assert.deepEqual(log, ["record:false", "play:v2"], "bookkeeping happens before the play call");
});

test("toggle：无记账回调（画廊形态）只切播不记账", () => {
  const log: string[] = [];
  const video = fakeVideo({ paused: false, log, tag: "v3" });
  const consumed = toggleVideoSlideClick({
    event: fakeEvent({ clientY: 100, rect: BODY_RECT }),
    getVideo: () => video,
  });
  assert.equal(consumed, true);
  assert.deepEqual(log, ["pause:v3"], "no bookkeeping entries without the callback");
});

test("controls 条区域点击：不消费、不 preventDefault、不切播、不记账", () => {
  const log: string[] = [];
  const video = fakeVideo({ paused: false, log, tag: "v4" });
  const event = fakeEvent({ clientY: BODY_RECT.bottom - VIDEO_CONTROLS_STRIP + 5, rect: BODY_RECT });
  const consumed = toggleVideoSlideClick({
    event,
    getVideo: () => video,
    onUserPausedChange: (paused) => log.push(`record:${paused}`),
  });
  assert.equal(consumed, false, "native-controls clicks stay with the browser");
  assert.equal(event.defaultPrevented, false);
  assert.deepEqual(log, [], "no playback or bookkeeping side effects");
});

test("controls 条判定：容器高度 0（测量未就绪）不视为条区，正常切播", () => {
  const log: string[] = [];
  const video = fakeVideo({ paused: true, log, tag: "v5" });
  const consumed = toggleVideoSlideClick({
    event: fakeEvent({ clientY: 999, rect: { height: 0, bottom: 0 } }),
    getVideo: () => video,
  });
  assert.equal(consumed, true, "degenerate rect falls through to the toggle path");
  assert.deepEqual(log, ["play:v5"]);
});

test("原生全屏内点击：不消费（交还原生控件）", () => {
  const log: string[] = [];
  const video = fakeVideo({ paused: false, log, tag: "v6" });
  const event = fakeEvent({ clientY: 100, rect: BODY_RECT, fullscreen: true });
  const consumed = toggleVideoSlideClick({
    event,
    getVideo: () => video,
    onUserPausedChange: (paused) => log.push(`record:${paused}`),
  });
  assert.equal(consumed, false);
  assert.equal(event.defaultPrevented, false, "fullscreen clicks keep native semantics");
  assert.deepEqual(log, []);
});

test("视频元素缺失（单 ref 未挂载 / Map 未命中）：preventDefault 已发生，不切播", () => {
  for (const missing of [null, undefined] as const) {
    const log: string[] = [];
    const event = fakeEvent({ clientY: 100, rect: BODY_RECT });
    const consumed = toggleVideoSlideClick({
      event,
      getVideo: () => missing,
      onUserPausedChange: (paused) => log.push(`record:${paused}`),
    });
    assert.equal(consumed, true, "click is still consumed (suppressed) without a video");
    assert.equal(event.defaultPrevented, true, "WebKit suppression already applied");
    assert.deepEqual(log, [], "no bookkeeping without a toggle");
  }
});

test("Map 形态视频元素获取（画廊按附件 id 查表）：命中即切播", () => {
  const log: string[] = [];
  const video = fakeVideo({ paused: false, log, tag: "v7" });
  const refs = new Map<number, HTMLVideoElement>([[7, video]]);
  const consumed = toggleVideoSlideClick({
    event: fakeEvent({ clientY: 100, rect: BODY_RECT }),
    getVideo: () => refs.get(7),
  });
  assert.equal(consumed, true);
  assert.deepEqual(log, ["pause:v7"]);
});

test("strip → 全屏豁免判定顺序：条区命中优先于全屏检查（两者都不切拨）", () => {
  /* 条区 + 全屏同时成立：返回不消费且 preventDefault 未触发——证明条区
     检查先返回（与既有宿主实现逐行同序）。 */
  const log: string[] = [];
  const video = fakeVideo({ paused: false, log, tag: "v8" });
  const event = fakeEvent({ clientY: BODY_RECT.bottom - 10, rect: BODY_RECT, fullscreen: true });
  const consumed = toggleVideoSlideClick({
    event,
    getVideo: () => video,
  });
  assert.equal(consumed, false);
  assert.equal(event.defaultPrevented, false);
  assert.deepEqual(log, []);
});
