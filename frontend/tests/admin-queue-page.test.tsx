import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";

import { api } from "@/lib/api";

import { cleanup, installDom, waitFor, within } from "./runtime-test-helpers";

const requireForMocks = createRequire(import.meta.url) as NodeRequire & {
  extensions: NodeJS.RequireExtensions;
};
requireForMocks.extensions[".css"] = () => undefined;

const originalGet = api.get;
const originalConsoleError = console.error;

// F-B001（#393）：锁死前端 QueueTopic 与后端 queue.QueueStats（observability.go）
// 的字段契约 —— {topic,depth,pending_count,failed_total}。此前前端读
// {name,lag,failure_count}，主题名与 key 全 undefined：5 行空白 + duplicate key 告警。
const backendStatsPayload = {
  topics: [
    { topic: "content.review", depth: 12, consumed_total: 34, failed_total: 2, pending_count: 4 },
    { topic: "ip.review", depth: 1, consumed_total: 9, failed_total: 0, pending_count: 0 },
    { topic: "notification.create", depth: 30, consumed_total: 3, failed_total: 0, pending_count: 4 },
    { topic: "count.download", depth: 0, consumed_total: 0, failed_total: 0, pending_count: 0 },
    { topic: "content.embedding", depth: 7, consumed_total: 11, failed_total: 5, pending_count: 2 },
  ],
  dlq_count: 1,
  queue_enabled: true,
};

// 镜像 frontend/messages/en.json 的 admin.queue 真实 catalog（禁止发明不存在的 key）。
const intlMessages = {
  common: {
    retry: "Retry",
  },
  admin: {
    queue: {
      title: "Queue Monitor",
      stats: "Queue Stats",
      noStats: "No queue data available",
      topic: "Topic",
      depth: "Depth",
      lag: "Lag",
      failures: "Failures",
      dlq: "Dead Letter Queue",
      noDLQ: "DLQ is empty",
      loadFailed: "Failed to load queue data",
      readOnlyNote:
        "This page monitors per-topic depth, lag and failures; dead-letter entries can be replayed back to their original topic (the original entry stays in the DLQ).",
      replay: "Replay",
      replayTitle: "Replay dead-letter entry",
      replayConfirm:
        'Re-deliver entry {id} back to its original topic "{topic}"? The original entry stays in the dead-letter queue and is not removed.',
      replaySuccess:
        "Entry {id} re-delivered to its original topic (original entry kept in the DLQ)",
      replayFailed: "Failed to replay entry {id}. Please try again.",
      retries: "{count} retries",
    },
  },
};

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  console.error = originalConsoleError;
});

function installApiGetMock(options?: { stats?: unknown; dlq?: unknown }) {
  const calls: string[] = [];
  api.get = (async (path: string) => {
    calls.push(path);
    if (path === "/api/v1/admin/queue/stats") {
      return options?.stats !== undefined ? options.stats : backendStatsPayload;
    }
    if (path === "/api/v1/admin/queue/dlq") {
      return options?.dlq !== undefined ? options.dlq : { entries: [], count: 0 };
    }
    throw new Error(`unexpected api.get path: ${path}`);
  }) as typeof api.get;
  return calls;
}

async function renderQueuePage() {
  const { render } = await import("@testing-library/react");
  const pageModule = await import("../app/(protected)/admin/queue/page");
  const AdminQueuePage = pageModule.default;

  const consoleErrors: string[] = [];
  console.error = (...args: unknown[]) => {
    const line = args.map(String).join(" ");
    if (line.includes("ENVIRONMENT_FALLBACK")) {
      return;
    }
    consoleErrors.push(line);
    originalConsoleError(...args);
  };

  const view = render(
    <IntlProvider locale="en" messages={intlMessages}>
      <AdminQueuePage />
    </IntlProvider>,
  );
  return { view, consoleErrors };
}

test("renders topic rows from the backend contract with real names and per-column values", async () => {
  installDom();
  installApiGetMock();
  const { view, consoleErrors } = await renderQueuePage();

  await waitFor(() => {
    assert.equal(view.getAllByRole("row").length, 6); // header + 5 topics
  });

  const expectedRows: Array<[string, string, string, string]> = [
    ["content.review", "12", "4", "2"],
    ["ip.review", "1", "0", "0"],
    ["notification.create", "30", "4", "0"],
    ["count.download", "0", "0", "0"],
    ["content.embedding", "7", "2", "5"],
  ];
  const bodyRows = view.getAllByRole("row").slice(1);
  expectedRows.forEach(([topic, depth, pending, failed], i) => {
    const cells = within(bodyRows[i]).getAllByRole("cell");
    assert.equal(cells.length, 4, `row ${topic} cell count`);
    assert.equal(cells[0].textContent, topic, `row ${i} topic name cell`);
    assert.equal(cells[1].textContent, depth, `row ${topic} depth cell`);
    assert.equal(cells[2].textContent, pending, `row ${topic} pending(lag) cell`);
    assert.equal(cells[3].textContent, failed, `row ${topic} failures cell`);
  });

  const bodyText = view.container.textContent ?? "";
  assert.ok(!bodyText.includes("undefined"), "no undefined leaked into the topic table");

  const duplicateKeyErrors = consoleErrors.filter((line) => line.includes("same key"));
  assert.deepEqual(duplicateKeyErrors, [], "React duplicate key warnings must not fire");
});

test("shows the empty state when the backend reports no topics", async () => {
  installDom();
  installApiGetMock({ stats: { topics: [], dlq_count: 0, queue_enabled: true } });
  const { view } = await renderQueuePage();

  await waitFor(() => {
    assert.ok(view.getByText(intlMessages.admin.queue.noStats));
  });
  assert.equal(view.queryAllByRole("row").length, 0);
});
