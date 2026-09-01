import {
  ensureCSRFHeader,
  getAccessToken,
  refreshAccessTokenHeader,
  refreshCSRFHeader,
} from "@/lib/api";
import { normalizeAgentEvent } from "@/lib/agent";

export interface AgentStreamCitation {
  content_id: number;
  title: string;
  zone: string;
  excerpt?: string;
  content_version?: number;
  chunk_key?: string;
  chunk_index?: number;
  route?: string;
  source?: "bm25" | "vector" | "hybrid_rrf";
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
  | {
      type: "error";
      error_code?: string;
      error_message?: string;
      degraded?: boolean;
      degraded_reason?: "provider_error";
    };

export interface AgentStreamHandlers {
  onEvent: (event: AgentStreamEvent) => void;
  onError?: (error: Error) => void;
  onClose?: () => void;
}

/** Carries the backend error code (e.g. AGENT_RATE_LIMIT_EXCEEDED) so the
 *  workspace can render dedicated copy instead of a generic failure. */
export class AgentStreamError extends Error {
  readonly status: number;
  readonly code?: string;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = "AgentStreamError";
    this.status = status;
    this.code = code;
  }
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
  } catch (error) {
    // CSRF is a local security precondition. Surface the safe client error and
    // do not send a knowingly invalid state-changing request to the server.
    handlers.onError?.(error instanceof Error ? error : new Error("security token unavailable"));
    return;
  }
  const send = () =>
    fetchImpl(url, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
      signal,
      credentials: "include",
    });

  let res: Response;
  let csrfRetried = false;
  let accessRetried = false;
  for (;;) {
    try {
      res = await send();
    } catch (error) {
      if ((error as Error).name !== "AbortError") handlers.onError?.(error as Error);
      return;
    }
    if (res.ok) break;

    const code = await readErrorCode(res);
    try {
      if (res.status === 403 && code === "CSRF_TOKEN_INVALID" && !csrfRetried) {
        csrfRetried = true;
        await refreshCSRFHeader(headers);
        continue;
      }
      if (res.status === 401 && code === "TOKEN_EXPIRED" && token && !accessRetried) {
        accessRetried = true;
        if (await refreshAccessTokenHeader(headers)) continue;
      }
    } catch (error) {
      handlers.onError?.(error instanceof Error ? error : new Error("authentication recovery failed"));
      return;
    }
    handlers.onError?.(new AgentStreamError(`agent stream failed: ${res.status}`, res.status, code || undefined));
    return;
  }
  if (!res.ok || !res.body) {
    handlers.onError?.(new AgentStreamError(`agent stream failed: ${res.status}`, res.status));
    return;
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  // SSE 事件行可能跨网络分块边界：必须行缓冲，只处理完整行，残行留待下一分块；
  // 上限防御恶意超长行（done 事件含全量 answer+citations，正常远小于此值）。
  const maxBufferedLineLength = 2 * 1024 * 1024;
  let buffered = "";
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buffered += decoder.decode(value, { stream: true });
      if (buffered.length > maxBufferedLineLength) {
        throw new Error("agent stream aborted: single line exceeded buffer limit");
      }
      let newlineIndex = buffered.indexOf("\n");
      while (newlineIndex >= 0) {
        const line = buffered.slice(0, newlineIndex);
        buffered = buffered.slice(newlineIndex + 1);
        newlineIndex = buffered.indexOf("\n");
        const event = parseAgentStreamLine(line);
        if (event) handlers.onEvent(event);
      }
    }
    const tail = buffered + decoder.decode();
    if (tail) {
      const event = parseAgentStreamLine(tail);
      if (event) handlers.onEvent(event);
    }
  } catch (error) {
    if ((error as Error).name !== "AbortError") handlers.onError?.(error as Error);
    return;
  }
  handlers.onClose?.();
}

async function readErrorCode(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as { code?: unknown };
    return typeof body.code === "string" ? body.code : "";
  } catch {
    return "";
  }
}
