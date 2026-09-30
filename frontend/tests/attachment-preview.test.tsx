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
import { cleanup, fireEvent, render, waitFor } from "./runtime-test-helpers";

/* #722 预览点击化 + 音频播放器 + 3D 视图句柄接线回归。
 *
 * 覆盖票面验收的单测三项：
 * 1. 预览按钮状态机（resolvePreviewVariant 路由合同 + 组件默认不挂载
 *    查看器、点击后才挂载）；
 * 2. 播放器交互（时长显示、键盘 seek、播放/暂停接线）；
 * 3. 视图控制器与清理函数接口区分的回归测试（裸清理函数因函数自带
 *    Function.prototype.apply 结构兼容曾冒充视图控制器，运行时守卫拒绝）。
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
  await waitFor(() => assert.ok(getByTestId("audio-player"), "player mounts after click"));
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
      assert.ok(getByTestId("audio-time").textContent?.includes("2:05"), "duration shows as 2:05");
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

test.afterEach(() => cleanup());
