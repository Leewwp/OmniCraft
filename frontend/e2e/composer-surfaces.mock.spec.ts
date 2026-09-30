import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { expect, test, type Page } from "@playwright/test";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

/* #413 F6a 公共 Composer 浏览器冒烟（mocked）：4 处聊天式输入面全部为
   内嵌按钮形态，按键语义按显式模式落地，自动增高上限转内部滚动，
   提交链路真发到端点。isComposing 防护由 tests/composer.test.tsx 单测
   锁定（真 IME 无法在合成键盘中产生）。 */

const USER = { id: 42, username: "Ada", role: "user", avatar_url: "" };

const CONVERSATIONS = [
  {
    /* 参与方必须含当前登录用户（42=Ada）：ConversationList 以
       participants.find(p => p.id !== user.id) 取对方——不含自己时
       会把第一位误当对方渲染（t97 同理：其 user id=1 在列）。 */
    id: 42,
    participants: [
      { id: 42, username: "Ada", avatar_url: "" },
      { id: 2, username: "bob", avatar_url: "" },
    ],
    last_message: { id: 20, text: "你好", sender_id: 2, created_at: "2026-09-01T00:00:00Z" },
    unread_count: 0,
    updated_at: "2026-09-01T00:00:00Z",
  },
];

const POSTS: string[] = [];

let server: ReturnType<typeof createServer> | null = null;

test.beforeAll(async () => {
  server = createServer(handleSsrApi);
  await new Promise<void>((resolve, reject) => {
    server?.once("error", reject);
    server?.listen(18_080, "127.0.0.1", () => resolve());
  });
});

test.afterAll(async () => {
  await new Promise<void>((resolve, reject) => {
    if (!server) return resolve();
    server.close((error) => (error ? reject(error) : resolve()));
  });
  server = null;
});

function handleSsrApi(req: IncomingMessage, res: ServerResponse) {
  const url = new URL(req.url ?? "/", "http://127.0.0.1:18080");
  if (url.pathname === "/api/v1/contents/601") {
    return json(res, 200, {
      content: {
        id: 601,
        title: "Composer 冒烟内容",
        description: "评论区 Composer 冒烟",
        body: "正文。",
        content_type: "article",
        category: "art",
        zone: "fanwork",
        status: "published",
        author: { id: 43, username: "Bob" },
        created_at: "2026-07-01T00:00:00Z",
      },
      attachments: [],
      tags: [],
    });
  }
  if (url.pathname === "/api/v1/ips/1") {
    return json(res, 200, {
      ip: { id: 1, name: "星尘计划", description: "composer smoke", category: "anime", tags: [], follower_count: 3, is_following: false },
      stats: { follower_count: 3, discussion_count: 12, work_count: 9 },
    });
  }
  if (url.pathname === "/api/v1/ips/1/contents") {
    return json(res, 200, { contents: [], total: 0, type_counts: {} });
  }
  if (url.pathname === "/api/v1/ips/1/discussions") {
    return json(res, 200, {
      discussions: [
        { id: 11, title: "公告", body: "正文", reply_count: 1, view_count: 3, created_at: "2026-08-01T00:00:00Z", author: { id: 2, username: "alice" } },
      ],
      total: 1,
    });
  }
  if (url.pathname === "/api/v1/ips/1/proposals") {
    return json(res, 200, { proposals: [], total: 0, min_votes: 10, pass_threshold: 0.6 });
  }
  if (url.pathname === "/api/v1/discussions/11") {
    return json(res, 200, {
      discussion: { id: 11, title: "公告", body: "正文", created_at: "2026-08-01T00:00:00Z", author: { id: 2, username: "alice" } },
      comments: [],
    });
  }
  if (url.pathname === "/api/v1/messages") {
    return json(res, 200, { conversations: CONVERSATIONS });
  }
  if (url.pathname === "/api/v1/messages/7") {
    return json(res, 200, {
      messages: [
        { id: 1, sender_id: 43, text: "你好", created_at: "2026-09-01T00:00:00Z" },
      ],
    });
  }
  return json(res, 404, { code: "NOT_FOUND", message: url.pathname });
}

function json(res: ServerResponse, status: number, body: unknown) {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(body));
}

const FULL_USER = {
  id: 42,
  email: "ada@example.com",
  username: "Ada",
  avatar_url: "",
  bio: "",
  reputation: 10,
  preferred_locale: "zh",
  role: "user",
  is_banned: false,
  email_verified_at: "2026-01-01T00:00:00Z",
};

