"use client";

import { useEffect } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import { Shield, FileText, Users, AlertTriangle, Settings, Tags, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";

const ADMIN_NAV = [
  { href: "/admin/ips", label: "IP 库管理", icon: Shield },
  { href: "/admin/contents", label: "内容终审", icon: FileText },
  { href: "/admin/users", label: "用户管理", icon: Users },
  { href: "/admin/appeal", label: "申诉处理", icon: AlertTriangle },
  { href: "/admin/categories", label: "分类管理", icon: Tags },
  { href: "/admin/config", label: "系统配置", icon: Settings },
];

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const { user, isLoading } = useAuth();
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (!isLoading && (!user || user.role !== "admin")) {
      router.replace("/");
    }
  }, [user, isLoading, router]);

  if (isLoading) {
    return (
      <div className="mx-auto w-full max-w-7xl px-4 py-6 text-sm text-muted-foreground">
        加载中...
      </div>
    );
  }

  if (!user || user.role !== "admin") {
    return (
      <div className="mx-auto flex w-full max-w-lg flex-col items-center justify-center px-4 py-20 text-center">
        <Shield className="h-12 w-12 text-muted-foreground" />
        <h1 className="mt-4 text-xl font-bold tracking-tight">403 访问被拒绝</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          你没有管理员权限，无法访问此页面。
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-7xl gap-0 px-0">
      {/* Sidebar nav - hidden on mobile */}
      <aside className="hidden w-[220px] shrink-0 border-r border-border bg-canvas-subtle md:block">
        <nav className="sticky top-14 flex flex-col gap-0.5 p-3">
          <div className="mb-3 px-3 py-1">
            <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              管理后台
            </p>
          </div>
          {ADMIN_NAV.map((item) => {
            const isActive = pathname.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "flex items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors",
                  isActive
                    ? "bg-accent/10 text-accent font-medium"
                    : "text-foreground/70 hover:bg-muted hover:text-foreground"
                )}
              >
                <item.icon className="h-4 w-4 shrink-0" />
                <span>{item.label}</span>
                {isActive && <ChevronRight className="ml-auto h-3.5 w-3.5 shrink-0" />}
              </Link>
            );
          })}
        </nav>
      </aside>

      {/* Mobile nav tabs */}
      <div className="w-full overflow-x-auto border-b border-border bg-canvas-subtle md:hidden">
        <div className="flex gap-0 px-2 py-2">
          {ADMIN_NAV.map((item) => {
            const isActive = pathname.startsWith(item.href);
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
                {item.label}
              </Link>
            );
          })}
        </div>
      </div>

      {/* Main content */}
      <main className="min-h-[calc(100vh-3.5rem)] w-full flex-1 overflow-auto">
        {children}
      </main>
    </div>
  );
}
