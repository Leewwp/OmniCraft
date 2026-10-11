import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import { OverlayVariantLayout, type OverlayVariantLayoutProps } from "@/components/content/OverlayVariantLayout";
import type { MediaGalleryItem } from "@/components/content/MediaGallery";
import { testMessages } from "./runtime-test-helpers";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * D6 #863 放大态多类型化 + 跨可放大项导航（spec §3.2 放大态行 / §9.6 末两段）：
 * - 可放大序列 = image / model3d / text_pdf（稳定附件 id 映射左列序列）；
 *   独立计数（跳过音频/文档/乐谱/视频位），边界 clamp。
 * - 入口：image = 主体点击或右上放大钮；model3d/text_pdf = 仅右上放大钮
 *   （画布/embed 点击交原生交互）；audio/document/乐谱/视频 = 无入口。
 * - 打开定位正确项（3D 位按钮 → 打开后当前项 = 该 3D，不是首图）。
 * - 关闭返回最后所看项（左列轮播定位到该项）并恢复触发焦点。
 * - Esc 只关最上层；浮层键盘翻页在查看器打开期间不劫持（既有语义保持）。
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

/** bucket.example 测试域 fetch 桩：slide 附件 URL（model3d/document 等
 * 查看器族加载）确定性 404（组件内 error 降级路径），其余 fetch 原样放行
 * （audio/video 元素加载不经过 fetch）。文件级 beforeEach 统一安装，用例
 * 不依赖真实 DNS 解析结果——通配/劫持 DNS 或代理环境下 fetch 可能挂起或
 * 返回非预期响应，waitFor 会闪断。 */
function installSlideFetchStub() {
  const g = globalThis as unknown as { fetch?: typeof fetch };
  const original = g.fetch;
  g.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    if (String(input).includes("bucket.example")) {
      return new Response("stub-not-found", { status: 404 });
    }
    return original ? original(input, init) : Promise.reject(new Error("no fetch"));
  }) as typeof fetch;
  return {
    restore: () => {
      g.fetch = original;
    },
  };
}

function imageItem(id: number, overrides: Partial<MediaGalleryItem> = {}): MediaGalleryItem {
  return { id, url: `/seed-media/d6/image-${id}.svg`, type: "image", width: 900, height: 1200, ...overrides };
}

function slideItem(id: number, type: MediaGalleryItem["type"], overrides: Partial<MediaGalleryItem> = {}): MediaGalleryItem {
  return {
    id,
    url: `https://bucket.example/d6/${type}-${id}`,
    type,
    width: 300,
    height: 400,
    fileName: `${type}-${id}.bin`,
    fileSize: 1024,
    ...overrides,
  };
}

/** 混排六位链：image(1) / audio(2) / model3d(3) / text_pdf(4) / sheet_music_pdf(5) / video(6)。
 * 可放大序列 = [image-1, model3d-3, text_pdf-4]。 */
function mixedChain(): MediaGalleryItem[] {
  return [
    imageItem(1),
    slideItem(2, "audio"),
    slideItem(3, "model3d"),
    slideItem(4, "text_pdf"),
    slideItem(5, "sheet_music_pdf"),
    slideItem(6, "video", { posterUrl: "/poster.jpg" }),
  ];
}

type LayoutProps = Omit<OverlayVariantLayoutProps, "children"> & { children?: React.ReactNode };

