"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  PanelLeftClose, PanelLeft, FilePlus, LayoutDashboard,
  FileText, GitPullRequest, Users, Tags, BarChart3, DollarSign,
} from "lucide-react";
import { cn } from "@/lib/utils";

const groups = [
  {
    label: "内容发布",
    items: [
      { icon: FilePlus, label: "发布创作", href: "/studio/publish/original" },
    ],
  },
  {
    label: "数据看板",
    items: [
      { icon: LayoutDashboard, label: "数据概览", href: "/studio/overview" },
      { icon: BarChart3, label: "粉丝分析", href: "/studio/followers" },
      { icon: FileText, label: "我的内容", href: "/studio/contents" },
    ],
  },
  {
    label: "协作管理",
    items: [
      { icon: GitPullRequest, label: "PR 管理", href: "/studio/pr-requests" },
      { icon: Users, label: "贡献者", href: "/studio/contributors" },
      { icon: Tags, label: "标签建议", href: "/studio/tag-suggestions" },
      { icon: DollarSign, label: "收益数据", href: "/studio/revenue" },
    ],
  },
];

const itemBase =
  "flex items-center gap-2.5 rounded-[6px] px-3 py-2 text-[13px] font-medium transition-all duration-100 w-full";

const itemActive =
  "bg-[#EEF2FF] text-[#4F46E5] font-semibold";

const itemIdle =
  "text-[#52525B] hover:text-[#18181B] hover:bg-[#F2F2F2]";

const collapsedItem =
  "justify-center px-[8px] py-[8px] w-auto";

export function StudioSidebar() {
  const pathname = usePathname();
  const [collapsed, setCollapsed] = useState(false);

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
        title={collapsed ? "展开侧边栏" : "收起侧边栏"}
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
            <span>收起侧边栏</span>
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