async function login(page: Page) {
  await mockPublicApis(page);
  /* 无 token 启动时 AuthContext 先 refresh 成功拿 token，再拉 me（collab 同款）。 */
  await mockApiRoute(page, "**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ tokens: { access_token: "test-composer-token" } }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/auth/me", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ user: FULL_USER, capabilities: { can_interact: true, interaction_denial_reason: "" } }),
    }),
  );
  /* 消息中心 B 站式改造（#509）后页面还拉通知未读数与通知列表——
     缺 mock 会被 api-guard 中止并让页面落 404 兜底（t97 同款补齐）。 */
  await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      /* 响应形状与 t97 一致：unread_counts 包装（裸形状会让消费端崩页）。 */
      body: JSON.stringify({ unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/notifications**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ notifications: [], total: 0 }),
    }),
  );
  /* 消息中心（#509）私信三栏会拉对话方的用户画像（可能跨 API origin）——
     失败会让页面落 404 兜底。 */
  await mockApiRoute(page, "**/api/v1/users/43", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ user: { id: 43, username: "Bob", avatar_url: "" } }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/users/42", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ user: FULL_USER }),
    }),
  );
}

async function mockSubmitCapture(page: Page, pattern: string) {
  await mockApiRoute(page, pattern, (route) => {
    POSTS.push(route.request().url());
    return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
  });
}

test.beforeEach(() => {
  POSTS.length = 0;
});

test("discussion reply composer: multi-line grow, Enter submits, button anchored bottom-right", async ({ page }) => {
  await login(page);
  await mockSubmitCapture(page, "**/api/v1/discussions/11/comments");
  await mockApiRoute(page, "**/api/v1/ips/1", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        ip: { id: 1, name: "星尘计划", description: "composer smoke", category: "anime", tags: [], follower_count: 3, is_following: false },
        stats: { follower_count: 3, discussion_count: 12, work_count: 9 },
      }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/ips/1/discussions**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        discussions: [
          { id: 11, title: "公告", body: "正文", reply_count: 1, view_count: 3, created_at: "2026-08-01T00:00:00Z", author: { id: 2, username: "alice" } },
        ],
        total: 1,
      }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/discussions/11", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        discussion: { id: 11, title: "公告", body: "正文", created_at: "2026-08-01T00:00:00Z", author: { id: 2, username: "alice" } },
        comments: [],
      }),
    }),
  );

  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/ip/1?tab=discussions&d=11");
  const textarea = page.locator("textarea").last();
  await expect(textarea).toBeVisible({ timeout: 15_000 });

  /* 内嵌按钮：位于 wrapper 内右下角。 */
  const wrap = page.locator("textarea").last().locator("xpath=..");
  const button = wrap.getByRole("button");
  await expect(button).toBeVisible();

  /* 多行增长 + Shift+Enter 换行不提交。 */
  await textarea.fill("第一行");
  await textarea.press("Shift+Enter");
  await page.keyboard.type("第二行");
  const grownHeight = await textarea.evaluate((el) => el.getBoundingClientRect().height);
  expect(grownHeight).toBeGreaterThan(32);

  /* 大量文本：封顶 208px 转内部滚动。 */
  await textarea.fill("很长的内容\n".repeat(40));
  await page.waitForTimeout(120);
  const capped = await textarea.evaluate((el) => ({
    height: el.getBoundingClientRect().height,
    overflow: getComputedStyle(el).overflowY,
  }));
  expect(capped.height).toBeLessThanOrEqual(209);
  expect(["auto", "scroll"]).toContain(capped.overflow);

  /* Enter 提交（keyMode=enter）：POST 到讨论评论端点。 */
  await textarea.fill("冒烟回复");
  await textarea.press("Enter");
  await expect
    .poll(() => POSTS.filter((u) => u.includes("/discussions/11/comments")).length)
    .toBeGreaterThan(0);
  await page.screenshot({ path: "../screenshots/413-discussion-reply.png", fullPage: false });
});

