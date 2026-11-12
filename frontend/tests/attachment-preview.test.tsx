import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";

import {
  AttachmentPreview,
  resolvePreviewVariant,
  type PreviewableAttachment,
} from "@/components/content/AttachmentPreview";
import { AudioPlayer, formatAudioTime } from "@/components/content/AudioPlayer";
import { asSceneHandle } from "@/components/content/ModelViewer";
import { clearPublicConfigCache } from "@/lib/public-config";
import { cleanup, fireEvent, render, waitFor } from "./runtime-test-helpers";

/* #722 预览点击化 + 音频播放器 + 3D 视图句柄接线回归。
 *
 * 覆盖票面验收的单测三项：
 * 1. 预览按钮状态机（resolvePreviewVariant 路由合同 + 组件默认不挂载
 *    查看器、点击后才挂载）；
 * 2. 播放器交互（时长显示、键盘 seek、播放/暂停接线）；
 * 3. 视图控制器与清理函数接口区分的回归测试（裸清理函数因函数自带
 *    Function.prototype.apply 结构兼容曾冒充视图控制器，运行时守卫拒绝）。
 *
 * C1 #920 首次护栏：document/model3d 附件预览装配经 SlideViewer 的组件级
 * 用例（此前 3D/文档装配零护栏）——入口点击挂载、签名 URL/文件名映射、
 * 宿主挂载即取的公开配置注入驱动预算降级。
 */

function renderWithIntl(ui: React.ReactElement) {
  return render(<IntlProvider locale="en" messages={enMessages as never}>{ui}</IntlProvider>);
}

function attachment(partial: Partial<PreviewableAttachment>): PreviewableAttachment {
  return { id: 1, ...partial };
}

test("resolvePreviewVariant: scan gate first, then viewer families by file_type", () => {
  assert.equal(
    resolvePreviewVariant(attachment({ file_type: "document", oss_url: "u", scan_status: "pending" })),
    "scan-card",
  );
  assert.equal(resolvePreviewVariant(attachment({ file_type: "document", oss_url: "u", scan_status: "clean" })), "entry");
  assert.equal(
    resolvePreviewVariant(attachment({ file_type: "document", oss_url: "u", scan_status: "not_required" })),
    "entry",
  );
  assert.equal(resolvePreviewVariant(attachment({ file_type: "document" })), "download-card", "missing URL falls back");
  assert.equal(
    resolvePreviewVariant(attachment({ file_type: "model3d", oss_url: "u", original_file_name: "part.stl" })),
    "entry",
  );
  assert.equal(
    resolvePreviewVariant(attachment({ file_type: "model3d", oss_url: "u", original_file_name: "part.mtl" })),
    "download-card",
    "V1: MTL never previews",
  );
  assert.equal(resolvePreviewVariant(attachment({ file_type: "audio", oss_url: "u" })), "entry", "#722 audio joins preview");
  assert.equal(resolvePreviewVariant(attachment({ file_type: "audio" })), "download-card");
  assert.equal(resolvePreviewVariant(attachment({ file_type: "other", oss_url: "u" })), "download-card");
});

test("entry state machine: default renders entry card only, click mounts the player", async () => {
  const { container, getByTestId, queryByTestId } = renderWithIntl(
    <AttachmentPreview
      attachment={attachment({
        id: 7,
        file_type: "audio",
        oss_url: "https://example.com/a.mp3",
        original_file_name: "demo.mp3",
        file_size: 1024,
        scan_status: "clean",
      })}
      contentId={1}
      allowCopy={false}
    />,
  );
  assert.ok(getByTestId("attachment-preview-entry"), "entry card is the default surface");
  assert.ok(getByTestId("attachment-preview-button"));
  assert.equal(container.querySelector("audio"), null, "no audio element mounted before click (no byte request)");

  fireEvent.click(getByTestId("attachment-preview-button"));
  await waitFor(() => {
    if (!(getByTestId("audio-player"))) throw new Error("player mounts after click");
  });
  assert.equal(container.querySelector("audio")?.getAttribute("src"), "https://example.com/a.mp3");
  assert.equal(queryByTestId("attachment-preview-entry"), null, "entry card is replaced by the viewer");
});

