"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { MessageSquare, Send } from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { Button } from "@/components/ui/button";

interface Conversation {
  id: number;
  participants?: { id?: number; username?: string; avatar_url?: string }[];
  last_message?: { text?: string; created_at?: string };
}

interface Message {
  id: number;
  sender_id: number;
  text: string;
  created_at?: string;
}

interface ChatWindowProps {
  conversation: Conversation | null;
  onBack?: () => void;
}

export function ChatWindow({ conversation, onBack }: ChatWindowProps) {
  const { user } = useAuth();
  const [messages, setMessages] = useState<Message[]>([]);
  const [text, setText] = useState("");
  const [loading, setLoading] = useState(false);
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
          `/api/v1/conversations/${conversation!.id}/messages`
        );
        if (cancelled) return;
        setMessages(data.messages || []);
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
    if (!text.trim() || !conversation || !user) return;
    const body = text.trim();
    setText("");
    const recipient = conversation.participants?.find((p) => p.id !== user.id);
    try {
      await api.post(`/api/v1/conversations/${conversation.id}/messages`, {
        text: body,
      });
      setMessages((prev) => [
        ...prev,
        { id: Date.now(), sender_id: user.id!, text: body },
      ]);
      scrollToBottom();
    } catch {
      // ignore
    }
  }

  const otherUser = conversation?.participants?.find((p) => p.id !== user?.id);

  if (!conversation) {
    return (
      <div className="flex h-full flex-1 items-center justify-center text-sm text-muted-foreground">
        <div className="flex flex-col items-center gap-3">
          <MessageSquare className="h-10 w-10 text-muted-foreground/30" />
          <p>选择一个对话开始聊天</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col rounded-md border border-border bg-card">
      <div className="flex items-center gap-3 border-b border-border px-4 py-3">
        {onBack && (
          <button
            onClick={onBack}
            className="text-sm text-muted-foreground hover:text-foreground md:hidden"
          >
            ← 返回
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
            className="flex-1 overflow-y-auto px-4 py-3 space-y-2"
          >
            {messages.length === 0 ? (
              <p className="text-center text-sm text-muted-foreground pt-8">
                发送第一条消息开始对话
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
                    <p className="whitespace-pre-wrap break-words">{m.text}</p>
                    {m.created_at && (
                      <p className="mt-0.5 text-right text-[10px] opacity-70">
                        {new Date(m.created_at).toLocaleTimeString("zh-CN", {
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
              placeholder="输入消息..."
              className="flex-1 rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
            />
            <Button
              size="sm"
              className="h-8 w-8 p-0"
              onClick={() => void sendMessage()}
              disabled={!text.trim()}
            >
              <Send className="h-4 w-4" />
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
