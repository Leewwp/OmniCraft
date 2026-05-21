"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations, useLocale } from "next-intl";
import { Bell, Heart, MessageCircle, UserPlus, GitPullRequest, Info } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";

interface Notification {
  id: number;
  type: string;
  channel: string;
  body: string;
  is_read: boolean;
  created_at: string;
}

const channelIcons: Record<string, React.ReactNode> = {
  reply: <MessageCircle className="h-3.5 w-3.5" />,
  like: <Heart className="h-3.5 w-3.5" />,
  follow: <UserPlus className="h-3.5 w-3.5" />,
  pr: <GitPullRequest className="h-3.5 w-3.5" />,
  system: <Info className="h-3.5 w-3.5" />,
};

export function NotificationDropdown() {
  const t = useTranslations();
  const locale = useLocale();
  const { user, unreadCounts } = useAuth();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [notifications, setNotifications] = useState<Notification[]>([]);

  useEffect(() => {
    if (!user || !open) return;
    let cancelled = false;
    async function fetchData() {
      try {
        const data = await api.get<{ notifications?: Notification[] }>(
          "/api/v1/notifications?page_size=5"
        );
        if (cancelled) return;
        setNotifications((data.notifications || []).slice(0, 5));
      } catch {
        // silent
      }
    }
    fetchData();
    return () => { cancelled = true; };
  }, [user, open]);

  if (!user) return null;

  const channelLabels: Record<string, string> = {
    reply: t("notification.channelReply"),
    like: t("notification.channelLike"),
    follow: t("notification.channelFollow"),
    pr: t("notification.channelPR"),
    system: t("notification.channelSystem"),
  };

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className={cn(
          "relative inline-flex h-8 w-8 items-center justify-center rounded-md transition-all duration-150 hover:bg-muted active:scale-90"
        )}
        aria-label={t("nav.notifications")}
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
          <div className="absolute right-0 top-full z-50 mt-1 w-80 rounded-md border border-border bg-card shadow-md">
            <div className="flex items-center justify-between border-b border-border px-4 py-2">
              <span className="text-sm font-medium">{t("nav.notifications")}</span>
              <button
                onClick={() => {
                  setOpen(false);
                  router.push("/messages");
                }}
                className="text-xs text-accent hover:underline"
              >
                {t("common.clickToView")}
              </button>
            </div>
            {unreadCounts.total > 0 && (
              <div className="flex gap-1 border-b border-border/50 px-3 py-1.5 text-xs text-muted-foreground">
                {Object.entries(channelLabels).map(([ch, label]) =>
                  unreadCounts[ch as keyof typeof unreadCounts] > 0 ? (
                    <span key={ch} className="inline-flex items-center gap-1 rounded bg-muted px-1.5 py-0.5">
                      {channelIcons[ch]}
                      {label} {unreadCounts[ch as keyof typeof unreadCounts]}
                    </span>
                  ) : null
                )}
              </div>
            )}
            <div className="max-h-72 overflow-y-auto">
              {notifications.length === 0 ? (
                <p className="px-4 py-6 text-center text-sm text-muted-foreground">
                  {t("messages.noMessages")}
                </p>
              ) : (
                notifications.map((n) => (
                  <button
                    key={n.id}
                    onClick={() => {
                      setOpen(false);
                      router.push("/messages");
                    }}
                    className={cn(
                      "flex w-full items-start gap-3 border-b border-border/50 px-4 py-2.5 text-left transition-colors hover:bg-muted/50",
                      !n.is_read && "bg-accent/5"
                    )}
                  >
                    <span
                      className={cn(
                        "mt-1.5 h-2 w-2 shrink-0 rounded-full",
                        n.is_read ? "bg-transparent" : "bg-accent"
                      )}
                    />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm">{n.body}</p>
                      <p className="mt-0.5 text-xs text-muted-foreground">
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
