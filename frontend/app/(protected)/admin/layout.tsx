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
          "hidden shrink-0 border-r border-border bg-canvas-subtle transition-[width] duration-200 motion-reduce:transition-none min-[701px]:block",
          collapsed ? "w-12" : "w-[228px]"
        )}
        aria-label={t("admin.title")}
      >
        <nav className="sticky top-0 flex flex-col gap-0.5 p-3">
          {!collapsed && (
            <div className="mb-3 flex items-center justify-between px-1">
              <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {t('admin.title')}
              </p>
              <button
                type="button"
                aria-label={toggleLabel}
                title={toggleLabel}
                className="inline-flex size-8 items-center justify-center rounded-md text-muted-foreground hover:bg-canvas-default hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
                onClick={() => setCollapsed(true)}
              >
                <PanelLeftClose className="size-4" />
              </button>
            </div>
          )}
          {collapsed && (
            <button
              type="button"
              aria-label={toggleLabel}
              title={toggleLabel}
              className="mb-3 inline-flex size-9 items-center justify-center self-center rounded-md text-muted-foreground hover:bg-canvas-default hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
              onClick={() => setCollapsed(false)}
            >
              <PanelLeft className="size-4" />
            </button>
          )}
          {ADMIN_NAV.map((item) => {
            const isActive = pathname.startsWith(item.href);
            const label = t(`admin.${item.labelKey}`);
            return (
              <Link
                key={item.href}
                href={item.href}
                title={collapsed ? label : undefined}
                className={cn(
                  "flex min-h-10 items-center gap-2.5 rounded-md px-3 py-2 text-sm outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
                  isActive
                    ? "bg-accent-subtle text-accent-emphasis font-medium"
                    : "text-muted-foreground hover:bg-canvas-default hover:text-foreground",
                  collapsed && "justify-center px-2"
                )}
              >
                <item.icon className="h-4 w-4 shrink-0" />
                {!collapsed && <span>{label}</span>}
                {!collapsed && isActive && <ChevronRight className="ml-auto h-3.5 w-3.5 shrink-0" />}
              </Link>
            );
          })}
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
