import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";

import { SlideViewer, asViewerSlide, type ViewerSlideItem } from "@/components/content/SlideViewer";
import type { MediaGalleryItem } from "@/components/content/MediaGallery";
import { AuthProvider } from "@/contexts/AuthContext";
import { ToastProvider } from "@/components/ui/Toast";
import { clearPublicConfigCache } from "@/lib/public-config";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * C1 #920 SlideViewer 装配级用例：三家族（model3d / document / audio）×
 * 关键 props 映射——配置字段→预算 props（经降级态可观）、文件名回退
 * （fileName ?? ""）、布尔化（allowCopy→下载入口）/归零（contentId ?? 0）
 * 约定、可选配置注入 vs 模块内 usePublicConfig 兜底、React.lazy+Suspense
 * 懒挂载语义。只断言接口外部行为，不断言懒加载写法本身。
 * jsdom：ModelViewer/DocumentViewer 的 oss_url fetch 桩为确定性结果
 * （404 → error 态 / csv 文本 → grid 态 / 不触发 → budget 降级态）。
 * ──────────────────────────────────────────────────────────────────────────── */

const CONFIG_URL = "/api/v1/config/public";

interface FetchLog {
  urls: string[];
}

/** fetch 桩：按 URL 分派（config → JSON；csv → 文本；其余 → 404），记录请求序。 */
function installFetchStub(options?: { config?: unknown; csv?: string }) {
  const log: FetchLog = { urls: [] };
  const g = globalThis as unknown as { fetch?: typeof fetch };
  const original = g.fetch;
  g.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    log.urls.push(url);
    if (url.includes(CONFIG_URL)) {
      if (options?.config) return new Response(JSON.stringify(options.config), { status: 200 });
      return new Response("{}", { status: 200 });
    }
    if (options?.csv !== undefined && url.endsWith(".csv")) {
      return new Response(options.csv, { status: 200 });
    }
    return new Response("stub-not-found", { status: 404 });
  }) as typeof fetch;
  return {
    log,
    restore: () => {
      g.fetch = original;
    },
  };
}

function slideItem(id: number, type: "model3d" | "document" | "audio", overrides: Partial<ViewerSlideItem> = {}): ViewerSlideItem {
  return {
    id,
    url: `https://bucket.example/c1/${type}-${id}.bin`,
    type,
    width: 300,
    height: 400,
    fileName: `${type}-${id}.bin`,
    fileSize: 2048,
    ...overrides,
  };
}

function renderViewer(ui: React.ReactElement) {
  return render(
    <IntlProvider locale="en" messages={enMessages as never}>
      {ui}
    </IntlProvider>,
  );
}

/** allowCopy=true 断言需要下载按钮的 Toast/Auth 上下文。 */
function renderViewerWithProviders(ui: React.ReactElement) {
  return render(
    <IntlProvider locale="en" messages={enMessages as never}>
      <ToastProvider>
        <AuthProvider>{ui}</AuthProvider>
      </ToastProvider>
    </IntlProvider>,
  );
}

async function waitForTestid(testid: string, message?: string) {
  const deadline = Date.now() + 4000;
  for (;;) {
    const found = document.querySelector(`[data-testid="${testid}"]`);
    if (found) return found;
    if (Date.now() > deadline) assert.fail(message ?? `expected [data-testid="${testid}"] to mount`);
    await new Promise((resolve) => window.setTimeout(resolve, 20));
  }
}

test.beforeEach(() => {
  const dom = installDom();
  /* AuthProvider（allowCopy 用例）经 lib/auth 读写 token 存储。 */
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    writable: true,
    value: dom.window.localStorage,
  });
  /* fetchPublicConfig 模块级 TTL 缓存跨用例共享：逐用例清空，钉死兜底链路。 */
  clearPublicConfigCache();
});
test.afterEach(() => cleanup());

/* ── 类型收窄守卫 ─────────────────────────────────────────────────────────── */

test("asViewerSlide：三查看器族收窄，其余类型（含图片/视频/乐谱/pdf）返回 null", () => {
  const cases: Array<[MediaGalleryItem, boolean]> = [
    [{ id: 1, url: "u", type: "model3d" }, true],
    [{ id: 2, url: "u", type: "document" }, true],
    [{ id: 3, url: "u", type: "audio" }, true],
    [{ id: 4, url: "u", type: "image" }, false],
    [{ id: 5, url: "u", type: "video" }, false],
    [{ id: 6, url: "u", type: "text_pdf" }, false],
    [{ id: 7, url: "u", type: "sheet_music_xml" }, false],
  ];
  for (const [item, expected] of cases) {
    const narrowed = asViewerSlide(item);
    assert.equal(narrowed !== null, expected, `type=${item.type} narrows as ${expected}`);
    if (narrowed) assert.equal(narrowed.type, item.type, "identity narrowing keeps the item");
  }
});

/* ── 懒挂载语义（Suspense fallback=null：未就绪期间无查看器 DOM） ────────── */

