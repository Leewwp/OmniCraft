"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Bell } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { cn } from "@/lib/utils";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  DM_CHANNEL,
  MESSAGE_CHANNELS,
  type MessageChannel,
} from "@/components/messages/MessageCategoryNav";

/* 顶栏通知下拉（SP-18 #509 建立；SP-19 G1-1 重建于共享 DropdownMenu）：
   与「语言/个人/主题」菜单完全同交互——纯点击展开、点选项直达、点外/Esc
   关闭、键盘可达（Base UI 自带），动画随共享组件统一（fade+zoom-in-95），
   桌面不再有 hover 展开通道。职责仍是分类直达菜单（图标 + 文案 + 未读
   徽标含私信 + 底部「查看全部」）；徽标复用 unread-count 拉取节奏。 */

export function NotificationDropdown() {
  const t = useTranslations();
  const { user, unreadCounts } = useAuth();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  // 私信未读不在 unread-count 管线内（会话聚合）——面板展开时拉一次，不轮询。
  const [dmUnread, setDmUnread] = useState(0);

  const menuItems: { key: MessageChannel; label: string; badge: number }[] = [
    ...MESSAGE_CHANNELS.map((def) => ({
      key: def.key,
      label: t(def.labelKey),
      badge: def.key === "all" ? unreadCounts.total ?? 0 : (unreadCounts[def.key as keyof typeof unreadCounts] ?? 0),
    })),
    { key: "dm", label: t(DM_CHANNEL.labelKey), badge: dmUnread },
  ];

  useEffect(() => {
    if (!user || !open) return;
    let cancelled = false;
    api
      .get<{ conversations?: { unread_count?: number; unread?: boolean }[] }>("/api/v1/messages")
      .then((data) => {
        if (cancelled) return;
        setDmUnread(
          (data.conversations ?? []).reduce((sum, c) => sum + (c.unread_count ?? (c.unread ? 1 : 0)), 0),
        );
      })
      .catch((e) => {
        silentError(e, { component: "NotificationDropdown", action: "dmUnread" });
      });
    return () => {
      cancelled = true;
    };
  }, [user, open]);

  if (!user) return null;

  function selectChannel(channel: MessageChannel) {
    setOpen(false);
    router.push(channel === "all" ? "/messages" : `/messages?channel=${channel}`);
  }

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger
        aria-label={t("nav.notifications")}
        className="relative inline-flex h-11 w-11 items-center justify-center rounded-md transition-colors duration-150 hover:bg-canvas-subtle active:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-accent-emphasis data-popup-open:bg-canvas-subtle"
      >
        <Bell className="h-4 w-4" />
        {unreadCounts.total > 0 && (
          <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-bold text-white">
            {unreadCounts.total > 99 ? "99+" : unreadCounts.total}
          </span>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-72 p-0">
        <div className="border-b border-border-default px-4 py-2 text-sm font-medium">
          {t("nav.notifications")}
        </div>
        <div className="py-1.5">
          {menuItems.map((item) => (
            <DropdownMenuItem
              key={item.key}
              onClick={() => selectChannel(item.key)}
              className="min-h-11 gap-2.5 rounded-none px-4 text-sm text-fg-default focus:bg-canvas-subtle focus:text-fg-default"
            >
              <span aria-hidden="true">{menuIcon(item.key)}</span>
              <span>{item.label}</span>
              {item.badge > 0 && (
                <span className="ml-auto inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-accent-emphasis px-1.5 text-[10px] font-medium leading-none text-white">
                  {item.badge > 99 ? "99+" : item.badge}
                </span>
              )}
            </DropdownMenuItem>
          ))}
        </div>
        <DropdownMenuItem
          onClick={() => {
            setOpen(false);
            router.push("/messages");
          }}
          className="justify-center rounded-none border-t border-border-default px-4 py-2.5 text-center text-xs font-medium text-accent-emphasis focus:bg-canvas-subtle focus:text-accent-emphasis"
        >
          {t("messages.dropdown.viewAll")} →
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function menuIcon(channel: MessageChannel) {
  const def = channel === "dm" ? DM_CHANNEL : MESSAGE_CHANNELS.find((c) => c.key === channel);
  return def?.icon ?? MESSAGE_CHANNELS[0].icon;
}
