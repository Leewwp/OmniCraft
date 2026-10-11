import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import {
  buildOverlayMedia,
  selectOverlaySlides,
  useOverlayMedia,
} from "@/lib/overlay-media";
import type { AttachmentData, NormalizedContentDetailResponse } from "@/lib/content";
import { act, cleanup, installDom, render } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * D5 #862 可预览媒体链契约（spec §9.6 / §3.2）：
 * - 浮层媒体链从 image/video 扩展为逐附件 slide 序列：audio/model3d/document
 *   （需签名 oss_url 且扫描门 clean/not_required）、text 仅持久化 MIME=
 *   application/pdf、sheet_music 逐附件（musicxml/midi/pdf；mscz/mscx 不进）；
 *   .mtl 不进；mod 不进（固定包裹占位）。
 * - 排序 = 后端稳定顺序（sort_order ASC NULLS LAST, id ASC），前端不重排、0 有效。
 * - 回退链不变：无可预览项 → 内容封面 → 自动文字封面。
 * - selectMediaItems 对独立详情页的既有契约不回归（见 media-gallery 既有测试）。
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
      id: 9100,
      content_type: contentType,
      cover_image_url: "",
      cover_width: 0,
      cover_height: 0,
      title: "D5 slide chain",
      ...extra,
    },
    attachments,
    tags: [],
    series_memberships: [],
  } as unknown as NormalizedContentDetailResponse;
}

/* ── 链构建：逐附件资格与 slide 形态 ─────────────────────────────────────── */

test("D5 逐附件 slide 构建：audio/model3d/document/text-pdf/sheet_music 均进链且保留附件顺序", () => {
  const attachments = [
    attachment({ id: 11, file_type: "audio", oss_url: "https://oss/audio.wav", original_file_name: "narration.wav", file_size: 2048, scan_status: "clean" }),
    attachment({ id: 12, file_type: "model3d", oss_url: "https://oss/bracket.stl", original_file_name: "bracket.stl", file_size: 4096, scan_status: "not_required" }),
    attachment({ id: 13, file_type: "document", oss_url: "https://oss/sheet.csv", original_file_name: "sheet.csv", file_size: 512, scan_status: "clean" }),
    attachment({ id: 14, file_type: "text", mime_type: "application/pdf", oss_url: "https://oss/readme.pdf", original_file_name: "readme.pdf", scan_status: "clean" }),
    attachment({ id: 15, file_type: "sheet_music_xml", oss_url: "https://oss/score.musicxml", original_file_name: "score.musicxml", scan_status: "clean" }),
    attachment({ id: 16, file_type: "sheet_music_midi", oss_url: "https://oss/score.mid", original_file_name: "score.mid", scan_status: "clean" }),
    attachment({ id: 17, file_type: "sheet_music_pdf", oss_url: "https://oss/score.pdf", original_file_name: "score.pdf", scan_status: "clean" }),
  ];
  /* 输入顺序即后端 sort_order/id 顺序（含 sort_order=0 起步）：前端不得重排。 */
  const slides = selectOverlaySlides(attachments);
  assert.deepEqual(
    slides.map((slide) => slide.type),
    ["audio", "model3d", "document", "text_pdf", "sheet_music_xml", "sheet_music_midi", "sheet_music_pdf"],
  );
  assert.deepEqual(
    slides.map((slide) => slide.id),
    [11, 12, 13, 14, 15, 16, 17],
    "slide keeps the stable attachment id",
  );
  assert.equal(slides[0].url, "https://oss/audio.wav");
  assert.equal(slides[0].fileName, "narration.wav");
  assert.equal(slides[0].fileSize, 2048);
  /* 新类型名义几何（无 intrinsic 宽高）：300×400 = 3:4。 */
  assert.equal(slides[0].width, 300);
  assert.equal(slides[0].height, 400);
});

