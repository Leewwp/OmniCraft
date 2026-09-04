"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations, useLocale } from "next-intl";
import { Bell, Heart, MessageCircle, UserPlus, GitPullRequest, Info, Megaphone } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import { resolveNotificationHref } from "@/lib/notification-url";

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

const channelIcons: Record<string, React.ReactNode> = {
  reply: <MessageCircle className="h-3.5 w-3.5" />,
  like: <Heart className="h-3.5 w-3.5" />,
  follow: <UserPlus className="h-3.5 w-3.5" />,
  pr: <GitPullRequest className="h-3.5 w-3.5" />,
  system: <Info className="h-3.5 w-3.5" />,
  broadcast: <Megaphone className="h-3.5 w-3.5" />,
};

export function NotificationDropdown() {
  const t = useTranslations();
  const locale = useLocale();
  const { user, unreadCounts } = useAuth();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  const [retry, setRetry] = useState(0);
  const triggerRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!user || !open) return;
    let cancelled = false;
    async function fetchData() {
      try {
        setLoading(true);
        setError(false);
        const data = await api.get<{ notifications?: Notification[] }>(
          "/api/v1/notifications?page_size=5"
        );
        if (cancelled) return;
        setNotifications((data.notifications || []).slice(0, 5));
      } catch {
        if (!cancelled) setError(true);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    fetchData();
    return () => { cancelled = true; };
  }, [user, open, retry]);

  useEffect(() => {
    if (!open) return;
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false);
        triggerRef.current?.focus();
      }
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [open]);

  if (!user) return null;

  // 下拉项点击（FIX-31b）：未读先标记已读（不阻塞跳转），统一走深链映射
  // （讨论二跳、申诉/反馈/私信各归位）。
  async function openNotification(n: Notification) {
    setOpen(false);
    if (!n.is_read) {
      setNotifications((prev) => prev.map((item) => (item.id === n.id ? { ...item, is_read: true } : item)));
      api.patch(`/api/v1/notifications/${n.id}/read`, {}).catch(() => {
        // 标记失败静默：下次轮询/打开下拉自然校正
      });
    }
    const href = await resolveNotificationHref(n);
    router.push(href);
  }

  const channelLabels: Record<string, string> = {
    reply: t("notification.channelReply"),
    like: t("notification.channelLike"),
    follow: t("notification.channelFollow"),
    pr: t("notification.channelPR"),
    system: t("notification.channelSystem"),
    broadcast: t("notification.channelBroadcast"),
  };

  return (
    <div className="relative">
      <button
        type="button"
        ref={triggerRef}
        onClick={() => setOpen(!open)}
        className={cn(
          "relative inline-flex h-11 w-11 items-center justify-center rounded-md transition-colors duration-150 hover:bg-canvas-subtle active:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-accent-emphasis"
        )}
        aria-label={t("nav.notifications")}
        aria-expanded={open}
        aria-haspopup="dialog"
      >
        <Bell className="h-4 w-4" />
        {unreadCounts.total > 0 && (
          <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-bold text-white">
            {unreadCounts.total > 99 ? "99+" : unreadCounts.total}
          </span>
        )}
      </button>

      {open && (
        <>
          <div
            className="fixed inset-0 z-40"
            onClick={() => setOpen(false)}
          />
          <div role="dialog" aria-label={t("nav.notifications")} className="absolute right-0 top-full z-50 mt-1 w-[min(20rem,calc(100vw-2rem))] rounded-md border border-border-default bg-canvas-default shadow-none">
            <div className="flex items-center justify-between border-b border-border-default px-4 py-2">
              <span className="text-sm font-medium">{t("nav.notifications")}</span>
<button
                onClick={() => {
                  setOpen(false);
                  router.push("/messages");
                }}
                className="min-h-11 rounded-md px-2 text-xs font-medium text-accent-emphasis hover:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-accent-emphasis"
              >
                {t("common.clickToView")}
              </button>
            </div>
            {unreadCounts.total > 0 && (
              <div className="flex flex-wrap gap-1 border-b border-border-default px-3 py-1.5 text-xs text-fg-muted">
                {Object.entries(channelLabels).map(([ch, label]) =>
                  unreadCounts[ch as keyof typeof unreadCounts] > 0 ? (
                    <span key={ch} className="inline-flex min-h-6 items-center gap-1 rounded bg-canvas-subtle px-1.5 py-0.5">
                      {channelIcons[ch]}
                      {label} {unreadCounts[ch as keyof typeof unreadCounts]}
                    </span>
                  ) : null
                )}
              </div>
            )}
            <div className="max-h-72 overflow-y-auto">
              {loading ? (
                <div className="space-y-2 p-3" aria-label={t("common.loading")}>
                  {[1, 2, 3].map((item) => <div key={item} className="h-12 animate-pulse rounded-md bg-canvas-subtle" />)}
                </div>
              ) : error ? (
                <div className="flex flex-col items-center gap-2 px-4 py-6 text-center" role="alert">
                  <p className="text-sm text-fg-muted">{t("messages.error.notifications")}</p>
                  <button type="button" onClick={() => setRetry((value) => value + 1)} className="min-h-11 rounded-md border border-border-default px-3 text-sm text-accent-emphasis hover:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-accent-emphasis">
                    {t("messages.error.retry")}
                  </button>
                </div>
              ) : notifications.length === 0 ? (
                <p className="px-4 py-6 text-center text-sm text-fg-muted">
                  {t("messages.notifications.emptyTitle")}
                </p>
              ) : (
                notifications.map((n) => (
                  <button
                    key={n.id}
                    onClick={() => { void openNotification(n); }}
                    className={cn(
                      "flex min-h-[72px] w-full items-start gap-3 border-b border-border-default px-4 py-3 text-left transition-colors hover:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-inset focus:ring-accent-emphasis",
                      !n.is_read && "bg-accent-subtle"
                    )}
                    aria-label={`${n.channel === "broadcast" ? t("messages.broadcast.label") : channelLabels[n.channel] ?? t("notification.channelSystem")} · ${!n.is_read ? t("messages.a11y.unread") : t("messages.read")}`}
                  >
                    <span className="mt-0.5 shrink-0 text-fg-muted" aria-hidden="true">{channelIcons[n.channel] ?? channelIcons.system}</span>
                    <div className="min-w-0 flex-1">
                      {n.channel === "broadcast" && <span className="text-xs font-medium text-accent-emphasis">{t("messages.broadcast.label")}</span>}
                      <p className="truncate text-sm">{n.body}</p>
                      <p className="mt-0.5 text-xs text-fg-muted">
                        {new Date(n.created_at).toLocaleString(locale === "en" ? "en-US" : "zh-CN")}
                      </p>
                    </div>
                  </button>
                ))
              )}
            </div>
          </div>
        </>
      )}
    </div>
  );
}
