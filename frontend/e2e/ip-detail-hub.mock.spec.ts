import { expect, test, type Page } from "@playwright/test";
import { createServer, type Server } from "node:http";

/* #290 IP 详情页贴吧式社区枢纽 mock 契约：
 * 单页 query 驱动（?tab=&type=&sort=&q=）、三模块页内切换、IP 内搜索计数收缩、
 * 讨论帖详情浮层、提案卡渲染；旧子路由 301 收敛。 */

const IP = {
  id: 1,
  name: "星尘计划",
  description: "mock ip for hub contract",
  category: "anime",
  tags: ["科幻"],
  follower_count: 3,
  is_following: false,
};

const STATS = { follower_count: 3, discussion_count: 12, work_count: 9 };

function contentItem(id: number, title: string, contentType: string) {
  return {
    id,
    title,
    content_type: contentType,
    zone: "fanwork",
    status: "published",
    author: { id: 2, username: "alice", avatar_url: "" },
    created_at: "2026-08-01T00:00:00Z",
    view_count: id * 10,
    like_count: 1,
  };
}

const CONTENTS = [
  contentItem(100, "星尘全部内容样板", "image"),
  contentItem(101, "星尘图片精选集", "image"),
  contentItem(102, "星尘视频日志", "video"),
  contentItem(103, "星尘壁纸包", "image"),
  contentItem(104, "星尘角色立绘", "image"),
  contentItem(105, "星尘混剪视频", "video"),
  contentItem(106, "星尘设定集", "image"),
  contentItem(107, "星尘配音片段", "video"),
  contentItem(108, "星尘同人曲", "audio"),
];

const DISCUSSIONS = [
  {
    id: 11,
    title: "公告：星尘共创守则",
    body: "置顶公告正文",
    is_pinned: true,
    reply_count: 4,
    view_count: 30,
    created_at: "2026-08-01T00:00:00Z",
    author: { id: 2, username: "alice", avatar_url: "" },
  },
  {
    id: 12,
    title: "聊聊第二集的作画",
    body: "作画讨论正文",
    is_pinned: false,
    reply_count: 1,
    view_count: 8,
    created_at: "2026-08-02T00:00:00Z",
    author: { id: 2, username: "alice", avatar_url: "" },
  },
  ...[13, 14, 15, 16, 17, 18, 19, 20, 21, 22].map((id, i) => ({
    id,
    title: `第${i + 3}集观感交流`,
    body: `第${i + 3}集正文`,
    is_pinned: false,
    reply_count: i,
    view_count: i * 2,
    created_at: "2026-08-03T00:00:00Z",
    author: { id: 2, username: "alice", avatar_url: "" },
  })),
];

const PROPOSALS = [
  {
    id: 21,
    ip_id: 1,
    proposer_id: 2,
    proposer: { id: 2, username: "alice" },
    status: "open",
    description_change: "把简介改为更完整的设定介绍",
    cover_url_change: null,
    tags_add: ["太空歌剧"],
    tags_remove: null,
    yes_votes: 1,
    no_votes: 0,
    created_at: "2026-08-20T00:00:00Z",
    deadline_at: "2026-09-20T00:00:00Z",
    my_vote: null,
  },
  {
    id: 20,
    ip_id: 1,
    proposer_id: 2,
    proposer: { id: 2, username: "alice" },
    status: "rejected",
    description_change: "旧的封面更换提案",
    cover_url_change: "https://example.com/new-cover.png",
    tags_add: null,
    tags_remove: null,
    yes_votes: 2,
    no_votes: 3,
    created_at: "2026-08-01T00:00:00Z",
    deadline_at: "2026-08-08T00:00:00Z",
    closed_at: "2026-08-08T00:00:00Z",
    my_vote: null,
  },
];

// The hub page fetches on the server for SSR, which page.route cannot
// intercept. The mocked webServer points both the browser and Next SSR at
// http://127.0.0.1:18080, so a local stub serves both sides.
let stub: Server;

