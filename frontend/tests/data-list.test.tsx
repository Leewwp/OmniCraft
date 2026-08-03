import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import { DataList } from "@/components/ui/data-list";
import { cleanup, fireEvent, installDom, render } from "./runtime-test-helpers";

const messages = {
  common: { loading: "Loading", retry: "Retry", noData: "No data", next: "Next page", processing: "Loading more" },
};

function renderList(props: Partial<React.ComponentProps<typeof DataList<string>>> = {}) {
  return render(
    <IntlProvider locale="en" messages={messages}>
      <DataList<string>
        items={[]}
        loading={false}
        empty={<p>Nothing here</p>}
        renderItem={(item) => <span>{item}</span>}
        {...props}
      />
    </IntlProvider>,
  );
}

test.afterEach(() => cleanup());

test("DataList renders loading, empty, and retryable error states", () => {
  installDom();
  const loading = renderList({ loading: true, loadingState: <div>Rows loading</div> });
  assert.ok(loading.getByRole("status"));
  assert.ok(loading.getByText("Rows loading"));

  cleanup();
  installDom();
  const empty = renderList();
  assert.ok(empty.getByText("Nothing here"));

  cleanup();
  installDom();
  let retries = 0;
  const error = renderList({ error: "Could not load", onRetry: () => { retries += 1; } });
  assert.ok(error.getByRole("alert"));
  fireEvent.click(error.getByRole("button", { name: "Retry" }));
  assert.equal(retries, 1);
});

test("DataList keeps prior rows during pagination failures and prevents duplicate next-page requests", () => {
  installDom();
  let loads = 0;
  const view = renderList({
    items: ["first"],
    hasMore: true,
    loadingMore: false,
    onLoadMore: () => { loads += 1; },
    error: "Page two failed",
    onRetry: () => {},
  });
  assert.ok(view.getByText("first"));
  const next = view.getByRole("button", { name: "Retry" });
  fireEvent.click(next);
  assert.equal(loads, 0);
  const more = view.getByRole("button", { name: "Next page" });
  fireEvent.click(more);
  fireEvent.click(more);
  assert.equal(loads, 1);
});

test("DataList marks the end of a list and exposes no next-page action", () => {
  installDom();
  const view = renderList({ items: ["last"], hasMore: false });
  assert.ok(view.getByText("last"));
  assert.equal(view.queryByRole("button", { name: "Next page" }), null);
});
