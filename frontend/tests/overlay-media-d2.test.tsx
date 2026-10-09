import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import {
  MEDIA_READY_BUDGET_MS,
  probeVideoMetadataSize,
  useOverlayMedia,
} from "@/lib/overlay-media";
import type { NormalizedContentDetailResponse } from "@/lib/content";
import { act, cleanup, installDom, render, waitFor } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * D2 #859 媒体链类型正确性契约（spec §9.3）：
 * - 图片才走 Image 探针 + coverRenderSrc 规范变体 + 失败换占位图；
 * - 视频用附件正数尺寸或 video loadedmetadata 实测，绝不能把视频 URL 交给
 *   prefetchCoverVariant（图片优化器）或 Image 探针；
 * - 视频探针失败/超时保留真实视频 URL（不换占位图/SVG），仍可播放可重试；
 * - 共享有界就绪预算：须短于浮层入场保险（ENTRANCE_SAFETY_MS=2000），
 *   超时按稳定 fallback 完成当前入场。
 * 断言失败值绝不传 DOM 节点（node:test 序列化挂死），统一 Boolean()/字符串。
 * ──────────────────────────────────────────────────────────────────────────── */

test.afterEach(() => cleanup());
installDom();

interface FakeImage {
  src: string;
  decoding: string;
  naturalWidth: number;
  naturalHeight: number;
  decode: () => Promise<void>;
  onload: (() => void) | null;
  onerror: (() => void) | null;
  _src?: string;
}

/** 可控 Image 桩：记录实例与 src，测试手动触发 onload/onerror。 */
function installImageSpy() {
  const instances: FakeImage[] = [];
  const OrigImage = window.Image;
  function ImageSpy(this: FakeImage) {
    this.decoding = "";
    this.naturalWidth = 0;
    this.naturalHeight = 0;
    this.decode = () => Promise.resolve();
    this.onload = null;
    this.onerror = null;
    Object.defineProperty(this, "src", {
      set(value: string) {
        this._src = value;
        instances.push(this);
      },
      get(this: FakeImage & { _src?: string }) {
        return this._src ?? "";
      },
    });
  }
  (window as unknown as { Image: unknown }).Image = ImageSpy;
  return {
    instances,
    restore: () => {
      window.Image = OrigImage;
    },
  };
}

interface FakeVideo {
  preload: string;
  src: string;
  videoWidth: number;
  videoHeight: number;
  onloadedmetadata: (() => void) | null;
  onerror: (() => void) | null;
}

/** video 探针桩：拦截 document.createElement("video")，测试手动回报元数据/错误。 */
function installVideoProbeStub() {
  const created: FakeVideo[] = [];
  const originalCreate = document.createElement.bind(document);
  (document as unknown as { createElement: typeof document.createElement }).createElement = ((
    tag: string,
    options?: ElementCreationOptions,
  ) => {
    if (String(tag).toLowerCase() === "video") {
      const fake: FakeVideo = {
        preload: "",
        src: "",
        videoWidth: 0,
        videoHeight: 0,
        onloadedmetadata: null,
        onerror: null,
      };
      created.push(fake);
      return fake as unknown as HTMLVideoElement;
    }
    return originalCreate(tag, options);
  }) as typeof document.createElement;
  return {
    created,
    restore: () => {
      (document as unknown as { createElement: typeof document.createElement }).createElement =
        originalCreate;
    },
  };
}

function buildDetail(
  contentType: "image" | "video",
  attachment: {
    id: number;
    file_type: "image" | "video";
    oss_url: string;
    width?: number;
    height?: number;
  },
): NormalizedContentDetailResponse {
  return {
    content: {
      id: 900 + attachment.id,
      content_type: contentType,
      cover_image_url: "",
      cover_width: 0,
      cover_height: 0,
      title: "D2 media chain",
    },
    attachments: [attachment],
    tags: [],
    series_memberships: [],
  } as unknown as NormalizedContentDetailResponse;
}

