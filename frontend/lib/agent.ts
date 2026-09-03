import type {
  AgentStreamCitation,
  AgentStreamEvent,
  AgentStreamTool,
} from "@/lib/agent-stream";

/**
 * ui-spec `## Page: /agent` AgentCitation（camelCase）—— 供引用卡片组件与
 * 共享 ContentDetailOverlay 接线使用。由服务端流式 citation 经 toAgentCitation
 * 转换而来，不允许直接以未校验数据构造。
 */
export interface AgentCitation {
  contentId: number;
  title: string;
  zone: "original" | "fanwork";
  excerpt?: string;
  contentVersion?: number;
  chunkKey?: string;
  chunkIndex?: number;
  route?: string;
  source?: "bm25" | "vector" | "hybrid_rrf";
}

export function toAgentCitation(citation: AgentStreamCitation): AgentCitation {
  const zone: AgentCitation["zone"] = citation.zone === "original" ? "original" : "fanwork";
  const normalized: AgentCitation = { contentId: citation.content_id, title: citation.title, zone };
  if (citation.excerpt !== undefined) normalized.excerpt = citation.excerpt;
  if (citation.content_version !== undefined) normalized.contentVersion = citation.content_version;
  if (citation.chunk_key !== undefined) normalized.chunkKey = citation.chunk_key;
  if (citation.chunk_index !== undefined) normalized.chunkIndex = citation.chunk_index;
  if (citation.route !== undefined) normalized.route = citation.route;
  if (citation.source !== undefined) normalized.source = citation.source;
  return normalized;
}

/**
 * 拒绝畸形 citation：content_id 必须为正整数，title 非空，zone 仅接受
 * original/fanwork。返回 null 时调用方必须丢弃该对象（不得渲染为可交互元素）。
 */
export function normalizeAgentCitation(raw: unknown): AgentStreamCitation | null {
  if (typeof raw !== "object" || raw === null) return null;
  const candidate = raw as Record<string, unknown>;
  const contentId = candidate.content_id;
  const title = candidate.title;
  const zone = candidate.zone;
  if (
    typeof contentId !== "number" ||
    !Number.isInteger(contentId) ||
    contentId <= 0 ||
    typeof title !== "string" ||
    title.trim() === "" ||
    typeof zone !== "string" ||
    (zone !== "original" && zone !== "fanwork")
  ) {
    return null;
  }
  const normalized: AgentStreamCitation = {
    content_id: contentId,
    title: title.trim(),
    zone,
  };
  const expandedFields = [
    candidate.content_version,
    candidate.chunk_key,
    candidate.chunk_index,
    candidate.route,
    candidate.source,
  ];
  const hasExpandedFields = expandedFields.some((field) => field !== undefined);
  if (hasExpandedFields && expandedFields.some((field) => field === undefined)) return null;
  const contentVersion = candidate.content_version;
  if (contentVersion !== undefined) {
    if (typeof contentVersion !== "number" || !Number.isInteger(contentVersion) || contentVersion <= 0) return null;
    normalized.content_version = contentVersion;
  }
  const chunkKey = candidate.chunk_key;
  if (chunkKey !== undefined) {
    if (typeof chunkKey !== "string" || !/^[0-9a-f]{64}$/.test(chunkKey)) return null;
    normalized.chunk_key = chunkKey;
  }
  const chunkIndex = candidate.chunk_index;
  if (chunkIndex !== undefined) {
    if (typeof chunkIndex !== "number" || !Number.isInteger(chunkIndex) || chunkIndex < 0) return null;
    normalized.chunk_index = chunkIndex;
  }
  const route = candidate.route;
  if (route !== undefined) {
    if (typeof route !== "string") return null;
    const expectedRoute = zone === "original" ? `/original/${contentId}` : `/content/${contentId}`;
    if (route !== expectedRoute) return null;
    normalized.route = route;
  }
  const source = candidate.source;
  if (source !== undefined) {
    if (source !== "bm25" && source !== "vector" && source !== "hybrid_rrf") return null;
    normalized.source = source;
  }
  const excerpt = candidate.excerpt;
  if (hasExpandedFields && (typeof excerpt !== "string" || excerpt.trim() === "")) return null;
  if (typeof excerpt === "string" && excerpt.trim() !== "") {
    normalized.excerpt = excerpt;
  }
  return normalized;
}

/**
 * 拒绝畸形 tool 事件：name 非空字符串，status 必须是服务端契约允许值
 * （success/error/skipped）或运行中状态（running）。duration_ms 仅接受
 * 非负有限数字。工具状态只承载展示所需字段，不含 raw args 或内部推理。
 */
