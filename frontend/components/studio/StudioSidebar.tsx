"use client";

import { useState, useEffect, useMemo } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  PanelLeftClose, PanelLeft, FilePlus, LayoutDashboard,
  FileText, GitPullRequest, Users, Tags, BarChart3, DollarSign,
} from "lucide-react";
import { cn } from "@/lib/utils";

const itemBase =
  "flex items-center gap-2.5 rounded-[6px] px-3 py-2 text-[13px] font-medium transition-all duration-100 w-full select-none active:scale-[0.97]";

const itemActive =
  "bg-[#EEF2FF] text-[#4F46E5] font-semibold";

const itemIdle =
  "text-[#52525B] hover:text-[#18181B] hover:bg-[#F2F2F2]";

const collapsedItem =
  "justify-center px-[8px] py-[8px] w-auto";

export function StudioSidebar() {
  const t = useTranslations();
  const pathname = usePathname();
  const [collapsed, setCollapsed] = useState(false);

  const groups = useMemo(() => [
    {
      label: t('studio.sidebar.publish'),
      items: [
        { icon: FilePlus, label: t('studio.sidebar.publishContent'), href: "/studio/publish/original" },
      ],
    },
    {
      label: t('studio.sidebar.analytics'),
      items: [
        { icon: LayoutDashboard, label: t('studio.sidebar.overview'), href: "/studio/overview" },
        { icon: BarChart3, label: t('studio.sidebar.followers'), href: "/studio/followers" },
        { icon: FileText, label: t('studio.sidebar.myContent'), href: "/studio/contents" },
      ],
    },
    {
      label: t('studio.sidebar.collaboration'),
      items: [
        { icon: GitPullRequest, label: t('studio.sidebar.prManagement'), href: "/studio/pr-requests" },
        { icon: Users, label: t('studio.sidebar.contributors'), href: "/studio/contributors" },
        { icon: Tags, label: t('studio.sidebar.tagSuggestions'), href: "/studio/tag-suggestions" },
        { icon: DollarSign, label: t('studio.sidebar.revenue'), href: "/studio/revenue" },
      ],
    },
  ], [t]);

  useEffect(() => {
    const stored = localStorage.getItem("studio_sidebar_collapsed");
    if (stored === "true") setCollapsed(true);
  }, []);

  function toggle() {
    setCollapsed((prev) => {
      const next = !prev;
      localStorage.setItem("studio_sidebar_collapsed", String(next));
      return next;
    });
  }

  return (
    <aside
      className={cn(
        "flex-shrink-0 overflow-y-auto overflow-x-hidden bg-white py-3 transition-[width] duration-200",
        collapsed ? "w-[52px]" : "w-56"
      )}
    >
      {/* Toggle */}
      <button
        type="button"
        onClick={toggle}
        title={collapsed ? t('studio.sidebar.expand') : t('studio.sidebar.collapse')}
        className={cn(
          itemBase,
          "text-[#52525B] hover:text-[#18181B] hover:bg-[#F2F2F2]",
          collapsed
            ? "mx-auto w-9 justify-center px-0"
            : "mx-3 mb-3 w-[calc(100%-24px)]"
        )}
      >
        {collapsed ? (
          <PanelLeft className="h-4 w-4 flex-shrink-0" />
        ) : (
          <>
            <PanelLeftClose className="h-4 w-4 flex-shrink-0" />
            <span>{t('studio.sidebar.collapse')}</span>
          </>
        )}
      </button>

      {groups.map((group, gi) => (
        <div key={gi} className="mb-1">
          {group.label && (
            <div className={cn(
              "px-3 pb-1.5 pt-2 text-[10.5px] font-semibold uppercase tracking-wider text-[#A1A1AA]",
              collapsed && "hidden"
            )}>
              {group.label}
            </div>
          )}
          <ul className={cn("space-y-0.5", collapsed ? "px-0" : "px-3")}>
            {group.items.map((item, ii) => {
              const isActive =
                pathname === item.href ||
                (item.href !== "/studio/publish/original" &&
                  pathname.startsWith(item.href + "/"));

              const classes = cn(
                itemBase,
                isActive ? itemActive : itemIdle,
                collapsed && collapsedItem
              );

              return (
                <li key={ii}>
                  <Link
                    href={item.href}
                    data-label={collapsed ? item.label : undefined}
                    title={collapsed ? item.label : undefined}
                    className={classes}
                  >
                    <item.icon className="h-4 w-4 flex-shrink-0" />
                    {!collapsed && <span className="truncate">{item.label}</span>}
                  </Link>
                </li>
              );
            })}
          </ul>
        </div>
      ))}
    </aside>
  );
}
