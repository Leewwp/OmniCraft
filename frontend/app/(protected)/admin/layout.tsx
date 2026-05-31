"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import { Shield, FileText, Users, AlertTriangle, Settings, Tags, Bot, MessageSquare, ListOrdered, ChevronRight, PanelLeftClose, PanelLeft } from "lucide-react";
import { Footer } from "@/components/layout/Footer";
import { cn } from "@/lib/utils";

const ADMIN_NAV = [
  { href: "/admin/ips", labelKey: "navIps", icon: Shield },
  { href: "/admin/contents", labelKey: "navContents", icon: FileText },
  { href: "/admin/users", labelKey: "navUsers", icon: Users },
  { href: "/admin/appeal", labelKey: "navAppeals", icon: AlertTriangle },
  { href: "/admin/feedback", labelKey: "navFeedback", icon: MessageSquare },
  { href: "/admin/categories", labelKey: "navCategories", icon: Tags },
  { href: "/admin/queue", labelKey: "navQueue", icon: ListOrdered },
  { href: "/admin/config", labelKey: "navConfig", icon: Settings },
  { href: "/admin/agent-config", labelKey: "navAgentConfig", icon: Bot },
];

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const t = useTranslations();
  const { user, isLoading } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const [sidebarOpen, setSidebarOpen] = useState(true);

  useEffect(() => {
    if (!isLoading && user && user.role !== "admin") {
      router.replace("/");
    }
  }, [user, isLoading, router]);

  if (isLoading) {
    return (
      <div className="mx-auto w-full max-w-7xl px-4 py-6 text-sm text-muted-foreground">
        {t('common.loading')}
      </div>
    );
  }

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
    <div className="mx-auto flex w-full max-w-7xl gap-0 px-0">
      <aside
        className={cn(
          "hidden shrink-0 border-r border-transparent bg-background transition-all duration-200 md:block",
          sidebarOpen ? "w-[220px]" : "w-[52px]"
        )}
      >
        <nav className="sticky top-14 flex flex-col gap-0.5 p-3">
          {sidebarOpen && (
            <div className="mb-3 flex items-center justify-between px-1">
              <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {t('admin.title')}
              </p>
              <button
                type="button"
                className="rounded p-0.5 hover:bg-muted"
                onClick={() => setSidebarOpen(false)}
              >
                <PanelLeftClose className="h-3.5 w-3.5 text-muted-foreground" />
              </button>
            </div>
          )}
          {!sidebarOpen && (
            <button
              type="button"
              className="mb-3 self-end rounded p-1 hover:bg-muted"
              onClick={() => setSidebarOpen(true)}
            >
              <PanelLeft className="h-4 w-4 text-muted-foreground" />
            </button>
          )}
          {ADMIN_NAV.map((item) => {
            const isActive = pathname.startsWith(item.href);
            const label = t(`admin.${item.labelKey}`);
            return (
              <Link
                key={item.href}
                href={item.href}
                title={!sidebarOpen ? label : undefined}
                className={cn(
                  "flex items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors",
                  isActive
                    ? "bg-accent-subtle text-accent-emphasis font-medium"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground",
                  !sidebarOpen && "justify-center px-2"
                )}
              >
                <item.icon className="h-4 w-4 shrink-0" />
                {sidebarOpen && <span>{label}</span>}
                {sidebarOpen && isActive && <ChevronRight className="ml-auto h-3.5 w-3.5 shrink-0" />}
              </Link>
            );
          })}
        </nav>
      </aside>

      <div className="w-full overflow-x-auto border-b border-border bg-background md:hidden">
        <div className="flex gap-0 px-2 py-2">
          {ADMIN_NAV.map((item) => {
            const isActive = pathname.startsWith(item.href);
            const label = t(`admin.${item.labelKey}`);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "shrink-0 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                  isActive
                    ? "bg-accent/10 text-accent"
                    : "text-foreground/60 hover:bg-muted hover:text-foreground"
                )}
              >
                {label}
              </Link>
            );
          })}
        </div>
      </div>

      <div className="flex flex-1 flex-col">
        <main className="flex-1 w-full overflow-auto">
          {children}
        </main>
        <Footer />
      </div>
    </div>
  );
}
