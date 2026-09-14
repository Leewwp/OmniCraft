"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Bell } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { cn } from "@/lib/utils";
import {
  DM_CHANNEL,
  MESSAGE_CHANNELS,
  type MessageChannel,
} from "@/components/messages/MessageCategoryNav";

/* 顶栏通知下拉（SP-18 #509 §4.3/§5.2，B 站式）：职责收敛为分类直达菜单——
   图标 + 文案 + 未读徽标（含私信）+ 底部「查看全部」；不再展示最近通知条目
   （行为变更，spec 默认接受）。交互：hover 150ms 展开 / 移出 250ms 收起
   （移入面板取消）；触屏点击切换；键盘 Enter/↓ 展开、↑↓ 移动、Enter 选中、
   Esc 收起，菜单项可 tab。徽标复用 unread-count 拉取节奏，不新增轮询。 */

const OPEN_DELAY_MS = 150;
const CLOSE_DELAY_MS = 250;

export function NotificationDropdown() {
  const t = useTranslations();
  const { user, unreadCounts } = useAuth();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [focusIndex, setFocusIndex] = useState(-1);
  // 私信未读不在 unread-count 管线内（会话聚合）——面板展开时拉一次，不轮询。
  const [dmUnread, setDmUnread] = useState(0);
  const wrapRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const openTimerRef = useRef<number | null>(null);
  const closeTimerRef = useRef<number | null>(null);

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

  function clearTimers() {
    if (openTimerRef.current !== null) window.clearTimeout(openTimerRef.current);
    if (closeTimerRef.current !== null) window.clearTimeout(closeTimerRef.current);
    openTimerRef.current = null;
    closeTimerRef.current = null;
  }

  const openNow = () => {
    clearTimers();
    setOpen(true);
  };
  const closeNow = () => {
    clearTimers();
    setOpen(false);
    setFocusIndex(-1);
  };
  const scheduleOpen = () => {
    clearTimers();
    openTimerRef.current = window.setTimeout(openNow, OPEN_DELAY_MS);
  };
  const scheduleClose = () => {
    clearTimers();
    closeTimerRef.current = window.setTimeout(closeNow, CLOSE_DELAY_MS);
  };

  useEffect(() => () => clearTimers(), []);

  /* 键盘：Esc 收起；面板内 ↑↓ 在菜单项间移动、Enter 选中（§5.2）。 */
  useEffect(() => {
    if (!open) return;
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        closeNow();
        triggerRef.current?.focus();
        return;
      }
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        const delta = event.key === "ArrowDown" ? 1 : -1;
        const next = (focusIndex + delta + menuItems.length + 1) % (menuItems.length + 1);
        setFocusIndex(next === menuItems.length ? -1 : next);
        const buttons = wrapRef.current?.querySelectorAll<HTMLAnchorElement>("a[data-menu-item]");
        const target = next === menuItems.length ? triggerRef.current : buttons?.[next];
        target?.focus();
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  });

  if (!user) return null;

  function selectChannel(channel: MessageChannel) {
    closeNow();
    router.push(channel === "all" ? "/messages" : `/messages?channel=${channel}`);
  }

  return (
    <div
      ref={wrapRef}
      className="relative"
      onPointerEnter={scheduleOpen}
      onPointerLeave={scheduleClose}
    >
      <button
        type="button"
        ref={triggerRef}
        onClick={() => (open ? closeNow() : openNow())}
        onKeyDown={(event) => {
          if (!open && (event.key === "Enter" || event.key === "ArrowDown")) {
            event.preventDefault();
            openNow();
          }
        }}
        aria-label={t("nav.notifications")}
        aria-expanded={open}
        aria-haspopup="dialog"
        className="relative inline-flex h-11 w-11 items-center justify-center rounded-md transition-colors duration-150 hover:bg-canvas-subtle active:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-accent-emphasis"
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
          {/* 触屏/外点关闭层（hover 态由 250ms 延迟接管，不冲突）。 */}
          <div className="fixed inset-0 z-40 md:hidden" onClick={closeNow} aria-hidden="true" />
          <div
            role="dialog"
            aria-label={t("nav.notifications")}
            onPointerEnter={clearTimers}
            className="absolute right-0 top-full z-50 mt-1 w-72 rounded-lg border border-border-default bg-canvas-default shadow-md"
          >
            <div className="border-b border-border-default px-4 py-2 text-sm font-medium">
              {t("nav.notifications")}
            </div>
            <div className="py-1.5" role="menu">
              {menuItems.map((item, index) => (
                <a
                  key={item.key}
                  data-menu-item=""
                  href={item.key === "all" ? "/messages" : `/messages?channel=${item.key}`}
                  role="menuitem"
                  tabIndex={focusIndex === index ? 0 : -1}
                  onClick={(event) => {
                    event.preventDefault();
                    selectChannel(item.key);
                  }}
                  className={cn(
                    "flex min-h-11 items-center gap-2.5 px-4 text-sm text-fg-default transition-colors hover:bg-canvas-subtle focus:outline-none focus:bg-canvas-subtle focus:ring-2 focus:ring-inset focus:ring-accent-emphasis",
                    focusIndex === index && "bg-canvas-subtle",
                  )}
                >
                  <span aria-hidden="true">{menuIcon(item.key)}</span>
                  <span>{item.label}</span>
                  {item.badge > 0 && (
                    <span className="ml-auto inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-accent-emphasis px-1.5 text-[10px] font-medium leading-none text-white">
                      {item.badge > 99 ? "99+" : item.badge}
                    </span>
                  )}
                </a>
              ))}
            </div>
            <a
              href="/messages"
              onClick={(event) => {
                event.preventDefault();
                closeNow();
                router.push("/messages");
              }}
              className="block border-t border-border-default px-4 py-2.5 text-center text-xs font-medium text-accent-emphasis transition-colors hover:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-inset focus:ring-accent-emphasis"
            >
              {t("messages.dropdown.viewAll")} →
            </a>
          </div>
        </>
      )}
    </div>
  );
}

function menuIcon(channel: MessageChannel) {
  const def = channel === "dm" ? DM_CHANNEL : MESSAGE_CHANNELS.find((c) => c.key === channel);
  return def?.icon ?? MESSAGE_CHANNELS[0].icon;
}