test("entry state machine: scan-blocked attachment never mounts a viewer", () => {
  const { container, getByTestId } = renderWithIntl(
    <AttachmentPreview
      attachment={attachment({
        file_type: "document",
        oss_url: "u",
        scan_status: "scanning",
      })}
      contentId={1}
      allowCopy={false}
    />,
  );
  assert.ok(getByTestId("attachment-scan-card"));
  assert.equal(container.querySelector("audio"), null);
});

test("formatAudioTime: m:ss with placeholder for unknown durations", () => {
  assert.equal(formatAudioTime(0), "0:00");
  assert.equal(formatAudioTime(9), "0:09");
  assert.equal(formatAudioTime(65), "1:05");
  assert.equal(formatAudioTime(125), "2:05");
  assert.equal(formatAudioTime(NaN), "--:--");
  assert.equal(formatAudioTime(Infinity), "--:--");
  assert.equal(formatAudioTime(-1), "--:--");
});

test("audio player: duration renders after loadedmetadata, keyboard seek updates position", async () => {
  // jsdom 的 HTMLMediaElement.play/pause 未实现——桩化并记录调用。
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
    const { getByTestId } = renderWithIntl(
      <AudioPlayer url="https://example.com/a.mp3" fileName="demo.mp3" contentId={1} attachmentId={7} allowCopy={false} />,
    );
    const audio = getByTestId("audio-player").querySelector("audio");
    assert.ok(audio);

    Object.defineProperty(audio, "duration", { value: 125, configurable: true });
    fireEvent(audio, new window.Event("loadedmetadata"));
    await waitFor(() => {
            if (!(getByTestId("audio-time").textContent?.includes("2:05"))) throw new Error("duration shows as 2:05");
    });

    fireEvent.click(getByTestId("audio-play-button"));
    assert.deepEqual(calls, ["play"], "play button drives native play()");

    const slider = getByTestId("audio-progress");
    assert.equal(slider.getAttribute("role"), "slider");
    assert.equal(slider.getAttribute("tabindex"), "0", "seek bar is keyboard focusable");
    fireEvent.keyDown(slider, { key: "ArrowRight" });
    assert.equal(Math.round(audio.currentTime), 5, "arrow right seeks +5s");
    assert.equal(slider.getAttribute("aria-valuenow"), "5");
    fireEvent.keyDown(slider, { key: "Home" });
    assert.equal(audio.currentTime, 0, "home rewinds to start");
  } finally {
    media.play = originalPlay;
    media.pause = originalPause;
    media.load = originalLoad;
  }
});

test("scene handle guard: a bare cleanup function must never pass as a view controller", () => {
  // 复现 #722 接线陷阱的机制：清理函数自带 Function.prototype.apply，
  // 对它调用 .apply(view) 实际执行的是卸载逻辑。
  let canvasRemoved = false;
  const cleanup = () => {
    canvasRemoved = true;
  };
  const mistyped = cleanup as unknown as { apply: (view: string) => void };
  mistyped.apply("top");
  assert.equal(canvasRemoved, true, "sanity: the historical trap really executes cleanup");

  assert.equal(asSceneHandle(cleanup), null, "runtime guard rejects bare functions");
  assert.equal(asSceneHandle({ apply: (view: string) => view }), null, "apply-only shape is not a handle");
  assert.equal(asSceneHandle(null), null);

  const handle = { setView: () => undefined, dispose: () => undefined };
  assert.equal(asSceneHandle(handle), handle, "proper {setView, dispose} objects pass");
});

/* ── C1 #920：document/model3d 装配首次护栏（经 SlideViewer 收口后） ──────── */

const CONFIG_URL = "/api/v1/config/public";

/** fetch 桩：config → 注入 JSON；csv → 文本；其余 URL → 404（error 态可断言）。 */
function installFetchStub(options?: { config?: unknown; csv?: string }) {
  const g = globalThis as unknown as { fetch?: typeof fetch };
  const original = g.fetch;
  const requested: string[] = [];
  g.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    requested.push(url);
    if (url.includes(CONFIG_URL)) {
      return new Response(JSON.stringify(options?.config ?? {}), { status: 200 });
    }
    if (options?.csv !== undefined && url.endsWith(".csv")) {
      return new Response(options.csv, { status: 200 });
    }
    return new Response("stub-not-found", { status: 404 });
  }) as typeof fetch;
  return {
    requested,
    restore: () => {
      g.fetch = original;
    },
  };
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

