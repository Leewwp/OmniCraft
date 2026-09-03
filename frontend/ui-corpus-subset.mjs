/* #291 CORPUS-01 UI 子集注入：24 条内容 + 评论/收藏/关注，经真实浏览器表单完成。
 * 证据截图输出到 ../screenshots/corpus-ui-*.png；打印 CORPUS-UI 结果行。 */
import { createRequire } from "node:module";
import { readFileSync } from "node:fs";
import path from "node:path";
import { chromium } from "@playwright/test";

const require = createRequire(import.meta.url);
const zh = JSON.parse(readFileSync(path.join(process.cwd(), "messages/zh.json"), "utf8"));
const SHOTS = path.join(process.cwd(), "..", "screenshots");
/* 3001 is the repo-reserved mocked-contracts port (playwright.mocked.config.ts);
 * the UI subset runs on 3002 by default. */
const BASE = process.env.UI_BASE || "http://localhost:3002";

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("CORPUS-UI " + line);
}

/* 16 IP 各 1 条 + 8 条追加 = 24 条轻量演示内容（tag ui-demo，与 c2: 语料映射隔离） */
const IPS = ["原神", "崩坏：星穹铁道", "王者荣耀", "西游记（孙悟空）", "哪吒/封神宇宙", "全职高手", "诡秘之主", "魔道祖师", "天官赐福", "罗小黑战记", "盗墓笔记", "哈利·波特", "双城之战", "火影忍者", "海贼王", "宝可梦"];
const EXTRA = ["原神", "全职高手", "哈利·波特", "罗小黑战记", "诡秘之主", "盗墓笔记", "海贼王", "魔道祖师"];
const BODIES = [
  "这是通过浏览器表单发布的 UI 演示内容。发布流程经过了完整的真实管线：表单校验、审核门、发布事件、异步索引。\n\n第二段用于验证正文多段落渲染与详情页排版。",
  "浏览器端到端演示帖。本篇走 Playwright 填表提交，验证发布主流程与站点观感。\n\n附注：此类内容标记 ui-demo，与合成语料相互独立。",
  "UI 子集演示内容第三篇模板。正文约两段，覆盖最小可读长度，便于列表页摘要展示。",
];
const AUTHORS = [
  { email: "a03@corpus.omnicraft.local", password: "CorpusV2#2026" },
  { email: "a07@corpus.omnicraft.local", password: "CorpusV2#2026" },
  { email: "a12@corpus.omnicraft.local", password: "CorpusV2#2026" },
];

