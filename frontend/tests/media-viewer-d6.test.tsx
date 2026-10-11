import assert from "node:assert/strict";
import test from "node:test";
import React from "react";

import { MediaViewer } from "@/components/content/MediaViewer";
import type { MediaGalleryItem } from "@/components/content/MediaGallery";
import { act, cleanup, fireEvent, installDom, renderWithIntl } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * D6 #863 MediaViewer 多类型化（spec §3.2 放大态行 / §9.6 末两段）：
 * - model3d slide = 全屏 ModelViewer 复用；text_pdf slide = 全屏 embed；
 *   image = 既有 1–5x 缩放平移不变；video = 既有调用方兼容渲染（不丢项）。
 * - 3D/PDF 输入优先：画布/embed 上不绑定翻页手势（触摸滑动不翻页），
 *   翻页只走显式箭头；边界 clamp。
 * - 放大态统一左右箭头 + 独立序号：序号 = 调用方传入序列内的位置
 *   （浮层侧由调用方传过滤后的可放大序列实现独立计数）。
 * jsdom 注意：ModelViewer 的 oss_url fetch 需桩为确定性失败（DNS 真网络
 * 请求在测试环境不可控），走组件内 error 降级分支即可断言挂载形态。
 * ──────────────────────────────────────────────────────────────────────────── */

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

/** ModelViewer oss_url fetch 桩：model3d slide 的 URL 确定性 404（组件内
 * 走 error 降级态，data-testid=model-viewer-error），其余 fetch 原样放行。 */
