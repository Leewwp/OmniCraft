import test from "node:test";
import assert from "node:assert/strict";
import React, { useRef, useState } from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api, ApiRequestError } from "@/lib/api";
import { OverlayVariantLayout } from "@/components/content/OverlayVariantLayout";
import type { MediaGalleryItem } from "@/components/content/MediaGallery";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * D2 #859 播放生命周期 + 翻页三件套契约（spec §9.3）：
 * - 活跃且可见的当前 video：autoPlay + muted + playsInline；play() 被拒不影响
 *   手动播放兜底；点击主体只切一次播放/暂停；视频绝不进 MediaViewer；
 * - 切位 / 层失活（压入下一内容层、上层弹窗挂起）：停止（pause）；恢复激活只有
 *   当前项自动续播；用户主动暂停后恢复激活不自动续播；
 * - 翻页三件套仅多项显示：底部圆点常显、箭头 + N/M 徽标 hover 与 focus-within
 *   显现、触屏（hover:none）恒显可发现、触控目标 44px、边界 clamp 不循环；
 * - 键盘 ←/→ 只作用于最顶活动媒体区：不劫持输入框/contentEditable、查看器打开
 *   时不切位、切位按钮点击不触发放大查看器；
 * - 移动 <960 overlay 行内画廊（MediaGallery xhs 行为）同契约：视频点击原地
 *   播放/暂停、不进查看器；宿主独立详情页默认契约不回归（无 prop 时旧行为）。
 * jsdom 无布局/无媒体解码：play/pause/paused 用原型桩模拟；hover/触屏显隐断言
 * 以 Tailwind class 契约标记承载，真值由真 Chromium 验收（screenshots/）。
 * 断言失败值绝不传 DOM 节点（node:test 序列化挂死），统一 Boolean()/字符串。
 * ──────────────────────────────────────────────────────────────────────────── */

/* ── 媒体播放桩：play/pause 记录调用；paused 读取桩内播放态 ───────────────── */

interface PlaybackStub {
  calls: Array<{ identity: HTMLVideoElement; action: "play" | "pause" }>;
  isPlaying: (el: HTMLVideoElement) => boolean;
  callsFor: (el: HTMLVideoElement) => Array<"play" | "pause">;
  restore: () => void;
}

function installPlaybackStub(): PlaybackStub {
  const proto = window.HTMLMediaElement.prototype as unknown as {
    play?: () => Promise<void> | undefined;
    pause?: () => void;
  };
  const originalPlay = proto.play;
  const originalPause = proto.pause;
  const pausedDescriptor = Object.getOwnPropertyDescriptor(
    window.HTMLMediaElement.prototype,
    "paused",
  );
  const calls: PlaybackStub["calls"] = [];
  const playing = new WeakSet<HTMLVideoElement>();
  (window.HTMLMediaElement.prototype as unknown as { play: () => Promise<void> }).play =
    function playStub(this: HTMLVideoElement) {
      playing.add(this);
      calls.push({ identity: this, action: "play" });
      return Promise.resolve();
    };
  (window.HTMLMediaElement.prototype as unknown as { pause: () => void }).pause =
    function pauseStub(this: HTMLVideoElement) {
      playing.delete(this);
      calls.push({ identity: this, action: "pause" });
    };
  Object.defineProperty(window.HTMLMediaElement.prototype, "paused", {
    configurable: true,
    get(this: HTMLVideoElement) {
      return !playing.has(this);
    },
  });
  return {
    calls,
    isPlaying: (el) => playing.has(el),
    callsFor: (el) => calls.filter((call) => call.identity === el).map((call) => call.action),
    restore: () => {
      (window.HTMLMediaElement.prototype as unknown as { play?: typeof originalPlay }).play =
        originalPlay;
      (window.HTMLMediaElement.prototype as unknown as { pause?: typeof originalPause }).pause =
        originalPause;
      if (pausedDescriptor) {
        Object.defineProperty(window.HTMLMediaElement.prototype, "paused", pausedDescriptor);
      }
    },
  };
}