test("懒挂载：model3d 首帧渲染 Suspense 空位，chunk 就绪后查看器进场（fallback 不留痕）", async () => {
  /* 本用例须保持在本文件首个 model3d 挂载用例的位置：chunk 一旦加载进模块
     缓存，lazy 首帧同步解析，「首帧无查看器 DOM」不再可观测（声明序 = 执行序）。 */
  const fetchStub = installFetchStub();
  try {
    const item = slideItem(10, "model3d", {
      url: "https://bucket.example/c1/part-10.stl",
      fileName: "part.stl",
    });
    renderViewer(<SlideViewer item={item} />);
    /* chunk 加载是异步的：同步首帧不应有任何查看器 testid（fallback=null）。 */
    assert.equal(document.querySelector('[data-testid^="model-viewer-"]'), null, "no viewer DOM before the lazy chunk lands");
    await waitForTestid("model-viewer-error", "fetch stub 404 lands on the error card after mounting");
  } finally {
    fetchStub.restore();
  }
});

/* ── document 家族 ────────────────────────────────────────────────────────── */

test("document 装配：url 透传（csv 字节请求 = slide url）、网格渲染、注入配置预算不拦截小文件", async () => {
  const fetchStub = installFetchStub({
    config: { upload: { document_preview_max_mb: 1 } },
    csv: "name,qty\nbracket,2\n",
  });
  try {
    const item = slideItem(31, "document", {
      url: "https://bucket.example/c1/report-31.csv",
      fileName: "report.csv",
      fileSize: 2048,
    });
    renderViewer(<SlideViewer item={item} config={{ upload: { document_preview_max_mb: 1 } } as never} />);
    await waitForTestid("document-viewer-grid", "csv document renders the grid");
    assert.ok(document.body.textContent?.includes("bracket"), "csv rows render (bytes flowed through)");
    assert.ok(
      fetchStub.log.urls.some((url) => url.includes("report-31.csv")),
      "viewer fetches the slide url",
    );
  } finally {
    fetchStub.restore();
  }
});

test("document 预算映射：注入配置 document_preview_max_mb=1 → 超限文件同步降级（不发起正文请求）", async () => {
  const fetchStub = installFetchStub();
  try {
    const item = slideItem(32, "document", {
      url: "https://bucket.example/c1/big-32.csv",
      fileName: "big.csv",
      fileSize: 5 * 1024 * 1024,
    });
    renderViewer(<SlideViewer item={item} config={{ upload: { document_preview_max_mb: 1 } } as never} />);
    await waitForTestid("document-viewer-degraded", "over-budget document degrades via the injected config budget");
    assert.equal(
      fetchStub.log.urls.some((url) => url.includes("big-32.csv")),
      false,
      "budget check runs before any byte request",
    );
  } finally {
    fetchStub.restore();
  }
});

test("document fileName 回退：fileName 缺省映射为空串（不崩溃，ext 判定走默认路径）", async () => {
  const fetchStub = installFetchStub();
  try {
    const item = slideItem(33, "document", { url: "https://bucket.example/c1/doc-33", fileName: undefined });
    renderViewer(<SlideViewer item={item} />);
    await waitForTestid("document-viewer-error", "extension-less fetch stub 404 lands on the error card");
  } finally {
    fetchStub.restore();
  }
});

/* ── model3d 家族 ─────────────────────────────────────────────────────────── */

test("model3d 预算映射：注入配置 model3d_max_preview_mb=1 → 超限降级；懒挂载后呈现", async () => {
  const fetchStub = installFetchStub();
  try {
    const item = slideItem(7, "model3d", {
      url: "https://bucket.example/c1/part-7.stl",
      fileName: "part.stl",
      fileSize: 50 * 1024 * 1024,
    });
    renderViewer(<SlideViewer item={item} config={{ upload: { model3d_max_preview_mb: 1 } } as never} />);
    await waitForTestid("model-viewer-degraded", "over-budget model degrades via the injected config budget");
    assert.equal(
      fetchStub.log.urls.some((url) => url.includes("part-7.stl")),
      false,
      "model budget check precedes the byte request",
    );
  } finally {
    fetchStub.restore();
  }
});

test("model3d 配置兜底：未注入 config 时模块内 usePublicConfig 生效（config 预算晚到仍触发降级）", async () => {
  /* fileSize=10MB 低于组件默认预算 15MB：首帧按默认预算发起字节请求；
     兜底 config 到达（预算 1MB）后 effect 重估 → 降级。两步都可见 = 兜底链路活。 */
  const fetchStub = installFetchStub({ config: { upload: { model3d_max_preview_mb: 1 } } });
  try {
    const item = slideItem(8, "model3d", {
      url: "https://bucket.example/c1/part-8.stl",
      fileName: "part.stl",
      fileSize: 10 * 1024 * 1024,
    });
    renderViewer(<SlideViewer item={item} />);
    await waitForTestid("model-viewer-degraded", "fallback config budget eventually degrades the model");
    assert.ok(
      fetchStub.log.urls.some((url) => url.includes("part-8.stl")),
      "default-budget pass requested the bytes first (fallback config arrived later)",
    );
    assert.ok(fetchStub.log.urls.some((url) => url.includes(CONFIG_URL)), "module fetched the public config itself");
  } finally {
    fetchStub.restore();
  }
});

