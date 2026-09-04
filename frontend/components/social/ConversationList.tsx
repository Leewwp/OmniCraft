"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { MessageSquare, Search } from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";

export interface Conversation {
  id: number;
  participants?: { id?: number; username?: string; avatar_url?: string }[];
  last_message?: { text?: string; body?: string; msg_type?: string; created_at?: string };
  created_at?: string;
  updated_at?: string;
  unread_count?: number;
  unread?: boolean;
}

interface ConversationListProps {
  onSelect: (conversation: Conversation) => void;
  activeId?: number;
  onRetry?: () => void;
  // 会话未读总数上抛（FIX-31b/F-093）：消息页会话 Tab 计数改真实聚合。
  onUnreadCountChange?: (count: number) => void;
}

export function ConversationList({ onSelect, activeId, onRetry, onUnreadCountChange }: ConversationListProps) {
  const t = useTranslations();
  const locale = useLocale();
  const { user } = useAuth();
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [search, setSearch] = useState("");

  const loadConversations = useCallback(async () => {
    try {
      setError(false);
      const data = await api.get<{ conversations?: Conversation[] }>("/api/v1/messages");
      const next = data.conversations || [];
      setConversations(next);
      onUnreadCountChange?.(next.reduce((sum, c) => sum + (c.unread_count ?? (c.unread ? 1 : 0)), 0));
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [onUnreadCountChange]);

  useEffect(() => {
    if (user) void loadConversations();
  }, [user, loadConversations]);

  const filtered = (search.trim()
    ? conversations.filter((conversation) => {
        const other = conversation.participants?.find((participant) => participant.id !== user?.id);
        return other?.username?.toLowerCase().includes(search.toLowerCase());
      })
    : conversations).slice().sort((a, b) => parseDate(b.updated_at ?? b.created_at) - parseDate(a.updated_at ?? a.created_at));

  if (loading) {
    return (
      <div aria-label={t("messages.a11y.conversationList")} className="divide-y divide-border-default">
        {[1, 2, 3, 4, 5, 6].map((item) => (
          <div key={item} className="flex min-h-16 items-center gap-3 px-3 py-3">
            <div className="h-10 w-10 shrink-0 animate-pulse rounded-full bg-canvas-subtle" />
            <div className="flex-1 space-y-2">
              <div className="h-3 w-2/3 animate-pulse rounded bg-canvas-subtle" />
              <div className="h-3 w-5/6 animate-pulse rounded bg-canvas-subtle" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="flex h-full min-w-0 flex-col" aria-label={t("messages.a11y.conversationList")}>
      <div className="relative border-b border-border-default px-3 py-3">
        <Search className="pointer-events-none absolute left-5 top-1/2 h-4 w-4 -translate-y-1/2 text-fg-muted" aria-hidden="true" />
        <input
          type="search"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          aria-label={t("messages.conversations.searchLabel")}
          placeholder={t("messages.conversations.searchLabel")}
          className="min-h-11 w-full rounded-md border border-border-default bg-canvas-default py-1.5 pl-8 pr-2 text-sm text-fg-default focus:outline-none focus:ring-2 focus:ring-accent-emphasis"
        />
      </div>
      <div className="flex-1 overflow-y-auto">
        {error ? (
          <div className="flex flex-col items-center gap-3 px-4 py-10 text-center">
            <MessageSquare className="h-6 w-6 text-fg-muted" aria-hidden="true" />
            <p className="text-sm text-fg-muted">{t("messages.error.conversations")}</p>
            <button type="button" onClick={() => { void loadConversations(); onRetry?.(); }} className="min-h-11 rounded-md border border-border-default px-3 text-sm text-accent-emphasis hover:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-accent-emphasis">
              {t("messages.conversations.retry")}
            </button>
          </div>
        ) : filtered.length === 0 ? (
          <div className="flex flex-col items-center gap-2 px-4 py-10 text-center">
            <MessageSquare className="h-6 w-6 text-fg-muted" aria-hidden="true" />
            <p className="text-sm font-medium text-fg-default">{t("messages.conversations.emptyTitle")}</p>
            <p className="text-xs text-fg-muted">{t("messages.conversations.emptyDescription")}</p>
          </div>
        ) : (
          filtered.map((conversation) => {
            const other = conversation.participants?.find((participant) => participant.id !== user?.id);
            const unreadCount = conversation.unread_count ?? (conversation.unread ? 1 : 0);
            const lastMessage = conversation.last_message?.msg_type === "collab_invite"
              ? t("messages.conversations.collabInviteSummary")
              : conversation.last_message?.text ?? conversation.last_message?.body ?? t("messages.conversations.startConversation");
            return (
              <button
                key={conversation.id}
                type="button"
                onClick={() => onSelect(conversation)}
                aria-current={activeId === conversation.id ? "true" : undefined}
                className={`flex min-h-16 w-full items-center border-b border-border-default border-l-2 px-3 py-3 text-left transition-colors hover:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-inset focus:ring-accent-emphasis ${activeId === conversation.id ? "border-l-accent-emphasis bg-canvas-subtle" : "border-l-transparent"}`}
              >
                <div className="flex min-w-0 w-full items-center gap-3">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-accent-subtle text-sm font-semibold text-accent-emphasis">
                    {(other?.username ?? "?").slice(0, 1).toUpperCase()}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center justify-between gap-2">
                      <span className={`truncate text-sm ${unreadCount > 0 ? "font-semibold text-fg-default" : "font-medium text-fg-default"}`}>
                        {other?.username ?? t("messages.conversations.unknownParticipant")}
                      </span>
                      {unreadCount > 0 && (
                        <span className="inline-flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-accent-emphasis px-1.5 text-[10px] font-medium leading-none text-white" aria-label={t("messages.conversations.unreadCount", { count: unreadCount })}>
                          {unreadCount > 99 ? "99+" : unreadCount}
                        </span>
                      )}
                    </div>
                    <p className="mt-0.5 line-clamp-1 text-xs text-fg-muted">{lastMessage}</p>
                    <time className="mt-1 block text-[11px] text-fg-muted" dateTime={conversation.updated_at ?? conversation.created_at}>
                      {formatTime(conversation.updated_at ?? conversation.created_at, locale, t("messages.conversations.timeUnknown"))}
                    </time>
                  </div>
                </div>
              </button>
            );
          })
        )}
      </div>
    </div>
  );
}

function parseDate(value: string | undefined) {
  if (!value) return 0;
  const timestamp = Date.parse(value);
  return Number.isNaN(timestamp) ? 0 : timestamp;
}

function formatTime(value: string | undefined, locale: string, fallback: string) {
  if (!value) return fallback;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? fallback : date.toLocaleString(locale === "en" ? "en-US" : "zh-CN");
}