/* ── dialog/showModal 桩（jsdom 原生 dialog API 不可用，同 D1 契约）──────────── */

function installDialogStubs() {
  const prototype = window.HTMLDialogElement?.prototype as unknown as
    | (HTMLDialogElement & { showModal?: unknown; close?: unknown })
    | undefined;
  if (!prototype) return;
  prototype.showModal = function showModalStub(this: HTMLDialogElement) {
    this.setAttribute("open", "");
  };
  prototype.close = function closeStub(this: HTMLDialogElement) {
    this.removeAttribute("open");
  };
}

/* ── 直渲染 OverlayVariantLayout 的夹具 ──────────────────────────────────── */

const VIDEO_A: MediaGalleryItem = {
  id: 631,
  url: "https://cdn.example/d2/video-a-1009.mp4",
  type: "video",
  width: 1280,
  height: 720,
  posterUrl: "https://cdn.example/d2/video-a-poster-1009.jpg",
};
const VIDEO_B: MediaGalleryItem = {
  id: 632,
  url: "https://cdn.example/d2/video-b-1009.mp4",
  type: "video",
  width: 720,
  height: 1280,
};
const IMAGE_A: MediaGalleryItem = {
  id: 633,
  url: "https://cdn.example/d2/image-a-1009.jpg",
  type: "image",
  width: 900,
  height: 1200,
};
const IMAGE_B: MediaGalleryItem = {
  id: 634,
  url: "https://cdn.example/d2/image-b-1009.jpg",
  type: "image",
  width: 1600,
  height: 900,
};

function LayoutHarness({
  media,
  initiallyActive = true,
}: {
  media: MediaGalleryItem[];
  initiallyActive?: boolean;
}) {
  const [active, setActive] = useState(initiallyActive);
  return (
    <IntlProvider locale="en" messages={enMessages}>
      <button type="button" onClick={() => setActive(false)}>
        Suspend layer
      </button>
      <button type="button" onClick={() => setActive(true)}>
        Resume layer
      </button>
      <input data-testid="d2-reply-input" aria-label="Reply input" />
      <OverlayVariantLayout media={media} active={active}>
        <p>right column</p>
      </OverlayVariantLayout>
    </IntlProvider>
  );
}

function renderLayout(media: MediaGalleryItem[], initiallyActive?: boolean) {
  installDom();
  installDialogStubs();
  return render(<LayoutHarness media={media} initiallyActive={initiallyActive} />);
}

function paneVideo(): HTMLVideoElement {
  const el = document.querySelector<HTMLVideoElement>('[data-slot="variant-media-pane"] video');
  assert.ok(el, "media pane must render a <video> element");
  return el;
}

async function waitForPlayCall(stub: PlaybackStub, el: HTMLVideoElement) {
  await waitFor(() => {
    assert.ok(
      stub.callsFor(el).includes("play"),
      "active video must attempt playback (autoPlay path)",
    );
  });
}