test("model3d allowCopy 布尔化：缺省（undefined）无下载入口，true 时渲染下载按钮", async () => {
  const fetchStub = installFetchStub({ config: { upload: { model3d_max_preview_mb: 1 } } });
  try {
    const item = slideItem(9, "model3d", {
      url: "https://bucket.example/c1/part-9.stl",
      fileName: "part.stl",
      fileSize: 50 * 1024 * 1024,
    });
    const view = renderViewerWithProviders(
      <SlideViewer item={item} config={{ upload: { model3d_max_preview_mb: 1 } } as never} />,
    );
    await waitForTestid("model-viewer-degraded");
    assert.equal(
      document.querySelector('[data-testid="model-viewer-degraded"] button'),
      null,
      "allowCopy omitted (undefined → false) hides the download entry",
    );
    await act(async () => {
      view.rerender(
        <IntlProvider locale="en" messages={enMessages as never}>
          <ToastProvider>
            <AuthProvider>
              <SlideViewer item={item} allowCopy={true} config={{ upload: { model3d_max_preview_mb: 1 } } as never} />
            </AuthProvider>
          </ToastProvider>
        </IntlProvider>,
      );
    });
    const download = document.querySelector('[data-testid="model-viewer-degraded"] button');
    assert.ok(download, "allowCopy=true renders the download entry on the degraded card");
  } finally {
    fetchStub.restore();
  }
});

/* ── audio 家族 ───────────────────────────────────────────────────────────── */

test("audio 装配：url/fileName 透传、fileName 缺省回退空串、挂起信号透传（suspended → pause）", async () => {
  /* HTMLMediaElement play/pause 桩（jsdom 未实现）。 */
  const calls: string[] = [];
  const media = window.HTMLMediaElement.prototype as unknown as Record<string, unknown>;
  const originalPlay = media.play;
  const originalPause = media.pause;
  const originalLoad = media.load;
  media.play = () => {
    calls.push("play");
    return Promise.resolve();
  };
  media.pause = () => {
    calls.push("pause");
  };
  media.load = () => {
    calls.push("load");
  };
  try {
    const item = slideItem(3, "audio", {
      url: "https://bucket.example/c1/track-3.mp3",
      fileName: "track.mp3",
      fileSize: 123,
    });
    const view = renderViewer(<SlideViewer item={item} contentId={0} allowCopy={false} />);
    await waitFor(() => {
      if (!(document.querySelector('[data-testid="audio-player"]'))) throw new Error("audio family mounts the player");
    });
    const player = document.querySelector('[data-testid="audio-player"]');
    assert.ok(player);
    const audio = player.querySelector("audio");
    assert.ok(audio, "native audio element mounts");
    assert.equal(audio?.getAttribute("src"), "https://bucket.example/c1/track-3.mp3", "slide url maps to the audio src");
    assert.ok(player.textContent?.includes("track.mp3"), "fileName maps to the player title row");

    /* 用户起播后挂起：suspended 透传到 AudioPlayer（pause 调用）。 */
    const playButton = player.querySelector<HTMLButtonElement>('[data-testid="audio-play-button"]');
    assert.ok(playButton);
    await act(async () => {
      fireEvent.click(playButton);
    });
    const plays = calls.filter((op) => op === "play").length;
    assert.ok(plays >= 1, "user-initiated playback starts");
    await act(async () => {
      view.rerender(
        <IntlProvider locale="en" messages={enMessages as never}>
          <SlideViewer item={item} contentId={0} allowCopy={false} suspended={true} />
        </IntlProvider>,
      );
    });
    assert.ok(calls.includes("pause"), "suspended=true passthrough pauses the player");

    /* fileName 缺省回退空串：标题行渲染空内容不崩溃。 */
    await act(async () => {
      view.rerender(
        <IntlProvider locale="en" messages={enMessages as never}>
          <SlideViewer
            item={slideItem(4, "audio", { url: "https://bucket.example/c1/track-4.mp3", fileName: undefined })}
            contentId={0}
            allowCopy={false}
          />
        </IntlProvider>,
      );
    });
    const fallbackPlayer = document.querySelector('[data-testid="audio-player"]');
    assert.ok(fallbackPlayer, "player remounts with the empty-string fileName fallback");
    assert.equal(fallbackPlayer.textContent?.includes("track-3.mp3"), false, "no stale fileName text");
  } finally {
    media.play = originalPlay;
    media.pause = originalPause;
    media.load = originalLoad;
  }
});
