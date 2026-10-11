import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import { OverlayVariantLayout, type OverlayVariantLayoutProps } from "@/components/content/OverlayVariantLayout";
import type { MediaGalleryItem } from "@/components/content/MediaGallery";
import { testMessages } from "./runtime-test-helpers";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * D5 #862 统一版式逐类型轮播位契约（spec §3.2/§9.6）：
 * - 逐类型位形态：audio = 占位图为底 + 底部 AudioPlayer 条；model3d/document
 *   = 对应查看器；pdf（text/sheet_music 族）= 原生 embed；sheet_music_xml =
 *   OSMD 容器、midi = 回放条。
 * - 当前位才挂载重型查看器：切位卸载旧位（audio 停播释放）；挂起（非顶层）
 *   时 audio 暂停。
 * - 新类型首项挂载即 settle（不等 2s 入场保险）。
 * - >9 项隐藏圆点（N/M 徽标 + 箭头保留）；混排链 slide 加类型小角标。
 * - MediaViewer 只收 image slides；非图片主体点击不开查看器。
 * - 查看器位提供「刷新预览」有限重试（onMediaRefresh），按附件 id 保持当前位。
 * 断言失败值绝不传 DOM 节点（node:test 序列化挂死），统一 Boolean()/字符串。
 * ──────────────────────────────────────────────────────────────────────────── */

const AREA = { width: 1000, height: 800 };

