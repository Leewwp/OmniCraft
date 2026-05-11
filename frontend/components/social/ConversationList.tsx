"use client";

import { useEffect, useState, useCallback } from "react";
import { MessageSquare, Search } from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";

interface Conversation {
  id: number;
  participants?: { id?: number; username?: string; avatar_url?: string }[];
  last_message?: { text?: string; created_at?: string };
  created_at?: string;
  unread?: boolean;
}

interface ConversationListProps {
  onSelect: (conversation: Conversation) => void;
  activeId?: number;
}

export function ConversationList({ onSelect, activeId }: ConversationListProps) {
  const { user } = useAuth();
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");

  const loadConversations = useCallback(async () => {
    try {
      const data = await api.get<{ conversations?: Conversation[] }>(
        "/api/v1/conversations"
      );
      setConversations(data.conversations || []);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!user) return;
    void loadConversations();
  }, [user, loadConversations]);

  const filtered = search.trim()
    ? conversations.filter((c) => {
        const other = c.participants?.find((p) => p.id !== user?.id);
        return other?.username?.toLowerCase().includes(search.toLowerCase());
      })
    : conversations;

  if (loading) {
    return (
      <div className="space-y-1 px-2">
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-12 animate-pulse rounded-md bg-muted/60" />
        ))}
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <div className="relative border-b border-border px-3 py-2">
        <Search className="pointer-events-none absolute left-5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="搜索对话..."
          className="w-full rounded-md border border-border bg-background py-1.5 pl-7 pr-2 text-xs focus:outline-none focus:ring-1 focus:ring-accent"
        />
      </div>
      <div className="flex-1 overflow-y-auto">
        {filtered.length === 0 ? (
          <div className="flex flex-col items-center gap-2 py-8 text-center">
            <MessageSquare className="h-6 w-6 text-muted-foreground/30" />
            <p className="text-xs text-muted-foreground">
              {search.trim() ? "未找到匹配对话" : "暂无对话"}
            </p>
          </div>
        ) : (
          filtered.map((c) => {
            const other = c.participants?.find((p) => p.id !== user?.id);
            return (
              <button
                key={c.id}
                onClick={() => onSelect(c)}
                className={`w-full border-b border-border/50 px-4 py-3 text-left transition-colors hover:bg-muted/30 ${
                  activeId === c.id ? "bg-accent/5" : ""
                }`}
              >
                <div className="flex items-center gap-2">
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold">
                    {(other?.username ?? "#").slice(0, 1).toUpperCase()}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center justify-between">
                      <span
                        className={`truncate text-sm ${
                          c.unread ? "font-semibold text-foreground" : "text-foreground"
                        }`}
                      >
                        {other?.username ?? `#${c.id}`}
                      </span>
                      {c.unread && (
                        <span className="h-2 w-2 shrink-0 rounded-full bg-accent" />
                      )}
                    </div>
                    <p className="mt-0.5 truncate text-xs text-muted-foreground">
                      {c.last_message?.text ?? "开始对话"}
                    </p>
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
