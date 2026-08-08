import { ensureCSRFHeader, getAccessToken } from "@/lib/api";
import { normalizeAgentEvent } from "@/lib/agent";

export interface AgentStreamCitation {
  content_id: number;
  title: string;
  zone: string;
  excerpt?: string;
}

export interface AgentStreamTool {
  name: string;
  status: "running" | "success" | "failed" | "error" | "skipped";
  duration_ms?: number;
}

export type AgentStreamEvent =
  | { type: "start"; trace_id?: string; conversation_id?: number; answer_kind?: string }
  | { type: "tool_status"; tool?: AgentStreamTool }
  | { type: "delta"; delta?: string }
  | { type: "citation"; citation?: AgentStreamCitation }
  | { type: "usage"; usage?: unknown }
  | {
      type: "done";
      conversation_id?: number;
      answer_kind?: string;
      answer?: string;
      citations?: AgentStreamCitation[];
      tools?: AgentStreamTool[];
      degraded?: boolean;
    }
  | { type: "error"; error_code?: string; error_message?: string };

export interface AgentStreamHandlers {
  onEvent: (event: AgentStreamEvent) => void;
  onError?: (error: Error) => void;
  onClose?: () => void;
}

/** 解析一行 SSE 数据。仅接受 `data: <json>` 且带 `type` 字段的事件；[DONE] 与空行忽略。
 *  事件经 lib/agent.ts 的 typed normalizer 校验，畸形 citation/tool 事件直接拒绝。 */
export function parseAgentStreamLine(line: string): AgentStreamEvent | null {
  if (!line.startsWith("data:")) return null;
  const raw = line.slice(5).trim();
  if (!raw || raw === "[DONE]") return null;
  try {
    const parsed = JSON.parse(raw) as unknown;
    return normalizeAgentEvent(parsed);
  } catch {
    return null;
  }
}

/**
 * 以服务端流式契约发起一次 Agent 对话请求（surface/content_id 由调用方负责）。
 * 逐行解析 SSE 事件并转发给 onEvent；流结束调 onClose，非 OK/网络失败调 onError。
 * 客户端取消（AbortError）不视为错误。
 */
export async function startAgentStream(
  fetchImpl: typeof fetch,
  url: string,
  body: unknown,
  handlers: AgentStreamHandlers,
  signal?: AbortSignal,
): Promise<void> {
  const token = getAccessToken();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
  // /agent/chat/stream sits behind the backend CSRF middleware; the stream
  // must carry X-CSRF-Token and credentials like every other mutating call.
  try {
    await ensureCSRFHeader(headers);
  } catch {
    // No token available: let the request fail server-side instead of
    // crashing the stream reader.
  }
  let res: Response;
  try {
    res = await fetchImpl(url, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
      signal,
      credentials: "include",
    });
  } catch (error) {
    if ((error as Error).name !== "AbortError") handlers.onError?.(error as Error);
    return;
  }
  if (!res.ok || !res.body) {
    handlers.onError?.(new Error(`agent stream failed: ${res.status}`));
    return;
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      const text = decoder.decode(value, { stream: true });
      for (const line of text.split("\n")) {
        const event = parseAgentStreamLine(line);
        if (event) handlers.onEvent(event);
      }
    }
  } catch (error) {
    if ((error as Error).name !== "AbortError") handlers.onError?.(error as Error);
    return;
  }
  handlers.onClose?.();
}
