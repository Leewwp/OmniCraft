/* T40（FIX-36d）：ReviewCard 受控内容预览——案件切换（跳过/投票后前进）时
 * 预览态必须整体重置：父组件复用同一实例（无 key），残留上一案的已加载内容
 * 会造成张冠李戴（比盲投更危险）。 */
import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { act, cleanup, fireEvent, render } from "./runtime-test-helpers";

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;

/* api stub：按 content id 返回标题可区分的内容；Toast/错误上报打桩隔离。 */
const apiGetCalls: string[] = [];
Module._load = function loadWithStub(request, parent, isMain) {
  if (request === "@/lib/api") {
    return {
      api: {
        get: async (url: string) => {
          apiGetCalls.push(url);
          return {
            content: {
              id: url.endsWith("8001") ? 8001 : 8002,
              title: url.endsWith("8001") ? "Preview Content A" : "Preview Content B",
              description: "desc",
              content_type: "article",
            },
            attachments: [],
          };
        },
        post: async () => ({}),
      },
      ApiRequestError: class ApiRequestError extends Error {},
    };
  }
  if (request === "@/components/ui/Toast") {
    return { useToast: () => ({ toast: () => undefined }) };
  }
  if (request === "@/lib/user-facing-error") {
    return { getUserFacingErrorKey: () => "common.error" };
  }
  if (request === "@/lib/error-handler") {
    return { silentError: () => undefined };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

const { default: ReviewCard } = {} as typeof import("@/components/judge/ReviewCard");

const baseCase = (id: number, targetId: number) => ({
  id,
  target_type: "article",
  target_id: targetId,
  vote_approve: 0,
  vote_reject: 0,
  min_votes: 10,
  status: "open" as const,
  created_at: new Date().toISOString(),
});

function renderCard(
  ReviewCard: React.ComponentType<{
    judgeCase: ReturnType<typeof baseCase>;
    disabled: boolean;
    submitting: boolean;
    onVote: () => void;
  }>,
  judgeCase: ReturnType<typeof baseCase>
) {
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <ReviewCard judgeCase={judgeCase} disabled={false} submitting={false} onVote={() => undefined} />
    </IntlProvider>
  );
}

test("switching case resets controlled preview to idle (no stale content)", async () => {
  const { default: ReviewCard } = await import("@/components/judge/ReviewCard");
  const { container, rerender } = renderCard(ReviewCard, baseCase(4201, 8001));

  const loadButton = () =>
    Array.from(container.querySelectorAll("button")).find((b) =>
      b.textContent?.includes("View content")
    );

  /* 案件 A：点击后加载内容本体 */
  assert.ok(loadButton(), "案件 A 应有「View content」按钮（idle 态）");
  await act(async () => {
    fireEvent.click(loadButton()!);
  });
  assert.ok(container.textContent?.includes("Preview Content A"), "点击后显示 A 内容");

  /* 父组件复用实例切换到案件 B（等价 onSkip 后 currentIndex 前进） */
  rerender(
    <IntlProvider locale="en" messages={enMessages}>
      <ReviewCard judgeCase={baseCase(4202, 8002)} disabled={false} submitting={false} onVote={() => undefined} />
    </IntlProvider>
  );

  assert.ok(
    !container.textContent?.includes("Preview Content A"),
    "切换案件后不得残留上一案内容（张冠李戴）"
  );
  assert.ok(loadButton(), "切换案件后预览应回到 idle（重新出现「View content」）");

  /* B 案件重新点击加载的是 B 的内容（重新发起请求） */
  await act(async () => {
    fireEvent.click(loadButton()!);
  });
  assert.ok(container.textContent?.includes("Preview Content B"), "B 案件加载 B 内容");
  assert.ok(apiGetCalls.some((u) => u.endsWith("/8002")), "B 案件请求指向 target 8002");

  cleanup();
  Module._load = originalModuleLoad;
});