function installResizeObserverStub() {
  class ResizeObserverStub {
    cb: ResizeObserverCallback;
    constructor(cb: ResizeObserverCallback) {
      this.cb = cb;
    }
    observe = (target: Element) => {
      this.cb(
        [
          {
            target,
            contentRect: {
              ...AREA,
              x: 0,
              y: 0,
              top: 0,
              left: 0,
              right: AREA.width,
              bottom: AREA.height,
              toJSON() {},
            },
          } as unknown as ResizeObserverEntry,
        ],
        this as unknown as ResizeObserver,
      );
    };
    unobserve() {}
    disconnect() {}
  }
  (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = ResizeObserverStub;
}

function installDialogStubs() {
  const prototype = window.HTMLDialogElement?.prototype as unknown as HTMLDialogElement | undefined;
  if (!prototype) return;
  prototype.showModal = function showModalStub(this: HTMLDialogElement) {
    this.setAttribute("open", "");
  };
  prototype.close = function closeStub(this: HTMLDialogElement) {
    this.removeAttribute("open");
  };
}

interface MediaCall {
  op: "play" | "pause";
  tag: string;
}

/** HTMLMediaElement play/pause 桩：记录调用序并同步翻转 paused（audio 停播断言用）。 */
function installMediaElementStubs() {
  const calls: MediaCall[] = [];
  const proto = window.HTMLMediaElement.prototype as unknown as {
    play: () => Promise<void>;
    pause: () => void;
  };
  const originalPlay = proto.play;
  const originalPause = proto.pause;
  const setPaused = (el: HTMLMediaElement, value: boolean) => {
    Object.defineProperty(el, "paused", { value, configurable: true, writable: true });
  };
  proto.play = function playStub(this: HTMLMediaElement) {
    calls.push({ op: "play", tag: this.getAttribute("src") ?? "unknown" });
    setPaused(this, false);
    return Promise.resolve();
  };
  proto.pause = function pauseStub(this: HTMLMediaElement) {
    calls.push({ op: "pause", tag: this.getAttribute("src") ?? "unknown" });
    setPaused(this, true);
  };
  return {
    calls,
    restore: () => {
      proto.play = originalPlay;
      proto.pause = originalPause;
    },
  };
}

/** MIDIPlayer 需要 AudioContext（jsdom 无实现）：最小桩只满足构造/close。 */
function installAudioContextStub() {
  class AudioContextStub {
    state = "running";
    resume() {}
    close() {
      return Promise.resolve();
    }
  }
  /* 组件代码经 tsx 在 Node 全局解析裸标识符：必须挂 globalThis 而非 window。 */
  const g = globalThis as unknown as { AudioContext?: unknown };
  const original = g.AudioContext;
  g.AudioContext = AudioContextStub;
  return {
    restore: () => {
      if (original === undefined) delete g.AudioContext;
      else g.AudioContext = original;
    },
  };
}

/** 规格审查 C-1/C-4：MIDIPlayer 全链回放桩——AudioContext（记录 suspend/
 *  resume 并翻转 state）、soundfont XHR（MIDI.js 文本）、slide URL 的 MIDI
 *  字节 fetch、sample-player 判定所需的 AudioBuffer 全局。init 走真实
 *  midi-player-js/soundfont-player 包，挂起/续播语义断言在组件契约层。 */
function installMidiPlaybackStubs(options?: { trackTicks?: number }) {
  const records = { suspend: 0, resume: 0 };
  class AudioContextStub {
    state = "running";
    destination = {};
    suspend() {
      records.suspend += 1;
      this.state = "suspended";
      return Promise.resolve();
    }
    resume() {
      records.resume += 1;
      this.state = "running";
      return Promise.resolve();
    }
    close() {
      return Promise.resolve();
    }
    createGain() {
      return { gain: { value: 1 }, connect() {}, disconnect() {} };
    }
    decodeAudioData(_buffer: ArrayBuffer, success: (decoded: unknown) => void) {
      success({});
    }
  }
  const g = globalThis as unknown as {
    AudioContext?: unknown;
    AudioBuffer?: unknown;
    XMLHttpRequest?: unknown;
    fetch?: typeof fetch;
  };
  const originals = {
    audioContext: g.AudioContext,
    audioBuffer: g.AudioBuffer,
    xhr: g.XMLHttpRequest,
    fetch: g.fetch,
  };
  g.AudioContext = AudioContextStub;
  g.AudioBuffer = class AudioBufferStub {};
  /* soundfont-player → audio-loader 经 XHR 拉 MIDI.js 文本（对象尾逗号是
     midiJsToJson 的切片解析契约）。 */
  class XHRStub {
    onload?: () => void;
    open() {}
    send() {
      const text = 'MIDI.Soundfont.acoustic_grand_piano = {"A0":"data:audio/mp3;base64,AAAA",};';
      Object.defineProperty(this, "response", { value: text, configurable: true });
      Object.defineProperty(this, "responseText", { value: text, configurable: true });
      Object.defineProperty(this, "status", { value: 200, configurable: true });
      this.onload?.();
    }
  }
  g.XMLHttpRequest = XHRStub;
  /* slide oss_url → 最小 type-0 MIDI：头 + 单 track（delta(trackTicks) + 仅
     end-of-track）。默认 trackTicks=0 与既有空 track 字节完全一致；进度基准
     用例传更长轨道（如 4800 ticks = 480 ticks/quarter × 4 = 5s @120bpm），
     避免播放中 EOF 触发 player.stop() 清零位置。 */
  const trackTicks = options?.trackTicks ?? 0;
  const vlq: number[] = [];
  let vlqValue = trackTicks;
  do {
    vlq.unshift(vlqValue & 0x7f);
    vlqValue >>>= 7;
  } while (vlqValue > 0);
  for (let i = 0; i < vlq.length - 1; i += 1) vlq[i] |= 0x80;
  const trackData = [...vlq, 0xff, 0x2f, 0x00];
  const midiBytes = new Uint8Array([
    0x4d, 0x54, 0x68, 0x64, 0, 0, 0, 6, 0, 0, 0, 1, 0x01, 0xe0,
    0x4d, 0x54, 0x72, 0x6b, 0, 0, 0, trackData.length, ...trackData,
  ]);
  const originalFetch = g.fetch;
  const passthroughFetch: typeof fetch = (input, init) =>
    (originalFetch ?? globalThis.fetch.bind(globalThis))(input, init);
  g.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    if (String(input).includes("sheet_music_midi")) {
      return new Response(midiBytes.slice().buffer, { status: 200 });
    }
    return passthroughFetch(input, init);
  }) as typeof fetch;
  return {
    records,
    restore: () => {
      g.AudioContext = originals.audioContext;
      if (originals.audioBuffer === undefined) delete g.AudioBuffer;
      else g.AudioBuffer = originals.audioBuffer;
      if (originals.xhr === undefined) delete g.XMLHttpRequest;
      else g.XMLHttpRequest = originals.xhr;
      g.fetch = originals.fetch;
    },
  };
}

