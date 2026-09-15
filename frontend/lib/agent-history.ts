import { normalizeAgentCitation } from "@/lib/agent";
import type { AgentStreamCitation, AgentStreamTool } from "@/lib/agent-stream";

/** 历史端点消息行（GET /api/v1/agent/conversations/:id 的 messages[]）。
 *  #538：phase="tools" 行带持久化工具步骤摘要（tools[]），与流式
 *  tool_status 事件同构；moderation 行由端点做内容脱敏（content 省略）。 */
export interface AgentHistoryMessageDTO {
  id: number;
  role: string;
  content?: string | null;
  phase?: string;
  moderation?: string;
  /** 端点原始引用（可能畸形；由 normalizeAgentCitation 剔除后才进渲染行）。 */
  citations?: unknown[];
  tools?: AgentStreamTool[];
}

/** 工作台渲染消息（历史回放与流式行内共用的最小形态）。 */
export interface AgentHistoryWorkspaceMessage {
  id: number;
  role: "user" | "assistant";
  content: string;
  /** A-02：think 行独立成消息（仅展示层）；#538：tools 行回放工具步骤条。 */
  phase?: "think" | "tools";
  moderationBlocked?: boolean;
  /** 2026-09-06 实测修复：引用随答案消息持久化（历史回放的跳转入口）。 */
  citations?: AgentStreamCitation[];
  /** #538：phase="tools" 行的步骤摘要（AgentToolStatus live=false 渲染）。 */
  tools?: AgentStreamTool[];
}

/** 服务端历史行 → 工作台渲染行。
 *  - think/tools phase 行按原顺序保留（与流式轮内渲染同构）；
 *  - blocked 行替换为占位文案（moderationBlocked 标记）；
 *  - 空内容非 phase 行剔除（no_evidence/degraded 撤答不留空泡）。 */
export function mapAgentHistoryMessages(
  messages: AgentHistoryMessageDTO[],
  moderationPlaceholder: string,
): AgentHistoryWorkspaceMessage[] {
  return messages
    .filter(
      (message) =>
        message.role === "user" ||
        message.phase === "think" ||
        message.phase === "tools" ||
        message.moderation === "blocked" ||
        (message.content ?? "").trim() !== "",
    )
    .map((message): AgentHistoryWorkspaceMessage => {
      if (message.moderation === "blocked") {
        return {
          id: message.id,
          role: "assistant",
          content: moderationPlaceholder,
          moderationBlocked: true,
        };
      }
      if (message.phase === "think") {
        return {
          id: message.id,
          role: "assistant",
          content: message.content ?? "",
          phase: "think",
        };
      }
      if (message.phase === "tools") {
        return {
          id: message.id,
          role: "assistant",
          content: "",
          phase: "tools",
          tools: message.tools ?? [],
        };
      }
      const validCitations = (message.citations ?? []).filter(
        (citation): citation is AgentStreamCitation => normalizeAgentCitation(citation) !== null,
      );
      return {
        id: message.id,
        role: message.role === "user" ? "user" : "assistant",
        content: message.content ?? "",
        ...(validCitations.length > 0 ? { citations: validCitations } : {}),
      };
    });
}
