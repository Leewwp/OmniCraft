"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft, LoaderCircle, MessageSquare, Send } from "lucide-react";
import { ApiRequestError, api } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { Button } from "@/components/ui/button";
import { useToast } from "@/components/ui/Toast";
import type { Conversation } from "@/components/social/ConversationList";

interface Message {
  id: number;
  sender_id: number;
  text?: string;
  body?: string;
  msg_type?: string;
  created_at?: string;
}

interface ChatWindowProps {
  conversation: Conversation | null;
  onBack?: () => void;
}

export function ChatWindow({ conversation, onBack }: ChatWindowProps) {
  const t = useTranslations();
  const locale = useLocale();
  const { user } = useAuth();
  const { toast } = useToast();
  const [messages, setMessages] = useState<Message[]>([]);
  const [text, setText] = useState("");
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [isSending, setIsSending] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const requestIdRef = useRef(0);

  const scrollToBottom = useCallback(() => {
    requestAnimationFrame(() => {
      if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    });
  }, []);

  const loadMessages = useCallback(async () => {
    if (!conversation) return;
    const requestId = ++requestIdRef.current;
    const conversationId = conversation.id;
    setLoading(true);
    setLoadError(false);
    try {
      const data = await api.get<{ messages?: Message[] }>(`/api/v1/messages/${conversationId}`);
      if (requestIdRef.current !== requestId) return;
      setMessages(normalizeLoadedMessages(data.messages || []));
    } catch {
      if (requestIdRef.current === requestId) setLoadError(true);
    } finally {
      if (requestIdRef.current === requestId) setLoading(false);
    }
  }, [conversation]);

  useEffect(() => {
    setMessages([]);
    void loadMessages();
  }, [loadMessages]);

  useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  async function sendMessage() {
    if (isSending || !text.trim() || !conversation || !user) return;
    const body = text.trim();
    const recipient = conversation.participants?.find((participant) => participant.id !== user.id);
    if (!recipient?.id) return;
    setIsSending(true);
    try {
      const data = await api.post<{ message?: Message }>("/api/v1/messages", { recipient_id: recipient.id, text: body });
      setText((current) => (current.trim() === body ? "" : current));
      setMessages((current) => [
        ...current,
        normalizeMessage(data.message ?? { id: Date.now(), sender_id: user.id!, text: body, body }),
      ]);
    } catch (error) {
      toast("error", error instanceof ApiRequestError && error.code === "DM_REPLY_REQUIRED"
        ? t("messages.chat.replyRequired")
        : t("messages.error.send"));
    } finally {
      setIsSending(false);
    }
  }

  const otherUser = conversation?.participants?.find((participant) => participant.id !== user?.id);

  if (!conversation) {
    return (
      <div className="flex h-full min-h-80 flex-1 items-center justify-center text-sm text-fg-muted">
        <div className="flex flex-col items-center gap-3 text-center">
          <MessageSquare className="h-10 w-10 text-fg-muted" aria-hidden="true" />
          <p>{t("messages.chat.selectConversation")}</p>
        </div>
      </div>
    );
  }

  return (
    <section className="flex h-full min-h-[420px] min-w-0 flex-col rounded-md border border-border-default bg-canvas-default shadow-none">
      <header className="flex min-h-16 items-center gap-3 border-b border-border-default px-4 py-3">
        {onBack && (
          <button type="button" onClick={onBack} aria-label={t("common.back")} className="inline-flex min-h-11 items-center gap-1.5 rounded-md px-2 text-sm text-fg-muted hover:bg-canvas-subtle hover:text-fg-default focus:outline-none focus:ring-2 focus:ring-accent-emphasis md:hidden">
            <ArrowLeft className="h-4 w-4" aria-hidden="true" />
            {t("common.back")}
          </button>
        )}
        <div className="flex h-10 w-10 items-center justify-center rounded-full bg-accent-subtle text-sm font-semibold text-accent-emphasis">
          {(otherUser?.username ?? "?").slice(0, 1).toUpperCase()}
        </div>
        <span className="truncate text-sm font-medium text-fg-default">
          {otherUser?.username ?? t("messages.chat.recipientUnavailable")}
        </span>
      </header>

      {loading ? (
        <div className="flex-1 space-y-3 px-4 py-4" aria-label={t("common.loading")}>
          {[1, 2, 3, 4, 5, 6].map((item) => (
            <div key={item} className={`h-12 animate-pulse rounded-lg bg-canvas-subtle ${item % 2 ? "mr-auto w-1/2" : "ml-auto w-2/3"}`} />
          ))}
        </div>
      ) : loadError ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-4 text-center" role="alert">
          <p className="text-sm text-fg-muted">{t("messages.error.chat")}</p>
          <button type="button" onClick={() => void loadMessages()} className="min-h-11 rounded-md border border-border-default px-3 text-sm text-accent-emphasis hover:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-accent-emphasis">
            {t("messages.chat.retry")}
          </button>
        </div>
      ) : (
          <div ref={scrollRef} role="log" aria-live="polite" aria-label={t("messages.a11y.messageList")} className="flex-1 space-y-3 overflow-y-auto px-4 py-3">
            {messages.length === 0 ? (
              <div className="pt-8 text-center">
                <p className="text-sm font-medium text-fg-default">{t("messages.chat.emptyTitle")}</p>
                <p className="mt-1 text-xs text-fg-muted">{t("messages.chat.emptyDescription")}</p>
              </div>
            ) : messages.map((message) => {
              const own = message.sender_id === user?.id;
              const content = message.msg_type === "collab_invite"
                ? t("messages.chat.collabInviteSummary")
                : message.msg_type && message.msg_type !== "text"
                ? t("messages.chat.unsupportedMessage")
                : message.text ?? message.body ?? t("messages.chat.unsupportedMessage");
              return (
                <div key={message.id} className={`flex ${own ? "justify-end" : "justify-start"}`}>
                  <div className={`max-w-[86%] rounded-lg px-3 py-2 text-sm md:max-w-[70%] ${own ? "bg-accent-emphasis text-white" : "bg-canvas-subtle text-fg-default"}`}>
                    <p className="whitespace-pre-wrap break-words">{content}</p>
                    <time className="mt-0.5 block text-right text-[10px] opacity-75" dateTime={message.created_at}>
                      {formatMessageTime(message.created_at, locale, t("messages.chat.timeUnknown"))}
                    </time>
                  </div>
                </div>
              );
            })}
          </div>
      )}
      <div className="flex items-end gap-2 border-t border-border-default p-3">
        <textarea
          value={text}
          rows={1}
          disabled={loading}
          onChange={(event) => setText(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter" && !event.shiftKey) {
              event.preventDefault();
              void sendMessage();
            }
          }}
          aria-label={t("messages.chat.inputLabel")}
          placeholder={t("messages.chat.inputPlaceholder")}
          className="min-h-11 max-h-36 flex-1 resize-y rounded-md border border-border-default bg-canvas-default px-3 py-2 text-sm text-fg-default focus:outline-none focus:ring-2 focus:ring-accent-emphasis disabled:cursor-not-allowed disabled:opacity-60"
        />
        <Button size="sm" className="h-11 w-11 shrink-0 p-0" aria-label={t("messages.chat.send")} onClick={() => void sendMessage()} disabled={loading || isSending || !text.trim()}>
          {isSending ? <LoaderCircle className="h-4 w-4 animate-spin" aria-hidden="true" /> : <Send className="h-4 w-4" aria-hidden="true" />}
          <span className="sr-only">{isSending ? t("messages.chat.sending") : t("messages.chat.send")}</span>
        </Button>
      </div>
    </section>
  );
}

function normalizeLoadedMessages(messages: Message[]): Message[] {
  return messages.map(normalizeMessage).sort(compareMessagesChronologically);
}

function normalizeMessage(message: Message): Message {
  const text = message.text ?? message.body ?? "";
  return { ...message, text, body: message.body ?? text };
}

function compareMessagesChronologically(a: Message, b: Message): number {
  const aTime = parseMessageTime(a.created_at);
  const bTime = parseMessageTime(b.created_at);
  return aTime === bTime ? a.id - b.id : aTime - bTime;
}

function parseMessageTime(value?: string): number {
  if (!value) return 0;
  const timestamp = Date.parse(value);
  return Number.isNaN(timestamp) ? 0 : timestamp;
}

function formatMessageTime(value: string | undefined, locale: string, fallback: string) {
  if (!value) return fallback;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return fallback;
  return date.toLocaleTimeString(locale === "en" ? "en-US" : "zh-CN", { hour: "2-digit", minute: "2-digit" });
}
