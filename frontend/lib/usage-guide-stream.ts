import { getAccessToken } from "@/lib/api";

/**
 * SP-25 FR-04（中-9）：内容详情「使用指南」面板的流式 GET 客户端。
 *
 * 后端 GET /agent/usage-guide/:id?stream=true 有两种响应形态：
 * ① 结构化指南已存在（SP-16 #447 结构化优先）→ 直接返回 JSON {guide}；
 * ② 现场生成 → SSE（event: delta/done/error，gin SSEvent 序列化）。
 * 两种形态都在此收口。行缓冲对齐 agent-stream：SSE 事件行可能跨网络分块
 * 边界，只处理完整行、残行留待下一分块，上限 2MB 防御超长行；GET 无需
 * CSRF，但路由在 authReq 之后，须携带 Authorization。
 */
export interface UsageGuideStreamHandlers {
  onDelta?: (delta: string) => void;
  onDone?: () => void;
  onError?: (error: Error) => void;
  onClose?: () => void;
}

export async function streamUsageGuide(
  fetchImpl: typeof fetch,
  url: string,
  handlers: UsageGuideStreamHandlers,
  signal?: AbortSignal,
): Promise<void> {
  const token = getAccessToken();
  let res: Response;
  try {
    res = await fetchImpl(url, {
      method: "GET",
      headers: {
        Accept: "text/event-stream, application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      signal,
      credentials: "include",
    });
  } catch (error) {
    if ((error as Error).name !== "AbortError") handlers.onError?.(error as Error);
    return;
  }
  if (!res.ok || !res.body) {
    handlers.onError?.(new Error(`usage guide request failed: ${res.status}`));
    return;
  }

  const contentType = res.headers.get("content-type") || "";
  if (contentType.includes("application/json")) {
    try {
      const data = (await res.json()) as { guide?: string };
      if (typeof data.guide === "string" && data.guide) {
        handlers.onDelta?.(data.guide);
      }
    } catch (error) {
      handlers.onError?.(error instanceof Error ? error : new Error("usage guide payload malformed"));
      return;
    }
    handlers.onDone?.();
    handlers.onClose?.();
    return;
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  const maxBufferedLineLength = 2 * 1024 * 1024;
  let buffered = "";
  let sawError = false;

  const handleLine = (line: string): "continue" | "stop" => {
    // gin SSEvent 写出 `data:{...}`（冒号后无空格）；slice(5)+trim 与
    // agent-stream 的 parseAgentStreamLine 同规，两种形态都收。
    if (!line.startsWith("data:")) return "continue";
    const payload = line.slice(5).trim();
    if (!payload) return "continue";
    let ev: { type?: string; delta?: string; error_message?: string };
    try {
      ev = JSON.parse(payload) as { type?: string; delta?: string; error_message?: string };
    } catch {
      // 半行 JSON 绝不拼进正文：不完整行不会走到这里（行缓冲保证），畸形
      // 完整行按错误处理。
      sawError = true;
      handlers.onError?.(new Error("usage guide stream malformed"));
      return "stop";
    }
    if (ev.type === "delta" && typeof ev.delta === "string") {
      handlers.onDelta?.(ev.delta);
      return "continue";
    }
    if (ev.type === "done") {
      return "stop";
    }
    if (ev.type === "error") {
      sawError = true;
      handlers.onError?.(new Error(ev.error_message || "provider unavailable"));
      return "stop";
    }
    return "continue";
  };

  try {
    outer: for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buffered += decoder.decode(value, { stream: true });
      if (buffered.length > maxBufferedLineLength) {
        throw new Error("usage guide stream exceeded buffer limit");
      }
      let newlineIndex = buffered.indexOf("\n");
      while (newlineIndex >= 0) {
        const line = buffered.slice(0, newlineIndex);
        buffered = buffered.slice(newlineIndex + 1);
        newlineIndex = buffered.indexOf("\n");
        if (handleLine(line) === "stop") break outer;
      }
    }
    if (!sawError && buffered) {
      handleLine(buffered + decoder.decode());
    }
  } catch (error) {
    if ((error as Error).name !== "AbortError") handlers.onError?.(error as Error);
    return;
  }
  if (!sawError) {
    handlers.onDone?.();
    handlers.onClose?.();
  }
}
