import assert from "node:assert/strict";
import test from "node:test";
import { JSDOM } from "jsdom";
import { fireEvent, render, waitFor, cleanup } from "@testing-library/react";
import { IntlProvider } from "use-intl";

const messages = {
  common: {
    loading: "Loading...",
  },
  content: {
    noSheetMusic: "No sheet music files",
    pdfSheetMusic: "PDF Sheet Music",
    sheetMusicFile: "Sheet Music File",
    sheetMusicPreview: "Sheet Music Preview",
    sheetMusicPreviewHint: "Preview hint",
    sheetMusicAttachments: "Sheet Music Attachments",
    download: "Download",
    downloadSheetMusic: "Download Sheet Music",
    downloading: "Downloading...",
    downloadFailed: "Download failed",
  },
};

function installDom() {
  const dom = new JSDOM("<!doctype html><html><body></body></html>", {
    url: "https://app.leeppp.online/",
  });

  for (const [key, value] of Object.entries({
    window: dom.window,
    document: dom.window.document,
    navigator: dom.window.navigator,
    HTMLElement: dom.window.HTMLElement,
    Node: dom.window.Node,
    Event: dom.window.Event,
    MutationObserver: dom.window.MutationObserver,
  })) {
    Object.defineProperty(globalThis, key, {
      configurable: true,
      writable: true,
      value,
    });
  }

  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  window.requestAnimationFrame = (callback: FrameRequestCallback) =>
    window.setTimeout(() => callback(performance.now()), 0) as unknown as number;
  window.cancelAnimationFrame = (handle: number) => window.clearTimeout(handle);

  return dom;
}

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

type ViewerProps = {
  contentId: number;
  attachments: ViewerAttachment[];
  allowCopy?: boolean;
  className?: string;
};

type ViewerAttachment = {
  id: number;
  file_type?: string;
  mime_type?: string;
  oss_key?: string;
  oss_url?: string;
  file_size?: number;
};

async function renderViewer(props: ViewerProps) {
  const [{ ToastProvider }, { SheetMusicViewer }] = await Promise.all([
    import("@/components/ui/Toast"),
    import("@/components/content/SheetMusicViewer"),
  ]);

  const originalConsoleError = console.error;
  console.error = (...args: unknown[]) => {
    if (args.some((arg) => typeof arg === "object" && arg !== null && "code" in arg && (arg as { code?: string }).code === "ENVIRONMENT_FALLBACK")) {
      return;
    }
    originalConsoleError(...args);
  };

  try {
    return render(
      <IntlProvider locale="en" messages={messages}>
        <ToastProvider>
          <SheetMusicViewer {...props} />
        </ToastProvider>
      </IntlProvider>,
    );
  } finally {
    console.error = originalConsoleError;
  }
}

test.afterEach(() => {
  cleanup();
});

test("attachments with only oss_key still render a download CTA that calls the authorized download API", async () => {
  const dom = installDom();
  document.cookie = "csrf-token=download-token";

  const fetchCalls: string[] = [];
  const openCalls: Array<[string | URL | undefined, string | undefined, string | undefined]> = [];

  globalThis.fetch = (async (input: string | URL | Request) => {
    fetchCalls.push(String(input));
    return jsonResponse(200, {
      download_url: "https://downloads.example/sheet-only",
      expires_in: 300,
    });
  }) as typeof fetch;
  window.open = ((url?: string | URL, target?: string, features?: string) => {
    openCalls.push([url, target, features]);
    return null;
  }) as typeof window.open;

  const view = await renderViewer({
    contentId: 12,
    allowCopy: true,
    attachments: [{ id: 101, file_type: "sheet_music_mscz", oss_key: "scores/solo.mscz" }],
  });

  fireEvent.click(view.getByRole("button", { name: "Download Sheet Music" }));

  await waitFor(() => assert.equal(fetchCalls.length, 1));
  assert.equal(fetchCalls[0], "http://localhost:8080/api/v1/contents/12/download?attachment_id=101");
  assert.deepEqual(openCalls[0], ["https://downloads.example/sheet-only", "_blank", "noopener,noreferrer"]);

  dom.window.close();
});

test("previewable pdf attachments keep an authorized download CTA instead of a direct OSS anchor", async () => {
  const dom = installDom();
  document.cookie = "csrf-token=download-token";

  const fetchCalls: string[] = [];
  globalThis.fetch = (async (input: string | URL | Request) => {
    fetchCalls.push(String(input));
    return jsonResponse(200, {
      download_url: "https://downloads.example/preview.pdf",
      expires_in: 300,
    });
  }) as typeof fetch;
  window.open = (() => null) as typeof window.open;

  const view = await renderViewer({
    contentId: 45,
    allowCopy: true,
    attachments: [
      {
        id: 202,
        file_type: "sheet_music_pdf",
        mime_type: "application/pdf",
        oss_key: "scores/preview.pdf",
        oss_url: "https://oss.example/preview.pdf",
      },
    ],
  });

  assert.ok(view.container.querySelector('embed[src="https://oss.example/preview.pdf"]'));
  assert.equal(view.container.querySelector('a[href="https://oss.example/preview.pdf"]'), null);

  fireEvent.click(view.getByRole("button", { name: "Download Sheet Music" }));

  await waitFor(() => assert.equal(fetchCalls.length, 1));
  assert.equal(fetchCalls[0], "http://localhost:8080/api/v1/contents/45/download?attachment_id=202");

  dom.window.close();
});

test("mixed preview and download-only attachments each render usable download CTAs", async () => {
  const dom = installDom();
  document.cookie = "csrf-token=download-token";

  const fetchCalls: string[] = [];
  globalThis.fetch = (async (input: string | URL | Request) => {
    fetchCalls.push(String(input));
    return jsonResponse(200, {
      download_url: `https://downloads.example/${fetchCalls.length}`,
      expires_in: 300,
    });
  }) as typeof fetch;
  window.open = (() => null) as typeof window.open;

  const view = await renderViewer({
    contentId: 77,
    allowCopy: true,
    attachments: [
      {
        id: 301,
        file_type: "sheet_music_pdf",
        mime_type: "application/pdf",
        oss_key: "scores/main.pdf",
        oss_url: "https://oss.example/main.pdf",
      },
      {
        id: 302,
        file_type: "sheet_music_mscx",
        oss_key: "scores/bonus.mscx",
      },
    ],
  });

  const buttons = view.getAllByRole("button", { name: "Download Sheet Music" });
  assert.equal(buttons.length, 2);

  fireEvent.click(buttons[0]!);
  fireEvent.click(buttons[1]!);

  await waitFor(() => assert.equal(fetchCalls.length, 2));
  assert.deepEqual(fetchCalls, [
    "http://localhost:8080/api/v1/contents/77/download?attachment_id=301",
    "http://localhost:8080/api/v1/contents/77/download?attachment_id=302",
  ]);

  dom.window.close();
});
