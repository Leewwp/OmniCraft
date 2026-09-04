"use client";

import { Suspense, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { NotificationList } from "@/components/social/NotificationList";
import { ConversationList, type Conversation } from "@/components/social/ConversationList";
import { ChatWindow } from "@/components/social/ChatWindow";
import { MarkdownRenderer } from "@/components/content/MarkdownRenderer";
import type { Notification } from "@/components/social/NotificationList";

export default function MessagesPage() {
  return (
    <Suspense fallback={null}>
      <MessagesPageContent />
    </Suspense>
  );
}

function MessagesPageContent() {
  const t = useTranslations();
  const { user } = useAuth();
  const searchParams = useSearchParams();
  const [tab, setTab] = useState<"notifications" | "messages">(
    searchParams.get("tab") === "messages" ? "messages" : "notifications"
  );
  const [activeConv, setActiveConv] = useState<Conversation | null>(null);
  const [selectedNotification, setSelectedNotification] = useState<Notification | null>(null);
  const [unreadCount, setUnreadCount] = useState(0);
  // 会话 Tab 未读真实聚合（FIX-31b/F-093）：由 ConversationList 上抛。
  const [convUnreadCount, setConvUnreadCount] = useState(0);

  // ?tab=messages 深链（FIX-31b：私信通知归位到会话 Tab）。
  useEffect(() => {
    if (searchParams.get("tab") === "messages") setTab("messages");
  }, [searchParams]);

  return (
    <div className="mx-auto w-full max-w-[1180px] space-y-4 px-4 py-6 md:px-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-fg-default">{t("messages.title")}</h1>
          <p className="mt-1 text-sm text-fg-muted" aria-live="polite">
            {tab === "notifications"
              ? t("messages.tabs.notificationsCount", { count: unreadCount })
              : t("messages.tabs.conversationsCount", { count: convUnreadCount })}
          </p>
        </div>
      </div>

      <div role="tablist" aria-label={t("messages.a11y.tabs")} className="flex gap-1 border-b border-border-default">
        {(["notifications", "messages"] as const).map((tKey) => (
          <button
            key={tKey}
            type="button"
            role="tab"
            id={`messages-tab-${tKey}`}
            aria-controls={`messages-panel-${tKey}`}
            aria-selected={tab === tKey}
            onClick={() => { setTab(tKey); setActiveConv(null); setSelectedNotification(null); }}
            className={`min-h-11 border-b-2 px-4 py-2 text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-accent-emphasis ${
              tab === tKey
                ? "border-accent-emphasis font-medium text-accent-emphasis"
                : "border-transparent text-fg-muted hover:bg-canvas-subtle hover:text-fg-default"
            }`}
          >
            {tKey === "notifications"
              ? t("messages.tabs.notifications")
              : t("messages.tabs.conversations")}
          </button>
        ))}
      </div>

      {tab === "notifications" && (
        <section id="messages-panel-notifications" role="tabpanel" aria-labelledby="messages-tab-notifications" className="min-h-[420px]">
          <div className="hidden min-[701px]:grid min-[701px]:grid-cols-[280px_minmax(0,1fr)] min-[1101px]:grid-cols-[320px_minmax(0,1fr)]">
            <div className="min-w-0 border-r border-border-default pr-4">
              <NotificationList onUnreadCountChange={setUnreadCount} onSelect={setSelectedNotification} />
            </div>
            <div className="flex min-h-[420px] min-w-0 items-center justify-center px-8 text-center">
              {selectedNotification ? (
                <article className="w-full max-w-2xl text-left">
                  <h2 className="text-lg font-semibold text-fg-default">{selectedNotification.title ?? t("messages.notifications.detailTitle")}</h2>
                  <time className="mt-2 block text-xs text-fg-muted" dateTime={selectedNotification.created_at}>{new Date(selectedNotification.created_at).toLocaleString()}</time>
                  <MarkdownRenderer content={selectedNotification.body} className="mt-4 text-sm text-fg-default" />
                </article>
              ) : (
                <div>
                  <h2 className="text-sm font-medium text-fg-default">{t("messages.notifications.selectTitle")}</h2>
                  <p className="mt-1 text-xs text-fg-muted">{t("messages.notifications.selectDescription")}</p>
                </div>
              )}
            </div>
          </div>
          <div className="min-[701px]:hidden">
            <NotificationList onUnreadCountChange={setUnreadCount} onSelect={setSelectedNotification} />
          </div>
        </section>
      )}

      {tab === "messages" && (
        <section id="messages-panel-messages" role="tabpanel" aria-labelledby="messages-tab-messages" className="min-h-[420px]">
          <div className="hidden min-[701px]:grid min-[701px]:grid-cols-[280px_minmax(0,1fr)] min-[1101px]:grid-cols-[320px_minmax(0,1fr)]">
            <div className="min-w-0 overflow-hidden rounded-l-md border border-r-0 border-border-default bg-card">
              <ConversationList
                onSelect={(c) => setActiveConv(c)}
                activeId={activeConv?.id}
                onUnreadCountChange={setConvUnreadCount}
              />
            </div>
            <div className="min-w-0">
              <ChatWindow conversation={activeConv} />
            </div>
          </div>

          <div className="min-[701px]:hidden">
            {activeConv ? (
              <ChatWindow
                conversation={activeConv}
                onBack={() => setActiveConv(null)}
              />
            ) : (
              <ConversationList
                onSelect={(c) => setActiveConv(c)}
                activeId={undefined}
                onUnreadCountChange={setConvUnreadCount}
              />
            )}
          </div>
        </section>
      )}
    </div>
  );
}