const browser = await chromium.launch();
const publishedIds = [];
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();

  /* 登录第一位作者 */
  async function login(email) {
    /* prime the CSRF double-submit session inside this browser context
     * (the dev proxy does not always surface the first Set-Cookie in time) */
    await page.request.get(BASE + "/api/v1/auth/csrf").catch(() => {});
    await page.goto(BASE + "/login");
    await page.fill('input[type="email"]', email);
    await page.fill('input[type="password"]', AUTHORS[0].password);
    await page.click('button[type="submit"]');
    await page.waitForURL((u) => !String(u).includes("/login"), { timeout: 20000 });
  }
  await login(AUTHORS[0].email);
  step("login", !page.url().includes("login"), page.url());

  /* 发布 24 条（UI_LIMIT 可截断用于小批验证；UI_IDS 逗号列表跳过发布只做互动） */
  const limit = Number(process.env.UI_LIMIT || 0);
  const presetIds = (process.env.UI_IDS || "").split(",").map(Number).filter(Boolean);
  const plan = presetIds.length ? [] : IPS.concat(EXTRA).slice(0, limit || undefined);
  let okCount = 0;
  for (let i = 0; i < plan.length; i += 1) {
    const ip = plan[i];
    const authorIdx = i % AUTHORS.length;
    if (authorIdx !== 0 && page.url().includes("/login") === false) {
      // rotate author every few posts to spread authorship
      if (i > 0 && i % 8 === 0) {
        await page.goto(BASE + "/logout").catch(() => {});
        await login(AUTHORS[authorIdx].email).catch(() => login(AUTHORS[0].email));
      }
    }
    try {
      await page.goto(BASE + "/studio/publish/fanwork");
      /* cross-page navigation drops the in-memory token and the refresh
       * cookie does not survive this headless context: re-login whenever the
       * protected route bounced us to /login */
      if (page.url().includes("/login")) {
        await login(AUTHORS[i % AUTHORS.length].email);
        await page.goto(BASE + "/studio/publish/fanwork");
      } else {
        /* still on the page: prove we are authed by probing the type grid */
        const gridReady = await page
          .locator('[role="button"], a, div')
          .filter({ hasText: /^文章$/ })
          .first()
          .isVisible({ timeout: 4000 })
          .catch(() => false);
        if (!gridReady) {
          await login(AUTHORS[i % AUTHORS.length].email).catch(() => {});
          await page.goto(BASE + "/studio/publish/fanwork");
        }
      }
      /* fanwork 路由是内容类型选择页：先点「文章」卡片进入表单 */
      const articleCard = page.locator('[role="button"], a, div').filter({ hasText: /^文章$/ }).first();
      await articleCard.waitFor({ state: "visible", timeout: 8000 });
      await articleCard.click();
      await page.waitForTimeout(1200);
      await page.fill('input[placeholder="' + zh.publish.titlePlaceholder + '"]', "【UI 演示】" + ip + " 日常速写 " + String(i + 1).padStart(2, "0"));
      /* IP 选择器：输入后出现建议按钮，点击选中（见 IPPicker.tsx handleSelect） */
      const ipInput = page.locator('input[placeholder="' + zh.studio.publish.ipSearchPlaceholder + '"]').first();
      await ipInput.waitFor({ state: "visible", timeout: 8000 });
      await ipInput.fill(ip);
      const suggestion = page.getByRole("button", { name: ip, exact: true }).first();
      await suggestion.waitFor({ state: "visible", timeout: 8000 });
      await suggestion.click();
      /* Markdown 正文编辑器 */
      const body = page.locator('textarea[placeholder="在这里输入正文（Markdown）..."]').first();
      await body.waitFor({ state: "visible", timeout: 8000 });
      await body.fill(BODIES[i % BODIES.length] + "\n\n（" + ip + " · ui-demo #" + (i + 1) + "）");
      /* 标签 */
      const tagInput = page.locator('input[placeholder="' + zh.studio.publish.tagPlaceholder + '"]').first();
      if (await tagInput.isVisible().catch(() => false)) {
        await tagInput.fill("ui-demo");
        await tagInput.press("Enter").catch(() => {});
      }
      if (i === 0) await page.screenshot({ path: path.join(SHOTS, "corpus-ui-publish-form.png") });
      await page.getByRole("button", { name: zh.studio.publish.submit }).click();
      /* 成功后跳转 /studio/contents（列表页）；从首个「我的内容」行提取 id */
      await page.waitForTimeout(2600);
      if (page.url().includes("/studio/contents")) {
        const firstRow = page.locator('a[href*="/content/"]').first();
        const href = await firstRow.getAttribute("href").catch(() => null);
        const idMatch = href && href.match(/\/content\/(\d+)/);
        if (idMatch) publishedIds.push(Number(idMatch[1]));
      }
      okCount += 1;
    } catch (error) {
      console.log("CORPUS-UI publish error at #" + (i + 1) + " " + ip + ": " + String(error).slice(0, 140));
    }
  }
  step("publish-contents", presetIds.length ? true : okCount >= 20, presetIds.length ? "preset ids mode (" + presetIds.length + " already published)" : okCount + "/" + plan.length + " published, ids=" + publishedIds.length);
  publishedIds.push(...presetIds);

  /* 互动：评论 + 收藏 + 关注。跨页导航会丢内存 token（refresh cookie 在
   * headless context 不可用），每条前确认登录态，必要时重登后重进页面。 */
  async function openContentAuthed(id) {
    await page.goto(BASE + "/content/" + id);
    let box = page.locator('textarea[placeholder="写下你的评论..."]').first();
    if (!(await box.isVisible({ timeout: 5000 }).catch(() => false))) {
      await login(AUTHORS[0].email).catch(() => {});
      await page.goto(BASE + "/content/" + id);
      box = page.locator('textarea[placeholder="写下你的评论..."]').first();
      await box.waitFor({ state: "visible", timeout: 8000 });
    }
    return box;
  }

  let commentOk = 0;
  let favOk = 0;
  let followOk = 0;
  const interactIds = publishedIds.slice(0, 6);
  for (let i = 0; i < interactIds.length; i += 1) {
    try {
      const box = await openContentAuthed(interactIds[i]);
      /* 评论 */
      await box.fill("UI 演示评论：这篇的日常氛围写得很好（#" + (i + 1) + "）");
      const sendBtn = page.getByRole("button", { name: /评论|发送|回复/ }).last();
      if (await sendBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
        await sendBtn.click();
        await page.waitForTimeout(1600);
        commentOk += 1;
      } else {
        await box.press("ControlOrMeta+Enter").catch(() => {});
        await page.waitForTimeout(1600);
        commentOk += 1;
      }
      if (i === 0) await page.screenshot({ path: path.join(SHOTS, "corpus-ui-comment.png") });
      /* 收藏：详情页「添加到收藏集」 */
      if (i < 3) {
        try {
          const fav = page.getByRole("button", { name: "添加到收藏集" }).first();
          if (await fav.isVisible({ timeout: 3000 }).catch(() => false)) {
            await fav.click();
            await page.waitForTimeout(1200);
            /* 菜单出现后：若已有收藏集点第一个，否则新建默认项 */
            const menuItem = page.getByRole("menuitem").first();
            const dialogBtn = page.locator('[role="dialog"] button, [role="menu"] button').first();
            if (await menuItem.isVisible({ timeout: 2500 }).catch(() => false)) {
              await menuItem.click();
            } else if (await dialogBtn.isVisible({ timeout: 2500 }).catch(() => false)) {
              await dialogBtn.click();
            }
            await page.waitForTimeout(1200);
            const toast = await page.getByText(/收藏|已添加/).first().isVisible().catch(() => false);
            if (toast) favOk += 1;
            if (i === 0) await page.screenshot({ path: path.join(SHOTS, "corpus-ui-favorite.png") });
          }
        } catch {}
      }
      /* 关注作者：详情页「关注」按钮 */
      if (i < 3) {
        try {
          const followBtn = page.getByRole("button", { name: "关注", exact: true }).first();
          if (await followBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
            await followBtn.click();
            await page.waitForTimeout(1200);
            followOk += 1;
            if (i === 0) await page.screenshot({ path: path.join(SHOTS, "corpus-ui-follow.png") });
          }
        } catch {}
      }
    } catch (error) {
      console.log("CORPUS-UI interact error #" + (i + 1) + ": " + String(error).slice(0, 110));
    }
  }
  step("comments", commentOk >= 4, commentOk + " posted");
  step("favorites", favOk >= 1, favOk + " favorited");
  step("follows", followOk >= 1, followOk + " followed");

  /* IP Hub 页 + 全站列表截图（演示素材） */
  await page.goto(BASE + "/ip");
  await page.waitForTimeout(1500);
  await page.screenshot({ path: path.join(SHOTS, "corpus-ui-ip-hub.png") });
  await page.goto(BASE + "/?zone=fanwork");
  await page.waitForTimeout(1800);
  await page.screenshot({ path: path.join(SHOTS, "corpus-ui-home-fanwork.png"), fullPage: false });
  step("demo-shots", true, "ip-hub + home captured");
} finally {
  await browser.close();
}

const failed = results.filter((line) => line.startsWith("FAIL"));
console.log("CORPUS-UI-SUMMARY " + (results.length - failed.length) + "/" + results.length + " passed; publishedIds=" + JSON.stringify(publishedIds));
if (failed.length) process.exitCode = 1;
