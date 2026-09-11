import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { useOverlayMedia } from "@/lib/overlay-media";
import type { NormalizedContentDetailResponse } from "@/lib/content";
import { cleanup, installDom, render, waitFor } from "./runtime-test-helpers";

test.afterEach(() => cleanup());
installDom();

/* #409 F1 同源预热：detail 落定瞬间媒体链首项的规范变体（w=1080）须发起预取
   （prefetchCoverVariant）——展示 URL 逐响应签名轮换 + 卡片封面与媒体集常为
   不同文件，落定后 holdSrc→真实媒体的切换若不预热必然撞冷缓存。 */
function installImageSpy() {
  const srcs: string[] = [];
  const OrigImage = window.Image;
  function ImageSpy(this: { src: string; decoding: string; decode: () => Promise<void> }) {
    this.decoding = "";
    Object.defineProperty(this, "src", {
      set(value: string) {
        srcs.push(value);
      },
      get() {
        return srcs[srcs.length - 1] ?? "";
      },
    });
    this.decode = () => Promise.resolve();
  }
  (window as unknown as { Image: unknown }).Image = ImageSpy;
  return { srcs, restore: () => (window.Image = OrigImage) };
}

function buildDetail(attachmentUrl: string, width: number, height: number): NormalizedContentDetailResponse {
  return {
    content: {
      id: 103,
      content_type: "image",
      cover_image_url: "",
      cover_width: 0,
      cover_height: 0,
      title: "t",
    },
    attachments: [
      {
        id: 501,
        file_type: "image",
        oss_url: attachmentUrl,
        width,
        height,
      },
    ],
    tags: [],
    series_memberships: [],
  } as unknown as NormalizedContentDetailResponse;
}

test("useOverlayMedia 预取首项媒体的规范变体（尺寸齐全也预取）", async () => {
  const spy = installImageSpy();
  try {
    const detail = buildDetail("https://bucket.example/prefetch-bitmap-0909.jpg", 800, 600);
    const Probe = () => {
      useOverlayMedia(detail);
      return null;
    };
    render(<Probe />);
    await waitFor(() => assert.ok(spy.srcs.length >= 1));
    assert.ok(
      spy.srcs.some(
        (src) =>
          src.includes("w=1080") &&
          src.includes(encodeURIComponent("https://bucket.example/prefetch-bitmap-0909.jpg")),
      ),
      `expected canonical variant prefetch, got: ${JSON.stringify(spy.srcs)}`,
    );
  } finally {
    spy.restore();
  }
});

test("useOverlayMedia 首项为 SVG 直通源时不发起预取", async () => {
  const spy = installImageSpy();
  try {
    const detail = buildDetail("https://bucket.example/prefetch-pass-0909.svg", 300, 400);
    const Probe = () => {
      useOverlayMedia(detail);
      return null;
    };
    render(<Probe />);
    await waitFor(() => assert.ok(true));
    assert.deepEqual(spy.srcs, []);
  } finally {
    spy.restore();
  }
});
