import assert from "node:assert/strict";
import test from "node:test";

import type { SeriesMembership } from "@/lib/content";
import { isValidSeriesMembership, isValidSeriesStepTarget, resolveSeriesStepTarget } from "@/lib/series-guard";

const validMembership: SeriesMembership = {
  series_id: 1,
  series_title: "山海纪行",
  series_zone: "original",
  current_index: 2,
  total: 3,
  previous: { id: 10, title: "启程" },
  next: { id: 12, title: "归途" },
};

function variant(overrides: Partial<SeriesMembership>): SeriesMembership {
  return { ...validMembership, ...overrides };
}

test("isValidSeriesMembership guards the membership boundaries (table-driven)", () => {
  const cases: Array<{ name: string; value: SeriesMembership | undefined; expected: boolean }> = [
    { name: "undefined membership（OverlayRelatedBlock 侧可传 undefined）", value: undefined, expected: false },
    { name: "合法基准（中位章节）", value: validMembership, expected: true },
    { name: "total === current_index（末章）合法", value: variant({ current_index: 3, total: 3 }), expected: true },
    { name: "series_id 非整数（1.5）", value: variant({ series_id: 1.5 }), expected: false },
    { name: "series_id = 0", value: variant({ series_id: 0 }), expected: false },
    { name: "series_id 负数", value: variant({ series_id: -2 }), expected: false },
    { name: "current_index 非整数（1.5）", value: variant({ current_index: 1.5 }), expected: false },
    { name: "current_index = 0", value: variant({ current_index: 0, total: 3 }), expected: false },
    { name: "current_index 负数", value: variant({ current_index: -1, total: 3 }), expected: false },
    { name: "total 非整数（1.5）", value: variant({ total: 1.5 }), expected: false },
    { name: "total = 0", value: variant({ current_index: 1, total: 0 }), expected: false },
    { name: "total 负数", value: variant({ current_index: 1, total: -3 }), expected: false },
    { name: "total < current_index", value: variant({ current_index: 4, total: 3 }), expected: false },
    { name: "series_title 空串", value: variant({ series_title: "" }), expected: false },
    { name: "series_title 纯空白", value: variant({ series_title: "   " }), expected: false },
    { name: "series_title 前后空白但 trim 后非空", value: variant({ series_title: " 山海纪行 " }), expected: true },
  ];
  for (const { name, value, expected } of cases) {
    assert.equal(isValidSeriesMembership(value), expected, name);
  }
});

test("isValidSeriesStepTarget guards id/title boundaries (table-driven)", () => {
  const cases: Array<{ name: string; value: { id: number; title: string } | undefined; expected: boolean }> = [
    { name: "undefined target（previous/next 可缺省）", value: undefined, expected: false },
    { name: "合法目标", value: { id: 12, title: "归途" }, expected: true },
    { name: "标题前后空白但 trim 后非空", value: { id: 12, title: " 归途 " }, expected: true },
    { name: "id 非整数（1.5）", value: { id: 1.5, title: "归途" }, expected: false },
    { name: "id = 0", value: { id: 0, title: "归途" }, expected: false },
    { name: "id 负数", value: { id: -1, title: "归途" }, expected: false },
    { name: "title 空串", value: { id: 12, title: "" }, expected: false },
    { name: "title 纯空白", value: { id: 12, title: "   " }, expected: false },
  ];
  for (const { name, value, expected } of cases) {
    assert.equal(isValidSeriesStepTarget(value), expected, name);
  }
});

test("isValidSeriesStepTarget keeps the caller-side concrete type (directory chapter items)", () => {
  const chapter = { id: 31, title: "下篇", zone: "fanwork" as const };
  const items = [chapter, { id: 0, title: "坏章" }].filter((item) => isValidSeriesStepTarget(item));
  assert.deepEqual(items, [chapter], "invalid chapter ids must be filtered and valid items keep their full shape");
});

test("resolveSeriesStepTarget enforces first/last boundaries (table-driven)", () => {
  const firstChapter = variant({ current_index: 1, total: 3 });
  const lastChapter = variant({ current_index: 3, total: 3 });
  const cases: Array<{
    name: string;
    membership: SeriesMembership | undefined;
    step: "previous" | "next";
    expected: { id: number; title: string } | undefined;
  }> = [
    { name: "membership 缺省 → 无上一章", membership: undefined, step: "previous", expected: undefined },
    { name: "membership 缺省 → 无下一章", membership: undefined, step: "next", expected: undefined },
    {
      name: "membership 不合法（total < current_index）→ 无上一章",
      membership: variant({ current_index: 4, total: 3 }),
      step: "previous",
      expected: undefined,
    },
    {
      name: "current_index = 1 禁用 previous（即使目标合法）",
      membership: firstChapter,
      step: "previous",
      expected: undefined,
    },
    {
      name: "current_index = 1 且 next 合法 → next 可解析",
      membership: firstChapter,
      step: "next",
      expected: { id: 12, title: "归途" },
    },
    {
      name: "current_index = total 禁用 next（即使目标合法）",
      membership: lastChapter,
      step: "next",
      expected: undefined,
    },
    {
      name: "current_index = total 且 previous 合法 → previous 可解析",
      membership: lastChapter,
      step: "previous",
      expected: { id: 10, title: "启程" },
    },
    {
      name: "中位章节两侧均可解析 → previous",
      membership: validMembership,
      step: "previous",
      expected: { id: 10, title: "启程" },
    },
    {
      name: "中位章节两侧均可解析 → next",
      membership: validMembership,
      step: "next",
      expected: { id: 12, title: "归途" },
    },
    {
      name: "未越界但 previous 目标 id 非法 → 无上一章",
      membership: variant({ previous: { id: 0, title: "启程" } }),
      step: "previous",
      expected: undefined,
    },
    {
      name: "未越界但 next 目标标题空白 → 无下一章",
      membership: variant({ next: { id: 12, title: "   " } }),
      step: "next",
      expected: undefined,
    },
    {
      name: "previous 缺省（首篇前置数据缺失）→ 无上一章",
      membership: variant({ previous: undefined }),
      step: "previous",
      expected: undefined,
    },
    {
      name: "next 缺省 → 无下一章",
      membership: variant({ next: undefined }),
      step: "next",
      expected: undefined,
    },
  ];
  for (const { name, membership, step, expected } of cases) {
    assert.deepEqual(resolveSeriesStepTarget(membership, step), expected, name);
  }
});
