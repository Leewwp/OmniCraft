import test from "node:test";
import assert from "node:assert/strict";
import {
  OVERLAY_GEOMETRY_TRANSITION,
  OVERLAY_PANE_MIN_WIDTH,
  OVERLAY_RIGHT_COL_WIDTH,
  OVERLAY_SHELL_MAX_WIDTH,
  computeOverlayGeometry,
  contentFrozenRatio,
  overlayShellMaxWidth,
  slideTierOf,
} from "@/lib/overlay-geometry";
import type { MediaGalleryItem, MediaSlideType } from "@/components/content/MediaGallery";

/* ────────────────────────────────────────────────────────────────────────────
 * E1 #953 几何引擎纯函数契约（spec 四项裁决 2/4 + 实测参数）：
 * - 档位判定：文档族（document/text_pdf/乐谱族/model3d）= reading 宽舞台；
 *   image/video/audio = visual；
 * - 内容冻结比例 = 媒体链首 slide intrinsic 比例（空链/缺尺寸回退 3/4）；
 * - 宽度推导：visual = 壳高 × 冻结比例（clamp 最小宽/可用宽上限）；
 *   reading = 取满可用宽；壳宽 = 媒体列 + 固定右列 440，上限
 *   min(1320, 视口宽 − 2rem)。
 * ──────────────────────────────────────────────────────────────────────────── */

function slide(type: MediaSlideType, width?: number, height?: number): MediaGalleryItem {
  return { id: type.length * 100 + (width ?? 0), url: "u", type, width, height };
}

/* ── 档位判定 ─────────────────────────────────────────────────────────────── */

test("E1 档位判定：文档族 = reading 宽舞台，影像/音频 = visual", () => {
  assert.equal(slideTierOf(slide("document")), "reading", "document（docx/xlsx/csv）");
  assert.equal(slideTierOf(slide("text_pdf")), "reading", "text 族 pdf");
  assert.equal(slideTierOf(slide("sheet_music_xml")), "reading", "乐谱 musicxml");
  assert.equal(slideTierOf(slide("sheet_music_midi")), "reading", "乐谱 midi");
  assert.equal(slideTierOf(slide("sheet_music_pdf")), "reading", "乐谱族 pdf");
  assert.equal(slideTierOf(slide("model3d")), "reading", "3D 模型");
  assert.equal(slideTierOf(slide("image")), "visual", "图片");
  assert.equal(slideTierOf(slide("video")), "visual", "视频");
  assert.equal(slideTierOf(slide("audio")), "visual", "音频（占位图+播放条位，保持比例定宽）");
});

/* ── 内容冻结比例 ─────────────────────────────────────────────────────────── */

test("E1 内容冻结比例 = 首 slide intrinsic 比例；空链/缺尺寸回退防御比例", () => {
  assert.equal(contentFrozenRatio([slide("image", 900, 1200), slide("image", 1600, 900)]), 0.75, "首 slide 3:4 冻结，第二张比例不参与");
  assert.equal(contentFrozenRatio([slide("video", 1920, 1080)]), 1920 / 1080, "16:9 视频首项");
  assert.equal(contentFrozenRatio([slide("model3d", 300, 400)]), 0.75, "查看器族名义 3:4");
  assert.equal(contentFrozenRatio([slide("image")]), 0.75, "缺尺寸 → 3/4 防御比例");
  assert.equal(contentFrozenRatio([]), 0.75, "空链（loading/错误态）→ 3/4 占位");
});

/* ── 宽度推导 ─────────────────────────────────────────────────────────────── */

/* 基准夹具：视口 1440（上限 1320 生效）、壳高 800。
 * 可用宽 = 1320 − 440 = 880；3:4 → 600；1:1 → 800；16:9 → 1422→clamp 880。 */
const VIEWPORT_WIDE = 1440;
const SHELL_H = 800;

test("E1 visual 档：媒体列 = 壳高 × 冻结比例，壳宽 = 媒体列 + 固定右列", () => {
  const portrait = computeOverlayGeometry({ tier: "visual", frozenRatio: 0.75, shellHeight: SHELL_H, viewportWidth: VIEWPORT_WIDE });
  assert.equal(portrait.paneWidth, 600, "3:4 → 800×0.75");
  assert.equal(portrait.shellWidth, 600 + OVERLAY_RIGHT_COL_WIDTH, "壳宽 = 媒体列 + 440");

  const square = computeOverlayGeometry({ tier: "visual", frozenRatio: 1, shellHeight: SHELL_H, viewportWidth: VIEWPORT_WIDE });
  assert.equal(square.paneWidth, 800, "1:1 → 800");
  assert.equal(square.shellWidth, 800 + OVERLAY_RIGHT_COL_WIDTH);

  const wide = computeOverlayGeometry({ tier: "visual", frozenRatio: 9 / 16, shellHeight: SHELL_H, viewportWidth: VIEWPORT_WIDE });
  assert.equal(wide.paneWidth, 450, "9:16 → 450");
  assert.equal(wide.shellWidth, 450 + OVERLAY_RIGHT_COL_WIDTH);
});

