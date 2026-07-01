"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useTranslations, useLocale } from "next-intl";
import { useRouter } from "next/navigation";
import { Bell, Megaphone } from "lucide-react";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { MarkdownRenderer } from "@/components/content/MarkdownRenderer";
import { cn } from "@/lib/utils";

interface Notification {
  id: number;
  type: string;
  channel: string;
  title?: string;
  body: string;
  is_read: boolean;
  target_type?: string;
  target_id?: number;
  created_at: string;
}

interface NotificationListProps {
  initialChannel?: string;
  onUnreadCountChange?: (count: number) => void;
}

export function NotificationList({
  initialChannel = "",
  onUnreadCountChange,
}: NotificationListProps) {
  const t = useTranslations();
  const locale = useLocale();
  const router = useRouter();
  const [channel, setChannel] = useState(initialChannel);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const requestIdRef = useRef(0);

  const CHANNELS = [
    { key: "", label: t('notification.all') },
    { key: "reply", label: t('notification.reply') },
    { key: "like", label: t('notification.like') },
    { key: "follow", label: t('notification.channelFollow') },
    { key: "pr", label: t('notification.channelPR') },
    { key: "system", label: t('notification.system') },
    { key: "broadcast", label: t('notification.channelBroadcast') },
  ];

  const loadNotifications = useCallback(async () => {
    const requestId = requestIdRef.current + 1;
    requestIdRef.current = requestId;
    setError("");
    setLoading(true);
    try {
      const params = channel ? `?channel=${channel}` : "";
      const data = await api.get<{ notifications?: Notification[] }>(
        `/api/v1/notifications${params}`
      );
      if (requestIdRef.current !== requestId) return;
      const nextNotifications = data.notifications || [];
      setNotifications(nextNotifications);
      onUnreadCountChange?.(countUnread(nextNotifications));
    } catch (e) {
      if (requestIdRef.current !== requestId) return;
      setError(t(getUserFacingErrorKey(e, "common.loadFailed")));
      silentError(e, { component: 'NotificationList', action: 'loadNotifications' });
    } finally {
      if (requestIdRef.current === requestId) setLoading(false);
    }
  }, [channel, onUnreadCountChange, t]);

  useEffect(() => {
    void loadNotifications();
  }, [loadNotifications]);

  async function markRead(id: number) {
    try {
      await api.patch(`/api/v1/notifications/${id}/read`, {});
      setNotifications((prev) => {
        const next = prev.map((n) => (n.id === id ? { ...n, is_read: true } : n));
        onUnreadCountChange?.(countUnread(next));
        return next;
      });
    } catch (e) {
      silentError(e, { component: 'NotificationList', action: 'markRead' });
    }
  }

  function handleNotificationClick(n: Notification) {
    if (!n.is_read) markRead(n.id);
    const href = getNotificationHref(n);
    if (href) router.push(href);
  }

  async function markAllRead() {
    try {
      const params = channel ? `?channel=${channel}` : "";
      await api.post(`/api/v1/notifications/read-all${params}`, {});
      setNotifications((prev) => {
        const next = prev.map((n) => ({ ...n, is_read: true }));
        onUnreadCountChange?.(0);
        return next;
      });
    } catch (e) {
      silentError(e, { component: 'NotificationList', action: 'markAllRead' });
    }
  }

  const unreadCount = notifications.filter((n) => !n.is_read).length;

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div className="flex flex-wrap gap-1">
          {CHANNELS.map((c) => (
            <button
              key={c.key}
              onClick={() => setChannel(c.key)}
              className={`rounded-md px-3 py-1 text-xs transition-colors ${
                channel === c.key
                  ? "bg-accent/10 text-accent font-medium"
                  : "text-muted-foreground hover:bg-muted hover:text-foreground"
              }`}
            >
              {c.label}
            </button>
          ))}
        </div>
        {unreadCount > 0 && (
          <Button size="sm" variant="outline" onClick={markAllRead}>
            {t('messages.markAllRead')}
          </Button>
        )}
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {loading ? (
        <div className="space-y-2">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      ) : notifications.length === 0 ? (
        <EmptyState
          icon={Bell}
          title={t("messages.noMessages")}
          description={t("messages.noNotificationsHint")}
          className="px-4 py-12"
        />
      ) : (
        <div className="divide-y divide-border rounded-md border border-border bg-card">
          {notifications.map((n) => {
            const href = getNotificationHref(n);
            const canNavigate = Boolean(href);
            const isBroadcast = n.channel === "broadcast";
            const isSystemLike = isBroadcast || n.channel === "system";
            const channelLabel = getChannelLabel(n.channel, n.type, t);
            const targetLabel = canNavigate ? t("messages.notificationOpensTarget") : t("messages.notificationNoTarget");
            const itemClassName = cn(
              "flex w-full items-start justify-between gap-3 border-l-4 px-4 py-3 text-left transition-colors",
              "min-h-[84px]",
              canNavigate ? "cursor-pointer hover:bg-muted/50" : "cursor-default",
              isSystemLike ? "border-l-blue-500" : "border-l-transparent",
              n.is_read ? "bg-card" : "bg-accent/5",
            );
            const content = (
              <>
                <div className="min-w-0 flex-1 space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="inline-flex items-center gap-1 rounded-full border border-border bg-background px-2 py-0.5 text-xs text-muted-foreground">
                      {isBroadcast && <Megaphone className="h-3 w-3" aria-hidden="true" />}
                      {channelLabel}
                    </span>
                    {!n.is_read && (
                      <span className="h-2 w-2 shrink-0 rounded-full bg-accent" aria-label={t("messages.unreadIndicator")} />
                    )}
                  </div>
                  {n.title && <p className="text-sm font-medium text-foreground">{n.title}</p>}
                  <MarkdownRenderer
                    content={n.body}
                    className="prose-p:my-0 prose-strong:text-foreground text-sm"
                  />
                  <p className="text-xs text-muted-foreground">
                    {new Date(n.created_at).toLocaleString(locale === "en" ? "en-US" : "zh-CN")}
                  </p>
                </div>
                {!n.is_read && !canNavigate && (
                  <Button
                    size="sm"
                    variant="ghost"
                    className="ml-2 min-h-11 shrink-0"
                    onClick={() => markRead(n.id)}
                  >
                    {t('messages.read')}
                  </Button>
                )}
              </>
            );

            return canNavigate ? (
              <button
                key={n.id}
                type="button"
                aria-label={`${channelLabel}. ${targetLabel}`}
                onClick={() => handleNotificationClick(n)}
                className={itemClassName}
              >
                {content}
              </button>
            ) : (
              <div
                key={n.id}
                aria-label={`${channelLabel}. ${targetLabel}`}
                className={itemClassName}
              >
                {content}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

function getNotificationHref(n: Notification): string | null {
  if (!n.target_type || !isValidTargetId(n.target_id)) return null;
  switch (n.target_type) {
    case "content":
    case "discussion":
    case "comment":
      return `/content/${n.target_id}`;
    case "pr":
      return "/studio/pr-requests";
    case "user":
      return `/user/${n.target_id}`;
    case "ip":
      return `/ip/${n.target_id}`;
    default:
      return null;
  }
}

function isValidTargetId(targetId?: number): targetId is number {
  return typeof targetId === "number" && Number.isSafeInteger(targetId) && targetId > 0;
}

function countUnread(notifications: Notification[]): number {
  return notifications.filter((n) => !n.is_read).length;
}

function getChannelLabel(channel: string, type: string, t: ReturnType<typeof useTranslations>): string {
  switch (channel) {
    case "broadcast":
      return t("messages.broadcastLabel");
    case "system":
      return t("messages.systemLabel");
    case "reply":
      return t("notification.channelReply");
    case "like":
      return t("notification.channelLike");
    case "follow":
      return t("notification.channelFollow");
    case "pr":
      return t("notification.channelPR");
    default:
      return type;
  }
}