test.beforeAll(async () => {
  stub = createServer((req, res) => {
    const url = new URL(req.url ?? "/", "http://127.0.0.1:18080");
    // credentials:"include" 的跨域 fetch 要求 ACAO 回显精确 origin（不能用 *）
    const origin = req.headers.origin;
    const send = (payload: unknown, status = 200) => {
      res.writeHead(status, {
        "Content-Type": "application/json",
        ...(origin ? { "Access-Control-Allow-Origin": origin, "Access-Control-Allow-Credentials": "true", Vary: "Origin" } : { "Access-Control-Allow-Origin": "*" }),
        "Access-Control-Allow-Methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS",
        "Access-Control-Allow-Headers": "*",
        "Access-Control-Max-Age": "86400",
      });
      res.end(JSON.stringify(payload));
    };

    if (req.method === "OPTIONS") {
      // 预检响应 204 不得携带 body；credentials 模式下 Allow-Headers 不能用
      // 通配 *，必须回显请求头（fetch 规范）
      const requestHeaders = req.headers["access-control-request-headers"];
      res.writeHead(204, {
        ...(origin ? { "Access-Control-Allow-Origin": origin, "Access-Control-Allow-Credentials": "true", Vary: "Origin" } : { "Access-Control-Allow-Origin": "*" }),
        "Access-Control-Allow-Methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS",
        "Access-Control-Allow-Headers": requestHeaders || "*",
        "Access-Control-Max-Age": "86400",
      });
      res.end();
      return;
    }

    if (url.pathname === "/api/v1/ips/1") {
      return send({ ip: IP, stats: STATS });
    }

    if (url.pathname === "/api/v1/ips/1/contents") {
      const type = url.searchParams.get("type") || "all";
      const q = url.searchParams.get("q") || "";
      let items = CONTENTS;
      if (type !== "all") items = items.filter((c) => c.content_type === type);
      if (q) items = items.filter((c) => c.title.includes(q));
      // 分面：同可见性、不受 type 影响（与后端 CountByTypeWithinIP 语义一致）
      const facetSource = q ? CONTENTS.filter((c) => c.title.includes(q)) : CONTENTS;
      const type_counts: Record<string, number> = {};
      for (const c of facetSource) type_counts[c.content_type] = (type_counts[c.content_type] ?? 0) + 1;
      return send({ contents: items, total: items.length, type_counts });
    }

    if (url.pathname === "/api/v1/ips/1/discussions") {
      const q = url.searchParams.get("q") || "";
      const items = q ? DISCUSSIONS.filter((d) => d.title.includes(q)) : DISCUSSIONS;
      return send({ discussions: items, total: items.length });
    }

    if (url.pathname === "/api/v1/ips/1/proposals") {
      const status = url.searchParams.get("status") || "";
      const q = url.searchParams.get("q") || "";
      let items = PROPOSALS;
      if (status === "open" || status === "adopted" || status === "rejected") {
        items = items.filter((p) => p.status === status);
      } else if (status !== "all" && status !== "") {
        items = items.filter((p) => p.status === "adopted");
      }
      if (q) items = items.filter((p) => (p.description_change ?? "").includes(q));
      return send({ proposals: items, total: items.length, min_votes: 10, pass_threshold: 0.6 });
    }

    if (url.pathname === "/api/v1/discussions/11") {
      return send({ discussion: DISCUSSIONS[0], comments: [{ id: 31, body: "支持守则", author: { id: 3, username: "bob" } }] });
    }

    if (url.pathname === "/api/v1/config/public") {
      return send({
        features: {
          web_agent_enabled: false,
          desktop_deploy_enabled: false,
          creator_support_enabled: false,
          payment_enabled: false,
        },
        captcha: { provider: "bypass", prefix: "", scene_id: "", region: "cn" },
        client: { download_enabled: false, download_url: "", latest_version: "" },
        legal: { current_terms_version: "test", current_privacy_version: "test" },
      });
    }
    return send({ code: "NOT_FOUND", message: "stub" }, 404);
  });
  if (process.env.U03_REUSE_STUB !== "1") { await new Promise<void>((resolve) => stub.listen(18080, "127.0.0.1", resolve)); }
});

test.afterAll(async () => {
  await new Promise<void>((resolve) => stub.close(() => resolve()));
});

async function prepare(page: Page) {
  await page.context().addCookies([{ name: "NEXT_LOCALE", value: "zh", url: "http://127.0.0.1:3001" }]);
  await page.setViewportSize({ width: 1440, height: 900 });
}

test("hub tabs switch in place, sync the URL, and survive refresh", async ({ page }) => {
  await prepare(page);

  await page.goto("/ip/1");
  const tabs = page.getByRole("navigation", { name: "IP 模块" });
  await expect(tabs.getByRole("button", { name: /内容分享/ })).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByText("星尘全部内容样板")).toBeVisible();

  // Scroll down so in-place switching is observable, then switch without a
  // page jump: router.replace must not scroll back to the top.
  await page.evaluate(() => window.scrollTo(0, 400));
  const scrollBefore = await page.evaluate(() => window.scrollY);
  expect(scrollBefore).toBeGreaterThan(0);

  await tabs.getByRole("button", { name: /讨论区/ }).click();
  await expect(page).toHaveURL(/\/ip\/1\?tab=discussions/);
  await expect(tabs.getByRole("button", { name: /讨论区/ })).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByText("聊聊第二集的作画")).toBeVisible();
  expect(await page.evaluate(() => window.scrollY)).toBeGreaterThan(0);

  // Refresh restores the tab from the URL query.
  await page.reload();
  await expect(tabs.getByRole("button", { name: /讨论区/ })).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByText("聊聊第二集的作画")).toBeVisible();
});

