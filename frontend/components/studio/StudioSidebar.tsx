"use client";

import { useState, useEffect, useMemo } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  PanelLeftClose, PanelLeft, FilePlus, LayoutDashboard,
  FileText, GitPullRequest, Users, Tags, BarChart3, DollarSign, BookOpen, X,
} from "lucide-react";
import { cn } from "@/lib/utils";
import {
  STUDIO_SIDEBAR_STORAGE_KEY,
  useSidebarCollapse,
} from "@/lib/use-sidebar-collapse";

const itemBase =
  "flex min-h-10 w-full items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium outline-none transition-[color,background-color] duration-150 select-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background";

const itemActive =
  "bg-accent-subtle text-accent-emphasis font-semibold";

const itemIdle =
  "text-fg-muted hover:text-fg-default hover:bg-canvas-subtle";

const collapsedItem =
  "justify-center px-[8px] py-[8px] w-auto";

export function StudioSidebar() {
  const t = useTranslations();
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);
  const { collapsed, toggle } = useSidebarCollapse({
    storageKey: STUDIO_SIDEBAR_STORAGE_KEY,
  });

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
        { icon: BookOpen, label: t('studio.sidebar.series'), href: "/studio/series" },
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
    setMobileOpen(false);
  }, [pathname]);

  useEffect(() => {
    if (!mobileOpen) return;
    function handleEscape(event: KeyboardEvent) {
      if (event.key === "Escape") setMobileOpen(false);
    }
    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [mobileOpen]);

  const toggleLabel = collapsed
    ? t("studio.sidebar.expand")
    : t("studio.sidebar.collapse");

  return (
    <>
      {/* Desktop sidebar */}
      <aside
        className={cn(
          "hidden flex-shrink-0 overflow-visible border-r border-border bg-canvas-subtle py-3 transition-[width] duration-200 motion-reduce:transition-none min-[701px]:flex min-[701px]:flex-col",
          collapsed ? "w-12" : "w-[228px]"
        )}
        aria-label={t("studio.sidebar.analytics")}
      >
        {/* Toggle */}
        <button
          type="button"
          onClick={toggle}
          aria-label={toggleLabel}
          title={toggleLabel}
          className={cn(
            itemBase,
            "text-fg-muted hover:text-fg-default hover:bg-canvas-subtle",
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
            {gi > 0 && collapsed && <div className="mx-3 my-2 h-px bg-border" />}
            {group.label && (
              <div className={cn(
                "px-3 pb-1.5 pt-2 text-[10.5px] font-semibold uppercase tracking-wider text-fg-subtle",
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
                      className={cn(
                        classes,
                        "group relative",
                        isActive && "before:absolute before:bottom-2 before:left-0 before:top-2 before:w-[3px] before:rounded-r before:bg-accent-emphasis",
                      )}
                    >
                      <item.icon className="h-4 w-4 flex-shrink-0" />
                      {!collapsed && <span className="truncate">{item.label}</span>}
                      {collapsed && (
                        <span
                          aria-hidden="true"
                          className="pointer-events-none absolute left-full top-1/2 z-50 ml-2 -translate-y-1/2 whitespace-nowrap rounded-md border border-border bg-canvas-default px-3 py-1.5 text-sm text-foreground opacity-0 shadow-md transition-opacity delay-300 group-hover:opacity-100 group-focus-visible:opacity-100"
                        >
                          {item.label}
                        </span>
                      )}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </aside>

      <button
        type="button"
        aria-label={t("studio.sidebar.expand")}
        aria-expanded={mobileOpen}
        aria-controls="studio-mobile-navigation"
        onClick={() => setMobileOpen(true)}
        className="fixed left-4 top-[60px] z-30 inline-flex size-11 items-center justify-center rounded-md border border-border bg-card text-foreground shadow-sm focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 min-[701px]:hidden"
      >
        <PanelLeft className="size-5" />
      </button>

      {mobileOpen && (
        <div className="fixed inset-0 z-50 min-[701px]:hidden">
          <button
            type="button"
            aria-label={t("studio.sidebar.collapse")}
            className="absolute inset-0 bg-black/50"
            onClick={() => setMobileOpen(false)}
          />
          <aside
            id="studio-mobile-navigation"
            role="dialog"
            aria-modal="true"
            aria-label={t("studio.sidebar.analytics")}
            className="relative flex h-full w-[85vw] max-w-[320px] flex-col overflow-y-auto border-r border-border bg-canvas-subtle p-3 shadow-md"
          >
            <div className="mb-2 flex items-center justify-between px-1">
              <span className="text-sm font-semibold">{t("nav.dashboard")}</span>
              <button
                type="button"
                aria-label={t("studio.sidebar.collapse")}
                title={t("studio.sidebar.collapse")}
                onClick={() => setMobileOpen(false)}
                className="inline-flex size-11 items-center justify-center rounded-md text-fg-muted hover:bg-canvas-default hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
              >
                <X className="size-5" />
              </button>
            </div>
            {groups.map((group) => (
              <div key={group.label} className="mb-2">
                <div className="px-3 pb-1.5 pt-2 text-xs font-semibold uppercase tracking-wider text-fg-subtle">
                  {group.label}
                </div>
                <ul className="space-y-0.5">
                  {group.items.map((item) => {
                    const isActive =
                      pathname === item.href ||
                      (item.href !== "/studio/publish/original" && pathname.startsWith(item.href + "/"));
                    return (
                      <li key={item.href}>
                        <Link
                          href={item.href}
                          onClick={() => setMobileOpen(false)}
                          className={cn(itemBase, isActive ? itemActive : itemIdle)}
                        >
                          <item.icon className="size-4 shrink-0" />
                          <span className="truncate">{item.label}</span>
                        </Link>
                      </li>
                    );
                  })}
                </ul>
              </div>
            ))}
          </aside>
        </div>
      )}
    </>
  );
}
