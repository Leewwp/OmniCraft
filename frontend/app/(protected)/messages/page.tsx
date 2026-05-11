"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { NotificationList } from "@/components/social/NotificationList";
import { ConversationList } from "@/components/social/ConversationList";
import { ChatWindow } from "@/components/social/ChatWindow";

interface Conversation {
  id: number;
  participants?: { id?: number; username?: string }[];
  last_message?: { text?: string; created_at?: string };
}

export default function MessagesPage() {
  const t = useTranslations();
  const { user, isLoading } = useAuth();
  const [tab, setTab] = useState<"notifications" | "messages">("notifications");
  const [activeConv, setActiveConv] = useState<Conversation | null>(null);
  const [unreadCount, setUnreadCount] = useState(0);

  if (isLoading) {
    return (
      <div className="mx-auto w-full max-w-2xl px-4 py-6 text-sm text-muted-foreground">
        {t("common.loading")}
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-2xl space-y-4 px-4 py-6">
      {/* Header */}
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("messages.title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {tab === "notifications"
              ? t("messages.unreadCount", { count: unreadCount })
              : t("messages.conversationCount", { count: 0 })}
          </p>
        </div>
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
            {tKey === "notifications"
              ? t("messages.tabNotifications")
              : t("messages.tabMessages")}
          </button>
        ))}
      </div>

      {/* Notifications Tab */}
      {tab === "notifications" && (
        <NotificationList onUnreadCountChange={setUnreadCount} />
      )}

      {/* Messages Tab */}
      {tab === "messages" && (
        <div className="flex gap-3" style={{ minHeight: 420 }}>
          <div className="w-48 shrink-0 border-r border-border pr-3 hidden sm:block">
            <ConversationList
              onSelect={(c) => setActiveConv(c)}
              activeId={activeConv?.id}
            />
          </div>
          <div className="w-48 shrink-0 border-r border-border pr-3 sm:hidden">
            {activeConv ? (
              <ChatWindow
                conversation={activeConv}
                onBack={() => setActiveConv(null)}
              />
            ) : (
              <ConversationList
                onSelect={(c) => setActiveConv(c)}
                activeId={undefined}
              />
            )}
          </div>
          <div className="hidden flex-1 sm:flex">
            <ChatWindow conversation={activeConv} />
          </div>
        </div>
      )}
    </div>
  );
}
