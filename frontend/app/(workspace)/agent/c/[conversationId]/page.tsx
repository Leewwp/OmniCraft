"use client";

import { useParams } from "next/navigation";

import { AgentWorkspaceGate } from "@/components/agent/AgentWorkspaceGate";

/**
 * FT-3（#695）会话路由化：每个会话拥有稳定、可直达的独立 URL。
 * key={id} 强制跨会话重挂载——initialConversationId 仅挂载时生效
 * （useState 初始化器语义），URL 切换即导航即重挂载。
 * #854：双主体壳——游客深链本人游客会话同样回放；他人/过期会话在
 * AgentWorkspace 内 404/410 → replace("/agent")。
 */
export default function AgentConversationPage() {
  const params = useParams<{ conversationId: string }>();
  const raw = params?.conversationId;
  const id = typeof raw === "string" ? Number(raw) : Number.NaN;

  if (!Number.isSafeInteger(id) || id <= 0) {
    /* 非法 id（非正整数）视同空态入口渲染；不产出会话请求。 */
    return <AgentWorkspaceGate key="invalid" />;
  }
  return <AgentWorkspaceGate key={id} initialConversationId={id} />;
}
