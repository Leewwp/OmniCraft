import assert from "node:assert/strict";
import test from "node:test";
import React from "react";

import { cleanup, fireEvent, installDom, renderWithIntl, waitFor } from "./runtime-test-helpers";

import type { UploadItem, UploadedAsset } from "@/components/content/FileUploader";

test.afterEach(() => {
  cleanup();
});

/**
 * Minimal XMLHttpRequest mock that simulates a successful PUT upload.
 */
function installMockXHR(status = 200) {
  const OriginalXHR = globalThis.XMLHttpRequest;

  class MockXHR {
    private _method = "";
    private _url = "";
    public upload: { onprogress: ((ev: ProgressEvent) => void) | null } = { onprogress: null };
    public status = status;
    public onload: (() => void) | null = null;
    public onerror: (() => void) | null = null;

    open(method: string, url: string) {
      this._method = method;
      this._url = url;
    }

    setRequestHeader() {
      // no-op
    }

    send() {
      // Simulate async upload completion on next tick
      setTimeout(() => {
        if (this.onload) this.onload();
      }, 0);
    }
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (globalThis as any).XMLHttpRequest = MockXHR;

  return function restore() {
    (globalThis as unknown as { XMLHttpRequest: typeof OriginalXHR }).XMLHttpRequest = OriginalXHR;
  };
}

test("FileUploader stores grant_id from upload token response and passes it through onUploaded", async () => {
  installDom();

  const apiModule = await import("@/lib/api");
  const uploaderModule = await import("@/components/content/FileUploader");
  const FileUploader = uploaderModule.FileUploader;

  const originalPost = apiModule.api.post;
  const uploadedAssets: UploadedAsset[] = [];

  apiModule.api.post = (async (path: string, _body: unknown) => {
    if (path === "/api/v1/contents/oss-token") {
      return {
        upload_url: "https://uploads.example/test.png",
        oss_key: "uploads/user42/test.png",
        grant_id: "grant-abc-123",
        expires_in: 3600,
      };
    }
    throw new Error(`unexpected api.post path ${path}`);
  }) as typeof apiModule.api.post;

  const restoreXHR = installMockXHR();

  try {
    const view = renderWithIntl(
      <FileUploader
        fileType="image"
        maxMB={20}
        accept="image/*"
        onUploaded={(files) => { uploadedAssets.push(...files); }}
      />,
    );

    const fileInput = view.container.querySelector('input[type="file"]') as HTMLInputElement;
    assert.ok(fileInput, "file input should exist");

    const file = new File(["fake-image"], "test.png", { type: "image/png" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    await waitFor(() => assert.equal(uploadedAssets.length, 1, "onUploaded should be called with one asset"));

    assert.equal(uploadedAssets[0].grantId, "grant-abc-123", "grantId should match token response grant_id");
    assert.equal(uploadedAssets[0].ossKey, "uploads/user42/test.png", "ossKey should match token response");
    assert.equal(uploadedAssets[0].fileName, "test.png", "fileName should match uploaded file name");
  } finally {
    apiModule.api.post = originalPost;
    restoreXHR();
  }
});

test("FileUploader handles missing grant_id gracefully (grantId is undefined)", async () => {
  installDom();

  const apiModule = await import("@/lib/api");
  const uploaderModule = await import("@/components/content/FileUploader");
  const FileUploader = uploaderModule.FileUploader;

  const originalPost = apiModule.api.post;
  const uploadedAssets: UploadedAsset[] = [];

  apiModule.api.post = (async (path: string, _body: unknown) => {
    if (path === "/api/v1/contents/oss-token") {
      return {
        upload_url: "https://uploads.example/test.png",
        oss_key: "uploads/user42/test.png",
        // grant_id intentionally omitted
        expires_in: 3600,
      };
    }
    throw new Error(`unexpected api.post path ${path}`);
  }) as typeof apiModule.api.post;

  const restoreXHR = installMockXHR();

  try {
    const view = renderWithIntl(
      <FileUploader
        fileType="image"
        maxMB={20}
        accept="image/*"
        onUploaded={(files) => { uploadedAssets.push(...files); }}
      />,
    );

    const fileInput = view.container.querySelector('input[type="file"]') as HTMLInputElement;
    assert.ok(fileInput, "file input should exist");

    const file = new File(["fake-image"], "test.png", { type: "image/png" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    await waitFor(() => assert.equal(uploadedAssets.length, 1, "onUploaded should still be called"));

    assert.equal(uploadedAssets[0].grantId, undefined, "grantId should be undefined when missing from token response");
    assert.equal(uploadedAssets[0].ossKey, "uploads/user42/test.png", "ossKey should still be populated");
  } finally {
    apiModule.api.post = originalPost;
    restoreXHR();
  }
});

function createDoneMediaItem(id: string, sortOrder: number): UploadItem {
  return {
    id,
    file: new File([id], `${id}.png`, { type: "image/png" }),
    type: "image",
    sortOrder,
    status: "done",
    previewUrl: `https://cdn.example/${id}.png`,
    fileName: `${id}.png`,
    fileType: "image",
    mimeType: "image/png",
    ossKey: `uploads/${id}.png`,
    grantId: `grant-${id}`,
    fileSize: 1,
    width: 100,
    height: 100,
  };
}

test("media FileUploader exposes keyboard reorder and remove actions with stable sort order", async () => {
  installDom();

  const uploaderModule = await import("@/components/content/FileUploader");
  const changes: UploadItem[][] = [];
  const view = renderWithIntl(
    <uploaderModule.FileUploader
      mode="media-gallery"
      contentType="image"
      minCount={2}
      maxCount={9}
      value={[createDoneMediaItem("a", 0), createDoneMediaItem("b", 1)]}
      onChange={(items) => changes.push(items)}
    />,
  );

  fireEvent.click(view.getByRole("button", { name: "Move a.png down" }));
  assert.deepEqual(changes.at(-1)?.map((item) => [item.fileName, item.sortOrder]), [["b.png", 0], ["a.png", 1]]);

  fireEvent.click(view.getByRole("button", { name: "Remove b.png" }));
  assert.deepEqual(changes.at(-1)?.map((item) => [item.fileName, item.sortOrder]), [["a.png", 0]]);
});

test("media FileUploader rejects a selection that exceeds the public count contract", async () => {
  installDom();

  const uploaderModule = await import("@/components/content/FileUploader");
  const view = renderWithIntl(
    <uploaderModule.FileUploader mode="media-gallery" contentType="image" minCount={2} maxCount={2} />,
  );
  const fileInput = view.container.querySelector('input[type="file"]') as HTMLInputElement;
  assert.ok(fileInput, "media file input should exist");

  fireEvent.change(fileInput, {
    target: {
      files: [
        new File(["one"], "one.png", { type: "image/png" }),
        new File(["two"], "two.png", { type: "image/png" }),
        new File(["three"], "three.png", { type: "image/png" }),
      ],
    },
  });

  await waitFor(() => assert.match(view.container.textContent ?? "", /up to 2 media files/));
});

test("media FileUploader localizes client-side validation errors", async () => {
  installDom();

  const uploaderModule = await import("@/components/content/FileUploader");
  const view = renderWithIntl(
    <uploaderModule.FileUploader mode="media-gallery" contentType="video" minCount={1} maxCount={3} />,
  );
  const fileInput = view.container.querySelector('input[type="file"]') as HTMLInputElement;
  assert.ok(fileInput, "media file input should exist");

  fireEvent.change(fileInput, {
    target: { files: [new File(["image"], "still.png", { type: "image/png" })] },
  });

  await waitFor(() => assert.match(view.container.textContent ?? "", /Choose a file matching the content type/));
  assert.doesNotMatch(view.container.textContent ?? "", /wrongType/);
});
