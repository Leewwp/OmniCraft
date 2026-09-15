import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import zhMessages from "@/messages/zh.json";
import { cleanup, fireEvent, installDom, render } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * SP-19 G2-1（#523）：zone="ip" 引用 —— normalizer 放行/拒绝面、卡片 IP 徽标
 * + 分类徽标（11 类词表单源）、toAgentCitation 透传 category。
 * ──────────────────────────────────────────────────────────────────────────── */

type AgentModule = typeof import("@/lib/agent");
type CardModule = typeof import("@/components/agent/AgentCitationCard");
let normalizeAgentCitation: AgentModule["normalizeAgentCitation"];
let toAgentCitation: AgentModule["toAgentCitation"];
let AgentCitationCard: CardModule["AgentCitationCard"];

test.before(async () => {
  const agentModule = await import("@/lib/agent");
  const cardModule = await import("@/components/agent/AgentCitationCard");
  normalizeAgentCitation = agentModule.normalizeAgentCitation;
  toAgentCitation = agentModule.toAgentCitation;
  AgentCitationCard = cardModule.AgentCitationCard;
});

test.afterEach(() => cleanup());

test("normalizer accepts a well-formed ip citation and keeps category", () => {
  assert.deepEqual(
    normalizeAgentCitation({
      content_id: 248,
      title: "宝可梦",
      zone: "ip",
      route: "/ip/248",
      excerpt: "简介摘录",
      category: "game",
    }),
    { content_id: 248, title: "宝可梦", zone: "ip", route: "/ip/248", excerpt: "简介摘录", category: "game" },
  );
  /* route/excerpt/category 均可省略（最小合法形态）。 */
  assert.deepEqual(
    normalizeAgentCitation({ content_id: 9, title: "银杏电台", zone: "ip" }),
    { content_id: 9, title: "银杏电台", zone: "ip" },
  );
});

test("normalizer rejects forged ip citations", () => {
  const base = { content_id: 7, title: "T", zone: "ip" } as const;
  /* 路由必须是 /ip/{id}：内容路由形态拒绝。 */
  assert.equal(normalizeAgentCitation({ ...base, route: "/original/7" }), null);
  assert.equal(normalizeAgentCitation({ ...base, route: "/ip/8" }), null);
  /* chunk 溯源字段与 ip 形状互斥（防形状混淆）。 */
  assert.equal(normalizeAgentCitation({ ...base, content_version: 3 }), null);
  assert.equal(normalizeAgentCitation({ ...base, chunk_key: "a".repeat(64) }), null);
  assert.equal(normalizeAgentCitation({ ...base, chunk_index: 1 }), null);
  assert.equal(normalizeAgentCitation({ ...base, source: "bm25" }), null);
  /* 类型错误拒绝；空串按缺席处理（与内容分支一致，重放安全）。 */
  assert.equal(normalizeAgentCitation({ ...base, category: 3 }), null);
  assert.equal(normalizeAgentCitation({ ...base, excerpt: 7 }), null);
  assert.deepEqual(
    normalizeAgentCitation({ ...base, category: "", excerpt: "" }),
    { content_id: 7, title: "T", zone: "ip" },
  );
});

test("normalizer tolerates zero-valued provenance fields on ip replay", () => {
  /* 历史重放形态（model.AgentCitation 无 omitempty）：溯源字段全零值视同缺席。 */
  assert.deepEqual(
    normalizeAgentCitation({
      content_id: 248,
      content_version: 0,
      chunk_key: "",
      chunk_index: 0,
      title: "宝可梦",
      zone: "ip",
      route: "/ip/248",
      excerpt: "简介",
      source: "",
      category: "game",
    }),
    { content_id: 248, title: "宝可梦", zone: "ip", route: "/ip/248", excerpt: "简介", category: "game" },
  );
});

test("toAgentCitation passes the ip zone and category through", () => {
  const normalized = toAgentCitation({
    content_id: 248,
    title: "宝可梦",
    zone: "ip",
    route: "/ip/248",
    category: "game",
  });
  assert.equal(normalized.zone, "ip");
  assert.equal(normalized.category, "game");
  assert.equal(normalized.route, "/ip/248");
  /* 内容引用不受影响。 */
  assert.equal(toAgentCitation({ content_id: 3, title: "T", zone: "fanwork" }).zone, "fanwork");
});

test("ip citation card renders IP badge, category label and excerpt", () => {
  installDom();
  const view = render(
    <IntlProvider locale="en" messages={enMessages}>
      <AgentCitationCard
        index={0}
        citation={{ contentId: 248, title: "宝可梦", zone: "ip", excerpt: "Pocket world", category: "game" }}
        onOpen={() => {}}
      />
    </IntlProvider>,
  );
  assert.ok(view.getByText("宝可梦"));
  assert.ok(view.getByText("IP"), "zone badge renders as IP");
  assert.ok(view.getByText("Games"), "category label renders from the 11-slug table");
  assert.ok(view.getByText("Pocket world"));
  /* 内容分区的旧徽标不出现在 ip 卡上。 */
  assert.equal(view.queryByText("Original"), null);
  assert.equal(view.queryByText("Fanwork"), null);
});

test("ip citation card falls back to the other-category label for unknown slugs", () => {
  installDom();
  const view = render(
    <IntlProvider locale="en" messages={enMessages}>
      <AgentCitationCard
        index={1}
        citation={{ contentId: 9, title: "未知类目 IP", zone: "ip", category: "not-a-slug" }}
        onOpen={() => {}}
      />
    </IntlProvider>,
  );
  assert.ok(view.getByText("Other"), "unknown slug falls back to the other label");
});

test("content citation cards keep the original/fanwork zone badge", () => {
  installDom();
  const view = render(
    <IntlProvider locale="zh" messages={zhMessages}>
      <AgentCitationCard
        index={0}
        citation={{ contentId: 100, title: "作品", zone: "original" }}
        onOpen={() => {}}
      />
    </IntlProvider>,
  );
  assert.ok(view.getByText("原文"));
  /* ip 徽标不出现。 */
  const badges = document.querySelectorAll("span.rounded");
  for (const badge of Array.from(badges)) {
    assert.notEqual(badge.textContent, "IP");
  }
});

test("ip citation card click reaches onOpen with the ip citation", () => {
  installDom();
  let opened: { zone: string; contentId: number } | null = null;
  const view = render(
    <IntlProvider locale="zh" messages={zhMessages}>
      <AgentCitationCard
        index={0}
        citation={{ contentId: 248, title: "宝可梦", zone: "ip", category: "game" }}
        onOpen={(citation) => {
          opened = { zone: citation.zone, contentId: citation.contentId };
        }}
      />
    </IntlProvider>,
  );
  fireEvent.click(view.getByRole("button"));
  assert.ok(opened);
  assert.equal(opened!.zone, "ip");
  assert.equal(opened!.contentId, 248);
});
