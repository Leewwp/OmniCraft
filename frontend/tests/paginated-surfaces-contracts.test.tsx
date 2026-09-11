import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(process.cwd());

function read(relativePath: string) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

test("all U-07 list surfaces delegate list states to DataList", () => {
  const pages = [
    "app/(protected)/history/page.tsx",
    "app/(protected)/appeals/page.tsx",
    "app/(protected)/rehab/page.tsx",
    "app/(protected)/feedback/mine/page.tsx",
    "app/(protected)/studio/overview/page.tsx",
    "app/(protected)/studio/followers/page.tsx",
    "app/(protected)/studio/contents/page.tsx",
  ];

  for (const page of pages) {
    const source = read(page);
    assert.match(source, /DataList/);
    assert.match(source, /onRetry=/);
  }

  for (const page of pages.filter((candidate) => !candidate.includes("studio/followers"))) {
    assert.match(read(page), /loadingMore/);
  }
});

const paginatedPageContracts = [
  ["appeals", "app/(protected)/appeals/page.tsx", /appeals\/me\?page=\$\{nextPage\}/],
  ["rehab courses", "app/(protected)/rehab/page.tsx", /rehab\/courses\?page=\$\{nextPage\}/],
  ["feedback tickets", "app/(protected)/feedback/mine/page.tsx", /feedback\/me\?page=\$\{nextPage\}/],
  ["studio contents", "app/(protected)/studio/contents/page.tsx", /users\/me\/contents\?page=\$\{nextPage\}/],
  ["studio ranking", "app/(protected)/studio/overview/page.tsx", /users\/me\/contents\?page=\$\{nextPage\}/],
] as const;

for (const [name, file, endpoint] of paginatedPageContracts) {
  test(`${name} keeps fetching and pagination state in the page`, () => {
    const source = read(file);
    assert.match(source, endpoint);
    assert.match(source, /set(?:Top)?Page\(nextPage\)/);
    assert.match(source, /set(?:Top)?HasMore\(/);
    assert.match(source, /onLoadMore=/);
    assert.match(source, /onRetry=/);
  });
}

test("paginated pages own cursor state and reset the first page for filters/search", () => {
  const history = read("app/(protected)/history/page.tsx");

  assert.match(history, /void load\(1, false\)/);
  assert.match(history, /page_size: "20"/);
});

test("studio content surfaces use the authenticated backend route and real envelope", () => {
  for (const page of [
    "app/(protected)/studio/overview/page.tsx",
    "app/(protected)/studio/contents/page.tsx",
  ]) {
    const source = read(page);
    assert.match(source, /\/api\/v1\/users\/me\/contents/);
    assert.match(source, /contents \?\? res\?\.data|contents \?\? contentsRes\?\.data/);
    assert.doesNotMatch(source, /\/api\/v1\/my\/contents/);
  }
});

test("pagination retries retain the failed cursor instead of replacing prior rows", () => {
  // #290 后 IP 讨论列表并入 /ip/[ipId] Hub 的 discussions tab（单页拉取，
  // 无游标分页），不再出现在该契约清单中。
  const pages = [
    "app/(protected)/history/page.tsx",
    "app/(protected)/appeals/page.tsx",
    "app/(protected)/rehab/page.tsx",
    "app/(protected)/feedback/mine/page.tsx",
    "app/(protected)/studio/overview/page.tsx",
    "app/(protected)/studio/contents/page.tsx",
  ];

  for (const page of pages) {
    const source = read(page);
    assert.match(source, /set(?:Top)?Page\((?:nextPage|requestedPage)\)/, `${page} records the requested cursor before fetching`);
  }
});