/** midi slide 回放按钮（loading 结束才出现；aria-label 承载播放态）。 */
async function midiPlayButton(): Promise<HTMLButtonElement> {
  return waitFor(
    () => {
      const btn = document.querySelector<HTMLButtonElement>(
        '[data-slot="slide-sheet-music"] button[aria-label]',
      );
      assert.ok(btn, "midi playback bar play button mounts after init");
      return btn;
    },
    { timeout: 4000 },
  );
}

function imageItem(id: number): MediaGalleryItem {
  return { id, url: `/seed-media/d5/image-${id}.svg`, type: "image", width: 900, height: 1200 };
}

function slideItem(id: number, type: MediaGalleryItem["type"], overrides: Partial<MediaGalleryItem> = {}): MediaGalleryItem {
  return {
    id,
    url: `https://bucket.example/d5/${type}-${id}`,
    type,
    width: 300,
    height: 400,
    fileName: `${type}-${id}.bin`,
    fileSize: 1024,
    ...overrides,
  };
}

type LayoutProps = Omit<OverlayVariantLayoutProps, "children"> & { children?: React.ReactNode };

function LayoutHarness(props: LayoutProps) {
  const { children, ...rest } = props;
  return (
    <IntlProvider locale="en" messages={testMessages}>
      <OverlayVariantLayout {...rest}>{children ?? <p>d5 harness body</p>}</OverlayVariantLayout>
    </IntlProvider>
  );
}

function renderLayout(props: LayoutProps) {
  installDom();
  installResizeObserverStub();
  installDialogStubs();
  return render(<LayoutHarness {...props} />);
}

function paneElement(): HTMLElement {
  const pane = document.querySelector<HTMLElement>('[data-slot="variant-media-pane"]');
  assert.ok(pane, "media pane must render");
  return pane;
}

function countBadgeText(): string {
  return document.querySelector<HTMLElement>('[data-slot="media-count-badge"]')?.textContent ?? "";
}

test.afterEach(() => cleanup());

/* ── 逐类型位形态 ─────────────────────────────────────────────────────────── */

test("D5 audio 位：占位图为底 + 底部悬浮 AudioPlayer 条，新类型首项挂载即 settle", async () => {
  const mediaStubs = installMediaElementStubs();
  try {
    const settled: string[] = [];
    renderLayout({
      media: [slideItem(1, "audio")],
      contentId: 9001,
      allowCopy: false,
      onFirstMediaSettled: (state) => settled.push(state),
    });
    const pane = paneElement();
    await waitFor(() => assert.ok(pane.querySelector('[data-testid="audio-player"]'), "floating AudioPlayer bar mounts"));
    assert.ok(pane.querySelector("img"), "audio placeholder art renders as the slide backdrop");
    assert.ok(
      pane.querySelector("audio")?.getAttribute("src")?.includes("audio-1"),
      "audio element binds the signed slide url",
    );
    assert.deepEqual(settled, ["ready"], "viewer-type first item settles immediately (no 2s safety wait)");
  } finally {
    mediaStubs.restore();
  }
});