test("document entry（#920 护栏）：点击后经 SlideViewer 挂载 DocumentViewer（csv 网格 + 签名 URL/文件名映射）", async () => {
  clearPublicConfigCache();
  const fetchStub = installFetchStub({ config: { upload: { document_preview_max_mb: 10 } }, csv: "name,qty\nbracket,2\n" });
  try {
    const { getByTestId, queryByTestId } = renderWithIntl(
      <AttachmentPreview
        attachment={attachment({
          id: 41,
          file_type: "document",
          oss_url: "https://bucket.example/c1/report-41.csv",
          original_file_name: "report.csv",
          file_size: 2048,
          scan_status: "clean",
        })}
        contentId={9}
        allowCopy={false}
      />,
    );
    assert.ok(getByTestId("attachment-preview-entry"), "entry card first (no byte request)");
    fireEvent.click(getByTestId("attachment-preview-button"));
    await waitForTestid("document-viewer-grid", "DocumentViewer mounts after the entry click");
    assert.ok(document.body.textContent?.includes("bracket"), "csv bytes render through the signed url");
    assert.ok(
      fetchStub.requested.some((url) => url.includes("report-41.csv")),
      "viewer fetches the signed oss_url",
    );
    assert.equal(queryByTestId("attachment-preview-entry"), null, "entry card is replaced");
  } finally {
    fetchStub.restore();
    clearPublicConfigCache();
  }
});

test("document entry（#920 护栏）：宿主挂载即取的公开配置注入驱动预算降级（超限不请求字节）", async () => {
  clearPublicConfigCache();
  const fetchStub = installFetchStub({ config: { upload: { document_preview_max_mb: 1 } } });
  try {
    const { getByTestId } = renderWithIntl(
      <AttachmentPreview
        attachment={attachment({
          id: 42,
          file_type: "document",
          oss_url: "https://bucket.example/c1/big-42.csv",
          original_file_name: "big.csv",
          file_size: 5 * 1024 * 1024,
          scan_status: "clean",
        })}
        contentId={9}
        allowCopy={false}
      />,
    );
    /* 「挂载→点击」窗口内配置先落定（现实时序：配置请求毫秒级返回，
       用户点击在其后）——注入预算在点击时就位。 */
    await waitFor(() => {
      if (!(fetchStub.requested.some((url) => url.includes(CONFIG_URL)))) throw new Error("host fetched the public config on mount");
    });
    await waitFor(() => {
      if (!(getByTestId("attachment-preview-entry"))) throw new Error("waitFor: condition not met");
    });
    fireEvent.click(getByTestId("attachment-preview-button"));
    await waitForTestid("document-viewer-degraded", "host-fetched config budget degrades the over-limit document");
    assert.equal(
      fetchStub.requested.some((url) => url.includes("big-42.csv")),
      false,
      "budget check precedes the byte request",
    );
  } finally {
    fetchStub.restore();
    clearPublicConfigCache();
  }
});

test("model3d entry（#920 护栏）：点击后经 SlideViewer 挂载 ModelViewer（懒分块 + 配置注入预算）", async () => {
  clearPublicConfigCache();
  const fetchStub = installFetchStub({ config: { upload: { model3d_max_preview_mb: 1 } } });
  try {
    const { getByTestId } = renderWithIntl(
      <AttachmentPreview
        attachment={attachment({
          id: 43,
          file_type: "model3d",
          oss_url: "https://bucket.example/c1/part-43.stl",
          original_file_name: "part.stl",
          file_size: 50 * 1024 * 1024,
          scan_status: "not_required",
        })}
        contentId={9}
        allowCopy={false}
      />,
    );
    fireEvent.click(getByTestId("attachment-preview-button"));
    await waitForTestid("model-viewer-degraded", "ModelViewer mounts with the host-injected model3d budget");
    assert.equal(
      fetchStub.requested.some((url) => url.includes("part-43.stl")),
      false,
      "model budget check precedes the byte request",
    );
  } finally {
    fetchStub.restore();
    clearPublicConfigCache();
  }
});

test.afterEach(() => cleanup());