test("D5 混排链：任意内容类型的 image/video 附件沿用既有契约进链（oss_key 兜底保留）", () => {
  const slides = selectOverlaySlides([
    attachment({ id: 21, file_type: "image", oss_key: "/seed/a.svg", width: 900, height: 1200, sort_order: 0 }),
    attachment({ id: 22, file_type: "document", oss_url: "https://oss/b.csv", scan_status: "clean" }),
    attachment({ id: 23, file_type: "video", oss_url: "https://oss/c.mp4", width: 1280, height: 720 }),
  ]);
  assert.deepEqual(
    slides.map((slide) => slide.type),
    ["image", "document", "video"],
    "attachment-family contents can mix image/video/previewable attachments in one chain",
  );
  assert.equal(slides[0].url, "/seed/a.svg", "image keeps the legacy oss_key fallback");
  assert.equal(slides[0].width, 900);
});

test("D5 资格拒绝：缺 oss_url / 扫描门拦截 / .mtl / mscz·mscx / text 非 pdf MIME / mod 均不进链", () => {
  const slides = selectOverlaySlides([
    attachment({ id: 31, file_type: "audio", oss_key: "/x/a.wav", scan_status: "clean" }),
    attachment({ id: 32, file_type: "document", oss_url: "https://oss/x.docx", scan_status: "pending" }),
    attachment({ id: 33, file_type: "model3d", oss_url: "https://oss/x.mtl", original_file_name: "x.mtl", scan_status: "clean" }),
    attachment({ id: 34, file_type: "sheet_music_mscz", oss_url: "https://oss/x.mscz", scan_status: "clean" }),
    attachment({ id: 35, file_type: "sheet_music_mscx", oss_url: "https://oss/x.mscx", scan_status: "clean" }),
    attachment({ id: 36, file_type: "text", mime_type: "text/plain", oss_url: "https://oss/x.txt", scan_status: "clean" }),
    attachment({ id: 37, file_type: "text", oss_url: "https://oss/x2.pdf", original_file_name: "x2.pdf", scan_status: "clean" }),
    attachment({ id: 38, file_type: "mod", oss_url: "https://oss/x.zip", scan_status: "clean" }),
  ]);
  assert.deepEqual(slides, [], "none of the ineligible attachments may become a slide");
});

test("D5 text 族仅持久化 MIME 精确等于 application/pdf（带参数/大小写归一，文件名不算数）", () => {
  const withParams = selectOverlaySlides([
    attachment({ id: 41, file_type: "text", mime_type: "application/pdf; charset=utf-8", oss_url: "https://oss/a.pdf", scan_status: "clean" }),
  ]);
  assert.equal(withParams.length, 1, "media type parameters are tolerated (parsed type equality)");

  const upperMime = selectOverlaySlides([
    attachment({ id: 42, file_type: "text", mime_type: "Application/PDF", oss_url: "https://oss/b.pdf", scan_status: "clean" }),
  ]);
  assert.equal(upperMime.length, 1, "media type comparison is case-insensitive");

  const pdfNameOnly = selectOverlaySlides([
    attachment({ id: 43, file_type: "text", mime_type: "application/octet-stream", oss_url: "https://oss/c.pdf", original_file_name: "c.pdf", scan_status: "clean" }),
  ]);
  assert.deepEqual(pdfNameOnly, [], ".pdf file name with a non-pdf persisted MIME never becomes a slide");
});

test("D5 sheet_music 子类型分派：file_type 优先、MIME 兜底（audio/midi、musicxml、application/pdf）", () => {
  const slides = selectOverlaySlides([
    attachment({ id: 51, file_type: "sheet_music_midi", oss_url: "https://oss/m1.mid", scan_status: "clean" }),
    attachment({ id: 52, file_type: "", mime_type: "audio/midi", oss_url: "https://oss/m2.mid", scan_status: "clean" }),
    attachment({ id: 53, file_type: "", mime_type: "application/vnd.recordare.musicxml+xml", oss_url: "https://oss/m3.musicxml", scan_status: "clean" }),
    attachment({ id: 54, file_type: "sheet_music", mime_type: "application/pdf", oss_url: "https://oss/m4.pdf", scan_status: "clean" }),
  ]);
  assert.deepEqual(
    slides.map((slide) => slide.type),
    ["sheet_music_midi", "sheet_music_midi", "sheet_music_xml", "sheet_music_pdf"],
  );
});

/* ── 回退链不回归 ────────────────────────────────────────────────────────── */

