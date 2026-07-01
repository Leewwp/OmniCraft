"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { useTranslations, useLocale } from "next-intl";
import { ArrowLeft, MessageSquare, Send } from "lucide-react";
import { ApiRequestError, api } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { Button } from "@/components/ui/button";
import { useToast } from "@/components/ui/Toast";

interface Conversation {
  id: number;
  participants?: { id?: number; username?: string; avatar_url?: string }[];
  last_message?: { text?: string; body?: string; created_at?: string };
}

interface Message {
  id: number;
  sender_id: number;
  text?: string;
  body?: string;
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
  const [isSending, setIsSending] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = useCallback(() => {
    setTimeout(() => {
      if (scrollRef.current) {
        scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
      }
    }, 50);
  }, []);

  useEffect(() => {
    if (!conversation) return;
    setLoading(true);
    setMessages([]);
    let cancelled = false;
    async function load() {
      try {
        const data = await api.get<{ messages?: Message[] }>(
          `/api/v1/messages/${conversation!.id}`
        );
        if (cancelled) return;
        setMessages(normalizeLoadedMessages(data.messages || []));
      } catch {
        // ignore
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => { cancelled = true; };
  }, [conversation]);

  useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  async function sendMessage() {
    if (isSending || !text.trim() || !conversation || !user) return;
    const body = text.trim();
    const recipient = conversation.participants?.find((p) => p.id !== user.id);
    if (!recipient?.id) return;
    setIsSending(true);
    try {
      const data = await api.post<{ message?: Message }>("/api/v1/messages", {
        recipient_id: recipient.id,
        text: body,
      });
      setText((current) => (current.trim() === body ? "" : current));
      setMessages((prev) => [
        ...prev,
        normalizeMessage(data.message ?? { id: Date.now(), sender_id: user.id!, text: body, body }),
      ]);
      scrollToBottom();
    } catch (err) {
      if (err instanceof ApiRequestError && err.code === "DM_REPLY_REQUIRED") {
        toast("warning", t("messages.dmReplyRequired"));
      }
    } finally {
      setIsSending(false);
    }
  }

  const otherUser = conversation?.participants?.find((p) => p.id !== user?.id);

  if (!conversation) {
    return (
      <div className="flex h-full flex-1 items-center justify-center text-sm text-muted-foreground">
        <div className="flex flex-col items-center gap-3">
          <MessageSquare className="h-10 w-10 text-muted-foreground/30" />
          <p>{t('messages.selectConversation')}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col rounded-md border border-border bg-card">
      <div className="flex items-center gap-3 border-b border-border px-4 py-3">
        {onBack && (
          <button
            type="button"
            onClick={onBack}
            className="inline-flex min-h-11 items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground md:hidden"
          >
            <ArrowLeft className="h-4 w-4" aria-hidden="true" />
            {t('common.back')}
          </button>
        )}
        <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold">
          {(otherUser?.username ?? "#").slice(0, 1).toUpperCase()}
        </div>
        <span className="text-sm font-medium">
          {otherUser?.username ?? `#${conversation.id}`}
        </span>
      </div>

      {loading ? (
        <div className="flex flex-1 items-center justify-center">
          <div className="h-5 w-5 animate-spin rounded-full border-2 border-muted border-t-accent" />
        </div>
      ) : (
        <>
          <div
            ref={scrollRef}
            aria-live="polite"
            className="flex-1 overflow-y-auto px-4 py-3 space-y-2"
          >
            {messages.length === 0 ? (
              <p className="text-center text-sm text-muted-foreground pt-8">
                {t('messages.noMessages')}
              </p>
            ) : (
              messages.map((m) => (
                <div
                  key={m.id}
                  className={`flex ${
                    m.sender_id === user?.id ? "justify-end" : "justify-start"
                  }`}
                >
                  <div
                    className={`max-w-[75%] rounded-lg px-3 py-2 text-sm ${
                      m.sender_id === user?.id
                        ? "bg-accent text-white"
                        : "bg-muted/40"
                    }`}
                  >
                    <p className="whitespace-pre-wrap break-words">{m.text ?? m.body ?? ""}</p>
                    {m.created_at && (
                      <p className="mt-0.5 text-right text-[10px] opacity-70">
                        {new Date(m.created_at).toLocaleTimeString(locale === "en" ? "en-US" : "zh-CN", {
                          hour: "2-digit",
                          minute: "2-digit",
                        })}
                      </p>
                    )}
                  </div>
                </div>
              ))
            )}
          </div>
          <div className="flex items-center gap-2 border-t border-border p-3">
            <input
              type="text"
              value={text}
              onChange={(e) => setText(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  void sendMessage();
                }
              }}
              placeholder={t('messages.replyPlaceholder')}
              className="flex-1 rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
            />
            <Button
              size="sm"
              className="h-11 w-11 p-0"
              aria-label={t("messages.sendMessage")}
              onClick={() => void sendMessage()}
              disabled={isSending || !text.trim()}
            >
              <Send className="h-4 w-4" />
            </Button>
          </div>
        </>
      )}
    </div>
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
  if (aTime !== bTime) return aTime - bTime;
  return a.id - b.id;
}

function parseMessageTime(value?: string): number {
  if (!value) return 0;
  const timestamp = Date.parse(value);
  return Number.isNaN(timestamp) ? 0 : timestamp;
}
