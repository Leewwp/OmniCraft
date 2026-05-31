import { useCallback, useRef, useState } from "react";
import { getAccessToken } from "@/lib/api";

interface UseSSEOptions {
  onMessage?: (data: string) => void;
  onError?: (error: Error) => void;
  onClose?: () => void;
}

interface UseSSEReturn {
  streaming: boolean;
  start: (url: string, body?: unknown, options?: UseSSEOptions) => void;
  stop: () => void;
}

export function useSSE(defaultOptions?: UseSSEOptions): UseSSEReturn {
  const [streaming, setStreaming] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  const stop = useCallback(() => {
    abortRef.current?.abort();
    setStreaming(false);
  }, []);

  const start = useCallback(
    (url: string, body?: unknown, options?: UseSSEOptions) => {
      const merged = { ...defaultOptions, ...options };
      stop();

      const controller = new AbortController();
      abortRef.current = controller;
      setStreaming(true);

      const token = getAccessToken();

      fetch(url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: body !== undefined ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      })
        .then(async (res) => {
          if (!res.ok || !res.body) {
            throw new Error(`SSE request failed: ${res.status}`);
          }

          const reader = res.body.getReader();
          const decoder = new TextDecoder();

          while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            const text = decoder.decode(value, { stream: true });
            const lines = text.split("\n");

            for (const line of lines) {
              if (line.startsWith("data: ")) {
                const data = line.slice(6).trim();
                if (data === "[DONE]") {
                  merged.onClose?.();
                  setStreaming(false);
                  return;
                }
                try {
                  const parsed = JSON.parse(data) as Record<string, unknown>;
                  merged.onMessage?.(
                    (parsed.delta as string) ??
                      (parsed.content as string) ??
                      data
                  );
                } catch {
                  merged.onMessage?.(data);
                }
              }
            }
          }

          merged.onClose?.();
          setStreaming(false);
        })
        .catch((err: Error) => {
          if (err.name !== "AbortError") {
            merged.onError?.(err);
          }
          setStreaming(false);
        });
    },
    [defaultOptions, stop]
  );

  return { streaming, start, stop };
}
