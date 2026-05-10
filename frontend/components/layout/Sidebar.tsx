"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { PanelLeftClose, PanelLeft } from "lucide-react";
import { cn } from "@/lib/utils";

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
    <div className="sidebar-trending px-3.5">
      <h4 className="pb-2 pt-1 text-[11px] font-semibold uppercase tracking-wider text-fg-subtle">
        {title}
      </h4>
      {entries.map((entry, i) => (
        <Link
          key={i}
          href={entry.href || "#"}
          className={cn(
            "flex items-center gap-2 rounded-md px-2.5 py-1.5 transition-colors hover:bg-muted",
            "group/trending"
          )}
        >
          <span
            className={cn(
              "w-[18px] flex-shrink-0 text-center text-xs font-bold text-fg-subtle",
              entry.rank === 1 && "text-rose-500",
              entry.rank === 2 && "text-amber-500",
              entry.rank === 3 && "text-violet-500"
            )}
          >
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

export function Sidebar({ sections = [], trending, className }: SidebarProps) {
  const pathname = usePathname();
  const [collapsed, setCollapsed] = useState(false);

  useEffect(() => {
    const stored = localStorage.getItem("sidebarCollapsed");
    if (stored === "true") setCollapsed(true);
  }, []);

  function toggle() {
    setCollapsed((prev) => {
      const next = !prev;
      localStorage.setItem("sidebarCollapsed", String(next));
      return next;
    });
  }

  return (
    <aside
      className={cn(
        "flex-shrink-0 overflow-y-auto overflow-x-hidden border-r border-transparent bg-background py-2 transition-[width] duration-200",
        collapsed ? "w-[48px]" : "w-[228px]",
        className
      )}
    >
      {/* Toggle button */}
      <button
        type="button"
        onClick={toggle}
        title={collapsed ? "展开侧边栏" : "收起侧边栏"}
        className={cn(
          "flex items-center gap-2 rounded-md px-3 py-1.5 text-[13px] font-medium text-fg-muted transition-colors hover:bg-muted hover:text-foreground",
          collapsed
            ? "mx-auto w-9 justify-center px-0"
            : "mx-3.5 mb-2 w-[calc(100%-28px)]"
        )}
      >
        {collapsed ? (
          <PanelLeft className="h-4 w-4 flex-shrink-0" />
        ) : (
          <>
            <PanelLeftClose className="h-4 w-4 flex-shrink-0" />
            <span className="toggle-text">收起侧边栏</span>
          </>
        )}
      </button>

      {/* Sections */}
      {sections.map((section, si) => (
        <div key={si}>
          {section.label && (
            <div
              className={cn(
                "px-3.5 pb-1.5 pt-2 text-[10.5px] font-semibold uppercase tracking-wider text-fg-subtle",
                collapsed && "hidden"
              )}
            >
              {section.label}
            </div>
          )}
          <ul className={cn("space-y-0.5", collapsed ? "px-0" : "px-3.5")}>
            {section.items.map((item, ii) => {
              const isActive =
                item.active !== undefined
                  ? item.active
                  : item.href
                    ? pathname === item.href || pathname.startsWith(item.href + "/")
                    : false;

              const inner = (
                <>
                  <span className="flex-shrink-0">{item.icon}</span>
                  {!collapsed && (
                    <>
                      <span className="item-text flex-1 truncate text-[13px] font-medium">
                        {item.label}
                      </span>
                      {item.count !== undefined && (
                        <span
                          className={cn(
                            "ml-auto rounded-full px-1.5 py-px text-[11px] font-medium",
                            isActive
                              ? "bg-[#C7D2FE] text-[var(--accent-emphasis)]"
                              : "bg-[var(--border-light)] text-fg-subtle"
                          )}
                        >
                          {item.count}
                        </span>
                      )}
                    </>
                  )}
                </>
              );

              const itemClasses = cn(
                "flex items-center gap-2.5 rounded-[6px] px-3 py-1.5 text-[13px] font-medium transition-all duration-100",
                isActive
                  ? "bg-[var(--accent-subtle)] text-[var(--accent-emphasis)] font-semibold"
                  : "text-fg-muted hover:bg-muted hover:text-foreground",
                collapsed && "relative justify-center px-0 py-2",
                item.onClick && "cursor-pointer"
              );

              if (item.href) {
                return (
                  <li key={ii}>
                    <Link
                      href={item.href}
                      className={itemClasses}
                      data-label={collapsed ? item.label : undefined}
                      title={collapsed ? item.label : undefined}
                    >
                      {inner}
                    </Link>
                  </li>
                );
              }

              return (
                <li key={ii}>
                  <button
                    type="button"
                    className={itemClasses}
                    onClick={item.onClick}
                    data-label={collapsed ? item.label : undefined}
                    title={collapsed ? item.label : undefined}
                  >
                    {inner}
                  </button>
                </li>
              );
            })}
          </ul>
        </div>
      ))}

      {/* Trending section */}
      {trending && !collapsed && (
        <>
          <div className="sidebar-divider my-2 h-px bg-transparent" />
          <TrendingSection title={trending.title} entries={trending.entries} />
        </>
      )}
    </aside>
  );
}
