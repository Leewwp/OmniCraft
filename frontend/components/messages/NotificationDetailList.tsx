"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Bell } from "lucide-react";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import {
  NotificationDetailItem,
  type DecoratedNotification,
} from "@/components/messages/NotificationDetailItem";

/* 分类通知详情列表（SP-18 #509 §4.1/§5.1）：channel 驱动拉取（空 = 全部聚合），
   滚动触底加载下一页；已读手动（条目级 hover 按钮 + 顶部全部已读，作用于当前
   分类）；读/全部已读后经 onCountsChanged 静默重拉 unread-count 校准徽标。 */

interface NotificationDetailListProps {
  channel: string;
  /** 当前分类标题（内容区头行左侧）。 */
  title: string;
  /** 已读操作后触发（左栏/下拉/顶栏徽标校准）。 */
  onCountsChanged?: () => void;
}

export function NotificationDetailList({ channel, title, onCountsChanged }: NotificationDetailListProps) {
  const t = useTranslations();
  const [items, setItems] = useState<DecoratedNotification[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const requestIdRef = useRef(0);
  const sentinelRef = useRef<HTMLDivElement>(null);

  const loadPage = useCallback(
    async (nextPage: number, replace: boolean) => {
      const requestId = ++requestIdRef.current;
      setError("");
      if (replace) setLoading(true);
      try {
        const params = new URLSearchParams({ page: String(nextPage), page_size: "20" });
        if (channel && channel !== "all") params.set("channel", channel);
        const data = await api.get<{ notifications?: DecoratedNotification[]; total?: number }>(
          `/api/v1/notifications?${params.toString()}`,
        );
        if (requestIdRef.current !== requestId) return;
        const next = data.notifications ?? [];
        setItems((prev) => (replace ? next : [...prev, ...next]));
        setTotal(data.total ?? next.length);
        setPage(nextPage);
      } catch (e) {
        if (requestIdRef.current !== requestId) return;
        setError(t(getUserFacingErrorKey(e, "common.loadFailed")));
        silentError(e, { component: "NotificationDetailList", action: "loadPage" });
      } finally {
        if (requestIdRef.current === requestId) setLoading(false);
      }
    },
    [channel, t],
  );

  useEffect(() => {
    setItems([]);
    void loadPage(1, true);
  }, [loadPage]);

  /* 滚动触底加载（§4.1）：哨兵进入视口且还有下一页时拉取；无
     IntersectionObserver 的环境（jsdom/极旧浏览器）静默降级为仅首页。 */
  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel || typeof IntersectionObserver === "undefined") return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting) && !loading && items.length < total) {
          void loadPage(page + 1, false);
        }
      },
      { rootMargin: "200px" },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [loadPage, loading, items.length, total, page]);

  const unreadCount = items.filter((n) => !n.is_read).length;

  function markRead(id: number) {
    setItems((prev) => prev.map((n) => (n.id === id ? { ...n, is_read: true } : n)));
    api.patch(`/api/v1/notifications/${id}/read`, {}).then(
      () => onCountsChanged?.(),
      (e) => silentError(e, { component: "NotificationDetailList", action: "markRead" }),
    );
  }

  async function markAllRead() {
    const params = channel && channel !== "all" ? `?channel=${channel}` : "";
    try {
      await api.post(`/api/v1/notifications/read-all${params}`, {});
      setItems((prev) => prev.map((n) => ({ ...n, is_read: true })));
      onCountsChanged?.();
    } catch (e) {
      silentError(e, { component: "NotificationDetailList", action: "markAllRead" });
    }
  }

  if (error && items.length === 0 && !loading) {
    return (
      <EmptyState
        icon={Bell}
        title={t("messages.error.notifications")}
        description={error}
        action={
          <Button type="button" variant="outline" onClick={() => void loadPage(1, true)}>
            {t("common.retry")}
          </Button>
        }
      />
    );
  }

  return (
    <div className="min-h-[420px] space-y-3">
      {/* 内容区头行：分类标题 + 全部已读（作用于当前分类，§3.1/§5.1）。 */}
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold text-fg-default">{title}</h2>
        {unreadCount > 0 && (
          <Button size="sm" variant="ghost" className="min-h-9 px-2 text-xs text-accent-emphasis" onClick={() => void markAllRead()}>
            {t("messages.markAllRead")}
          </Button>
        )}
      </div>

      {loading && items.length === 0 ? (
        <div className="space-y-3 p-2" aria-label={t("common.loading")}>
          {[1, 2].map((i) => (
            <div key={i} className="flex gap-3 p-2">
              <Skeleton className="h-8 w-8 shrink-0 rounded-full" />
              <div className="flex-1 space-y-2 py-1">
                <Skeleton className="h-4 w-2/5" />
                <Skeleton className="h-3 w-4/5" />
              </div>
            </div>
          ))}
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          icon={Bell}
          title={t("messages.noMessages")}
          description={t("messages.noNotificationsHint")}
          className="px-4 py-16"
        />
      ) : (
        <>
          <div className="overflow-hidden rounded-md border border-border-default bg-card">
            {items.map((n) => (
              <NotificationDetailItem key={n.id} notification={n} onMarkRead={markRead} />
            ))}
          </div>
          <div ref={sentinelRef} aria-hidden="true" className="h-px" />
          {items.length >= total ? (
            <p className="py-4 text-center text-xs text-fg-muted">{t("messages.detail.endOfList")}</p>
          ) : loading ? (
            <p className="py-4 text-center text-xs text-fg-muted">{t("common.loading")}</p>
          ) : null}
        </>
      )}
    </div>
  );
}
