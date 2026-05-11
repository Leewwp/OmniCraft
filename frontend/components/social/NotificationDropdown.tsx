"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Bell } from "lucide-react";
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

export function NotificationDropdown() {
  const { user } = useAuth();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);

  useEffect(() => {
    if (!user) return;
    let cancelled = false;
    async function fetchData() {
      try {
        const data = await api.get<{ notifications?: Notification[] }>(
          "/api/v1/notifications"
        );
        if (cancelled) return;
        const list = (data.notifications || []).slice(0, 5);
        setNotifications(list);
      } catch {
        // silent
      }
      try {
        const data = await api.get<{ reply?: number; like?: number; system?: number }>(
          "/api/v1/notifications/unread-count"
        );
        if (cancelled) return;
        const total = (data.reply || 0) + (data.like || 0) + (data.system || 0);
        setUnreadCount(total);
      } catch {
        // silent
      }
    }
    fetchData();
    const interval = setInterval(fetchData, 30000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [user]);

  if (!user) return null;

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className={cn(
          "relative inline-flex h-8 w-8 items-center justify-center rounded-md transition-all duration-150 hover:bg-muted active:scale-90"
        )}
        aria-label="通知"
      >
        <Bell className="h-4 w-4" />
        {unreadCount > 0 && (
          <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-bold text-white">
            {unreadCount > 9 ? "9+" : unreadCount}
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
              <span className="text-sm font-medium">通知</span>
              <button
                onClick={() => {
                  setOpen(false);
                  router.push("/messages");
                }}
                className="text-xs text-accent hover:underline"
              >
                查看全部
              </button>
            </div>
            <div className="max-h-72 overflow-y-auto">
              {notifications.length === 0 ? (
                <p className="px-4 py-6 text-center text-sm text-muted-foreground">
                  暂无通知
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
                        {new Date(n.created_at).toLocaleString("zh-CN")}
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
