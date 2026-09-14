"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { ConversationList, type Conversation } from "@/components/social/ConversationList";
import { ChatWindow } from "@/components/social/ChatWindow";
import {
  MESSAGE_CHANNELS,
  MessageCategoryNav,
  type MessageChannel,
} from "@/components/messages/MessageCategoryNav";
import { NotificationDetailList } from "@/components/messages/NotificationDetailList";

/* 消息中心（SP-18 #509，B 站式双栏）：
   左栏 = 分类导航（通知七分类 + 私信，未读 pill 徽标，选中态高亮可辨）；
   内容区 = 分类标题 + 全部已读 + 通知详情列表（§4.1）或私信三栏之会话+聊天
   （§4.2，复用 ConversationList/ChatWindow 换肤）。
   路由 = ?channel=（默认 all，?channel=dm&c=<会话> 深链）；旧 ?tab= 客户端
   重定向兼容（tab=messages→dm、tab=notifications→all）。 */

function normalizeChannel(raw: string | null): MessageChannel {
  const allowed: MessageChannel[] = ["all", "reply", "like", "follow", "pr", "system", "broadcast", "dm"];
  return allowed.includes(raw as MessageChannel) ? (raw as MessageChannel) : "all";
}

export default function MessagesPage() {
  return (
    <Suspense fallback={null}>
      <MessagesPageContent />
    </Suspense>
  );
}

function MessagesPageContent() {
  const t = useTranslations();
  const router = useRouter();
  const searchParams = useSearchParams();
  const { user, unreadCounts, refreshUser } = useAuth();
  const [dmUnread, setDmUnread] = useState(0);
  const [activeConv, setActiveConv] = useState<Conversation | null>(null);

  const channel = normalizeChannel(searchParams.get("channel"));
  const conversationIdParam = searchParams.get("c");

  /* 旧参兼容（§3.2）：?tab=messages → ?channel=dm、?tab=notifications → ?channel=all。 */
  useEffect(() => {
    const tab = searchParams.get("tab");
    if (tab === "messages" || tab === "notifications") {
      const next = new URLSearchParams(searchParams.toString());
      next.delete("tab");
      if (tab === "messages" && !next.get("channel")) next.set("channel", "dm");
      router.replace(`/messages${next.toString() ? `?${next.toString()}` : ""}`);
    }
  }, [searchParams, router]);

  const selectChannel = useCallback(
    (next: MessageChannel) => {
      setActiveConv(null);
      router.push(next === "all" ? "/messages" : `/messages?channel=${next}`);
    },
    [router],
  );

  const channelTitle = useMemo(() => {
    if (channel === "dm") return t("messages.tabs.conversations");
    const def = MESSAGE_CHANNELS.find((c) => c.key === channel);
    return t(def?.labelKey ?? "notification.all");
  }, [channel, t]);

  /* 徽标联动（§5.4）：读/全部已读后静默校准左栏/下拉/顶栏——refreshUser
     换 user 对象身份触发 AuthContext 的 unread-count 重拉管线。 */
  const refreshNotificationCounts = useCallback(() => {
    void refreshUser();
  }, [refreshUser]);

  /* 未登录保护：(protected) 布局已挡；此处防御式早退。 */
  if (!user) return null;

  return (
    <div className="mx-auto w-full max-w-[1180px] px-4 py-6 md:px-6">
      {/* 移动端：分类折叠为顶部水平滚动 chips（§3.1 响应式）。 */}
      <div className="min-[768px]:hidden">
        <MessageCategoryNav
          active={channel}
          unreadCounts={unreadCounts}
          dmUnread={dmUnread}
          onSelect={selectChannel}
          orientation="horizontal"
        />
      </div>

      <div className="grid gap-6 min-[768px]:grid-cols-[216px_minmax(0,1fr)]">
        {/* 左栏：页标题 + 分类导航（216px）。 */}
        <aside className="hidden min-[768px]:block">
          <h1 className="mb-3 px-3 text-lg font-bold tracking-tight text-fg-default">{t("messages.detail.categoryTitle")}</h1>
          <MessageCategoryNav active={channel} unreadCounts={unreadCounts} dmUnread={dmUnread} onSelect={selectChannel} />
        </aside>

        {/* 内容区。 */}
        <main className="min-w-0">
          {channel === "dm" ? (
            <section aria-label={channelTitle} className="min-h-[560px]">
              <div className="hidden min-[768px]:grid min-[768px]:grid-cols-[280px_minmax(0,1fr)]">
                <div className="min-w-0 overflow-hidden rounded-l-md border border-r-0 border-border-default bg-card">
                  <ConversationList
                    onSelect={(c) => {
                      setActiveConv(c);
                      router.replace(`/messages?channel=dm&c=${c.id}`);
                    }}
                    activeId={activeConv?.id}
                    onUnreadCountChange={setDmUnread}
                    initialSelectedId={conversationIdParam ? Number(conversationIdParam) : undefined}
                  />
                </div>
                <div className="min-w-0">
                  <ChatWindow conversation={activeConv} />
                </div>
              </div>
              <div className="min-[768px]:hidden">
                {activeConv ? (
                  <ChatWindow conversation={activeConv} onBack={() => setActiveConv(null)} />
                ) : (
                  <ConversationList
                    onSelect={(c) => {
                      setActiveConv(c);
                      router.replace(`/messages?channel=dm&c=${c.id}`);
                    }}
                    activeId={undefined}
                    onUnreadCountChange={setDmUnread}
                    initialSelectedId={conversationIdParam ? Number(conversationIdParam) : undefined}
                  />
                )}
              </div>
            </section>
          ) : (
            <NotificationDetailList
              channel={channel}
              title={channelTitle}
              onCountsChanged={refreshNotificationCounts}
            />
          )}
        </main>
      </div>
    </div>
  );
}