function installModelFetchStub() {
  const g = globalThis as unknown as { fetch?: typeof fetch };
  const original = g.fetch;
  g.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    if (String(input).includes("model3d-")) {
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

function makeItem(id: number, overrides: Partial<MediaGalleryItem> = {}): MediaGalleryItem {
  return {
    id,
    url: `/seed-media/d6/item-${id}.bin`,
    type: "image",
    width: 1000,
    height: 750,
    ...overrides,
  };
}

const mixedItems: MediaGalleryItem[] = [
  makeItem(1, { type: "image", url: "/seed-media/d6/model3d-1.png" }),
  makeItem(2, { type: "model3d", url: "https://bucket.example/d6/model3d-2.stl", fileName: "bracket.stl", fileSize: 1024 }),
  makeItem(3, { type: "text_pdf", url: "https://bucket.example/d6/text_pdf-3.pdf", fileName: "readme.pdf", fileSize: 2048 }),
];

function viewerDialog(): HTMLDialogElement {
  const dialog = document.body.querySelector("dialog");
  assert.ok(dialog, "viewer dialog must be mounted");
  return dialog as HTMLDialogElement;
}

function positionLabel(): string {
  const group = viewerDialog().querySelector('[role="group"]');
  assert.ok(group, "viewer position dots group must exist");
  return group?.getAttribute("aria-label") ?? "";
}

function viewerNext(): HTMLButtonElement {
  const next = viewerDialog().querySelector('button[aria-label="Next media"]') as HTMLButtonElement | null;
  assert.ok(next, "viewer next button must exist");
  return next as HTMLButtonElement;
}

function viewerPrevious(): HTMLButtonElement {
  const previous = viewerDialog().querySelector('button[aria-label="Previous media"]') as HTMLButtonElement | null;
  assert.ok(previous, "viewer previous button must exist");
  return previous as HTMLButtonElement;
}

function stageOf(): HTMLElement {
  const stage = document.body.querySelector('[data-slot="viewer-stage"]');
  assert.ok(stage, "viewer stage must exist");
  return stage as HTMLElement;
}

function pointerSwipe(from: { x: number; y: number }, to: { x: number; y: number }) {
  act(() => {
    fireEvent.pointerDown(stageOf(), { pointerId: 1, pointerType: "touch", clientX: from.x, clientY: from.y });
  });
  act(() => {
    fireEvent.pointerMove(stageOf(), { pointerId: 1, pointerType: "touch", clientX: to.x, clientY: to.y });
  });
  act(() => {
    fireEvent.pointerUp(stageOf(), { pointerId: 1, pointerType: "touch", clientX: to.x, clientY: to.y });
  });
}

function renderViewer(overrides: Partial<React.ComponentProps<typeof MediaViewer>> = {}) {
  const Comp = () => {
    const [open, setOpen] = React.useState(overrides.open ?? true);
    const [index, setIndex] = React.useState(overrides.index ?? 0);
    return (
      <MediaViewer
        items={overrides.items ?? mixedItems}
        index={index}
        open={open}
        onOpenChange={setOpen}
        onIndexChange={setIndex}
      />
    );
  };
  return renderWithIntl(<Comp />);
}

test.beforeEach(() => {
  installDom();
  installDialogStubs();
});
test.afterEach(() => cleanup());

/* ---------- model3d / text_pdf 全屏形态 ---------- */

test("D6 model3d slide：全屏复用 ModelViewer（懒挂载），无缩放控件", async () => {
  const fetchStub = installModelFetchStub();
  try {
    renderViewer({ index: 1 });
    await waitForTestid("model-viewer-error", "ModelViewer mounts fullscreen for the model3d slide");
    const dialog = viewerDialog();
    assert.equal(positionLabel(), "2 / 3", "position counts within the passed sequence");
    assert.equal(dialog.querySelector('button[aria-label="Zoom in"]'), null, "zoom controls are image-only");
    assert.equal(dialog.querySelector("img"), null, "no image element on the model3d slide");
    assert.equal(dialog.querySelector("video"), null, "no video element on the model3d slide");
  } finally {
    fetchStub.restore();
  }
});

test("D6 text_pdf slide：全屏原生 embed，无缩放控件", () => {
  renderViewer({ index: 2 });
  const dialog = viewerDialog();
  const embed = dialog.querySelector("embed");
  assert.ok(embed, "pdf slide renders the native embed");
  assert.equal(embed.getAttribute("type"), "application/pdf");
  assert.ok(embed.getAttribute("src")?.includes("text_pdf-3"));
  assert.equal(positionLabel(), "3 / 3");
  assert.equal(dialog.querySelector('button[aria-label="Zoom in"]'), null, "zoom controls are image-only");
});

test("D6 混排序列翻页：箭头跨 image→model3d→text_pdf 切换，边界 clamp", async () => {
  const fetchStub = installModelFetchStub();
  try {
    renderViewer({ index: 0 });
    assert.equal(positionLabel(), "1 / 3");
    await act(async () => {
      fireEvent.click(viewerNext());
    });
    await waitForTestid("model-viewer-error", "paging lands on the model3d slide");
    assert.equal(positionLabel(), "2 / 3");
    await act(async () => {
      fireEvent.click(viewerNext());
    });
    await waitForEmbed();
    assert.equal(positionLabel(), "3 / 3");
    assert.equal(viewerNext().disabled, true, "next clamps at the last enlargeable item");
    await act(async () => {
      fireEvent.click(viewerPrevious());
    });
    await waitForTestid("model-viewer-error", "previous returns to the model3d slide");
    await act(async () => {
      fireEvent.click(viewerPrevious());
    });
    assert.ok(viewerDialog().querySelector("img"), "back to the image slide");
  } finally {
    fetchStub.restore();
  }
});

/* ---------- 3D/PDF 输入优先（翻页手势不绑定） ---------- */

test("D6 输入优先：3D/PDF 位触摸滑动不翻页（画布/embed 原生交互优先）", async () => {
  const fetchStub = installModelFetchStub();
  try {
    renderViewer({ index: 1 });
    await waitForTestid("model-viewer-error");
    pointerSwipe({ x: 300, y: 200 }, { x: 80, y: 202 });
    assert.equal(positionLabel(), "2 / 3", "swipe over the 3D canvas does not page");

    await act(async () => {
      fireEvent.click(viewerNext());
    });
    await waitForEmbed();
    pointerSwipe({ x: 300, y: 200 }, { x: 80, y: 202 });
    assert.equal(positionLabel(), "3 / 3", "swipe over the pdf embed does not page");
  } finally {
    fetchStub.restore();
  }
});

/* ---------- 既有调用方兼容（video 不丢项） ---------- */

test("D6 兼容：既有调用方传 video 序列不崩溃、不悄然丢项（跨视频翻页照常）", () => {
  const legacyItems = [
    makeItem(1, { type: "image", url: "/seed-media/d6/compat-1.png" }),
    makeItem(2, { type: "video", url: "/seed-media/d6/compat-2.mp4", posterUrl: "/poster.jpg" }),
    makeItem(3, { type: "image", url: "/seed-media/d6/compat-3.png" }),
  ];
  renderViewer({ items: legacyItems, index: 0 });
  assert.equal(positionLabel(), "1 / 3");
  act(() => {
    fireEvent.click(viewerNext());
  });
  const video = viewerDialog().querySelector("video");
  assert.ok(video, "legacy video item still renders in the viewer (no silent drop)");
  assert.equal(video?.getAttribute("controls"), "");
  assert.equal(video?.getAttribute("poster"), "/poster.jpg");
  assert.equal(positionLabel(), "2 / 3", "video keeps its slot in the legacy sequence");
  act(() => {
    fireEvent.click(viewerNext());
  });
  assert.ok(viewerDialog().querySelector("img"), "paging continues past the video to the next image");
});

/* ---------- 单项/空集 ---------- */

test("D6 单个可放大项：不显示翻页控件与圆点（model3d）", async () => {
  const fetchStub = installModelFetchStub();
  try {
    renderViewer({ items: [mixedItems[1]] });
    await waitForTestid("model-viewer-error");
    const dialog = viewerDialog();
    assert.equal(dialog.querySelector('button[aria-label="Previous media"]'), null);
    assert.equal(dialog.querySelector('button[aria-label="Next media"]'), null);
    assert.equal(dialog.querySelector('[role="group"]'), null);
  } finally {
    fetchStub.restore();
  }
});

/* ---------- >9 可放大序列（spec §9.6「>9」覆盖，守卫型） ---------- */

test("D6 >9 可放大序列：12 项序号徽标正确、首末 disabled、越界 index clamp（守卫覆盖）", async () => {
  const items = Array.from(
    { length: 12 },
    (_, i) => makeItem(i + 1, { url: `/seed-media/d6/over9-${i + 1}.svg` }),
  );
  renderViewer({ items, index: 0 });
  assert.equal(positionLabel(), "1 / 12", "badge counts across the full >9 sequence");
  assert.equal(viewerPrevious().disabled, true, "previous disabled at the first item");

  /* 连续 next 到末位：序号逐位正确、末位 next disabled。 */
  for (let step = 2; step <= 12; step += 1) {
    await act(async () => {
      fireEvent.click(viewerNext());
    });
    assert.equal(positionLabel(), `${step} / 12`, `badge stays correct at step ${step}`);
  }
  assert.equal(viewerNext().disabled, true, "next disabled at the last item");
  assert.equal(viewerPrevious().disabled, false, "previous re-enables away from the first item");

  /* 越界 index clamp 到末位（防御：调用方序列缩短时不越界）。 */
  cleanup();
  renderViewer({ items, index: 99 });
  assert.equal(positionLabel(), "12 / 12", "out-of-range index clamps to the last item");
  assert.equal(viewerNext().disabled, true, "clamped position keeps the boundary disabled state");
});

test("D6 空序列：查看器不渲染任何内容", () => {
  renderViewer({ items: [], open: false });
  assert.equal(document.body.querySelector("dialog"), null);
});

/* ---------- Esc 只关最上层（多类型下保持） ---------- */

test("D6 Esc 关闭语义在 3D 位保持（capture 阶段、不传播）", async () => {
  const fetchStub = installModelFetchStub();
  try {
    const openCalls: boolean[] = [];
    const Comp = () => {
      const [open, setOpen] = React.useState(true);
      const [index, setIndex] = React.useState(1);
      return (
        <MediaViewer
          items={mixedItems}
          index={index}
          open={open}
          onOpenChange={(next) => {
            setOpen(next);
            openCalls.push(next);
          }}
          onIndexChange={setIndex}
        />
      );
    };
    renderWithIntl(<Comp />);
    await waitForTestid("model-viewer-error");
    let bubbledEsc = 0;
    window.addEventListener("keydown", () => (bubbledEsc += 1));
    act(() => {
      fireEvent.keyDown(window, { key: "Escape" });
    });
    assert.deepEqual(openCalls, [false], "Esc closes the viewer from a 3D slide");
    assert.equal(bubbledEsc, 0, "Esc never propagates to lower layers");
  } finally {
    fetchStub.restore();
  }
});

/* ---------- helpers ---------- */

async function waitForTestid(testid: string, message?: string) {
  const deadline = Date.now() + 4000;
  for (;;) {
    const found = document.querySelector(`[data-testid="${testid}"]`);
    if (found) return found;
    if (Date.now() > deadline) {
      assert.fail(message ?? `expected [data-testid="${testid}"] to mount`);
    }
    await new Promise((resolve) => window.setTimeout(resolve, 20));
  }
}

async function waitForEmbed() {
  const deadline = Date.now() + 4000;
  for (;;) {
    if (document.body.querySelector("dialog embed")) return;
    if (Date.now() > deadline) assert.fail("expected viewer embed to mount");
    await new Promise((resolve) => window.setTimeout(resolve, 20));
  }
}
