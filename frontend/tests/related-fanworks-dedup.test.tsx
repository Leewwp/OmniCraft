import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api } from "@/lib/api";
import { cleanup, installDom, render, waitFor } from "./runtime-test-helpers";
import type { ContentCardData } from "@/components/content/ContentCard";

/* 扇出收敛（2026-09-09 用户裁决）：浮层层内已拉取 related-fanworks 后经
   initialData 直供 RelatedFanworks——本文件锁定「直供零自拉 / 无直供自拉一次 /
   total=0 直供隐藏」三合同。 */

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;

Module._load = function loadWithStubs(request, parent, isMain) {
  if (request === "next/image") {
    return (props: Record<string, unknown>) =>
      React.createElement("img", { ...props, fill: undefined, sizes: undefined });
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

type RelatedFanworksModule = typeof import("@/components/content/RelatedFanworks");

let RelatedFanworks: RelatedFanworksModule["RelatedFanworks"];

const originalGet = api.get;
const RELATED_ENDPOINT = "/api/v1/contents/9/related-fanworks?page=1&page_size=8";

test.before(async () => {
  const module = await import("@/components/content/RelatedFanworks");
  RelatedFanworks = module.RelatedFanworks;
});

test.after(() => {
  api.get = originalGet;
});

test.afterEach(() => cleanup());

function renderWithEn(node: React.ReactNode) {
  return render(<IntlProvider locale="en" messages={enMessages}>{node}</IntlProvider>);
}

const FIXTURE_ITEMS = [
  { id: 31, title: "Related A", zone: "fanwork", content_type: "image" },
] as unknown as ContentCardData[];

test("initialData 直供时不发自拉请求，并回报 onData", async () => {
  installDom();
  const calls: string[] = [];
  const onDataItems: ContentCardData[][] = [];
  api.get = (async (requestPath: string) => {
    calls.push(requestPath);
    throw new Error(`unexpected api.get call: ${requestPath}`);
  }) as typeof api.get;

  renderWithEn(
    <RelatedFanworks
      sourceContentId={9}
      sourceZone="original"
      titleKey="relatedFanworks.derivatives.title"
      initialData={{ items: FIXTURE_ITEMS, total: 1 }}
      onData={(items) => onDataItems.push(items)}
    />,
  );
  await waitFor(() => {
    assert.ok(document.querySelector('[data-slot="related-fanworks"]'));
  });
  assert.match(document.body.textContent ?? "", /Related A/);
  assert.deepEqual(calls, []);
  assert.equal(onDataItems.length >= 1, true);
  assert.equal(onDataItems[0], FIXTURE_ITEMS);
});

test("无 initialData 时自拉一次并渲染", async () => {
  installDom();
  const calls: string[] = [];
  api.get = (async (requestPath: string) => {
    calls.push(requestPath);
    if (requestPath === RELATED_ENDPOINT) {
      return {
        contents: [{ id: 31, title: "Related A", zone: "fanwork", content_type: "image" }],
        total: 1,
      };
    }
    throw new Error(`unexpected api.get call: ${requestPath}`);
  }) as typeof api.get;

  renderWithEn(
    <RelatedFanworks
      sourceContentId={9}
      sourceZone="original"
      titleKey="relatedFanworks.derivatives.title"
    />,
  );
  await waitFor(() => {
    assert.ok(document.querySelector('[data-slot="related-fanworks"]'));
  });
  assert.deepEqual(calls, [RELATED_ENDPOINT]);
});

test("initialData total=0 时隐藏整行且不发请求", async () => {
  installDom();
  const calls: string[] = [];
  api.get = (async (requestPath: string) => {
    calls.push(requestPath);
    throw new Error(`unexpected api.get call: ${requestPath}`);
  }) as typeof api.get;

  renderWithEn(
    <RelatedFanworks
      sourceContentId={9}
      sourceZone="original"
      titleKey="relatedFanworks.derivatives.title"
      initialData={{ items: [], total: 0 }}
    />,
  );
  await waitFor(() => {
    /* 骨架先在（loading 态）后消失，才证明直供 effect 已跑、确为落定后隐藏。 */
    assert.ok(
      document.querySelector('[data-slot="related-fanworks"], [data-slot="related-fanworks-loading"]') ===
        null,
    );
  });
  assert.deepEqual(calls, []);
});