function HookProbe({ detail }: { detail: NormalizedContentDetailResponse | null }) {
  const { media, ready } = useOverlayMedia(detail);
  return (
    <div
      data-testid="probe"
      data-ready={ready ? "true" : "false"}
      data-media={JSON.stringify(media)}
    />
  );
}

async function renderHook(detail: NormalizedContentDetailResponse) {
  const view = render(<HookProbe detail={detail} />);
  return view;
}

function probeState(): { ready: boolean; media: Array<Record<string, unknown>> } {
  const el = document.querySelector('[data-testid="probe"]');
  assert.ok(el, "hook probe element must exist");
  return {
    ready: el.getAttribute("data-ready") === "true",
    media: JSON.parse(el.getAttribute("data-media") ?? "[]") as Array<Record<string, unknown>>,
  };
}

test("D2 视频缺尺寸：走 video 探针（loadedmetadata 回报尺寸），绝不进 Image 探针/图片优化器", async () => {
  const imageSpy = installImageSpy();
  const videoStub = installVideoProbeStub();
  const videoUrl = "https://bucket.example/d2/video-no-dims-1009.mp4";
  try {
    const detail = buildDetail("video", { id: 601, file_type: "video", oss_url: videoUrl });
    await renderHook(detail);

    await waitFor(() => assert.ok(videoStub.created.length >= 1, "video probe must be created"));
    const probe = videoStub.created[0];
    assert.equal(probe.src, videoUrl, "video probe loads the raw video URL (no optimizer variant)");
    assert.equal(probe.preload, "metadata");
    assert.equal(
      imageSpy.instances.length,
      0,
      "video URL must never reach the Image probe",
    );

    probe.videoWidth = 1280;
    probe.videoHeight = 720;
    await act(async () => {
      probe.onloadedmetadata?.();
    });
    await waitFor(() => assert.ok(probeState().ready, "metadata reports readiness"));
    const { media } = probeState();
    assert.equal(media.length, 1);
    assert.equal(media[0].width, 1280);
    assert.equal(media[0].height, 720);
    assert.equal(media[0].url, videoUrl, "real video URL is preserved");
  } finally {
    imageSpy.restore();
    videoStub.restore();
  }
});

test("D2 视频探针失败：保留真实视频 URL（不换占位图）并照常就绪", async () => {
  const imageSpy = installImageSpy();
  const videoStub = installVideoProbeStub();
  const videoUrl = "https://bucket.example/d2/video-fail-1009.mp4";
  try {
    const detail = buildDetail("video", { id: 602, file_type: "video", oss_url: videoUrl });
    await renderHook(detail);
    await waitFor(() => assert.ok(videoStub.created.length >= 1));

    await act(async () => {
      videoStub.created[0].onerror?.();
    });
    await waitFor(() => assert.ok(probeState().ready, "failure still settles readiness"));
    const { media } = probeState();
    assert.equal(media.length, 1);
    assert.equal(media[0].url, videoUrl, "probe failure must keep the playable video URL");
    assert.equal(
      String(media[0].url ?? "").startsWith("data:"),
      false,
      "video URL must not be swapped for the SVG placeholder",
    );
    assert.equal(media[0].width, undefined, "no fabricated dimensions on failure");
    assert.equal(media[0].height, undefined, "no fabricated dimensions on failure");
    assert.equal(imageSpy.instances.length, 0, "no Image probe for videos");
  } finally {
    imageSpy.restore();
    videoStub.restore();
  }
});

test("D2 视频首项不发起 prefetchCoverVariant（图片优化器预热），尺寸齐全直接就绪", async () => {
  const imageSpy = installImageSpy();
  const videoStub = installVideoProbeStub();
  const videoUrl = "https://bucket.example/d2/video-dims-1009.mp4";
  try {
    const detail = buildDetail("video", {
      id: 603,
      file_type: "video",
      oss_url: videoUrl,
      width: 1920,
      height: 1080,
    });
    await renderHook(detail);

    await waitFor(() => assert.ok(probeState().ready, "known dims settle immediately"));
    assert.deepEqual(videoStub.created, [], "no probe needed when dims are known");
    assert.equal(
      imageSpy.instances.filter((img) => img.src.includes("_next/image")).length,
      0,
      "video first item must not be prefetched through the image optimizer",
    );
    const { media } = probeState();
    assert.equal(media[0].url, videoUrl);
    assert.equal(media[0].width, 1920);
  } finally {
    imageSpy.restore();
    videoStub.restore();
  }
});

