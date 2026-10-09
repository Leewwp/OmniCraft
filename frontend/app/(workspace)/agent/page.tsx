"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { AgentWorkspaceGate } from "@/components/agent/AgentWorkspaceGate";

/* A-07：搜索页「问 AI 助手」入口经 /agent?q= 预填首轮问题（仅首挂载生效）。
 * useSearchParams 需要 Suspense 边界（CSR bailout）。#854：双态壳接管
 * 身份判定（登录用户 / 游客 / 登录引导），本页不再直接渲染工作台。 */
function AgentWorkspacePanel() {
  const searchParams = useSearchParams();
  const initialQuery = (searchParams.get("q") || "").trim();
  return <AgentWorkspaceGate initialQuery={initialQuery || undefined} />;
}

export default function AgentPage() {
  return (
    <div className="flex min-h-dvh flex-col">
      <Suspense fallback={null}>
        <AgentWorkspacePanel />
      </Suspense>
    </div>
  );
}