function LayoutHarness(props: LayoutProps) {
  const { children, ...rest } = props;
  return (
    <IntlProvider locale="en" messages={testMessages}>
      <OverlayVariantLayout {...rest}>{children ?? <p>d6 harness body</p>}</OverlayVariantLayout>
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

function viewerDialog(): HTMLDialogElement {
  const dialog = document.body.querySelector("dialog");
  assert.ok(dialog, "viewer dialog must be mounted");
  return dialog as HTMLDialogElement;
}

function viewerPosition(): string {
  const group = viewerDialog().querySelector('[role="group"]');
  return group?.getAttribute("aria-label") ?? "";
}

function enlargeButton(): HTMLButtonElement | null {
  return document.querySelector<HTMLButtonElement>('[data-slot="media-enlarge-button"]');
}

function coverElement(): HTMLElement {
  const cover = document.querySelector<HTMLElement>('[data-slot="detail-cover"]');
  assert.ok(cover, "detail-cover anchor must render");
  return cover;
}

function paneNext(): HTMLButtonElement {
  const next = document.querySelector<HTMLButtonElement>('button[aria-label="Next media"]');
  assert.ok(next, "pane next arrow must exist");
  return next;
}

async function goToSlide(target: number) {
  for (let at = countBadgeCurrent(); at < target; at += 1) {
    await act(async () => {
      fireEvent.click(paneNext());
    });
  }
}

function countBadgeCurrent(): number {
  const text = countBadgeText();
  const current = Number.parseInt(text.split("/")[0]?.trim() ?? "", 10);
  assert.ok(Number.isFinite(current), `count badge must parse (got "${text}")`);
  return current;
}

async function waitForViewerModel3d() {
  await waitFor(
    () => assert.ok(document.querySelector('[data-testid="model-viewer-error"], [data-testid="model-viewer-loading"]'), "fullscreen ModelViewer mounts"),
    { timeout: 4000 },
  );
}

async function flushRaf() {
  await new Promise((resolve) => window.setTimeout(resolve, 5));
}

let slideFetchStub: ReturnType<typeof installSlideFetchStub> | undefined;
test.beforeEach(() => {
  slideFetchStub = installSlideFetchStub();
});
test.afterEach(() => {
  slideFetchStub?.restore();
  slideFetchStub = undefined;
  cleanup();
});

/* ---------- 入口规则与稳定 id 映射 ---------- */

test("D6 3D 位右上放大钮：打开后当前项 = 该 3D（稳定 id 映射，不是首图）", async () => {
  renderLayout({ media: mixedChain(), contentId: 9101, allowCopy: false });
  await waitFor(() => assert.ok(countBadgeText() !== ""));
  await goToSlide(3);
  assert.equal(countBadgeText(), "3 / 6", "model3d slide is current in the pane");

  const button = enlargeButton();
  assert.ok(button, "model3d slide exposes the top-right enlarge button");
  await act(async () => {
    fireEvent.click(button);
    await Promise.resolve();
  });
  await waitForViewerModel3d();
  assert.equal(viewerPosition(), "2 / 3", "viewer opens directly at the enlargeable position of this 3D item");
  assert.ok(viewerDialog().querySelector("embed") === null, "not the pdf slide");
  assert.ok(viewerDialog().querySelector("img") === null, "not the first image slide");
});

test("D6 pdf 位放大钮：打开即定位该 pdf（3/3）；image 位也有放大钮（双入口）", async () => {
  renderLayout({ media: mixedChain(), contentId: 9101, allowCopy: false });
  await waitFor(() => assert.ok(countBadgeText() !== ""));
  await goToSlide(4);
  assert.equal(countBadgeText(), "4 / 6", "text_pdf slide is current");

  const pdfButton = enlargeButton();
  assert.ok(pdfButton, "text_pdf slide exposes the enlarge button");
  await act(async () => {
    fireEvent.click(pdfButton);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(viewerDialog().querySelector("embed"), "fullscreen embed mounts"));
  assert.equal(viewerPosition(), "3 / 3", "viewer opens directly at the pdf position");

  /* 关闭后回到 image 位：双入口（放大钮 + 主体点击）都在。 */
  const close = viewerDialog().querySelector<HTMLButtonElement>('button[aria-label="Close viewer"]');
  assert.ok(close);
  await act(async () => {
    fireEvent.click(close);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(!document.querySelector("dialog[open]")));
  await waitFor(() => assert.equal(countBadgeText(), "4 / 6", "close returns the pane to the last viewed item"));
  assert.ok(enlargeButton(), "image slide keeps the enlarge button entry");
});

test("D6 无放大入口位：audio/乐谱 pdf/video 无放大钮、主体点击不开查看器", async () => {
  renderLayout({ media: mixedChain(), contentId: 9101, allowCopy: false });
  await waitFor(() => assert.ok(countBadgeText() !== ""));

  /* audio 位（2/6）。 */
  await goToSlide(2);
  assert.equal(countBadgeText(), "2 / 6");
  assert.equal(enlargeButton(), null, "audio slide has no enlarge entry");
  await act(async () => {
    fireEvent.click(coverElement());
    await Promise.resolve();
  });
  assert.ok(!document.querySelector("dialog[open]"), "audio body click never opens the viewer");

  /* 乐谱族 pdf 位（5/6）：仍属乐谱位，不放大。 */
  await goToSlide(5);
  assert.equal(countBadgeText(), "5 / 6");
  assert.equal(enlargeButton(), null, "sheet_music pdf slide has no enlarge entry");
  await act(async () => {
    fireEvent.click(coverElement());
    await Promise.resolve();
  });
  assert.ok(!document.querySelector("dialog[open]"), "sheet music pdf body click never opens the viewer");

  /* video 位（6/6）：无入口（控制条原生全屏）。 */
  await goToSlide(6);
  assert.equal(countBadgeText(), "6 / 6");
  assert.equal(enlargeButton(), null, "video slide has no enlarge entry");
});

test("D6 无可放大项的链：整链无放大钮、不挂 MediaViewer", async () => {
  renderLayout({ media: [slideItem(1, "audio"), slideItem(2, "document")], contentId: 9101, allowCopy: false });
  await waitFor(() => assert.ok(document.querySelector('[data-testid^="document-viewer-"], [data-slot="slide-audio"]')));
  assert.equal(enlargeButton(), null);
  assert.ok(!document.querySelector("dialog"), "viewer stays unmounted when no enlargeable item exists");
});

/* ---------- 合成自动文字封面不可放大（spec §3.2 article 行 = — / §9.6） ---------- */

test("D6 自动文字封面链：无放大钮、主体点击无副作用不开查看器", async () => {
  renderLayout({ media: [imageItem(21, { autoCover: true })], contentId: 9101, allowCopy: false });
  await waitFor(() => assert.ok(coverElement()));
  assert.equal(Boolean(enlargeButton()), false, "synthesized auto text cover has no enlarge entry");
  const coverImg = coverElement().querySelector("img");
  assert.ok(coverImg, "auto cover renders an img element");
  assert.equal(coverImg.className.includes("cursor-zoom-in"), false, "auto cover img keeps the default cursor (body click is a no-op)");
  await act(async () => {
    fireEvent.click(coverElement());
    await Promise.resolve();
  });
  assert.ok(!document.querySelector("dialog"), "auto-cover body click never opens (or mounts) the viewer");
});

test("D6 真内容封面链：有放大钮、主体点击开查看器 1/1（真实图片保持可放大）", async () => {
  renderLayout({ media: [imageItem(22)], contentId: 9101, allowCopy: false });
  await waitFor(() => assert.ok(enlargeButton(), "real content cover keeps the enlarge button"));
  const coverImg = coverElement().querySelector("img");
  assert.ok(coverImg);
  assert.equal(coverImg.className.includes("cursor-zoom-in"), true, "real image keeps the zoom-in cursor (body click opens the viewer)");
  await act(async () => {
    fireEvent.click(coverElement());
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(document.querySelector("dialog[open]")));
  const position = document.querySelector('[data-slot="viewer-position"]')?.textContent ?? "";
  assert.equal(position, "1 / 1", "viewer opens at the only (real cover) item");
});

/* ---------- 独立计数（跳过非可放大位） ---------- */

test("D6 独立计数：image→model3d→text_pdf 之间切换，序号独立于左列轮播", async () => {
  renderLayout({ media: mixedChain(), contentId: 9101, allowCopy: false });
  await waitFor(() => assert.ok(countBadgeText() !== ""));

  /* image 位主体点击开查看器（既有入口），1/3。 */
  await act(async () => {
    fireEvent.click(coverElement());
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(document.querySelector("dialog[open]")));
  assert.equal(viewerPosition(), "1 / 3", "viewer sequence = enlargeable items only (skips audio)");

  /* 查看器内 next：直接到 model3d（跳过 audio 位）。 */
  const viewerNext = viewerDialog().querySelector<HTMLButtonElement>('button[aria-label="Next media"]');
  assert.ok(viewerNext);
  await act(async () => {
    fireEvent.click(viewerNext);
  });
  await waitForViewerModel3d();
  assert.equal(viewerPosition(), "2 / 3");
  assert.equal(countBadgeText(), "1 / 6", "pane position does not move while paging inside the viewer");

  /* 再 next 到 pdf（3/3），边界 clamp。 */
  const nextAgain = viewerDialog().querySelector<HTMLButtonElement>('button[aria-label="Next media"]');
  assert.ok(nextAgain);
  await act(async () => {
    fireEvent.click(nextAgain);
  });
  await waitFor(() => assert.ok(viewerDialog().querySelector("embed")));
  assert.equal(viewerPosition(), "3 / 3");
  const nextAtEnd = viewerDialog().querySelector<HTMLButtonElement>('button[aria-label="Next media"]');
  assert.ok(nextAtEnd);
  assert.equal(nextAtEnd.disabled, true, "viewer paging clamps at the enlargeable boundary");
});

/* ---------- 关闭回最后所看项 + 焦点恢复 ---------- */

test("D6 关闭回最后所看项：查看器内翻到 pdf 后关闭，左列定位 pdf 位并恢复触发焦点", async () => {
  renderLayout({ media: mixedChain(), contentId: 9101, allowCopy: false });
  await waitFor(() => assert.ok(countBadgeText() !== ""));
  await goToSlide(3);
  const button = enlargeButton();
  assert.ok(button);
  await act(async () => {
    fireEvent.click(button);
    await Promise.resolve();
  });
  await waitForViewerModel3d();

  /* 查看器内翻到 pdf（3/3），关闭。 */
  const viewerNext = viewerDialog().querySelector<HTMLButtonElement>('button[aria-label="Next media"]');
  assert.ok(viewerNext);
  await act(async () => {
    fireEvent.click(viewerNext);
  });
  await waitFor(() => assert.ok(viewerDialog().querySelector("embed")));
  const close = viewerDialog().querySelector<HTMLButtonElement>('button[aria-label="Close viewer"]');
  assert.ok(close);
  await act(async () => {
    fireEvent.click(close);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(!document.querySelector("dialog[open]")));
  await waitFor(() => assert.equal(countBadgeText(), "4 / 6", "pane lands on the last viewed (pdf) slide"));
  await flushRaf();
  assert.equal(document.activeElement, button, "focus returns to the enlarge trigger button");
});

test("D6 Esc 关闭同样回最后所看项（Esc 只关最上层，浮层本体不退层）", async () => {
  renderLayout({ media: mixedChain(), contentId: 9101, allowCopy: false });
  await waitFor(() => assert.ok(countBadgeText() !== ""));
  await act(async () => {
    fireEvent.click(coverElement());
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(document.querySelector("dialog[open]")));
  /* image → model3d。 */
  const viewerNext = viewerDialog().querySelector<HTMLButtonElement>('button[aria-label="Next media"]');
  assert.ok(viewerNext);
  await act(async () => {
    fireEvent.click(viewerNext);
  });
  await waitForViewerModel3d();
  await act(async () => {
    fireEvent.keyDown(window, { key: "Escape" });
  });
  await waitFor(() => assert.ok(!document.querySelector("dialog[open]"), "Esc closes only the viewer"));
  await waitFor(() => assert.equal(countBadgeText(), "3 / 6", "pane lands on the last viewed (model3d) slide"));
});

/* ---------- 键盘语义保持 ---------- */

test("D6 查看器打开期间浮层键盘翻页不劫持（←/→ 不动左列）", async () => {
  renderLayout({ media: mixedChain(), contentId: 9101, allowCopy: false });
  await waitFor(() => assert.ok(countBadgeText() !== ""));
  await act(async () => {
    fireEvent.click(coverElement());
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(document.querySelector("dialog[open]")));
  await act(async () => {
    fireEvent.keyDown(window, { key: "ArrowRight" });
  });
  assert.equal(countBadgeText(), "1 / 6", "overlay arrow keys stay inert while the viewer is open");
  assert.ok(document.querySelector("dialog[open]"), "viewer stays open (arrow is not Esc)");
});

/* ---------- 单个可放大项 ---------- */

test("D6 混排链仅一个可放大项：查看器内无翻页控件", async () => {
  renderLayout({ media: [slideItem(1, "audio"), slideItem(2, "model3d")], contentId: 9101, allowCopy: false });
  await waitFor(() => assert.ok(countBadgeText() !== ""));
  await goToSlide(2);
  const button = enlargeButton();
  assert.ok(button, "the only enlargeable item still exposes the button");
  await act(async () => {
    fireEvent.click(button);
    await Promise.resolve();
  });
  await waitForViewerModel3d();
  const dialog = viewerDialog();
  assert.equal(dialog.querySelector('button[aria-label="Next media"]'), null, "single enlargeable item hides viewer paging");
  assert.equal(dialog.querySelector('[role="group"]'), null);
});
