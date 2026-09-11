"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import { Shield, FileText, Users, AlertTriangle, Settings, Tags, Bot, MessageSquare, ListOrdered, LayoutDashboard, Flag, ScrollText, Megaphone, ChevronRight, PanelLeftClose, PanelLeft, X } from "lucide-react";
import { Footer } from "@/components/layout/Footer";
import { cn } from "@/lib/utils";
import {
  ADMIN_SIDEBAR_STORAGE_KEY,
  useSidebarCollapse,
} from "@/lib/use-sidebar-collapse";
import {
  SIDEBAR_ITEM_BASE,
  SIDEBAR_ITEM_TEXT_CLASS,
  SIDEBAR_LIST_NOSCROLL_CLASS,
  SIDEBAR_LIST_SCROLL_CLASS,
  SidebarSectionHeader,
  SidebarTooltip,
} from "@/components/layout/sidebar-shell";

const ADMIN_NAV = [
  { href: "/admin/dashboard", labelKey: "navDashboard", icon: LayoutDashboard },
  { href: "/admin/reports", labelKey: "navReports", icon: Flag },
  { href: "/admin/ips", labelKey: "navIps", icon: Shield },
  { href: "/admin/contents", labelKey: "navContents", icon: FileText },
  { href: "/admin/users", labelKey: "navUsers", icon: Users },
  { href: "/admin/appeal", labelKey: "navAppeals", icon: AlertTriangle },
  { href: "/admin/feedback", labelKey: "navFeedback", icon: MessageSquare },
  { href: "/admin/categories", labelKey: "navCategories", icon: Tags },
  { href: "/admin/queue", labelKey: "navQueue", icon: ListOrdered },
  { href: "/admin/audit-logs", labelKey: "navAuditLogs", icon: ScrollText },
  { href: "/admin/config", labelKey: "navConfig", icon: Settings },
  { href: "/admin/agent-config", labelKey: "navAgentConfig", icon: Bot },
  { href: "/admin/notifications", labelKey: "navNotifications", icon: Megaphone },
];

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const t = useTranslations();
  const { user, isLoading } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);
  const { collapsed, setCollapsed } = useSidebarCollapse({
    storageKey: ADMIN_SIDEBAR_STORAGE_KEY,
  });

  useEffect(() => {
    if (!isLoading && user && user.role !== "admin") {
      router.replace("/");
    }
  }, [user, isLoading, router]);

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

  if (isLoading) {
    return (
      <div className="mx-auto w-full max-w-7xl px-4 py-6 text-sm text-muted-foreground">
        {t('common.loading')}
      </div>
    );
  }

  const toggleLabel = collapsed
    ? t("studio.sidebar.expand")
    : t("studio.sidebar.collapse");

  if (!user || user.role !== "admin") {
    return (
      <div className="mx-auto flex w-full max-w-lg flex-col items-center justify-center px-4 py-20 text-center">
        <Shield className="h-12 w-12 text-muted-foreground" />
        <h1 className="mt-4 text-xl font-bold tracking-tight">{t('admin.accessDenied')}</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          {t('admin.accessDeniedMsg')}
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto flex min-h-screen w-full max-w-7xl flex-col gap-0 px-0 min-[701px]:flex-row">
      <aside
        className={cn(
          "hidden shrink-0 flex-col overflow-visible border-r border-border bg-canvas-subtle py-2 transition-[width] duration-200 motion-reduce:transition-none min-[701px]:flex",
          collapsed ? "w-12" : "w-[228px]"
        )}
        aria-label={t("admin.title")}
      >
        {/* Toggle：同一元素、位置固定（不随列表滚动），仅切换图标与文案 */}
        <button
          type="button"
          onClick={() => setCollapsed(!collapsed)}
          aria-label={toggleLabel}
          className={cn(
            SIDEBAR_ITEM_BASE,
            "group relative text-muted-foreground hover:bg-canvas-default hover:text-foreground",
            collapsed
              ? "mx-auto mb-2 w-9 justify-center px-0"
              : "mx-3 mb-2 w-[calc(100%-24px)]"
          )}
        >
          {collapsed ? (
            <>
              <PanelLeft className="h-4 w-4 shrink-0" />
              <SidebarTooltip label={toggleLabel} />
            </>
          ) : (
            <>
              <PanelLeftClose className="h-4 w-4 shrink-0" />
              <span className="truncate">{t('studio.sidebar.collapse')}</span>
            </>
          )}
        </button>

        {/* 列表：收起态不裁切（tooltip 逃逸）；展开态独立滚动（顶部按钮固定） */}
        <nav
          className={collapsed ? SIDEBAR_LIST_NOSCROLL_CLASS : SIDEBAR_LIST_SCROLL_CLASS}
          aria-label={t("admin.title")}
        >
          <SidebarSectionHeader label={t("admin.title")} collapsed={collapsed} />
          <ul className={cn("gap-0.5", collapsed ? "px-0" : "px-3")}>
            {ADMIN_NAV.map((item) => {
              const isActive = pathname.startsWith(item.href);
              const label = t(`admin.${item.labelKey}`);
              return (
                <li key={item.href} className="group relative" data-sidebar-anchor="item">
                  <Link
                    href={item.href}
                    aria-label={collapsed ? label : undefined}
                    className={cn(
                      SIDEBAR_ITEM_BASE,
                      isActive
                        ? "bg-accent-subtle text-accent-emphasis font-medium"
                        : "text-muted-foreground hover:bg-canvas-default hover:text-foreground",
                      collapsed && "justify-center px-2"
                    )}
                  >
                    <item.icon className="h-4 w-4 shrink-0" />
                    <span
                      className={cn(
                        SIDEBAR_ITEM_TEXT_CLASS,
                        collapsed ? "-translate-x-1 opacity-0" : "translate-x-0 opacity-100"
                      )}
                    >
                      <span className="flex-1 truncate">{label}</span>
                      {isActive && <ChevronRight className="ml-auto h-3.5 w-3.5 shrink-0" />}
                    </span>
                    {collapsed && <SidebarTooltip label={label} />}
                  </Link>
                </li>
              );
            })}
          </ul>
        </nav>
      </aside>

      <div className="sticky top-0 z-30 flex h-12 w-full items-center gap-3 border-b border-border bg-canvas-default px-3 min-[701px]:hidden">
        <button
          type="button"
          aria-label={t("nav.openMenu")}
          aria-expanded={mobileOpen}
          aria-controls="admin-mobile-navigation"
          onClick={() => setMobileOpen(true)}
          className="inline-flex size-10 items-center justify-center rounded-md text-foreground hover:bg-canvas-subtle focus-visible:ring-2 focus-visible:ring-ring"
        >
          <PanelLeft className="size-5" />
        </button>
        <span className="truncate text-sm font-semibold">{t("admin.title")}</span>
      </div>

      {mobileOpen && (
        <div className="fixed inset-0 z-50 min-[701px]:hidden">
          <button
            type="button"
            aria-label={t("studio.sidebar.collapse")}
            className="absolute inset-0 bg-black/50"
            onClick={() => setMobileOpen(false)}
          />
          <aside
            id="admin-mobile-navigation"
            role="dialog"
            aria-modal="true"
            aria-label={t("admin.title")}
            className="relative flex h-full w-[85vw] max-w-[320px] flex-col overflow-y-auto border-r border-border bg-canvas-subtle p-3 shadow-md"
          >
            <div className="mb-2 flex items-center justify-between px-1">
              <span className="text-sm font-semibold">{t("admin.title")}</span>
              <button
                type="button"
                aria-label={t("studio.sidebar.collapse")}
                title={t("studio.sidebar.collapse")}
                onClick={() => setMobileOpen(false)}
                className="inline-flex size-11 items-center justify-center rounded-md text-muted-foreground hover:bg-canvas-default hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
              >
                <X className="size-5" />
              </button>
            </div>
            <nav className="flex flex-col gap-0.5">
              {ADMIN_NAV.map((item) => {
                const isActive = pathname.startsWith(item.href);
                const label = t(`admin.${item.labelKey}`);
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    onClick={() => setMobileOpen(false)}
                    className={cn(
                      "flex min-h-11 items-center gap-3 rounded-md px-3 py-2 text-sm outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-ring",
                      isActive
                        ? "bg-accent-subtle font-medium text-accent-emphasis"
                        : "text-muted-foreground hover:bg-canvas-default hover:text-foreground",
                    )}
                  >
                    <item.icon className="size-4 shrink-0" />
                    <span>{label}</span>
                  </Link>
                );
              })}
            </nav>
          </aside>
        </div>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        <main className="flex-1 w-full overflow-auto">
          {children}
        </main>
        <Footer />
      </div>
    </div>
  );
}