test("share tab type pills carry the URL, show chip counts, and filter in place", async ({ page }) => {
  await prepare(page);

  await page.goto("/ip/1");
  const pills = page.getByRole("navigation", { name: "内容类型" });
  await expect(pills.getByRole("button", { name: /全部/ })).toHaveAttribute("aria-pressed", "true");
  // chips 计数（type_counts 分面）：全部 9、图片 6
  await expect(pills.getByRole("button", { name: /全部/ })).toContainText("9");
  await expect(pills.getByRole("button", { name: /图片/ })).toContainText("5");

  await pills.getByRole("button", { name: /图片/ }).click();
  await expect(page).toHaveURL(/\/ip\/1\?tab=share&type=image/);
  await expect(pills.getByRole("button", { name: /图片/ })).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByText("星尘图片精选集")).toBeVisible();
  await expect(page.getByText("星尘视频日志")).toHaveCount(0);
});

test("in-IP search filters every module, shrinks tab counts, and back returns", async ({ page }) => {
  await prepare(page);
  const qEncoded = encodeURIComponent("视频");

  await page.goto("/ip/1");
  const tabs = page.getByRole("navigation", { name: "IP 模块" });
  // 基线计数：分享 9（works）、提案 2（跨状态）
  await expect(tabs.getByRole("button", { name: /内容分享/ })).toContainText("9");
  await expect(tabs.getByRole("button", { name: /提案投票/ })).toContainText("2");

  // 回车提交搜索（输入过程不即时过滤）
  const search = page.getByRole("search").getByLabel("在 IP 内搜索…");
  await search.fill("视频");
  await page.keyboard.press("Enter");

  await expect(page).toHaveURL(new RegExp(`tab=share.*q=${qEncoded}`));
  await expect(page.getByText("星尘视频日志")).toBeVisible();
  await expect(page.getByText("星尘图片精选集")).toHaveCount(0);
  // tab 计数收缩：分享命中 2（视频 2 / 图片 0），讨论命中 0，提案命中 0
  await expect(tabs.getByRole("button", { name: /内容分享/ })).toContainText("2");
  await expect(tabs.getByRole("button", { name: /讨论区/ })).toContainText("0");
  const pills = page.getByRole("navigation", { name: "内容类型" });
  await expect(pills.getByRole("button", { name: /视频/ })).toContainText("2");
  await expect(pills.getByRole("button", { name: /图片/ })).toContainText("0");

  // ?q= 走 history push：浏览器后退回到全量
  await page.goBack();
  await expect(page).not.toHaveURL(new RegExp(`q=${qEncoded}`));
  await expect(page.getByText("星尘图片精选集")).toBeVisible();
  await expect(tabs.getByRole("button", { name: /内容分享/ })).toContainText("9");
});

test("discussion cards open the detail overlay and Esc closes it", async ({ page }) => {
  await prepare(page);

  await page.goto("/ip/1?tab=discussions");
  await expect(page.getByText("公告：星尘共创守则")).toBeVisible();
  await expect(page.getByText("置顶")).toBeVisible();

  await page.getByRole("button", { name: /公告：星尘共创守则/ }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("置顶公告正文")).toBeVisible();
  await expect(dialog.getByText("支持守则")).toBeVisible();

  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
});

test("proposals tab renders open proposal cards with the governance scale", async ({ page }) => {
  await prepare(page);

  await page.goto("/ip/1?tab=proposals");
  await expect(page.getByText("把简介改为更完整的设定介绍")).toBeVisible();
  await expect(page.getByText("太空歌剧")).toBeVisible();
  await expect(page.getByText("1/10 票")).toBeVisible();
  // 未登录：展示关注引导文案而非投票按钮
  await expect(page.getByText("关注该 IP 后即可参与共治投票").first()).toBeVisible();
  await expect(page.getByRole("button", { name: /赞成/ })).toHaveCount(0);
});

test("legacy /ip/[id]/[category] and /ip/[id]/discussions routes redirect to the hub query form", async ({ page }) => {
  await prepare(page);

  await page.goto("/ip/1/image");
  await expect(page).toHaveURL(/\/ip\/1\?tab=share&type=image/);
  await expect(page.getByRole("navigation", { name: "内容类型" }).getByRole("button", { name: /图片/ })).toHaveAttribute("aria-pressed", "true");

  await page.goto("/ip/1/discussions");
  await expect(page).toHaveURL(/\/ip\/1\?tab=discussions$/);

  // 帖详情深链带 ?d=（浮层直开该帖）；/discussions/new 是发帖页路由，
  // 数字约束必须放行它（BLOCKER 回归锚点）
  await page.goto("/ip/1/discussions/99");
  await expect(page).toHaveURL(/\/ip\/1\?tab=discussions&d=99/);
  await page.goto("/ip/1/discussions/new");
  await expect(page).not.toHaveURL(/tab=discussions/);
});
