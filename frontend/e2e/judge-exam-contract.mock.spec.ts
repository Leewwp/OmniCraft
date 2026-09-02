import { expect, test, type Page } from "@playwright/test";

import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

const USER = {
  id: 11,
  email: "judge-candidate@example.com",
  username: "judge-candidate",
  avatar_url: "",
  bio: "",
  reputation: 60,
  preferred_locale: "zh",
  role: "user",
  is_banned: false,
  email_verified_at: "2026-01-01T00:00:00Z",
  created_at: "2026-01-01T00:00:00Z",
};

function examQuestions(count: number) {
  return Array.from({ length: count }, (_, index) => ({
    id: index + 1,
    question: {
      prompt: `契约测试第 ${index + 1} 题`,
      options: { A: "选项甲", B: "选项乙" },
    },
  }));
}

async function mockJudgeExamSession(page: Page, submitBodies: unknown[]) {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ tokens: { access_token: "judge-exam-mock-token" } }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/auth/me", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        user: USER,
        csrf_token: "mock-csrf",
        capabilities: { can_interact: true, interaction_denial_reason: null },
      }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/judge/exam/article", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ questions: examQuestions(3) }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/judge/exam/submit", async (route) => {
    submitBodies.push(route.request().postDataJSON());
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        record: { id: 1, score: 3, total: 3, passed: true },
        passed: true,
      }),
    });
  });
}

test("judge exam submit posts the backend answer contract (answer, not answer_key)", async ({ page }) => {
  const submitBodies: unknown[] = [];
  await mockJudgeExamSession(page, submitBodies);

  await page.context().addCookies([{ name: "NEXT_LOCALE", value: "zh", url: "http://127.0.0.1:3001" }]);
  await page.goto("/judge/exam");
  await page.getByRole("button", { name: "文章" }).click();
  await expect(page.getByText("契约测试第 1 题")).toBeVisible();

  // answer all three questions (option A each)
  for (let i = 0; i < 3; i += 1) {
    await page.getByRole("button", { name: /^A\./ }).click();
    if (i < 2) {
      await page.getByRole("button", { name: "下一题" }).click();
    }
  }
  await page.getByRole("button", { name: "提交答案" }).click();

  await expect(page.getByText("考核通过", { exact: true })).toBeVisible();
  expect(submitBodies).toHaveLength(1);
  const body = submitBodies[0] as {
    content_type: string;
    answers: Array<Record<string, unknown>>;
  };
  expect(body.content_type).toBe("article");
  expect(body.answers).toHaveLength(3);
  // FIX-01: the backend binds `answer`; the legacy `answer_key` field silently scored 0.
  for (const answer of body.answers) {
    expect(answer).toHaveProperty("answer");
    expect(answer).not.toHaveProperty("answer_key");
  }
});
