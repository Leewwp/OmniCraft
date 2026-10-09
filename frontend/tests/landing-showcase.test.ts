import assert from "node:assert/strict";
import test from "node:test";

import {
  SHOWCASE_IP_SLOTS,
  SHOWCASE_MIN_VISIBLE,
  SHOWCASE_WORK_SLOTS,
  buildIpShowcase,
  buildWorkShowcase,
} from "@/lib/landing-showcase";

// ---------------------------------------------------------------------------
// 接缝 3（票 #853 / spec §12.2）：精选陈列选槽纯函数。
// 口径：主序=服务端 hot 返回顺序（前端不发明热度权重）；缺封面过滤；
// 按 id 去重（先到先得）；两区混排带分区徽标；不足用 newest 补槽；
// 补完 < SHOWCASE_MIN_VISIBLE 该子区整段隐藏（返回空数组）。
// ---------------------------------------------------------------------------

function work(id: number, overrides: Partial<{ cover: string | null; zone: string; title: string }> = {}) {
  return {
    id,
    title: overrides.title ?? `作品${id}`,
    zone: overrides.zone ?? "fanwork",
    cover_image_url: overrides.cover !== undefined ? overrides.cover : `/c/${id}.jpg`,
  };
}

function ip(id: number, overrides: Partial<{ cover: string | null; name: string }> = {}) {
  return {
    id,
    name: overrides.name ?? `IP${id}`,
    cover_url: overrides.cover !== undefined ? overrides.cover : `/ip/${id}.jpg`,
  };
}

test("work showcase keeps server hot order, filters coverless items, and caps at 10 slots", () => {
  const fanworkHot = [
    work(1),
    work(2, { cover: null }), // 缺封面 → 过滤
    work(3),
    work(4, { cover: "" }), // 空串封面 → 过滤
    work(5),
    work(6),
  ];
  const items = buildWorkShowcase({ fanworkHot, originalHot: [], fanworkNewest: [], originalNewest: [] });
  assert.deepEqual(items.map((i) => i.id), [1, 3, 5, 6]);
  assert.equal(items[0].zone, "fanwork");
  assert.equal(items[0].coverUrl, "/c/1.jpg");
});

test("work showcase interleaves both zones when both sides have entries (zone badge preserved)", () => {
  const fanworkHot = [work(1), work(2), work(3), work(4)];
  const originalHot = [work(101, { zone: "original" }), work(102, { zone: "original" })];
  const items = buildWorkShowcase({ fanworkHot, originalHot, fanworkNewest: [], originalNewest: [] });
  // 混排：两区轮流交错、区内保持服务端顺序、总数不超槽位
  assert.deepEqual(items.map((i) => i.id), [1, 101, 2, 102, 3, 4]);
  assert.deepEqual(items.map((i) => i.zone), ["fanwork", "original", "fanwork", "original", "fanwork", "fanwork"]);
});

test("work showcase fills remaining slots from newest lists without duplicates", () => {
  const fanworkHot = [work(1), work(2)];
  const originalHot = [work(101, { zone: "original" })];
  const fanworkNewest = [work(1), work(5), work(6)]; // 1 已入选 → 跳过
  const originalNewest = [work(102, { zone: "original" }), work(103, { zone: "original" })];
  const items = buildWorkShowcase({ fanworkHot, originalHot, fanworkNewest, originalNewest });
  // newest 两列交错后补槽，去重跳过已入选的 1
  assert.deepEqual(items.map((i) => i.id), [1, 101, 2, 102, 5, 103, 6]);
});

test("work showcase hides the whole section when fewer than min visible survive", () => {
  const items = buildWorkShowcase({
    fanworkHot: [work(1)],
    originalHot: [work(101, { zone: "original" })],
    fanworkNewest: [work(2)],
    originalNewest: [],
  });
  assert.deepEqual(items, []);
  assert.equal(SHOWCASE_MIN_VISIBLE, 4);
});

test("work showcase caps at the fixed slot count and exposes it", () => {
  const fanworkHot = Array.from({ length: 14 }, (_, i) => work(i + 1));
  const originalHot = Array.from({ length: 14 }, (_, i) => work(100 + i, { zone: "original" }));
  const items = buildWorkShowcase({ fanworkHot, originalHot, fanworkNewest: [], originalNewest: [] });
  assert.equal(items.length, SHOWCASE_WORK_SLOTS);
  assert.equal(SHOWCASE_WORK_SLOTS, 10);
});

test("work showcase is deterministic for fixed input", () => {
  const input = {
    fanworkHot: [work(1), work(2)],
    originalHot: [work(101, { zone: "original" })],
    fanworkNewest: [work(5)],
    originalNewest: [work(102, { zone: "original" })],
  };
  assert.deepEqual(buildWorkShowcase(input), buildWorkShowcase(input));
});

test("ip showcase uses most_contents order as primary and newest as fill, same filter/dedupe/hide rules", () => {
  const primary = [ip(1), ip(2, { cover: null }), ip(3), ip(4), ip(5)];
  const newest = [ip(1), ip(9), ip(8, { cover: "" })];
  const items = buildIpShowcase({ primary, newest });
  assert.deepEqual(items.map((i) => i.id), [1, 3, 4, 5, 9]);
  assert.equal(items[0].name, "IP1");

  const hidden = buildIpShowcase({ primary: [ip(1)], newest: [] });
  assert.deepEqual(hidden, []);
  assert.equal(SHOWCASE_IP_SLOTS, 10);
});