test("top-level comment composer: Ctrl/Cmd+Enter submits, bare Enter adds a line", async ({ page }) => {
  await login(page);
  /* 单一 handler 兼管 GET 列表与 POST 提交捕获——若拆两个 pattern，
     后注册的宽 pattern 会遮蔽先注册的捕获 handler（Playwright LIFO）。 */
  const SEED_COMMENT = {
    id: 501,
    author_id: 43,
    author: { id: 43, username: "Bob" },
    body: "既有评论（一级回复冒烟种子）",
    parent_id: null,
    like_count: 0,
    dislike_count: 0,
    created_at: "2026-09-01T00:00:00Z",
  };
  await mockApiRoute(page, "**/api/v1/social/comments**", (route) => {
    if (route.request().method() === "GET") {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ comments: [SEED_COMMENT], total: 1 }),
      });
    }
    POSTS.push(route.request().url());
    /* 回真实形状：submit/submitReply 会把 data.comment 追加进列表渲染，
       回 {} 会让 undefined 混进列表炸渲染。 */
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        comment: {
          id: 601,
          author_id: 42,
          author: { id: 42, username: "Ada", avatar_url: "" },
          body: "冒烟提交",
          parent_id: null,
          like_count: 0,
          dislike_count: 0,
          created_at: "2026-09-08T00:00:00Z",
        },
      }),
    });
  });

  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/content/601");
  /* 评论区在浮窗底部（关联块之后）：滚入视口再断言。 */
  const commentHeading = page.getByRole("heading", { name: /Comments/i }).first();
  await commentHeading.scrollIntoViewIfNeeded();
  const commentBox = page.locator("textarea").first();
  await expect(commentBox).toBeVisible({ timeout: 15_000 });

  await commentBox.fill("冒烟评论");
  /* 裸 Enter（keyMode=ctrl-enter）只换行，不提交。 */
  await commentBox.press("Enter");
  await page.waitForTimeout(250);
  expect(POSTS.filter((u) => u.includes("/social/comments")).length).toBe(0);

  /* Cmd/Ctrl+Enter 提交。 */
  await commentBox.press("ControlOrMeta+Enter");
  await expect
    .poll(() => POSTS.filter((u) => u.includes("/social/comments")).length)
    .toBeGreaterThan(0);
  await page.screenshot({ path: "../screenshots/413-comment-top-level.png", fullPage: false });

  /* 一级回复（keyMode=button-only）：展开后 Enter 不提交、仅按钮提交。 */
  const replyToggle = page.getByRole("button", { name: "Reply", exact: true }).first();
  await replyToggle.scrollIntoViewIfNeeded();
  await replyToggle.click();
  const replyBox = page.locator("textarea").nth(1);
  await expect(replyBox).toBeVisible({ timeout: 15_000 });
  await replyBox.fill("冒烟一级回复");
  await replyBox.press("Enter");
  await page.waitForTimeout(250);
  const postsBeforeButtonClick = POSTS.length;
  /* button-only 模式：Enter 不得提交（此前仅 1 笔顶层评论 POST）。 */
  expect(postsBeforeButtonClick).toBe(1);
  const replyWrap = replyBox.locator("xpath=..");
  await page.screenshot({ path: "../screenshots/413-comment-reply.png", fullPage: false });
  await replyWrap.getByRole("button").click();
  await expect
    .poll(() => POSTS.filter((u) => u.includes("/social/comments")).length)
    .toBeGreaterThan(1);
});

test("private chat composer: Enter sends, Shift+Enter keeps a newline", async ({ page }) => {
  await login(page);
  await mockApiRoute(page, "**/api/v1/messages", (route) => {
    if (route.request().method() === "GET") {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        /* 分页 meta 与 t97 同款（#509 后列表消费 page/page_size）。 */
        body: JSON.stringify({ conversations: CONVERSATIONS, page: 1, page_size: 20 }),
      });
    }
    POSTS.push(route.request().url());
    return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
  });
  /* 会话历史 GET /api/v1/messages/42；私信发送 POST 实际打 /api/v1/messages
     （ChatWindow sendMessage 契约），由上面的 handler 捕获。 */
  await mockApiRoute(page, "**/api/v1/messages/42", (route) => {
    if (route.request().method() !== "GET") {
      POSTS.push(route.request().url());
      return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
    }
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        messages: [{ id: 1, sender_id: 2, text: "你好", created_at: "2026-09-01T00:00:00Z" }],
        total: 1,
      }),
    });
  });

  await page.setViewportSize({ width: 1440, height: 900 });
  /* B 站式布局（#509）：默认分类页 → 选「私信」→ 会话列表 → 开会话 42
     （与 t97 已验证的导航流一致；zh cookie 与 t97 同款——本文件其余用例
     的既有文案断言（Comments/Reply 等）在 en 下运行，仅本用例走中文 UI）。 */
  await page.context().addCookies([{ name: "NEXT_LOCALE", value: "zh", path: "/", domain: "127.0.0.1" }]);
  await page.goto("/messages");
  await page.getByRole("button", { name: "私信", exact: true }).click();
  await page.waitForTimeout(400);
  await page.getByRole("button", { name: /bob/ }).first().click();
  /* 桌面双栏与移动分屏各渲染一个 ChatWindow；.last() 会选中
     min-[701px]:hidden 分屏里隐藏的输入框，取首个可见实例。 */
  const chatBox = page.locator("textarea").first();
  await expect(chatBox).toBeVisible({ timeout: 15_000 });

  await chatBox.fill("冒烟私信");
  await chatBox.press("Shift+Enter");
  await page.waitForTimeout(200);
  /* Shift+Enter 只换行不发送。 */
  expect(POSTS.length).toBe(0);

  /* ChatWindow 消息轮询会换节点重挂载，Enter 可能落在已卸载的旧输入框上
     （整键序列丢失、POST 不发）——fill+Enter+断言整体 toPass 重试。 */
  await expect(async () => {
    const box = page.locator("textarea").first();
    await box.click();
    await box.press("Enter");
    expect(POSTS.filter((u) => u.endsWith("/api/v1/messages")).length).toBeGreaterThan(0);
  }).toPass({ timeout: 15_000 });
  await page.screenshot({ path: "../screenshots/413-private-chat.png", fullPage: false });
});
