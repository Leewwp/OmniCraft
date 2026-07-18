import assert from "node:assert/strict";
import test from "node:test";

import { normalizeContentDetailResponse, normalizeContentListResponse, normalizeSeriesMemberships } from "@/lib/content";

test("normalizeContentDetailResponse composes top-level memberships with content", () => {
  const normalized = normalizeContentDetailResponse({
    content: { id: 9, title: "Chapter", zone: "original" },
    series_memberships: [
      { series_id: 1, series_title: "Arc", current_index: 1, total: 2, next: { id: 10, title: "Next" } },
    ],
    attachments: [],
    tags: ["serial"],
  });
  assert.equal(normalized.content?.series_memberships?.[0]?.series_title, "Arc");
  assert.equal(normalized.series_memberships.length, 1);

  const legacy = normalizeContentDetailResponse({
    Content: {
      ID: 9,
      Title: "Chapter",
      Zone: "original",
      SeriesMemberships: [{ SeriesID: 2, SeriesTitle: "Legacy", CurrentIndex: 1, Total: 1 }],
    },
  });
  assert.equal(legacy.content?.series_memberships?.[0]?.series_title, "Legacy");
});

test("normalizeSeriesMemberships accepts snake_case and PascalCase without truncation", () => {
  const memberships = normalizeSeriesMemberships([
    {
      series_id: 1,
      series_title: "First",
      series_zone: "original",
      current_index: 2,
      total: 3,
      previous: { id: 10, title: "Prev" },
      next: { id: 12, title: "Next" },
    },
    {
      SeriesID: "2",
      SeriesTitle: "Second",
      CurrentIndex: "1",
      Total: "1",
      Previous: { ID: 0, Title: "invalid" },
      Next: { ID: 20, Title: "" },
    },
    { series_id: 3, series_title: "Third", current_index: 1, total: 1 },
    { series_id: 4, series_title: "Fourth", current_index: 1, total: 1 },
  ]);

  assert.equal(memberships.length, 4);
  assert.deepEqual(memberships[0], {
    series_id: 1,
    series_title: "First",
    series_zone: "original",
    current_index: 2,
    total: 3,
    previous: { id: 10, title: "Prev" },
    next: { id: 12, title: "Next" },
  });
  assert.equal(memberships[1]?.previous, undefined);
  assert.equal(memberships[1]?.next, undefined);
});

test("normalizeSeriesMemberships drops invalid memberships and navigation summaries", () => {
  const memberships = normalizeSeriesMemberships([
    null,
    { series_id: 0, series_title: "No id", current_index: 1, total: 1 },
    { series_id: 1, series_title: "", current_index: 1, total: 1 },
    { series_id: 2, series_title: "Bad index", current_index: 0, total: 1 },
    { series_id: 3, series_title: "Bad total", current_index: 1, total: 0 },
    { series_id: 4, series_title: "Valid", current_index: 1, total: 1, next: { id: 5 } },
  ]);

  assert.deepEqual(memberships, [
    { series_id: 4, series_title: "Valid", current_index: 1, total: 1, next: undefined, previous: undefined },
  ]);
  assert.deepEqual(normalizeSeriesMemberships({}), []);
});

test("normalizeContentListResponse accepts the real my-contents envelope", () => {
  const contents = normalizeContentListResponse({ contents: [{ id: 9, title: "Chapter", zone: "original" }] });
  assert.equal(contents.length, 1);
  assert.equal(contents[0]?.id, 9);
  assert.equal(contents[0]?.title, "Chapter");
});
