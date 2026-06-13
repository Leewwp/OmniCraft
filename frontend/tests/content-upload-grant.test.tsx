import assert from "node:assert/strict";
import test from "node:test";
import React from "react";

import { cleanup, fireEvent, installDom, renderWithIntl, waitFor } from "./runtime-test-helpers";

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

  (globalThis as unknown as { XMLHttpRequest: typeof MockXHR }).XMLHttpRequest = MockXHR as unknown as typeof XMLHttpRequest;

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
  const uploadedAssets: uploaderModule.UploadedAsset[] = [];

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
  const uploadedAssets: uploaderModule.UploadedAsset[] = [];

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
