import test from "node:test";
import assert from "node:assert/strict";

import { buildOverlayMedia, isEnlargeableSlide } from "@/lib/overlay-media";
import type { MediaGalleryItem } from "@/components/content/MediaGallery";
import type { AttachmentData, NormalizedContentDetailResponse } from "@/lib/content";

/* ────────────────────────────────────────────────────────────────────────────
 * D6 #863 可放大 slide 谓词（spec §3.2 放大态行 / §9.6 末两段）：
 * 可放大序列仅三类 = 真实图片（image，不含 autoCover 标记的合成文字封面）、
 * model3d、text 族 pdf（text_pdf）。video/audio/document/乐谱族（xml/midi/pdf）
 * 一律无放大入口；自动文字封面/音频背景封面/video poster 不算独立图片附件
 * （后两者不是 slide 链成员，由 slide 分发层保证；自动文字封面会作为无可
 * 预览项时的唯一 fallback image slide 进链，按 autoCover 标记在谓词层排除）。
 * ──────────────────────────────────────────────────────────────────────────── */

function item(id: number, type: MediaGalleryItem["type"]): MediaGalleryItem {
  return { id, url: `https://bucket.example/d6/${type}-${id}`, type, width: 300, height: 400 };
}

function coverDetail(extra: Record<string, unknown> = {}): NormalizedContentDetailResponse {
  return {
    content: {
      id: 9100,
      content_type: "article",
      cover_image_url: "",
      cover_width: 0,
      cover_height: 0,
      title: "D6 cover chain",
      ...extra,
    },
    attachments: [] as AttachmentData[],
    tags: [],
    series_memberships: [],
  } as unknown as NormalizedContentDetailResponse;
}

test("D6 可放大谓词：image / model3d / text_pdf 为真", () => {
  assert.equal(isEnlargeableSlide(item(1, "image")), true);
  assert.equal(isEnlargeableSlide(item(2, "model3d")), true);
  assert.equal(isEnlargeableSlide(item(3, "text_pdf")), true);
});

test("D6 可放大谓词：video/audio/document/乐谱族一律为假（无放大入口）", () => {
  assert.equal(isEnlargeableSlide(item(4, "video")), false);
  assert.equal(isEnlargeableSlide(item(5, "audio")), false);
  assert.equal(isEnlargeableSlide(item(6, "document")), false);
  assert.equal(isEnlargeableSlide(item(7, "sheet_music_xml")), false);
  assert.equal(isEnlargeableSlide(item(8, "sheet_music_midi")), false);
  assert.equal(isEnlargeableSlide(item(9, "sheet_music_pdf")), false, "乐谱族 pdf 仍属乐谱位，不放大");
});

test("D6 可放大谓词：自动文字封面（autoCover 标记）为假——不算独立图片附件（spec §3.2 article 行 = —）", () => {
  assert.equal(
    isEnlargeableSlide({ ...item(11, "image"), autoCover: true }),
    false,
    "synthesized auto text cover must not be enlargeable",
  );
  /* 谓词按标记区分，不依赖 id 算术：同为 image 类型、无标记（真图/真封面）仍可放大。 */
  assert.equal(isEnlargeableSlide(item(12, "image")), true);
});

test("D6 buildOverlayMedia：自动文字封面带 autoCover 标记、真内容封面不带（spec §9.6）", () => {
  /* 无可预览 slide 且无 cover_image_url → 合成自动文字封面（渐变字牌）。 */
  const auto = buildOverlayMedia(coverDetail());
  assert.equal(auto.length, 1);
  assert.equal(auto[0].type, "image");
  assert.equal(auto[0].autoCover, true, "auto text cover slide carries the autoCover flag");

  /* 有 cover_image_url → 真内容封面（真实图片，保持可放大）。 */
  const real = buildOverlayMedia(coverDetail({ cover_image_url: "https://bucket.example/d6/real-cover.jpg" }));
  assert.equal(real.length, 1);
  assert.equal(real[0].type, "image");
  assert.notEqual(real[0].autoCover, true, "real content cover is not flagged and stays enlargeable");
});
