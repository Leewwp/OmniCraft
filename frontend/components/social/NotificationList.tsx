"use client";

import { useState, useEffect, useCallback } from "react";
import { useTranslations, useLocale } from "next-intl";
import { useRouter } from "next/navigation";
import { Bell } from "lucide-react";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";

interface Notification {
  id: number;
  type: string;
  channel: string;
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

  const CHANNELS = [
    { key: "", label: t('notification.all') },
    { key: "reply", label: t('notification.reply') },
    { key: "like", label: t('notification.like') },
    { key: "follow", label: t('notification.channelFollow') },
    { key: "pr", label: t('notification.channelPR') },
    { key: "system", label: t('notification.system') },
  ];

  const loadNotifications = useCallback(async () => {
    setError("");
    setLoading(true);
    try {
      const params = channel ? `?channel=${channel}` : "";
      const data = await api.get<{ notifications?: Notification[] }>(
        `/api/v1/notifications${params}`
      );
      setNotifications(data.notifications || []);
      if (onUnreadCountChange) {
        const unread = (data.notifications || []).filter((n) => !n.is_read).length;
        onUnreadCountChange(unread);
      }
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t('common.loadFailed'));
      silentError(e, { component: 'NotificationList', action: 'loadNotifications' });
    } finally {
      setLoading(false);
    }
  }, [channel, onUnreadCountChange, t]);

  useEffect(() => {
    void loadNotifications();
  }, [loadNotifications]);

  async function markRead(id: number) {
    try {
      await api.patch(`/api/v1/notifications/${id}/read`, {});
      setNotifications((prev) =>
        prev.map((n) => (n.id === id ? { ...n, is_read: true } : n))
      );
    } catch (e) {
      silentError(e, { component: 'NotificationList', action: 'markRead' });
    }
  }

  function handleNotificationClick(n: Notification) {
    if (!n.is_read) markRead(n.id);
    if (n.target_type && n.target_id) {
      switch (n.target_type) {
        case "content":
          router.push(`/content/${n.target_id}`);
          break;
        case "discussion":
        case "comment":
          router.push(`/content/${n.target_id}`);
          break;
        case "pr":
          router.push(`/studio/pr-requests`);
          break;
        case "user":
          router.push(`/user/${n.target_id}`);
          break;
        case "ip":
          router.push(`/ip/${n.target_id}`);
          break;
        default:
          router.push("/messages");
      }
    }
  }

  async function markAllRead() {
    try {
      const params = channel ? `?channel=${channel}` : "";
      await api.post(`/api/v1/notifications/read-all${params}`, {});
      setNotifications((prev) => prev.map((n) => ({ ...n, is_read: true })));
    } catch (e) {
      silentError(e, { component: 'NotificationList', action: 'markAllRead' });
    }
  }

  const unreadCount = notifications.filter((n) => !n.is_read).length;

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div className="flex gap-1">
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
            <div key={i} className="h-16 animate-pulse rounded-md bg-muted/60" />
          ))}
        </div>
      ) : notifications.length === 0 ? (
        <div className="flex flex-col items-center gap-3 rounded-md border border-border bg-card px-4 py-12 text-center">
          <Bell className="h-8 w-8 text-muted-foreground/40" />
          <p className="text-sm text-muted-foreground">{t('messages.noMessages')}</p>
        </div>
      ) : (
        <div className="space-y-1">
          {notifications.map((n) => (
            <button
              key={n.id}
              type="button"
              onClick={() => handleNotificationClick(n)}
              className={`flex w-full items-start justify-between rounded-md border px-4 py-3 text-left transition-colors hover:bg-muted/50 ${
                n.is_read
                  ? "border-border bg-card"
                  : "border-accent/30 bg-accent/5"
              }`}
            >
              <div className="min-w-0 flex-1 space-y-1">
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground capitalize">
                    {n.type}
                  </span>
                  {!n.is_read && (
                    <span className="h-2 w-2 shrink-0 rounded-full bg-accent" />
                  )}
                </div>
                <p className="text-sm">{n.body}</p>
                <p className="text-xs text-muted-foreground">
                  {new Date(n.created_at).toLocaleString(locale === "en" ? "en-US" : "zh-CN")}
                </p>
              </div>
              {!n.is_read && (
                <Button
                  size="sm"
                  variant="ghost"
                  className="ml-2 shrink-0"
                  onClick={() => markRead(n.id)}
                >
                  {t('messages.read')}
                </Button>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
