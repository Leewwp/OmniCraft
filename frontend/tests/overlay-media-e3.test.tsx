import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import {
  buildOverlayMedia,
  isEnlargeableSlide,
  overlayFallbackSlideFromAttachment,
  useOverlayMedia,
} from "@/lib/overlay-media";
import { contentFrozenRatio } from "@/lib/overlay-geometry";
import type { MediaGalleryItem } from "@/components/content/MediaGallery";
import type { AttachmentData, NormalizedContentDetailResponse } from "@/lib/content";
import { act, cleanup, installDom, render } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * E3 #955 媒体源装配优先级矩阵（spec 2026-11-07 四项裁决 3 + #955 验收）：
 * - 封面恒为首屏：有封面（cover_image_url 非空）→ 封面 slide 0 + 附件序列
 *   （可预览优先、不可预览托底，按附件稳定顺序混排）；视频/文档族内容同规；
 * - 封面=图片附件同一文件（图片内容封面常=首图）→ URL 归一化（剥签名
 *   query/hash）比对去重，保留封面位；
 * - 无封面 → 维持现状：附件序列优先 → 自动文字封面兜底；
 * - 不可预览附件（mscz/txt/mod 等）逐附件出 fallback_download 托底位；
 *   过扫描门且非 .mtl 才有托底；任何 default 态内容舞台非空；
 * - 封面缺尺寸（cover_width=0 全库现状）→ useOverlayMedia 图片探针实测
 *   intrinsic 后参与几何推导（contentFrozenRatio 消费首 slide）。
 * 断言失败值绝不传 DOM 节点（node:test 序列化挂死），统一 Boolean()/字符串。
 * ──────────────────────────────────────────────────────────────────────────── */

test.afterEach(() => cleanup());
installDom();

function attachment(partial: Partial<AttachmentData>): AttachmentData {
  return { id: 1, ...partial };
}

function detailOf(
  contentType: string,
  attachments: AttachmentData[],
  extra: Record<string, unknown> = {},
): NormalizedContentDetailResponse {
  return {
    content: {
      id: 9200,
      content_type: contentType,
      cover_image_url: "",
      cover_width: 0,
      cover_height: 0,
      title: "E3 assembly",
      ...extra,
    },
    attachments,
    tags: [],
    series_memberships: [],
  } as unknown as NormalizedContentDetailResponse;
}

/* ── 有封面：封面恒为首屏 ─────────────────────────────────────────────────── */

test("E3 有封面纯图集：封面 slide 0 + 图片序列；封面=首图同文件去重（保留封面位）", () => {
  const media = buildOverlayMedia(
    detailOf(
      "image",
      [
        attachment({ id: 11, file_type: "image", oss_url: "https://oss/first.jpg?Expires=1&Signature=A", width: 900, height: 1200 }),
        attachment({ id: 12, file_type: "image", oss_url: "https://oss/second.jpg?Expires=1&Signature=B", width: 800, height: 800 }),
      ],
      { cover_image_url: "https://oss/first.jpg?Expires=2&Signature=C", cover_width: 900, cover_height: 1200 },
    ),
  );
  assert.equal(media.length, 2, "cover=first-image dedup keeps one slide for the same file");
  assert.equal(media[0].type, "image");
  assert.equal(media[0].url, "https://oss/first.jpg?Expires=2&Signature=C", "slide 0 is the content cover itself");
  assert.ok(media[0].id < 0, "cover slide keeps the synthetic negative id convention");
  assert.equal(media[1].id, 12, "the duplicate first image drops, second image follows");
  assert.equal(media[1].url, "https://oss/second.jpg?Expires=1&Signature=B");
});

test("E3 封面与图片附件同文件（oss_key 兜底路径、本地形态）也去重", () => {
  const media = buildOverlayMedia(
    detailOf(
      "image",
      [
        attachment({ id: 21, file_type: "image", oss_key: "/seed/a.svg", width: 900, height: 1200 }),
        attachment({ id: 22, file_type: "image", oss_key: "/seed/b.svg", width: 900, height: 1200 }),
      ],
      { cover_image_url: "/seed/a.svg" },
    ),
  );
  assert.equal(media.length, 2);
  assert.ok(media[0].id < 0, "cover first");
  assert.equal(media[1].id, 22, "identical oss_key fallback path dedups too");
});

test("E3 有封面视频内容：封面 slide 0 + 视频正文（poster 语义不产生重复 slide）", () => {
  const media = buildOverlayMedia(
    detailOf(
      "video",
      [attachment({ id: 31, file_type: "video", oss_url: "https://oss/clip.mp4", width: 1280, height: 720 })],
      { cover_image_url: "https://oss/poster.jpg" },
    ),
  );
  assert.equal(media.length, 2, "video content with a cover: cover first, video body follows");
  assert.equal(media[0].type, "image");
  assert.equal(media[0].url, "https://oss/poster.jpg");
  assert.equal(media[1].type, "video");
  assert.equal(media[1].posterUrl, "https://oss/poster.jpg", "video poster keeps the cover (D2 contract)");
});