test("D5 audio 挂起（非顶层）暂停播放；切位卸载停播释放", async () => {
  const mediaStubs = installMediaElementStubs();
  try {
    const view = renderLayout({
      media: [slideItem(1, "audio"), imageItem(2)],
      contentId: 9001,
      allowCopy: false,
    });
    const pane = paneElement();
    await waitFor(() => assert.ok(pane.querySelector('[data-testid="audio-player"]')));

    /* 用户起播后挂起（压层）：暂停。 */
    const playButton = pane.querySelector<HTMLButtonElement>('[data-testid="audio-play-button"]');
    assert.ok(playButton);
    await act(async () => {
      fireEvent.click(playButton);
    });
    const playsAfterStart = mediaStubs.calls.filter((c) => c.op === "play").length;
    assert.ok(playsAfterStart >= 1, "user-initiated playback starts");

    await act(async () => {
      view.rerender(
        <LayoutHarness media={[slideItem(1, "audio"), imageItem(2)]} contentId={9001} allowCopy={false} active={false} />,
      );
    });
    assert.ok(
      mediaStubs.calls.some((c) => c.op === "pause"),
      "suspension pauses the audio slide",
    );

    /* 切位：AudioPlayer 卸载（当前位才挂载），audio 元素离场即停播释放。 */
    const next = document.querySelector<HTMLButtonElement>('button[aria-label="Next media"]');
    assert.ok(next);
    await act(async () => {
      view.rerender(<LayoutHarness media={[slideItem(1, "audio"), imageItem(2)]} contentId={9001} allowCopy={false} active={true} />);
    });
    await act(async () => {
      fireEvent.click(next);
    });
    await waitFor(() => assert.ok(!pane.querySelector('[data-testid="audio-player"]'), "switching slides unmounts the player"));
    assert.equal(pane.querySelectorAll("audio").length, 0, "audio element is released on slide switch");
  } finally {
    mediaStubs.restore();
  }
});

test("D5 document / pdf 位：对应查看器挂载且互斥（当前位单挂载）", async () => {
  const view = renderLayout({
    media: [slideItem(1, "document"), slideItem(2, "text_pdf")],
    contentId: 9001,
    allowCopy: false,
  });
  const pane = paneElement();
  /* document 位：DocumentViewer 动态加载，落定为 loading / 解析结果任一形态。 */
  await waitFor(
    () =>
      assert.ok(
        document.querySelector('[data-testid^="document-viewer-"]'),
        "DocumentViewer mounts in the slide",
      ),
    { timeout: 4000 },
  );
  assert.ok(!document.querySelector("embed"), "document slide renders no embed");

  /* 切到 text_pdf 位：文档查看器卸载，embed 进场。 */
  const next = document.querySelector<HTMLButtonElement>('button[aria-label="Next media"]');
  assert.ok(next);
  await act(async () => {
    fireEvent.click(next);
  });
  await waitFor(() => assert.ok(!document.querySelector('[data-testid^="document-viewer-"]'), "previous viewer unmounts on switch"));
  const embed = document.querySelector("embed");
  assert.ok(embed, "pdf slide renders the native embed");
  assert.equal(embed.getAttribute("type"), "application/pdf");
  assert.ok(embed.getAttribute("src")?.includes("text_pdf-2"));
  void view;
});

test("D5 sheet_music 位：musicxml → OSMD 容器、midi → 回放条位、sheet_pdf → embed", async () => {
  const audioCtx = installAudioContextStub();
  try {
    const view = renderLayout({
      media: [slideItem(1, "sheet_music_xml"), slideItem(2, "sheet_music_midi"), slideItem(3, "sheet_music_pdf")],
      contentId: 9001,
      allowCopy: false,
    });
    const pane = paneElement();
    assert.ok(pane.querySelector(".osmd-container"), "musicxml slide mounts the OSMD renderer container");

    const next = () => document.querySelector<HTMLButtonElement>('button[aria-label="Next media"]');
    assert.ok(next());
    await act(async () => {
      fireEvent.click(next()!);
    });
    await waitFor(() => assert.ok(!pane.querySelector(".osmd-container"), "OSMD unmounts on slide switch"));
    await waitFor(
      () => assert.ok(pane.querySelector("img"), "midi slide shows the score placeholder backdrop while the playback bar loads"),
    );

    await act(async () => {
      fireEvent.click(next()!);
    });
    await waitFor(() => assert.ok(document.querySelector('embed[type="application/pdf"]'), "sheet music pdf renders the embed"));
    void view;
  } finally {
    audioCtx.restore();
  }
});