/* ── overlay 全链路（移动画廊路径 / 层挂起）夹具 ─────────────────────────── */

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;
const authStub = {
  user: null,
  isLoading: false,
  unreadCounts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 },
  capabilities: { can_interact: false, interaction_denial_reason: "unavailable" },
  login: async () => undefined,
  logout: async () => undefined,
  refresh: async () => true,
  refreshUser: async () => undefined,
};
Module._load = function loadWithNavigationStub(request: string, parent: unknown, isMain: boolean) {
  if (request === "next/navigation") {
    return {
      useParams: () => ({}),
      useRouter: () => ({ push: () => undefined }),
    };
  }
  if (request === "@/contexts/AuthContext") {
    return {
      useAuth: () => authStub,
      interactionDenialKey: () => "capabilities.deniedUnknown",
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

type OverlayModule = typeof import("@/components/content/ContentDetailOverlay");
let ContentDetailOverlay: OverlayModule["ContentDetailOverlay"];

test.before(async () => {
  const overlayModule = await import("@/components/content/ContentDetailOverlay");
  await import("@/components/content/ContentDetailOverlayLayer");
  ContentDetailOverlay = overlayModule.ContentDetailOverlay;
});

/* 视频内容（尺寸齐全，免探测路径）。 */
const OVERLAY_VIDEO = {
  content: {
    id: 63,
    title: "Overlay Video Work",
    zone: "original",
    content_type: "video",
    author: { id: 9, username: "Media Author" },
    status: "published",
    description: "Overlay video body",
    cover_image_url: "/seed/d2-video-poster.svg",
  },
  attachments: [
    { id: 531, content_item_id: 63, file_type: "video", oss_key: "/seed/d2-v01.mp4", width: 1280, height: 720, sort_order: 0 },
  ],
  tags: [],
};

/* fanwork 视频 + 系列（下一章可点 → 压入下一内容层，验证层失活停播）。 */
const FANWORK_VIDEO = {
  content: {
    id: 67,
    title: "Fanwork Video Work",
    zone: "fanwork",
    content_type: "video",
    author: { id: 9, username: "Media Author" },
    status: "published",
    description: "Fanwork video body",
    ip: { id: 5, name: "Indigo IP" },
  },
  attachments: [
    { id: 561, content_item_id: 67, file_type: "video", oss_key: "/seed/d2-fw-v01.mp4", width: 1280, height: 720, sort_order: 0 },
  ],
  tags: [],
  series_memberships: [
    {
      series_id: 78,
      series_title: "Motion Studies",
      current_index: 1,
      total: 3,
      next: { id: 68, title: "Episode Two" },
    },
  ],
};

const EPISODE_TWO = {
  content: {
    id: 68,
    title: "Episode Two",
    zone: "fanwork",
    content_type: "video",
    author: { id: 9, username: "Media Author" },
    status: "published",
    description: "Episode two body",
  },
  attachments: [
    { id: 562, content_item_id: 68, file_type: "video", oss_key: "/seed/d2-fw-v02.mp4", width: 720, height: 1280, sort_order: 0 },
  ],
  tags: [],
  series_memberships: [
    {
      series_id: 78,
      series_title: "Motion Studies",
      current_index: 2,
      total: 3,
      previous: { id: 67, title: "Fanwork Video Work" },
    },
  ],
};

const detailByPath = new Map<string, unknown>([
  ["/api/v1/contents/63", OVERLAY_VIDEO],
  ["/api/v1/contents/67", FANWORK_VIDEO],
  ["/api/v1/contents/68", EPISODE_TWO],
]);

const originalGet = api.get;
const originalPost = api.post;

function installApiMock() {
  api.get = async function mockedGet<T>(requestPath: string): Promise<T> {
    if (requestPath.includes("/related-fanworks")) {
      return { contents: [], total: 0 } as T;
    }
    if (requestPath.startsWith("/api/v1/social/comments")) {
      return { comments: [] } as T;
    }
    if (requestPath.startsWith("/api/v1/contents?")) {
      return { contents: [], total: 0 } as T;
    }
    const contentIdMatch = requestPath.match(/^\/api\/v1\/contents\/(\d+)$/);
    if (contentIdMatch) {
      const found = detailByPath.get(`/api/v1/contents/${contentIdMatch[1]}`);
      if (found) return found as T;
    }
    throw new ApiRequestError("NOT_FOUND", "raw secret backend error", 404);
  };
  api.post = async function mockedPost<T>(): Promise<T> {
    throw new ApiRequestError("UNAUTHORIZED", "no session", 401);
  };
}

function restoreApiMocks() {
  api.get = originalGet;
  api.post = originalPost;
}

function OverlayHarness({ entryId, zone }: { entryId: number; zone: "original" | "fanwork" }) {
  const [entry, setEntry] = useState<{ id: number; zone: "original" | "fanwork" } | null>(null);
  const triggerRef = useRef<HTMLElement | null>(null);
  return (
    <>
      <button
        type="button"
        onClick={(event) => {
          triggerRef.current = event.currentTarget;
          setEntry({ id: entryId, zone });
        }}
      >
        Open overlay
      </button>
      {entry && (
        <ContentDetailOverlay
          key={`${entry.zone}:${entry.id}`}
          contentId={entry.id}
          zone={entry.zone}
          source="zone-page"
          open
          onOpenChange={(open) => {
            if (!open) setEntry(null);
          }}
          returnFocusRef={triggerRef}
        />
      )}
    </>
  );
}

function renderOverlay(node: React.ReactNode, desktopViewport: boolean) {
  installDom();
  installDialogStubs();
  if (desktopViewport) {
    window.matchMedia = ((query: string) => ({
      matches: query === "(min-width: 960px)",
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    })) as typeof window.matchMedia;
  } else {
    delete (window as { matchMedia?: typeof window.matchMedia }).matchMedia;
  }
  return render(<IntlProvider locale="en" messages={enMessages}>{node}</IntlProvider>);
}

async function openOverlay(view: ReturnType<typeof render>, expectedTitle?: string) {
  const trigger = view.getByRole("button", { name: "Open overlay" });
  await act(async () => {
    fireEvent.click(trigger);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("dialog")));
  if (expectedTitle) {
    await waitFor(() => assert.ok(view.getByRole("heading", { level: 1, name: expectedTitle })));
  }
  return trigger;
}

test.afterEach(() => {
  cleanup();
  restoreApiMocks();
});

/* ══ 桌面统一版式：视频播放生命周期（直渲染 OverlayVariantLayout） ══ */

test("D2 active video auto-plays muted inline; body click toggles playback; video never opens the viewer", async () => {
  const stub = installPlaybackStub();
  try {
    const view = renderLayout([VIDEO_A]);
    const video = paneVideo();
    assert.equal(video.muted, true, "inline video must be muted for autoplay policy");
    /* jsdom 不实现 autoPlay IDL 属性（恒 undefined）：断言 attribute 反射。 */
    assert.equal(video.hasAttribute("autoplay"), true, "inline video must carry autoPlay");
    assert.equal(Boolean(video.playsInline), true, "inline video must carry playsInline");
    await waitForPlayCall(stub, video);

    /* 点击主体 = 播放/暂停切换，且不打开 MediaViewer（无第二个 dialog）。 */
    await act(async () => {
      fireEvent.click(video);
      await Promise.resolve();
    });
    assert.equal(stub.callsFor(video).at(-1), "pause", "body click pauses the playing video");
    assert.ok(!document.querySelector("dialog[open]"), "video click must not open the viewer");

    await act(async () => {
      fireEvent.click(video);
      await Promise.resolve();
    });
    assert.equal(stub.callsFor(video).at(-1), "play", "second body click resumes playback");
    assert.ok(!document.querySelector("dialog[open]"), "still no viewer after resume click");

    /* N1 契约：主体点击必须 preventDefault——WebKit 显示 controls 时对主体
       click 有默认 togglePlayState，不抑制会与 JS 切播双触发互相抵消。 */
    const probe = new window.MouseEvent("click", { bubbles: true, cancelable: true });
    await act(async () => {
      video.dispatchEvent(probe);
      await Promise.resolve();
    });
    assert.equal(probe.defaultPrevented, true, "body click must preventDefault (WebKit native toggle guard)");
    assert.equal(stub.callsFor(video).at(-1), "pause", "probe click still toggles exactly once");
  } finally {
    stub.restore();
  }
});

test("D2 switching slides pauses the previous video; paging controls clamp and never open the viewer", async () => {
  const stub = installPlaybackStub();
  try {
    const view = renderLayout([VIDEO_A, VIDEO_B]);
    const videoA = paneVideo();
    await waitForPlayCall(stub, videoA);

    const next = view.getByRole("button", { name: "Next media" }) as HTMLButtonElement;
    const previous = view.getByRole("button", { name: "Previous media" }) as HTMLButtonElement;
    assert.equal(previous.disabled, true, "left arrow clamps at the first item");
    assert.equal(next.disabled, false, "right arrow enabled with a next item");

    await act(async () => {
      fireEvent.click(next);
      await Promise.resolve();
    });
    assert.ok(stub.callsFor(videoA).includes("pause"), "previous slide's video must stop");
    /* React 对同位 <video> 只 patch src（不重挂载）：pane 里是同一个元素，
       src 已切到 B 即为「slide B renders」。 */
    const videoB = paneVideo();
    assert.equal(videoB.getAttribute("src"), VIDEO_B.url, "slide B renders");
    await waitForPlayCall(stub, videoB);

    assert.equal(next.disabled, true, "right arrow clamps at the last item");
    assert.ok(!document.querySelector("dialog[open]"), "paging controls never open the viewer");
    const badge = document.querySelector('[data-slot="media-count-badge"]');
    assert.ok(badge?.textContent?.includes("2 / 2"), `badge shows clamp position, got: ${badge?.textContent ?? "none"}`);
  } finally {
    stub.restore();
  }
});

test("D2 layer suspension pauses media; only the current item auto-resumes; user pause survives resume", async () => {
  const stub = installPlaybackStub();
  try {
    const view = renderLayout([VIDEO_A]);
    const video = paneVideo();
    await waitForPlayCall(stub, video);

    /* 层失活（压入下一内容层 / 上层弹窗）：停止。 */
    await act(async () => {
      fireEvent.click(view.getByRole("button", { name: "Suspend layer" }));
      await Promise.resolve();
    });
    assert.equal(stub.callsFor(video).at(-1), "pause", "suspension must pause the active video");

    /* 退层回来：当前项自动恢复。 */
    await act(async () => {
      fireEvent.click(view.getByRole("button", { name: "Resume layer" }));
      await Promise.resolve();
    });
    assert.equal(stub.callsFor(video).at(-1), "play", "re-activation resumes the current item");

    /* 用户主动暂停后失活→恢复：不自动续播（用户意图优先）。 */
    await act(async () => {
      fireEvent.click(video);
      await Promise.resolve();
    });
    assert.equal(stub.callsFor(video).at(-1), "pause", "user pauses via body click");
    await act(async () => {
      fireEvent.click(view.getByRole("button", { name: "Suspend layer" }));
      await Promise.resolve();
    });
    await act(async () => {
      fireEvent.click(view.getByRole("button", { name: "Resume layer" }));
      await Promise.resolve();
    });
    const afterResume = stub.callsFor(video).slice(-1);
    assert.deepEqual(afterResume, ["pause"], "user-intended pause must not be overridden on resume");
  } finally {
    stub.restore();
  }
});

/* ══ 桌面统一版式：翻页三件套 + 键盘契约 ══ */

test("D2 pagination trio renders only for multiple items with hover/focus-within/touch visibility contract", async () => {
  const view = renderLayout([IMAGE_A, IMAGE_B]);

  const badge = document.querySelector<HTMLElement>('[data-slot="media-count-badge"]');
  assert.ok(badge, "multi-item set renders the N/M badge");
  assert.ok(badge?.textContent?.includes("1 / 2"), `badge reads 1 / 2, got: ${badge?.textContent ?? "none"}`);
  assert.ok(document.querySelector('[data-slot="media-paging-dots"]'), "bottom dots render for multi-item sets");

  const next = view.getByRole("button", { name: "Next media" });
  const arrowClasses = `${next.className} `;
  assert.ok(arrowClasses.includes("opacity-0"), "arrows default to hidden (dots-only default)");
  assert.ok(
    arrowClasses.includes("group-hover/media:opacity-100"),
    "arrows reveal on media pane hover",
  );
  assert.ok(
    arrowClasses.includes("group-focus-within/media:opacity-100"),
    "arrows reveal on focus-within",
  );
  assert.ok(
    arrowClasses.includes("[@media(hover:none)]:opacity-100"),
    "touch devices keep a discoverable always-on entry",
  );
  assert.ok(
    arrowClasses.includes("min-h-11") && arrowClasses.includes("min-w-11"),
    "arrows meet the 44px touch target",
  );
  const badgeClasses = `${badge?.className ?? ""} `;
  assert.ok(badgeClasses.includes("group-hover/media:opacity-100"), "badge reveals with the trio on hover");

  /* 单项内容：无三件套（应缺席的查询用 queryByRole，getByRole 缺席即抛）。 */
  cleanup();
  const single = renderLayout([IMAGE_A]);
  assert.ok(!single.queryByRole("button", { name: "Next media" }), "single item has no arrows");
  assert.ok(!document.querySelector('[data-slot="media-paging-dots"]'), "single item has no dots");
  assert.ok(!document.querySelector('[data-slot="media-count-badge"]'), "single item has no badge");
});

test("D2 keyboard arrows page only the top active media area: inputs and the viewer are never hijacked", async () => {
  const view = renderLayout([IMAGE_A, IMAGE_B]);

  const badgeText = () => document.querySelector('[data-slot="media-count-badge"]')?.textContent ?? "";
  assert.ok(badgeText().includes("1 / 2"));

  /* 媒体区键盘 ←/→ 切位（clamp）。 */
  await act(async () => {
    fireEvent.keyDown(window, { key: "ArrowRight" });
    await Promise.resolve();
  });
  assert.ok(badgeText().includes("2 / 2"), `ArrowRight pages forward, got: ${badgeText()}`);
  await act(async () => {
    fireEvent.keyDown(window, { key: "ArrowRight" });
    await Promise.resolve();
  });
  assert.ok(badgeText().includes("2 / 2"), "ArrowRight clamps at the last item");
  await act(async () => {
    fireEvent.keyDown(window, { key: "ArrowLeft" });
    await Promise.resolve();
  });
  assert.ok(badgeText().includes("1 / 2"), "ArrowLeft pages back");

  /* 输入框聚焦时按箭头：不切位。 */
  const input = document.querySelector<HTMLInputElement>('[data-testid="d2-reply-input"]');
  assert.ok(input, "harness input exists");
  await act(async () => {
    fireEvent.keyDown(input, { key: "ArrowRight" });
    await Promise.resolve();
  });
  assert.ok(badgeText().includes("1 / 2"), "arrow keys inside an input must not page the media");

  /* contentEditable（评论编辑器）聚焦时按箭头：不切位。 */
  const editor = document.createElement("div");
  editor.setAttribute("contenteditable", "true");
  editor.setAttribute("data-testid", "d2-comment-editor");
  document.body.appendChild(editor);
  await act(async () => {
    fireEvent.keyDown(editor, { key: "ArrowRight" });
    await Promise.resolve();
  });
  assert.ok(badgeText().includes("1 / 2"), "arrow keys inside contentEditable must not page the media");
  editor.remove();

  /* 查看器打开时按箭头：不切位（不劫持查看器）。 */
  const cover = document.querySelector<HTMLElement>('[data-slot="detail-cover"]');
  assert.ok(cover, "cover anchor exists");
  await act(async () => {
    fireEvent.click(cover);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(document.querySelector("dialog[open]"), "viewer dialog opens for images"));
  await act(async () => {
    fireEvent.keyDown(window, { key: "ArrowRight" });
    await Promise.resolve();
  });
  assert.ok(badgeText().includes("1 / 2"), "arrow keys with the viewer open must not page the pane");
});

test("D2 mixed media set: the viewer only ever receives image slides (video never enters the viewer)", async () => {
  const view = renderLayout([IMAGE_A, VIDEO_B]);

  /* 视频为当前项时点击：无查看器。 */
  const stub = installPlaybackStub();
  try {
    await act(async () => {
      fireEvent.click(view.getByRole("button", { name: "Next media" }));
      await Promise.resolve();
    });
    const video = paneVideo();
    await waitForPlayCall(stub, video);
    await act(async () => {
      fireEvent.click(video);
      await Promise.resolve();
    });
    assert.ok(!document.querySelector("dialog[open]"), "video body click must not open the viewer");

    /* 图片项点击打开查看器；查看器内按 ←/→ 不落到面板；关闭后面板翻页仍正常。 */
    await act(async () => {
      fireEvent.click(view.getByRole("button", { name: "Previous media" }));
      await Promise.resolve();
    });
    const cover = document.querySelector<HTMLElement>('[data-slot="detail-cover"]');
    assert.ok(cover);
    await act(async () => {
      fireEvent.click(cover);
      await Promise.resolve();
    });
    await waitFor(() => assert.ok(document.querySelector("dialog[open]"), "image click opens the viewer"));
    const close = await waitFor(() => {
      const el = document.querySelector<HTMLButtonElement>('dialog[open] button[aria-label="Close viewer"]');
      assert.ok(el, "viewer close button exists");
      return el;
    });
    await act(async () => {
      fireEvent.click(close);
      await Promise.resolve();
    });
    await waitFor(() => assert.ok(!document.querySelector("dialog[open]"), "viewer closes"));
    const badge = document.querySelector('[data-slot="media-count-badge"]');
    assert.ok(badge?.textContent?.includes("1 / 2"), `pane position intact after viewer round-trip, got: ${badge?.textContent ?? "none"}`);
  } finally {
    stub.restore();
  }
});

/* ══ overlay 全链路：层失活停播 + 移动画廊路径 ══ */

test("D2 pushing the next content layer suspends the video of the covered layer", async () => {
  installApiMock();
  const stub = installPlaybackStub();
  try {
    const view = renderOverlay(<OverlayHarness entryId={67} zone="fanwork" />, true);
    await openOverlay(view, "Fanwork Video Work");

    const video = await waitFor(() => {
      const el = document.querySelector<HTMLVideoElement>('[data-slot="variant-media-pane"] video');
      assert.ok(el, "unified pane renders the fanwork video");
      return el;
    });
    await waitForPlayCall(stub, video);

    /* #430 分帧：系列导航在 DeferredMount 内，等「Next chapter」按钮出现。 */
    const nextChapter = await waitFor(() => {
      const el = Array.from(document.querySelectorAll("button")).find((b) =>
        b.textContent?.includes("Next"),
      );
      assert.ok(el, "series next-chapter control renders");
      return el as HTMLButtonElement;
    });
    await act(async () => {
      fireEvent.click(nextChapter);
      await Promise.resolve();
    });
    await waitFor(() => {
      assert.ok(
        stub.callsFor(video).includes("pause"),
        "covered layer's video must stop when a new layer is pushed",
      );
    });
    await waitFor(() => assert.ok(view.getByRole("heading", { level: 1, name: "Episode Two" })));
    /* 覆盖层隐藏（display:none 树）下不该有第二个可见播放源。 */
    const videos = document.querySelectorAll("video");
    assert.ok(videos.length >= 1, "media tree intact");
  } finally {
    stub.restore();
  }
});

test("D2 mobile (<960) overlay gallery: video toggles in place and never opens the viewer", async () => {
  installApiMock();
  const stub = installPlaybackStub();
  try {
    const view = renderOverlay(<OverlayHarness entryId={63} zone="original" />, false);
    await openOverlay(view, "Overlay Video Work");

    const video = await waitFor(() => {
      const el = document.querySelector<HTMLVideoElement>("video");
      assert.ok(el, "mobile single column renders the inline gallery video");
      return el;
    });
    assert.equal(video.muted, true, "mobile overlay video is muted for autoplay policy");
    assert.equal(Boolean(video.playsInline), true, "mobile overlay video carries playsInline");
    await waitForPlayCall(stub, video);

    await act(async () => {
      fireEvent.click(video);
      await Promise.resolve();
    });
    assert.equal(stub.callsFor(video).at(-1), "pause", "mobile body click toggles to pause");
    assert.equal(
      document.querySelectorAll("dialog[open]").length,
      1,
      "video click must not stack the viewer above the overlay",
    );
    await act(async () => {
      fireEvent.click(video);
      await Promise.resolve();
    });
    assert.equal(stub.callsFor(video).at(-1), "play", "second mobile click resumes");
  } finally {
    stub.restore();
  }
});