test("E3 有封面文档族内容：封面 slide 0 + 文档序列（封面比例自然 = 冻结比例）", () => {
  const media = buildOverlayMedia(
    detailOf(
      "template",
      [attachment({ id: 41, file_type: "document", oss_url: "https://oss/sheet.csv", scan_status: "clean" })],
      { cover_image_url: "https://oss/cover.jpg", cover_width: 600, cover_height: 800 },
    ),
  );
  assert.deepEqual(
    media.map((slide) => slide.type),
    ["image", "document"],
  );
  assert.equal(contentFrozenRatio(media), 600 / 800, "frozen ratio derives from the cover slide (E1 consumer)");
});

/* ── 无封面：维持现状（附件序列优先 → 文字封面） ─────────────────────────── */

test("E3 无封面：可预览附件优先（首 slide = 附件，非文字封面）", () => {
  const media = buildOverlayMedia(
    detailOf("image", [
      attachment({ id: 51, file_type: "image", oss_key: "/seed/x.svg", width: 900, height: 1200 }),
      attachment({ id: 52, file_type: "image", oss_key: "/seed/y.svg", width: 900, height: 1200 }),
    ]),
  );
  assert.equal(media.length, 2);
  assert.equal(media[0].id, 51, "no cover → previewable attachments first (status quo)");
});

test("E3 纯文字无附件无封面：自动文字封面兜底（舞台非空）", () => {
  const media = buildOverlayMedia(detailOf("article", []));
  assert.equal(media.length, 1);
  assert.equal(media[0].type, "image");
  assert.equal(media[0].autoCover, true);
});

/* ── 不可预览托底位 ──────────────────────────────────────────────────────── */

test("E3 纯不可预览附件（mscz + txt + mod，无封面）：逐附件托底位进链", () => {
  const media = buildOverlayMedia(
    detailOf("sheet_music", [
      attachment({ id: 61, file_type: "sheet_music_mscz", oss_url: "https://oss/s.mscz", original_file_name: "s.mscz", file_size: 4096, scan_status: "clean" }),
      attachment({ id: 62, file_type: "text", mime_type: "text/plain", oss_url: "https://oss/n.txt", original_file_name: "n.txt", file_size: 256, scan_status: "clean" }),
      attachment({ id: 63, file_type: "mod", oss_key: "uploads/m.zip", original_file_name: "m.zip", file_size: 1048576, scan_status: "not_required" }),
    ]),
  );
  assert.equal(media.length, 3, "every non-previewable attachment gets its own fallback slide");
  assert.deepEqual(
    media.map((slide) => slide.type),
    ["fallback_download", "fallback_download", "fallback_download"],
  );
  assert.deepEqual(media.map((slide) => slide.id), [61, 62, 63], "stable attachment ids anchor the download action");
  assert.equal(media[0].fileName, "s.mscz");
  assert.equal(media[0].fileSize, 4096);
  assert.equal(media[0].fallbackFamily, "sheet_music_mscz");
  assert.equal(media[0].url, "", "no cover → empty backdrop url (placeholder art at render time)");
  assert.equal(media[0].width, 300, "nominal 3:4 viewer geometry");
});

test("E3 混合链（无封面）：可预览与托底按附件稳定顺序混排", () => {
  const media = buildOverlayMedia(
    detailOf("other", [
      attachment({ id: 71, file_type: "image", oss_key: "/seed/a.svg", width: 900, height: 1200 }),
      attachment({ id: 72, file_type: "sheet_music_mscz", oss_url: "https://oss/s.mscz", original_file_name: "s.mscz", scan_status: "clean" }),
      attachment({ id: 73, file_type: "document", oss_url: "https://oss/b.csv", scan_status: "clean" }),
    ]),
  );
  assert.deepEqual(
    media.map((slide) => slide.type),
    ["image", "fallback_download", "document"],
    "frontend never reorders: attachment order is the carousel order",
  );
});

test("E3 混合链（有封面）：封面 0 + 混排附件序列，任何内容打开舞台非空", () => {
  const media = buildOverlayMedia(
    detailOf("mod", [
      attachment({ id: 81, file_type: "mod", oss_url: "https://oss/m.zip", original_file_name: "m.zip", scan_status: "clean" }),
    ], { cover_image_url: "https://oss/mod-cover.jpg" }),
  );
  assert.deepEqual(
    media.map((slide) => slide.type),
    ["image", "fallback_download"],
  );
  assert.equal(media[1].url, "https://oss/mod-cover.jpg", "fallback backdrop reuses the content cover");
});

