"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { PanelLeftClose, PanelLeft } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  PUBLIC_SIDEBAR_STORAGE_KEY,
  useSidebarCollapse,
} from "@/lib/use-sidebar-collapse";
import {
  SIDEBAR_ITEM_BASE,
  SIDEBAR_ITEM_TEXT_CLASS,
  SIDEBAR_LIST_NOSCROLL_CLASS,
  SIDEBAR_LIST_SCROLL_CLASS,
  SidebarSectionHeader,
  SidebarTooltip,
} from "./sidebar-shell";

export interface SidebarItem {
  icon: React.ReactNode;
  label: string;
  href?: string;
  count?: string | number;
  active?: boolean;
  colorDot?: string;
  onClick?: () => void;
}

export interface TrendingEntry {
  rank: number;
  avatar?: React.ReactNode;
  name: string;
  stat: string;
  href?: string;
}

interface SidebarSection {
  label?: string;
  items: SidebarItem[];
}

interface SidebarProps {
  sections?: SidebarSection[];
  trending?: {
    title: string;
    entries: TrendingEntry[];
  };
  className?: string;
}

function TrendingSection({ title, entries }: NonNullable<SidebarProps["trending"]>) {
  return (
    <div className="px-3.5">
      <h4 className="pb-2 pt-1 text-[11px] font-semibold uppercase tracking-wider text-fg-subtle">
        {title}
      </h4>
      {entries.map((entry, i) => (
        <Link
          key={i}
          href={entry.href || "#"}
          className="flex items-center gap-2 rounded-md px-2.5 py-1.5 transition-colors hover:bg-muted"
        >
          <span className={cn(
            "w-[18px] flex-shrink-0 text-center text-xs font-bold text-fg-subtle",
            entry.rank === 1 && "text-rose-500",
            entry.rank === 2 && "text-amber-500",
            entry.rank === 3 && "text-violet-500"
          )}>
            {entry.rank}
          </span>
          {entry.avatar && (
            <span className="flex h-6 w-6 flex-shrink-0 items-center justify-center overflow-hidden rounded-md bg-muted text-[10px] text-fg-subtle">
              {entry.avatar}
            </span>
          )}
          <span className="min-w-0 flex-1">
            <span className="block truncate text-[12.5px] font-medium text-foreground">
              {entry.name}
            </span>
            <span className="block text-[10.5px] text-fg-subtle">{entry.stat}</span>
          </span>
        </Link>
      ))}
    </div>
  );
}

const itemActive =
  "bg-accent-subtle text-accent-emphasis font-semibold";

const itemIdle =
  "text-fg-muted hover:text-fg-default hover:bg-canvas-subtle";

const collapsedItem =
  "justify-center px-[8px] py-[8px] w-auto";

export function Sidebar({ sections = [], trending, className }: SidebarProps) {
  const t = useTranslations();
  // 脱离 Next 路由上下文渲染（组件测试环境）时 usePathname 返回 null。
  const pathname = usePathname() ?? "";
  const { collapsed, toggle } = useSidebarCollapse({
    storageKey: PUBLIC_SIDEBAR_STORAGE_KEY,
  });
  const toggleLabel = collapsed
    ? t("studio.sidebar.expand")
    : t("studio.sidebar.collapse");

  return (
    <aside
      className={cn(
        "flex flex-shrink-0 flex-col overflow-visible border-r border-border bg-canvas-default py-2 transition-[width] duration-200 motion-reduce:transition-none",
        collapsed ? "w-12" : "w-[228px]",
        className
      )}
      aria-label={t("nav.siteName")}
    >
      {/* Toggle button：同一元素、位置固定（不随列表滚动），仅切换图标与文案 */}
      <button
        type="button"
        onClick={toggle}
        aria-label={toggleLabel}
        className={cn(
          SIDEBAR_ITEM_BASE,
          "group relative text-fg-muted hover:text-fg-default hover:bg-canvas-subtle",
          collapsed
            ? "mx-auto mb-2 w-9 justify-center px-0"
            : "mx-3.5 mb-2 w-[calc(100%-28px)]"
        )}
      >
        {collapsed ? (
          <>
            <PanelLeft className="h-4 w-4 flex-shrink-0" />
            <SidebarTooltip label={toggleLabel} />
          </>
        ) : (
          <>
            <PanelLeftClose className="h-4 w-4 flex-shrink-0" />
            <span>{t('studio.sidebar.collapse')}</span>
          </>
        )}
      </button>

      {/* 图标列表：收起态不裁切（tooltip 逃逸）；展开态独立滚动（顶部按钮固定） */}
      <div className={collapsed ? SIDEBAR_LIST_NOSCROLL_CLASS : SIDEBAR_LIST_SCROLL_CLASS}>
        {sections.map((section, si) => (
          <div key={si}>
            {section.label && (
              <SidebarSectionHeader label={section.label} collapsed={collapsed} />
            )}
            <ul className={cn("space-y-0.5", collapsed ? "px-0" : "px-3.5")}>
              {section.items.map((item, ii) => {
                const isActive =
                  item.active !== undefined
                    ? item.active
                    : item.href
                      ? pathname === item.href || pathname.startsWith(item.href + "/")
                      : false;

                const classes = cn(
                  SIDEBAR_ITEM_BASE,
                  isActive ? itemActive : itemIdle,
                  collapsed && collapsedItem
                );

                const inner = (
                  <>
                    <span className="flex-shrink-0">{item.icon}</span>
                    {/* 文字/徽标 overlay：不进入 flex 流，translateX+opacity 进出，不推动图标 */}
                    <span
                      className={cn(
                        SIDEBAR_ITEM_TEXT_CLASS,
                        collapsed ? "-translate-x-1 opacity-0" : "translate-x-0 opacity-100"
                      )}
                    >
                      <span className="flex-1 truncate">{item.label}</span>
                      {item.count !== undefined && (
                        <span className={cn(
                          "ml-auto rounded-full px-1.5 py-px text-[11px] font-medium",
                          isActive
                            ? "bg-accent-emphasis/20 text-accent-emphasis"
                            : "bg-muted text-fg-subtle"
                        )}>
                          {item.count}
                        </span>
                      )}
                    </span>
                  </>
                );

                if (item.href) {
                  return (
                    <li key={ii} className="group relative" data-sidebar-anchor="item">
                      <Link
                        href={item.href}
                        className={classes}
                        aria-label={collapsed ? item.label : undefined}
                      >
                        {inner}
                        {collapsed && <SidebarTooltip label={item.label} />}
                      </Link>
                    </li>
                  );
                }

                return (
                  <li key={ii} className="group relative" data-sidebar-anchor="item">
                    <span
                      role="button"
                      tabIndex={0}
                      className={cn(classes, "cursor-pointer")}
                      onClick={item.onClick}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          item.onClick?.();
                        }
                      }}
                      aria-label={collapsed ? item.label : undefined}
                    >
                      {inner}
                      {collapsed && <SidebarTooltip label={item.label} />}
                    </span>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </div>

      {/* 热门区块：收起时条件渲染整段移除（不参与占位计算），展开时整体淡入 */}
      {trending && !collapsed && (
        <>
          <div className="my-2 h-px bg-transparent" />
          <TrendingSection title={trending.title} entries={trending.entries} />
        </>
      )}
    </aside>
  );
}
