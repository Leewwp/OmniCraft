"use client";

import { useEffect, useState, useRef } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { Bell, MessageSquare } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Send } from "lucide-react";

interface Notification {
  id: number;
  type: string;
  channel: string;
  body: string;
  is_read: boolean;
  created_at: string;
}

interface Conversation {
  id: number;
  participants?: { id?: number; username?: string }[];
  last_message?: { text?: string; created_at?: string };
  created_at?: string;
}

interface Message {
  id: number;
  sender_id: number;
  text: string;
  created_at?: string;
}

export default function MessagesPage() {
  const t = useTranslations();
  const { user, isLoading } = useAuth();
  const [tab, setTab] = useState<"notifications" | "messages">("notifications");

  // Notifications
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Conversations
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConv, setActiveConv] = useState<Conversation | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [text, setText] = useState("");
  const [msgLoading, setMsgLoading] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  /* ── Notifications ──────────────────────── */
  useEffect(() => {
    if (!user) return;
    void loadNotifications();
  }, [user]);

  async function loadNotifications() {
    setError("");
    setLoading(true);
    try {
      const data = await api.get<{ notifications?: Notification[] }>("/api/v1/notifications");
      setNotifications(data.notifications || []);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t("common.loadFailed"));
    } finally {
      setLoading(false);
    }
  }

  async function markRead(id: number) {
    try {
      await api.patch(`/api/v1/notifications/${id}/read`, {});
      setNotifications((prev) => prev.map((n) => (n.id === id ? { ...n, is_read: true } : n)));
    } catch { /* ignore */ }
  }

  async function markAllRead() {
    try {
      await api.post("/api/v1/notifications/read-all", {});
      setNotifications((prev) => prev.map((n) => ({ ...n, is_read: true })));
    } catch { /* ignore */ }
  }

  /* ── Conversations ──────────────────────── */
  useEffect(() => {
    if (!user || tab !== "messages") return;
    void loadConversations();
  }, [user, tab]);

  async function loadConversations() {
    try {
      const data = await api.get<{ conversations?: Conversation[] }>("/api/v1/messages");
      setConversations(data.conversations || []);
    } catch { /* ignore */ }
  }

  async function openConversation(conv: Conversation) {
    setActiveConv(conv);
    setMsgLoading(true);
    try {
      const data = await api.get<{ messages?: Message[] }>(`/api/v1/messages/${conv.id}`);
      setMessages(data.messages || []);
    } catch {
      setMessages([]);
    } finally {
      setMsgLoading(false);
      setTimeout(() => {
        if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
      }, 50);
    }
  }

  async function sendMessage() {
    if (!text.trim() || !activeConv) return;
    const body = text.trim();
    setText("");
    const recipient = activeConv.participants?.find((p) => p.id !== user?.id);
    if (!recipient?.id) return;
    try {
      await api.post("/api/v1/messages", { recipient_id: recipient.id, text: body });
      setMessages((prev) => [...prev, { id: Date.now(), sender_id: user!.id!, text: body }]);
      await loadConversations();
      setTimeout(() => {
        if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
      }, 50);
    } catch { /* ignore */ }
  }

  if (isLoading || loading) {
    return <div className="mx-auto w-full max-w-2xl px-4 py-6 text-sm text-muted-foreground">{t("common.loading")}</div>;
  }

  const unreadCount = notifications.filter((n) => !n.is_read).length;

  return (
    <div className="mx-auto w-full max-w-2xl space-y-4 px-4 py-6">
      {/* Header */}
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4 ">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("messages.title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {tab === "notifications"
              ? t("messages.unreadCount", { count: unreadCount })
              : t("messages.conversationCount", { count: conversations.length })}
          </p>
        </div>
        {tab === "notifications" && unreadCount > 0 && (
          <Button size="sm" variant="outline" onClick={() => void markAllRead()}>
            {t("messages.markAllRead")}
          </Button>
        )}
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-border">
        {(["notifications", "messages"] as const).map((tKey) => (
          <button
            key={tKey}
            onClick={() => setTab(tKey)}
            className={`px-4 py-2 text-sm border-b-2 transition-colors ${
              tab === tKey
                ? "border-foreground text-foreground font-medium"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            {tKey === "notifications" ? t("messages.tabNotifications") : t("messages.tabMessages")}
          </button>
        ))}
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {/* Notifications Tab */}
      {tab === "notifications" && (
        notifications.length === 0 ? (
          <div className="flex flex-col items-center gap-3 rounded-md border border-border bg-card p-12 text-center">
            <Bell className="h-8 w-8 text-muted-foreground/40" />
            <p className="text-sm text-muted-foreground">{t("messages.noMessages")}</p>
          </div>
        ) : (
          <div className="space-y-2">
            {notifications.map((n) => (
              <div
                key={n.id}
                className={`flex items-start justify-between rounded-md border p-3 ${
                  n.is_read ? "border-border bg-card" : "border-accent/30 bg-accent/5"
                }`}
              >
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-muted-foreground">{n.type}</span>
                    {!n.is_read && <span className="h-2 w-2 rounded-full bg-accent" />}
                  </div>
                  <p className="text-sm">{n.body}</p>
                  <p className="text-xs text-muted-foreground">{new Date(n.created_at).toLocaleString("zh-CN")}</p>
                </div>
                {!n.is_read && (
                  <Button size="sm" variant="ghost" onClick={() => void markRead(n.id)}>
                    {t("messages.read")}
                  </Button>
                )}
              </div>
            ))}
          </div>
        )
      )}

      {/* Messages Tab */}
      {tab === "messages" && (
        <div className="flex gap-3" style={{ minHeight: 400 }}>
          {/* Conversation list */}
          <div className="w-48 shrink-0 space-y-1 border-r border-border pr-3">
            {conversations.length === 0 ? (
              <div className="flex flex-col items-center gap-2 py-8 text-center">
                <MessageSquare className="h-6 w-6 text-muted-foreground/30" />
                <p className="text-xs text-muted-foreground">{t("messages.noConversations")}</p>
              </div>
            ) : (
              conversations.map((c) => {
                const other = c.participants?.find((p) => p.id !== user?.id);
                return (
                  <button
                    key={c.id}
                    onClick={() => openConversation(c)}
                    className={`w-full rounded-md px-3 py-2 text-left text-xs transition-colors ${
                      activeConv?.id === c.id
                        ? "bg-accent/10 text-accent font-medium"
                        : "hover:bg-muted/30 text-muted-foreground"
                    }`}
                  >
                    <span className="block truncate font-medium text-foreground">
                      {other?.username ?? `#${c.id}`}
                    </span>
                    <span className="block truncate mt-0.5">{c.last_message?.text ?? ""}</span>
                  </button>
                );
              })
            )}
          </div>

          {/* Chat window */}
          <div className="flex flex-1 flex-col rounded-md border border-border bg-card">
            {!activeConv ? (
              <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
                {t("messages.selectConversation")}
              </div>
            ) : msgLoading ? (
              <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
                {t("common.loading")}
              </div>
            ) : (
              <>
                <div ref={scrollRef} className="flex-1 overflow-y-auto p-3 space-y-2">
                  {messages.map((m) => (
                    <div
                      key={m.id}
                      className={`max-w-[75%] rounded-md px-3 py-2 text-sm ${
                        m.sender_id === user?.id
                          ? "ml-auto bg-accent text-white"
                          : "bg-muted/40"
                      }`}
                    >
                      {m.text}
                    </div>
                  ))}
                </div>
                <div className="flex items-center gap-2 border-t border-border p-2">
                  <input
                    type="text"
                    value={text}
                    onChange={(e) => setText(e.target.value)}
                    onKeyDown={(e) => { if (e.key === "Enter") sendMessage(); }}
                    placeholder={t("messages.replyPlaceholder")}
                    className="flex-1 rounded-md border border-border bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
                  />
                  <Button size="sm" className="h-8 w-8 p-0" onClick={sendMessage} disabled={!text.trim()}>
                    <Send className="h-4 w-4" />
                  </Button>
                </div>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
