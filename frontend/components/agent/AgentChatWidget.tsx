"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import { useTranslations } from "next-intl";
import { usePathname } from "next/navigation";
import { MessageCircle, X, Send, Loader2, RotateCcw, HelpCircle, Flag } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/AuthContext";
import { useSSE } from "@/lib/useSSE";
import { cn } from "@/lib/utils";
import {
  buildPageContext,
  sanitizePageContext,
  QUICK_PROMPTS,
  type AgentPageContext,
  type QuickPromptIntent,
} from "@/lib/agent-context";
import Link from "next/link";

const MAX_CONTEXT_MESSAGES = 10;

interface ChatMessage {
  role: "user" | "assistant";
  content: string;
}

interface AgentChatWidgetProps {
  className?: string;
  contentId?: number;
  contentTitle?: string;
  contentType?: string;
}

export function AgentChatWidget({ className, contentId, contentTitle, contentType }: AgentChatWidgetProps) {
  const t = useTranslations();
  const pathname = usePathname();
  const { user } = useAuth();
  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [error, setError] = useState("");
  const [showDowngrade, setShowDowngrade] = useState(false);
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
    onError: () => {
      setError(t("agent.chatError"));
      setShowDowngrade(true);
    },
    onClose: () => {
      setError("");
    },
  });

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  const getPageContext = useCallback((): AgentPageContext => {
    const ctx = buildPageContext();
    if (contentId) ctx.contentId = contentId;
    if (contentTitle) ctx.contentTitle = contentTitle;
    if (contentType) ctx.contentType = contentType;
    return sanitizePageContext(ctx);
  }, [contentId, contentTitle, contentType]);

  function handleSend(overrideMessage?: string) {
    const trimmed = (overrideMessage ?? input).trim();
    if (!trimmed || streaming) return;

    const userMsg: ChatMessage = { role: "user", content: trimmed };
    setMessages((prev) => [...prev, userMsg]);
    setInput("");
    setError("");
    setShowDowngrade(false);

    const contextMessages = [...messages, userMsg].slice(-MAX_CONTEXT_MESSAGES);

    const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
    start(`${apiUrl}/api/v1/agent/chat/stream`, {
      messages: contextMessages.map((m) => ({ role: m.role, content: m.content })),
      context: getPageContext(),
    });
  }

  function handleQuickPrompt(intent: QuickPromptIntent) {
    const promptMap: Record<QuickPromptIntent, string> = {
      page_help: t("agent.quickPageHelp"),
      download_help: t("agent.quickDownloadHelp"),
      publish_help: t("agent.quickPublishHelp"),
      desktop_client_help: t("agent.quickDesktopClientHelp"),
      report_problem: t("agent.quickReportProblem"),
    };
    handleSend(promptMap[intent]);
  }

  function handleRetry() {
    if (messages.length === 0) return;
    const lastUserMsg = [...messages].reverse().find((m) => m.role === "user");
    if (lastUserMsg) {
      setMessages((prev) => prev.slice(0, -1));
      handleSend(lastUserMsg.content);
    }
  }

  if (!user) return null;

  return (
    <div className={cn("fixed bottom-6 right-6 z-50", className)}>
      {open && (
        <div className="mb-3 flex h-[480px] w-[380px] flex-col rounded-md border border-border bg-card">
          <div className="flex items-center justify-between border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">{t("agent.chatTitle")}</h3>
            <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={() => setOpen(false)}>
              <X className="h-4 w-4" />
            </Button>
          </div>

          <div ref={scrollRef} className="flex-1 overflow-y-auto px-4 py-3 space-y-3">
            {messages.length === 0 && !error && (
              <div className="space-y-3 mt-4">
                <p className="text-sm text-muted-foreground text-center">
                  {t("agent.chatPlaceholder")}
                </p>
                <div className="flex flex-wrap gap-1.5 justify-center">
                  {QUICK_PROMPTS.map((intent) => (
                    <button
                      key={intent}
                      type="button"
                      onClick={() => handleQuickPrompt(intent)}
                      className="rounded border border-border px-2 py-1 text-xs text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-colors"
                    >
                      {t(`agent.quickPrompt_${intent}`)}
                    </button>
                  ))}
                </div>
              </div>
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
            {error && (
              <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                {error}
              </div>
            )}
            {showDowngrade && (
              <div className="space-y-2 rounded-md border border-amber-300/30 bg-amber-50/50 dark:bg-amber-900/10 px-3 py-2">
                <p className="text-xs text-amber-700 dark:text-amber-400">
                  {t("agent.chatDowngradeNotice")}
                </p>
                <div className="flex items-center gap-2">
                  <Button variant="outline" size="sm" className="h-6 text-xs" onClick={handleRetry}>
                    <RotateCcw className="mr-1 h-3 w-3" />
                    {t("agent.retry")}
                  </Button>
                  <Link href="/help" className="inline-flex items-center text-xs text-muted-foreground hover:text-foreground">
                    <HelpCircle className="mr-1 h-3 w-3" />
                    {t("agent.helpLink")}
                  </Link>
                  <Link href="/feedback" className="inline-flex items-center text-xs text-muted-foreground hover:text-foreground">
                    <Flag className="mr-1 h-3 w-3" />
                    {t("agent.feedbackLink")}
                  </Link>
                </div>
              </div>
            )}
          </div>

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
              onClick={() => handleSend()}
              disabled={streaming || !input.trim()}
            >
              {streaming ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
            </Button>
          </div>
        </div>
      )}

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