test("D2 图片项保持既有 Image 探针契约：onload 补尺寸、onerror 换占位图（不回归）", async () => {
  const imageSpy = installImageSpy();
  const videoStub = installVideoProbeStub();
  try {
    const detail = buildDetail("image", {
      id: 604,
      file_type: "image",
      oss_url: "https://bucket.example/d2/image-no-dims-1009.jpg",
    });
    await renderHook(detail);

    await waitFor(() => assert.ok(imageSpy.instances.length >= 1, "image probe used for images"));
    assert.ok(
      imageSpy.instances[0].src.includes("_next/image"),
      "image probe still goes through the canonical variant",
    );
    assert.deepEqual(videoStub.created, [], "no video probe for images");

    /* instances[0] = prefetchCoverVariant 的预热 Image；尺寸探针 = 最后创建的实例。 */
    const probeImg = imageSpy.instances[imageSpy.instances.length - 1];
    probeImg.naturalWidth = 800;
    probeImg.naturalHeight = 600;
    await act(async () => {
      probeImg.onload?.();
    });
    await waitFor(() => assert.ok(probeState().ready));
    let media = probeState().media;
    assert.equal(media[0].width, 800);
    assert.equal(media[0].height, 600);
  } finally {
    imageSpy.restore();
    videoStub.restore();
  }

  cleanup();
  const imageSpy2 = installImageSpy();
  try {
    const detail = buildDetail("image", {
      id: 605,
      file_type: "image",
      oss_url: "https://bucket.example/d2/image-broken-1009.jpg",
    });
    await renderHook(detail);
    await waitFor(() => assert.ok(imageSpy2.instances.length >= 1));
    await act(async () => {
      imageSpy2.instances[imageSpy2.instances.length - 1].onerror?.();
    });
    await waitFor(() => assert.ok(probeState().ready));
    const media = probeState().media;
    assert.ok(
      String(media[0].url ?? "").startsWith("data:image/svg+xml"),
      "image probe failure still swaps in the text-cover placeholder (existing image contract)",
    );
  } finally {
    imageSpy2.restore();
  }
});

test("D2 共享有界就绪预算：视频探针超时按稳定 fallback 结清，预算短于入场保险", async () => {
  assert.ok(
    MEDIA_READY_BUDGET_MS > 0 && MEDIA_READY_BUDGET_MS <= 1500,
    `readiness budget (${MEDIA_READY_BUDGET_MS}ms) must be bounded and shorter than the 2000ms entrance safety`,
  );

  const began = Date.now();
  const result = await probeVideoMetadataSize("https://bucket.example/d2/video-hang-1009.mp4", 40);
  assert.equal(result.ok, false, "hanging metadata resolves as failure within the budget");
  assert.equal(result.width, undefined);
  assert.equal(result.height, undefined);
  assert.ok(Date.now() - began < 2000, "timeout path resolves promptly, never hangs the caller");
});

test("D2 视频探针回报非正尺寸时按失败处理（不产出坏几何）", async () => {
  const videoStub = installVideoProbeStub();
  try {
    const pending = probeVideoMetadataSize("https://bucket.example/d2/video-zero-1009.mp4", 500);
    await waitFor(() => assert.ok(videoStub.created.length >= 1));
    const probe = videoStub.created[0];
    probe.videoWidth = 0;
    probe.videoHeight = 0;
    await act(async () => {
      probe.onloadedmetadata?.();
    });
    const result = await pending;
    assert.equal(result.ok, false, "0x0 metadata is treated as failure, not as a usable ratio");
  } finally {
    videoStub.restore();
  }
});