/* ── 三件套扩展：>9 溢出与混排角标 ───────────────────────────────────────── */

test("D5 >9 项隐藏圆点（N/M 徽标与箭头保留）；≤9 多项圆点照常", async () => {
  const tenImages = Array.from({ length: 10 }, (_, i) => imageItem(i + 1));
  const view = renderLayout({ media: tenImages });
  await waitFor(() => assert.ok(countBadgeText() !== ""));
  assert.equal(countBadgeText(), "1 / 10", "count badge keeps working past the dot cap");
  assert.ok(document.querySelector('button[aria-label="Next media"]'), "arrows stay available");
  assert.equal(document.querySelector('[data-slot="media-paging-dots"]'), null, "dots hide when media exceeds 9 items");

  await act(async () => {
    view.rerender(<LayoutHarness media={tenImages.slice(0, 9)} />);
  });
  await waitFor(() => assert.ok(document.querySelector('[data-slot="media-paging-dots"]'), "dots render for 9 items"));
  assert.equal(countBadgeText(), "1 / 9");
});

test("D5 混排链 slide 角落显示类型小角标；同质链不显示", async () => {
  const view = renderLayout({ media: [imageItem(1), slideItem(2, "document")] });
  await waitFor(() => assert.ok(countBadgeText() !== ""));
  const badge = document.querySelector<HTMLElement>('[data-slot="media-type-badge"]');
  assert.ok(badge, "mixed chain renders the per-slide type badge");
  assert.ok(badge.textContent && badge.textContent.length > 0, "badge carries the localized family label");

  await act(async () => {
    view.rerender(<LayoutHarness media={[imageItem(1), imageItem(2)]} />);
  });
  assert.equal(document.querySelector('[data-slot="media-type-badge"]'), null, "homogeneous chain shows no badge");
});

/* ── MediaViewer 只收 image slides ───────────────────────────────────────── */