test("E1 visual 档横图 clamp：超出可用宽收窄到上限（壳宽打满 1320）", () => {
  const landscape = computeOverlayGeometry({ tier: "visual", frozenRatio: 16 / 9, shellHeight: SHELL_H, viewportWidth: VIEWPORT_WIDE });
  assert.equal(landscape.paneWidth, 880, "16:9 → 1422 clamp 到可用宽 880");
  assert.equal(landscape.shellWidth, OVERLAY_SHELL_MAX_WIDTH, "壳宽 = 1320 上限");

  const ultraWide = computeOverlayGeometry({ tier: "visual", frozenRatio: 3, shellHeight: SHELL_H, viewportWidth: VIEWPORT_WIDE });
  assert.equal(ultraWide.paneWidth, 880, "任意横比例都 clamp 到同一可用宽");
  assert.equal(ultraWide.shellWidth, OVERLAY_SHELL_MAX_WIDTH);
});

test("E1 reading 档：开窗即宽舞台（媒体列取满可用宽），与 clamp 后的横图同宽", () => {
  const reading = computeOverlayGeometry({ tier: "reading", frozenRatio: 0.75, shellHeight: SHELL_H, viewportWidth: VIEWPORT_WIDE });
  assert.equal(reading.paneWidth, 880, "宽舞台 = 可用宽（冻结比例不参与）");
  assert.equal(reading.shellWidth, OVERLAY_SHELL_MAX_WIDTH, "壳宽打满 1320");
});

test("E1 媒体列最小宽度下限：窄长内容不塌缩", () => {
  const skinny = computeOverlayGeometry({ tier: "visual", frozenRatio: 0.1, shellHeight: SHELL_H, viewportWidth: VIEWPORT_WIDE });
  assert.equal(skinny.paneWidth, OVERLAY_PANE_MIN_WIDTH, "80 → 下限 280");
  assert.equal(skinny.shellWidth, 280 + OVERLAY_RIGHT_COL_WIDTH);
});

/* ── 视口上限 clamp ───────────────────────────────────────────────────────── */

test("E1 壳宽视口上限 = min(1320, 视口宽 − 2rem)", () => {
  assert.equal(overlayShellMaxWidth(1920), 1320, "大屏 → 1320 常数上限");
  assert.equal(overlayShellMaxWidth(1440), 1320, "1440 → 1408 > 1320 → 1320");
  assert.equal(overlayShellMaxWidth(1200), 1168, "1200 → 1168");
  assert.equal(overlayShellMaxWidth(1000), 968, "1000 → 968");
});

test("E1 中窄桌面视口：可用宽随之收缩，竖图也可能被 clamp", () => {
  /* 视口 1000：可用宽 = 968 − 440 = 528 < 600 → 3:4 clamp 到 528，壳宽 968。 */
  const clamped = computeOverlayGeometry({ tier: "visual", frozenRatio: 0.75, shellHeight: SHELL_H, viewportWidth: 1000 });
  assert.equal(clamped.paneWidth, 528);
  assert.equal(clamped.shellWidth, 968);

  /* 同视口下 9:16（450 < 528）不受影响——跨比例内容开窗宽度仍不同。 */
  const narrow = computeOverlayGeometry({ tier: "visual", frozenRatio: 9 / 16, shellHeight: SHELL_H, viewportWidth: 1000 });
  assert.equal(narrow.paneWidth, 450);
  assert.equal(narrow.shellWidth, 890);

  const readingNarrow = computeOverlayGeometry({ tier: "reading", frozenRatio: 0.75, shellHeight: SHELL_H, viewportWidth: 1000 });
  assert.equal(readingNarrow.paneWidth, 528, "宽舞台随视口收缩");
});

test("E1 极窄视口防御：可用宽保底媒体列最小宽度，壳宽不越过视口上限", () => {
  /* 视口 500（<960 移动路径，防御语义）：上限 468 < 280+440 → 壳宽取 468。 */
  const degenerate = computeOverlayGeometry({ tier: "reading", frozenRatio: 0.75, shellHeight: SHELL_H, viewportWidth: 500 });
  assert.equal(degenerate.paneWidth, OVERLAY_PANE_MIN_WIDTH, "可用宽保底 280");
  assert.equal(degenerate.shellWidth, 468, "壳宽 = min(280+440, 468) = 468");
});

test("E1 数值取整：亚像素壳高不产生小数像素宽", () => {
  const fractional = computeOverlayGeometry({ tier: "visual", frozenRatio: 0.75, shellHeight: 826.5, viewportWidth: VIEWPORT_WIDE });
  assert.equal(fractional.paneWidth, 620, "826.5×0.75 = 619.875 → 620");
  assert.equal(fractional.shellWidth, 1060);
});

test("E1 档位切换过渡常量：240ms SP-12 缓动（宽度布局动画仅值变化时触发）", () => {
  assert.equal(OVERLAY_GEOMETRY_TRANSITION, "width 240ms cubic-bezier(0.22,0.61,0.36,1)");
});
