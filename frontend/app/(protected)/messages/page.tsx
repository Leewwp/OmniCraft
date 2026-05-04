"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";

interface Notification {
  id: number;
  type: string;
  channel: string;
  body: string;
  is_read: boolean;
  created_at: string;
}

export default function MessagesPage() {
  const { user, isLoading } = useAuth();
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!user) return;
    void loadNotifications();
  }, [user]);

  async function loadNotifications() {
    setError("");
    setLoading(true);
    try {
      const data = await api.get<{ notifications?: Notification[] }>("/api/v1/notifications");
      setNotifications(data.notifications || []);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }

  async function markRead(id: number) {
    try {
      await api.patch(`/api/v1/notifications/${id}/read`, {});
      setNotifications((prev) => prev.map((n) => (n.id === id ? { ...n, is_read: true } : n)));
    } catch { /* ignore */ }
  }

  async function markAllRead() {
    try {
      await api.post("/api/v1/notifications/read-all", {});
      setNotifications((prev) => prev.map((n) => ({ ...n, is_read: true })));
    } catch { /* ignore */ }
  }

  if (isLoading || loading) {
    return <div className="mx-auto w-full max-w-2xl px-4 py-6 text-sm text-muted-foreground">加载中...</div>;
  }

  const unreadCount = notifications.filter((n) => !n.is_read).length;

  return (
    <div className="mx-auto w-full max-w-2xl space-y-4 px-4 py-6">
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4 shadow-none">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">消息中心</h1>
          <p className="mt-1 text-sm text-muted-foreground">{unreadCount} 条未读</p>
        </div>
        {unreadCount > 0 && (
          <Button size="sm" variant="outline" onClick={() => void markAllRead()}>
            全部已读
          </Button>
        )}
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {notifications.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center shadow-none">
          <p className="text-sm text-muted-foreground">暂无消息</p>
        </div>
      ) : (
        <div className="space-y-2">
          {notifications.map((n) => (
            <div
              key={n.id}
              className={`flex items-start justify-between rounded-md border p-3 shadow-none ${
                n.is_read ? "border-border bg-card" : "border-accent/30 bg-accent/5"
              }`}
            >
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground">{n.type}</span>
                  {!n.is_read && <span className="h-2 w-2 rounded-full bg-accent" />}
                </div>
                <p className="text-sm">{n.body}</p>
                <p className="text-xs text-muted-foreground">
                  {new Date(n.created_at).toLocaleString("zh-CN")}
                </p>
              </div>
              {!n.is_read && (
                <Button size="sm" variant="ghost" onClick={() => void markRead(n.id)}>
                  已读
                </Button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