test("D5 MediaViewer 只收 image slides：图片位点击开查看器，查看器位点击不开", async () => {
  const view = renderLayout({ media: [imageItem(1), slideItem(2, "document")] });
  await waitFor(() => assert.ok(countBadgeText() !== ""));

  const cover = document.querySelector<HTMLElement>('[data-slot="detail-cover"]');
  assert.ok(cover);
  await act(async () => {
    fireEvent.click(cover);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(document.querySelector("dialog[open]"), "image slide click opens the viewer"));

  const close = document.querySelector<HTMLButtonElement>('dialog[open] button[aria-label="Close viewer"]');
  assert.ok(close);
  await act(async () => {
    fireEvent.click(close);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(!document.querySelector("dialog[open]")));

  const next = document.querySelector<HTMLButtonElement>('button[aria-label="Next media"]');
  assert.ok(next);
  await act(async () => {
    fireEvent.click(next);
  });
  await waitFor(() => assert.ok(document.querySelector('[data-testid^="document-viewer-"]'), "document slide is current"));
  const documentCover = document.querySelector<HTMLElement>('[data-slot="detail-cover"]');
  assert.ok(documentCover);
  await act(async () => {
    fireEvent.click(documentCover);
    await Promise.resolve();
  });
  assert.ok(!document.querySelector("dialog[open]"), "viewer-type slide body click never opens the viewer");
  void view;
});

/* ── 签名过期有限重试 ────────────────────────────────────────────────────── */

test("D5 查看器位「刷新预览」：onMediaRefresh 回调触发且按附件 id 保持当前位", async () => {
  let refreshCount = 0;
  const onMediaRefresh = () => {
    refreshCount += 1;
  };
  const view = renderLayout({
    media: [imageItem(1), slideItem(2, "document")],
    contentId: 9001,
    allowCopy: false,
    onMediaRefresh,
  });
  await waitFor(() => assert.ok(countBadgeText() !== ""));

  /* 图片位不显示刷新钮（图片有既有加载语义）。 */
  assert.equal(document.querySelector('[data-slot="media-refresh-preview"]'), null, "image slide has no refresh control");

  const next = document.querySelector<HTMLButtonElement>('button[aria-label="Next media"]');
  assert.ok(next);
  await act(async () => {
    fireEvent.click(next);
  });
  const refresh = await waitFor(() => {
    const el = document.querySelector<HTMLButtonElement>('[data-slot="media-refresh-preview"]');
    assert.ok(el, "viewer slide offers the refresh control");
    return el;
  });
  await act(async () => {
    fireEvent.click(refresh);
  });
  assert.equal(refreshCount, 1, "refresh triggers the detail refetch callback");

  /* 重取详情后媒体链重建（新数组、同附件 id、新签名 URL）：当前位保持。 */
  await act(async () => {
    view.rerender(
      <LayoutHarness
        media={[imageItem(1), slideItem(2, "document", { url: "https://bucket.example/d5/refreshed-2" })]}
        contentId={9001}
        allowCopy={false}
        onMediaRefresh={onMediaRefresh}
      />,
    );
  });
  await waitFor(() => assert.equal(countBadgeText(), "2 / 2", "position stays anchored to the attachment id after the refresh rebuild"));
});

test("D5 未提供 onMediaRefresh 时不渲染刷新钮（独立调用方不承担重试语义）", async () => {
  renderLayout({ media: [slideItem(1, "document")], contentId: 9001, allowCopy: false });
  await waitFor(() => assert.ok(document.querySelector('[data-testid^="document-viewer-"]')));
  assert.equal(document.querySelector('[data-slot="media-refresh-preview"]'), null);
});

/* ── 规格审查 concerns 修复 ──────────────────────────────────────────────────
 * C-1：midi 位挂起（非顶层/上层弹窗/登录浮窗）停播；恢复不自动续播。
 * C-2：audio/midi 播放条形态位，底部圆点与类型角标上移让位。
 * C-4：midi 回放按钮 aria-label 走 i18n（复用 audio.{play,pause}）。
 * ──────────────────────────────────────────────────────────────────────────── */

test("D5 midi 回放按钮 aria-label 走 i18n（content.attachmentPreview.audio.play/pause）", async () => {
  const stubs = installMidiPlaybackStubs();
  try {
    renderLayout({ media: [slideItem(1, "sheet_music_midi")], contentId: 9001, allowCopy: false });
    const button = await midiPlayButton();
    assert.equal(button.getAttribute("aria-label"), "Play audio", "idle state uses the localized play label");
    await act(async () => {
      fireEvent.click(button);
    });
    assert.equal(button.getAttribute("aria-label"), "Pause audio", "playing state uses the localized pause label");
  } finally {
    stubs.restore();
  }
});

test("D5 midi 位挂起（非顶层）暂停回放：AudioContext 挂起 + 播放态复位；恢复不自动续播", async () => {
  const stubs = installMidiPlaybackStubs();
  try {
    const view = renderLayout({ media: [slideItem(1, "sheet_music_midi")], contentId: 9001, allowCopy: false });
    const button = await midiPlayButton();
    const idleLabel = button.getAttribute("aria-label");

    await act(async () => {
      fireEvent.click(button);
    });
    assert.notEqual(button.getAttribute("aria-label"), idleLabel, "user-initiated playback flips the play state");

    await act(async () => {
      view.rerender(
        <LayoutHarness media={[slideItem(1, "sheet_music_midi")]} contentId={9001} allowCopy={false} active={false} />,
      );
    });
    assert.ok(stubs.records.suspend >= 1, "suspension suspends the MIDI AudioContext");
    assert.equal(button.getAttribute("aria-label"), idleLabel, "suspension pauses the playback and resets the state");

    /* 恢复激活：起播权在用户——不自动续播、不隐式 resume。 */
    await act(async () => {
      view.rerender(
        <LayoutHarness media={[slideItem(1, "sheet_music_midi")]} contentId={9001} allowCopy={false} active={true} />,
      );
    });
    assert.equal(button.getAttribute("aria-label"), idleLabel, "unsuspending does not auto-resume playback");
    assert.equal(stubs.records.resume, 0, "no implicit AudioContext.resume without user intent");

    /* 用户再次起播：挂起的 ctx 由用户手势 resume。 */
    await act(async () => {
      fireEvent.click(button);
    });
    assert.notEqual(button.getAttribute("aria-label"), idleLabel, "user can start playback again after suspension");
    assert.ok(stubs.records.resume >= 1, "user-initiated playback resumes the suspended AudioContext");

    /* 收尾再挂起一次：取消回放进度帧循环，避免测试结束后遗留计时器。 */
    await act(async () => {
      view.rerender(
        <LayoutHarness media={[slideItem(1, "sheet_music_midi")]} contentId={9001} allowCopy={false} active={false} />,
      );
    });
  } finally {
    stubs.restore();
  }
});

test("D5 播放条形态位：底部圆点与类型角标上移让位（几何不遮播放条）", async () => {
  const view = renderLayout({ media: [imageItem(1), slideItem(2, "audio")] });
  await waitFor(() => assert.ok(countBadgeText() !== ""));
  const next = () => document.querySelector<HTMLButtonElement>('button[aria-label="Next media"]');
  assert.ok(next());
  await act(async () => {
    fireEvent.click(next()!);
  });
  await waitFor(() => assert.ok(document.querySelector('[data-slot="slide-audio"]'), "audio slide is current"));

  const dots = document.querySelector<HTMLElement>('[data-slot="media-paging-dots"]');
  assert.ok(dots, "dots render for the mixed chain");
  assert.ok(dots.classList.contains("bottom-32"), "dots yield above the floating player bar");
  const badge = document.querySelector<HTMLElement>('[data-slot="media-type-badge"]');
  assert.ok(badge, "type badge renders for the mixed chain");
  assert.ok(badge.classList.contains("bottom-32"), "type badge yields above the floating player bar");

  const prev = () => document.querySelector<HTMLButtonElement>('button[aria-label="Previous media"]');
  assert.ok(prev());
  await act(async () => {
    fireEvent.click(prev()!);
  });
  await waitFor(() => assert.ok(!document.querySelector('[data-slot="slide-audio"]'), "back on the image slide"));
  const dotsDefault = document.querySelector<HTMLElement>('[data-slot="media-paging-dots"]');
  assert.ok(dotsDefault, "dots still render on the image slide");
  assert.ok(dotsDefault.classList.contains("bottom-3"), "dots keep the default bottom geometry on non-player-bar slides");
  assert.ok(!dotsDefault.classList.contains("bottom-32"), "yield geometry only applies to player-bar slides");
  void view;
});

/* ── 代码质量审查 concerns 修复（2026-10-11，amend 进同一实现 commit）──────────
 * C4（既有债）：midi 暂停→续播时 UI 进度基准必须取播放器冻结位置（midi-player-js
 *   pause() 把当前 tick 存 startTick、play() 从该 tick 续播）——进度条不得归零
 *   重走，与声音位置一致。桩轨道 4800 ticks（5s @120bpm；库公开 getter
 *   getTotalTicks() 在 dryRun 后恒 0，组件 duration 落 30s 兜底，进度分母 = 30s，
 *   但 EOF 判定用内部 totalTicks 字段=4800——播放窗口内不会触发 stop 清零）。
 * C5（既有债）：播放中卸载组件，cleanup 必须显式 pause 停走 midi 调度器
 *   （ctx.close 已有，Player.pause 原型 spy 断言）。
 * ──────────────────────────────────────────────────────────────────────────── */

function midiProgressWidthPercent(): number {
  const bar = document.querySelector<HTMLElement>('[data-slot="slide-sheet-music"] .bg-accent');
  assert.ok(bar, "midi playback progress bar renders");
  const raw = bar.style.width;
  const value = Number.parseFloat(raw);
  assert.ok(Number.isFinite(value), `progress width parses as percent (got ${raw || "<empty>"})`);
  return value;
}

test("D5 midi 续播进度基准：暂停后续播进度条不归零，从暂停位置继续（质量审查 C4）", async () => {
  /* 真实计时器驱动（rAF = setTimeout(0)）；进度分母 = 30s 兜底 duration，
     250ms 播放窗 → ≥0.8%，续播归零路径回落到 ~0.2%，区分度 3 倍以上。 */
  const stubs = installMidiPlaybackStubs({ trackTicks: 4800 });
  try {
    renderLayout({ media: [slideItem(1, "sheet_music_midi")], contentId: 9001, allowCopy: false });
    const button = await midiPlayButton();

    await act(async () => {
      fireEvent.click(button);
    });
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 250));
    });
    await act(async () => {
      fireEvent.click(button);
    });
    const widthAtPause = midiProgressWidthPercent();
    assert.ok(widthAtPause >= 0.5, `progress advances before pause (got ${widthAtPause}%)`);

    /* 暂停保持：进度冻结不回退。 */
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 60));
    });
    assert.equal(midiProgressWidthPercent(), widthAtPause, "paused progress stays frozen");

    /* 续播：基准 = 播放器冻结位置——进度从暂停处继续，不归零重走。 */
    await act(async () => {
      fireEvent.click(button);
    });
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 80));
    });
    const widthAfterResume = midiProgressWidthPercent();
    assert.ok(
      widthAfterResume >= widthAtPause - 0.05,
      `resume continues from the paused position instead of restarting (paused ${widthAtPause}%, after ${widthAfterResume}%)`,
    );

    /* 收尾暂停，避免测试结束后遗留帧循环/调度器。 */
    await act(async () => {
      fireEvent.click(button);
    });
  } finally {
    stubs.restore();
  }
});