test("D5 回退链不变：无可预览附件 → 内容封面；无封面 → 自动文字封面；mod 保持包裹占位", () => {
  const withCover = buildOverlayMedia(
    detailOf("template", [attachment({ id: 61, file_type: "text", mime_type: "text/plain", oss_url: "https://oss/x.txt", scan_status: "clean" })], {
      cover_image_url: "/seed/cover.svg",
      cover_width: 600,
      cover_height: 800,
    }),
  );
  assert.equal(withCover.length, 1);
  assert.equal(withCover[0].type, "image");
  assert.equal(withCover[0].url, "/seed/cover.svg");
  assert.ok(withCover[0].id < 0, "cover fallback keeps the synthetic negative id");

  const textCover = buildOverlayMedia(detailOf("article", []));
  assert.equal(textCover.length, 1);
  assert.equal(textCover[0].type, "image");
  assert.ok(String(textCover[0].url).startsWith("data:image/svg+xml"), "text cover placeholder chain feeds the pane");

  const modChain = buildOverlayMedia(detailOf("mod", [attachment({ id: 62, file_type: "mod", oss_url: "https://oss/m.zip", scan_status: "clean" })]));
  assert.equal(modChain.length, 1, "mod never joins the carousel (fixed package placeholder card)");
  assert.ok(String(modChain[0].url).startsWith("data:image/svg+xml"));
});

test("D5 buildOverlayMedia：附件族内容首选项 = slide 链（不再直接落文字封面）", () => {
  const media = buildOverlayMedia(
    detailOf("audio", [attachment({ id: 71, file_type: "audio", oss_url: "https://oss/a.wav", original_file_name: "a.wav", scan_status: "clean" })], {
      cover_image_url: "/seed/cover.svg",
    }),
  );
  assert.equal(media.length, 1);
  assert.equal(media[0].type, "audio", "the signed audio attachment outranks the content cover");
});

/* ── useOverlayMedia：新类型不进探针集合，立即可就绪 ─────────────────────── */

function HookProbe({ detail }: { detail: NormalizedContentDetailResponse | null }) {
  const { media, ready } = useOverlayMedia(detail);
  return <div data-testid="probe" data-ready={ready ? "true" : "false"} data-media={JSON.stringify(media)} />;
}

test("D5 新类型 slide 链不做任何尺寸探针且立即 ready（首屏不抓附件正文）", async () => {
  const imageInstances: unknown[] = [];
  const OrigImage = window.Image;
  (window as unknown as { Image: unknown }).Image = function ImageSpy(this: unknown) {
    imageInstances.push(this);
  };
  const originalCreate = document.createElement.bind(document);
  let createdVideos = 0;
  (document as unknown as { createElement: typeof document.createElement }).createElement = ((
    tag: string,
    options?: ElementCreationOptions,
  ) => {
    if (String(tag).toLowerCase() === "video") createdVideos += 1;
    return originalCreate(tag, options);
  }) as typeof document.createElement;
  try {
    const detail = detailOf("sheet_music", [
      attachment({ id: 81, file_type: "sheet_music_xml", oss_url: "https://oss/s.musicxml", scan_status: "clean" }),
      attachment({ id: 82, file_type: "sheet_music_midi", oss_url: "https://oss/s.mid", scan_status: "clean" }),
    ]);
    render(<HookProbe detail={detail} />);
    await act(async () => {
      await Promise.resolve();
    });
    const probe = document.querySelector('[data-testid="probe"]');
    assert.ok(probe, "hook probe renders");
    assert.equal(probe.getAttribute("data-ready"), "true", "new-type chains settle immediately");
    assert.equal(imageInstances.length, 0, "no Image probe for viewer slides");
    assert.equal(createdVideos, 0, "no video probe for viewer slides");
    const media = JSON.parse(probe.getAttribute("data-media") ?? "[]") as Array<Record<string, unknown>>;
    assert.equal(media.length, 2);
    assert.equal(media[0].type, "sheet_music_xml");
  } finally {
    window.Image = OrigImage;
    (document as unknown as { createElement: typeof document.createElement }).createElement = originalCreate;
  }
});