test("E3 托底资格拒绝：扫描门拦截与 .mtl 伴生文件不产托底 slide", () => {
  assert.equal(
    overlayFallbackSlideFromAttachment(attachment({ id: 91, file_type: "sheet_music_mscz", oss_url: "https://oss/x.mscz", scan_status: "blocked" })),
    null,
    "scan-blocked rows keep the right-rail scan card, not a stage slide",
  );
  assert.equal(
    overlayFallbackSlideFromAttachment(attachment({ id: 92, file_type: "model3d", oss_url: "https://oss/x.mtl", original_file_name: "x.mtl", scan_status: "clean" })),
    null,
    ".mtl companion files stay out of the carousel",
  );
  const media = buildOverlayMedia(
    detailOf("3d_print", [
      attachment({ id: 91, file_type: "sheet_music_mscz", oss_url: "https://oss/x.mscz", scan_status: "blocked" }),
      attachment({ id: 92, file_type: "model3d", oss_url: "https://oss/x.mtl", original_file_name: "x.mtl", scan_status: "clean" }),
    ]),
  );
  assert.equal(media.length, 1, "only the auto text cover remains — still a non-empty stage");
  assert.equal(media[0].autoCover, true);
});

/* ── 放大语义：封面可放大、托底位不可放大 ─────────────────────────────────── */

test("E3 放大谓词：封面 slide 可放大（真图）；托底位不可放大（属性卡非媒体）", () => {
  const withCover = buildOverlayMedia(
    detailOf("mod", [attachment({ id: 93, file_type: "mod", oss_url: "https://oss/m.zip", scan_status: "clean" })], { cover_image_url: "https://oss/c.jpg" }),
  );
  assert.equal(isEnlargeableSlide(withCover[0]), true, "the real content cover stays enlargeable");
  assert.equal(isEnlargeableSlide(withCover[1]), false, "the fallback property card has no zoom entry");
});

/* ── 封面 intrinsic 实测接链（cover_width=0 全库现状） ───────────────────── */

function HookProbe({ detail }: { detail: NormalizedContentDetailResponse | null }) {
  const { media, ready } = useOverlayMedia(detail);
  return <div data-testid="probe" data-ready={ready ? "true" : "false"} data-media={JSON.stringify(media)} />;
}

test("E3 封面缺尺寸：useOverlayMedia 图片探针实测 intrinsic 并 patch（几何链路打通）", async () => {
  const imageInstances: Array<{ src: string; naturalWidth: number; naturalHeight: number; onload: (() => void) | null }> = [];
  const OrigImage = window.Image;
  (window as unknown as { Image: unknown }).Image = function ImageSpy(
    this: { src: string; naturalWidth: number; naturalHeight: number; onload: (() => void) | null },
  ) {
    this.naturalWidth = 0;
    this.naturalHeight = 0;
    this.onload = null;
    Object.defineProperty(this, "src", {
      set: (value: string) => {
        (this as { _src?: string })._src = value;
        imageInstances.push(this as unknown as typeof imageInstances[number]);
      },
      get: () => (this as { _src?: string })._src ?? "",
    });
  };
  try {
    const detail = detailOf(
      "template",
      [attachment({ id: 94, file_type: "document", oss_url: "https://oss/s.csv", scan_status: "clean" })],
      { cover_image_url: "https://oss/e3-cover.jpg", cover_width: 0, cover_height: 0 },
    );
    render(<HookProbe detail={detail} />);
    await act(async () => {
      await Promise.resolve();
    });
    /* 探针走规范变体（coverRenderSrc）——封面进 pendingImages 后实测链路生效。 */
    const probeImg = imageInstances[imageInstances.length - 1];
    assert.ok(probeImg, "cover without dims enters the image probe pipeline");
    assert.ok(probeImg.src.includes("_next/image"), `probe goes through the canonical variant (got ${probeImg.src})`);
    probeImg.naturalWidth = 1200;
    probeImg.naturalHeight = 500;
    await act(async () => {
      probeImg.onload?.();
    });
    const probe = document.querySelector('[data-testid="probe"]');
    assert.ok(probe);
    assert.equal(probe.getAttribute("data-ready"), "true", "probed cover settles readiness");
    const media = JSON.parse(probe.getAttribute("data-media") ?? "[]") as Array<Record<string, unknown>>;
    assert.equal(media.length, 2);
    assert.equal(media[0].width, 1200, "intrinsic width patched onto the cover slide");
    assert.equal(media[0].height, 500);
    assert.equal(
      contentFrozenRatio(media as unknown as MediaGalleryItem[]),
      1200 / 500,
      "geometry derives from the measured cover ratio",
    );
  } finally {
    window.Image = OrigImage;
  }
});

test("E3 托底位不进探针集合（无可信几何可实测，挂载即就绪）", async () => {
  const imageInstances: unknown[] = [];
  const OrigImage = window.Image;
  (window as unknown as { Image: unknown }).Image = function ImageSpy(this: unknown) {
    imageInstances.push(this);
  };
  try {
    const detail = detailOf("sheet_music", [
      attachment({ id: 95, file_type: "sheet_music_mscz", oss_url: "https://oss/s.mscz", scan_status: "clean" }),
    ]);
    render(<HookProbe detail={detail} />);
    await act(async () => {
      await Promise.resolve();
    });
    const probe = document.querySelector('[data-testid="probe"]');
    assert.ok(probe);
    assert.equal(probe.getAttribute("data-ready"), "true", "fallback-only chains settle immediately");
    assert.equal(imageInstances.length, 0, "no Image probe for fallback slides");
  } finally {
    window.Image = OrigImage;
  }
});
