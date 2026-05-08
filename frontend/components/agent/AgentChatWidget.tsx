"use client";

import { useState, useRef, useEffect } from "react";
import { useTranslations } from "next-intl";
import { MessageCircle, X, Send, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/AuthContext";
import { useSSE } from "@/lib/useSSE";
import { cn } from "@/lib/utils";

interface AgentChatWidgetProps {
  className?: string;
}

export function AgentChatWidget({ className }: AgentChatWidgetProps) {
  const t = useTranslations();
  const { user } = useAuth();
  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState<{ role: string; content: string }[]>([]);
  const [input, setInput] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);

  const { streaming, start, stop } = useSSE({
    onMessage: (delta) => {
      setMessages((prev) => {
        const next = [...prev];
        const last = next[next.length - 1];
        if (last && last.role === "assistant") {
          next[next.length - 1] = { ...last, content: last.content + delta };
        } else {
          next.push({ role: "assistant", content: delta });
        }
        return next;
      });
    },
  });

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  function handleSend() {
    const trimmed = input.trim();
    if (!trimmed || streaming) return;
    setMessages((prev) => [...prev, { role: "user", content: trimmed }]);
    setInput("");
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
    start(`${apiUrl}/api/v1/agent/chat/stream`, {
      messages: [{ role: "user", content: trimmed }],
    });
  }

  if (!user) return null;

  return (
    <div className={cn("fixed bottom-6 right-6 z-50", className)}>
      {open && (
        <div className="mb-3 flex h-[480px] w-[380px] flex-col rounded-md border border-border bg-card ">
          {/* Header */}
          <div className="flex items-center justify-between border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">{t("agent.chatTitle")}</h3>
            <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={() => setOpen(false)}>
              <X className="h-4 w-4" />
            </Button>
          </div>

          {/* Messages */}
          <div ref={scrollRef} className="flex-1 overflow-y-auto px-4 py-3 space-y-3">
            {messages.length === 0 && (
              <p className="text-sm text-muted-foreground text-center mt-8">
                {t("agent.chatPlaceholder")}
              </p>
            )}
            {messages.map((m, i) => (
              <div
                key={`msg-${i}-${m.role}`}
                className={cn(
                  "max-w-[85%] rounded-md px-3 py-2 text-sm",
                  m.role === "user"
                    ? "ml-auto bg-accent text-white"
                    : "bg-muted/40 text-foreground",
                )}
              >
                {m.content || <Loader2 className="h-3.5 w-3.5 animate-spin" />}
              </div>
            ))}
          </div>

          {/* Input */}
          <div className="flex items-center gap-2 border-t border-border px-3 py-2">
            <input
              type="text"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleSend();
              }}
              placeholder={t("agent.chatInputPlaceholder")}
              className="flex-1 rounded-md border border-border bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
              disabled={streaming}
            />
            <Button
              size="sm"
              className="h-8 w-8 p-0"
              onClick={handleSend}
              disabled={streaming || !input.trim()}
            >
              {streaming ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
            </Button>
          </div>
        </div>
      )}

      {/* Toggle button */}
      <Button
        size="sm"
        className="h-12 w-12 rounded-full p-0"
        onClick={() => setOpen(!open)}
      >
        {open ? <X className="h-5 w-5" /> : <MessageCircle className="h-5 w-5" />}
      </Button>
    </div>
  );
}