export function normalizeAgentTool(raw: unknown): AgentStreamTool | null {
  if (typeof raw !== "object" || raw === null) return null;
  const candidate = raw as Record<string, unknown>;
  const name = candidate.name;
  const status = candidate.status;
  if (typeof name !== "string" || name.trim() === "") return null;
  if (
    status !== "running" &&
    status !== "success" &&
    status !== "failed" &&
    status !== "error" &&
    status !== "skipped"
  ) {
    return null;
  }
  const normalized: AgentStreamTool = { name: name.trim(), status };
  const argsSummary = candidate.args_summary;
  if (typeof argsSummary === "string" && argsSummary.trim() !== "") {
    normalized.args_summary = argsSummary.trim();
  }
  const hits = candidate.hits;
  if (typeof hits === "number" && Number.isInteger(hits) && hits >= 0) {
    normalized.hits = hits;
  }
  const durationMs = candidate.duration_ms;
  if (typeof durationMs === "number" && Number.isFinite(durationMs) && durationMs >= 0) {
    normalized.duration_ms = Math.floor(durationMs);
  }
  return normalized;
}

/**
 * SSE 事件类型化归一化入口：畸形 citation/tool 事件整体拒绝（返回 null），
 * done 事件内嵌的 citations/tools 逐条过滤后保留合法项。上游
 * parseAgentStreamLine 使用本函数，保证组件层永远拿不到未校验数据。
 */
export function normalizeAgentEvent(raw: unknown): AgentStreamEvent | null {
  if (typeof raw !== "object" || raw === null) return null;
  const candidate = raw as Record<string, unknown>;
  const type = candidate.type;
  if (typeof type !== "string") return null;

  switch (type) {
    case "start": {
      const event: { type: "start"; trace_id?: string; conversation_id?: number; answer_kind?: string } = {
        type: "start",
      };
      if (typeof candidate.trace_id === "string" && candidate.trace_id !== "") {
        event.trace_id = candidate.trace_id;
      }
      const conversationId = candidate.conversation_id;
      if (typeof conversationId === "number" && Number.isInteger(conversationId) && conversationId > 0) {
        event.conversation_id = conversationId;
      }
      const answerKind = candidate.answer_kind;
      if (typeof answerKind === "string" && answerKind !== "") event.answer_kind = answerKind;
      return event;
    }
    case "think_delta": {
      const delta = candidate.delta;
      return typeof delta === "string" ? { type: "think_delta", delta } : null;
    }
    case "tool_status": {
      const tool = normalizeAgentTool(candidate.tool);
      return tool ? { type: "tool_status", tool } : null;
    }
    case "delta": {
      const delta = candidate.delta;
      return typeof delta === "string" ? { type: "delta", delta } : null;
    }
    case "citation": {
      const citation = normalizeAgentCitation(candidate.citation);
      return citation ? { type: "citation", citation } : null;
    }
    case "usage": {
      const usage = candidate.usage;
      return typeof usage === "object" && usage !== null ? { type: "usage", usage } : null;
    }
    case "done": {
      const event: { type: "done" } & AgentStreamEvent = { type: "done" };
      const traceId = candidate.trace_id;
      if (typeof traceId === "string" && traceId !== "") event.trace_id = traceId;
      const conversationId = candidate.conversation_id;
      if (typeof conversationId === "number" && Number.isInteger(conversationId) && conversationId > 0) {
        event.conversation_id = conversationId;
      }
      const messageId = candidate.message_id;
      if (typeof messageId === "number" && Number.isInteger(messageId) && messageId > 0) {
        event.message_id = messageId;
      }
      const answerKind = candidate.answer_kind;
      if (typeof answerKind === "string" && answerKind !== "") event.answer_kind = answerKind;
      const answer = candidate.answer;
      if (typeof answer === "string") event.answer = answer;
      if (Array.isArray(candidate.citations)) {
        event.citations = candidate.citations
          .map(normalizeAgentCitation)
          .filter((citation): citation is AgentStreamCitation => citation !== null);
      }
      if (Array.isArray(candidate.tools)) {
        event.tools = candidate.tools
          .map(normalizeAgentTool)
          .filter((tool): tool is AgentStreamTool => tool !== null);
      }
      const usage = candidate.usage;
      if (typeof usage === "object" && usage !== null) {
        const tokens = usage as Record<string, unknown>;
        if (typeof tokens.prompt_tokens === "number" && typeof tokens.completion_tokens === "number") {
          event.usage = {
            prompt_tokens: tokens.prompt_tokens,
            completion_tokens: tokens.completion_tokens,
          };
        }
      }
      if (typeof candidate.degraded === "boolean") event.degraded = candidate.degraded;
      return event;
    }
    case "error": {
      const event: Extract<AgentStreamEvent, { type: "error" }> = { type: "error" };
      const errorCode = candidate.error_code;
      if (typeof errorCode === "string" && errorCode !== "") event.error_code = errorCode;
      const errorMessage = candidate.error_message;
      if (typeof errorMessage === "string" && errorMessage !== "") event.error_message = errorMessage;
      if (candidate.degraded === true && candidate.degraded_reason === "provider_error") {
        event.degraded = true;
        event.degraded_reason = "provider_error";
      }
      return event;
    }
    default:
      return null;
  }
}
