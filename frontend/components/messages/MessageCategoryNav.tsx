"use client";

import { useTranslations } from "next-intl";
import {
  Bell,
  GitPullRequest,
  Heart,
  Inbox,
  Info,
  Mail,
  Megaphone,
  MessageCircle,
  UserPlus,
} from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import type { UnreadCounts } from "@/contexts/AuthContext";

/* 消息中心分类导航（SP-18 #509，B 站式左栏）：通知七分类 + 分隔线 + 私信；
   每项 = 频道图标 + 文案 + 未读 pill 徽标；选中态 bg-canvas-subtle 圆角块 +
   accent 文案/图标着色（与背景明确区分，解决「融入背景」痛点）。
   桌面 = 垂直侧栏（216px 列由页面网格承载），移动 = 顶部水平滚动 chips。 */

export type MessageChannel = "all" | "reply" | "like" | "follow" | "pr" | "system" | "broadcast" | "dm";

interface ChannelDef {
  key: MessageChannel;
  labelKey: string;
  icon: ReactNode;
}

/** 分类定义与图标（顶栏下拉与本栏共用；badge 键与 unread_counts 对齐）。 */
export const MESSAGE_CHANNELS: ChannelDef[] = [
  { key: "all", labelKey: "notification.all", icon: <Inbox className="h-4 w-4" /> },
  { key: "reply", labelKey: "notification.reply", icon: <MessageCircle className="h-4 w-4" /> },
  { key: "like", labelKey: "notification.like", icon: <Heart className="h-4 w-4" /> },
  { key: "follow", labelKey: "notification.follow", icon: <UserPlus className="h-4 w-4" /> },
  { key: "pr", labelKey: "notification.pr", icon: <GitPullRequest className="h-4 w-4" /> },
  { key: "system", labelKey: "notification.system", icon: <Info className="h-4 w-4" /> },
  { key: "broadcast", labelKey: "notification.channelBroadcast", icon: <Megaphone className="h-4 w-4" /> },
];

export const DM_CHANNEL: ChannelDef = { key: "dm", labelKey: "messages.tabs.conversations", icon: <Mail className="h-4 w-4" /> };

export function channelBadgeKey(channel: MessageChannel): string | null {
  return channel === "all" ? "total" : channel === "dm" ? null : channel;
}

interface CategoryItemProps {
  def: ChannelDef;
  active: boolean;
  badge: number;
  onSelect: (channel: MessageChannel) => void;
  /** 移动 chips 形态（水平滚动、pill 收紧）。 */
  compact?: boolean;
}

export function MessageCategoryItem({ def, active, badge, onSelect, compact }: CategoryItemProps) {
  const t = useTranslations();
  return (
    <button
      type="button"
      onClick={() => onSelect(def.key)}
      aria-current={active ? "page" : undefined}
      className={cn(
        "flex shrink-0 items-center gap-2 rounded-lg text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        compact ? "min-h-9 px-3 py-1.5" : "px-3 py-2",
        active
          ? "bg-canvas-subtle font-medium text-accent-emphasis [&>svg]:text-accent-emphasis"
          : "text-fg-muted hover:bg-canvas-subtle hover:text-fg-default [&>svg]:text-fg-muted",
      )}
    >
      {def.icon}
      <span className="whitespace-nowrap">{t(def.labelKey)}</span>
      {badge > 0 && (
        <span
          aria-label={t("messages.conversations.unreadCount", { count: badge })}
          className="ml-auto inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-accent-emphasis px-1.5 text-[10px] font-medium leading-none text-white"
        >
          {badge > 99 ? "99+" : badge}
        </span>
      )}
    </button>
  );
}

interface MessageCategoryNavProps {
  active: MessageChannel;
  /** 通知各频道未读（unread_counts，含 total）。 */
  unreadCounts: UnreadCounts;
  /** 私信会话未读聚合。 */
  dmUnread: number;
  onSelect: (channel: MessageChannel) => void;
  /** 垂直侧栏（桌面默认）或水平滚动 chips（移动，§3.1 响应式）。 */
  orientation?: "vertical" | "horizontal";
}

export function MessageCategoryNav({ active, unreadCounts, dmUnread, onSelect, orientation = "vertical" }: MessageCategoryNavProps) {
  const t = useTranslations();
  const badgeOf = (def: ChannelDef) => {
    if (def.key === "dm") return dmUnread;
    const key = def.key === "all" ? "total" : def.key;
    return unreadCounts[key as keyof UnreadCounts] ?? 0;
  };

  return (
    <nav
      aria-label={t("messages.a11y.tabs")}
      className={
        orientation === "horizontal"
          ? "flex flex-row gap-1 overflow-x-auto pb-1"
          : "flex flex-col gap-1"
      }
    >
      {MESSAGE_CHANNELS.map((def) => (
        <MessageCategoryItem
          key={def.key}
          def={def}
          active={active === def.key}
          badge={badgeOf(def)}
          onSelect={onSelect}
          compact={orientation === "horizontal"}
        />
      ))}
      {orientation === "vertical" && <div aria-hidden="true" className="my-2 border-t border-border-default" />}
      <MessageCategoryItem
        def={DM_CHANNEL}
        active={active === "dm"}
        badge={dmUnread}
        onSelect={onSelect}
        compact={orientation === "horizontal"}
      />
    </nav>
  );
}