test("D5 midi 播放中卸载：cleanup 显式 pause 停走 midi 调度器（质量审查 C5）", async () => {
  const stubs = installMidiPlaybackStubs();
  /* 组件 init 走动态 import（同进程同模块实例）；对 Player.prototype.pause
     spy 计数，断言卸载路径调用（ctx.close 之外补 Player.pause）。 */
  const midiModule = await import("midi-player-js");
  const PlayerCtor =
    midiModule.Player ??
    (midiModule as unknown as { default?: typeof midiModule }).default?.Player;
  assert.ok(PlayerCtor, "midi-player-js Player resolves under the test loader");
  type PauseFn = (this: unknown) => unknown;
  const proto = PlayerCtor.prototype as unknown as { pause: PauseFn };
  const originalPause = proto.pause;
  let pauseCalls = 0;
  proto.pause = function pauseSpy(this: unknown) {
    pauseCalls += 1;
    return originalPause.call(this);
  };
  try {
    const view = renderLayout({ media: [slideItem(1, "sheet_music_midi")], contentId: 9001, allowCopy: false });
    const button = await midiPlayButton();
    await act(async () => {
      fireEvent.click(button);
    });
    const callsWhilePlaying = pauseCalls;
    act(() => {
      view.unmount();
    });
    assert.ok(
      pauseCalls > callsWhilePlaying,
      "unmount while playing explicitly pauses the midi scheduler",
    );
  } finally {
    proto.pause = originalPause;
    stubs.restore();
  }
});
