"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useTheme } from "next-themes";
import {
  Brush,
  Clock,
  LayoutDashboard,
  LogOut,
  Menu,
  Monitor,
  Moon,
  Search,
  Settings,
  Shield,
  Sun,
  User,
} from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

function ThemeIcon({ theme, mounted }: { theme?: string; mounted: boolean }) {
  if (!mounted) {
    return <Monitor className="h-4 w-4" />;
  }
  if (theme === "dark") {
    return <Moon className="h-4 w-4" />;
  }
  if (theme === "light") {
    return <Sun className="h-4 w-4" />;
  }
  return <Monitor className="h-4 w-4" />;
}

export function Header() {
  const { user, logout } = useAuth();
  const { theme, setTheme, resolvedTheme } = useTheme();
  const router = useRouter();
  const [mobileSearchOpen, setMobileSearchOpen] = useState(false);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const activeTheme = theme === "system" ? resolvedTheme : theme;

  function goTo(path: string) {
    router.push(path);
  }

  return (
    <header className="sticky top-0 z-50 w-full border-b border-border bg-background shadow-none">
      <div className="mx-auto flex h-14 max-w-7xl items-center gap-4 px-4">
        <Link
          href="/"
          className="flex items-center gap-2 font-semibold text-foreground transition-opacity hover:opacity-80"
        >
          <Brush className="h-5 w-5 text-primary" />
          <span className="text-base">万象工坊</span>
        </Link>

        <nav className="hidden items-center gap-1 sm:flex">
          <Link
            href="/"
            className="rounded-md px-3 py-1.5 text-sm text-foreground/80 transition-colors hover:bg-muted hover:text-foreground"
          >
            二创区
          </Link>
          <Link
            href="/original"
            className="rounded-md px-3 py-1.5 text-sm text-foreground/80 transition-colors hover:bg-muted hover:text-foreground"
          >
            原创区
          </Link>
        </nav>

        <div className="flex flex-1 items-center gap-2">
          {/* Desktop search */}
          <div className="relative hidden max-w-sm flex-1 items-center sm:flex">
            <Search className="pointer-events-none absolute left-3 h-4 w-4 text-muted-foreground" />
            <input
              type="search"
              placeholder="搜索 IP、内容、创作者..."
              className="w-full rounded-md border border-border bg-muted/40 py-1.5 pl-9 pr-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
          {/* Mobile search toggle */}
          <div className="flex flex-1 justify-end sm:hidden">
            <button
              type="button"
              className="rounded-md p-2 hover:bg-muted"
              onClick={() => setMobileSearchOpen(!mobileSearchOpen)}
            >
              <Search className="h-4 w-4" />
            </button>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <DropdownMenu>
            <DropdownMenuTrigger
              className={cn(buttonVariants({ variant: "ghost", size: "icon" }), "h-8 w-8")}
            >
              <ThemeIcon theme={activeTheme} mounted={mounted} />
              <span className="sr-only">切换主题</span>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => setTheme("light")}>
                <Sun className="mr-2 h-4 w-4" />
                浅色
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => setTheme("dark")}>
                <Moon className="mr-2 h-4 w-4" />
                深色
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => setTheme("system")}>
                <Monitor className="mr-2 h-4 w-4" />
                跟随系统
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <DropdownMenu>
            <DropdownMenuTrigger
              className={cn(buttonVariants({ variant: "ghost", size: "icon" }), "h-8 w-8 sm:hidden")}
            >
              <Menu className="h-4 w-4" />
              <span className="sr-only">打开菜单</span>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-64">
              <div className="p-1.5">
                <div className="relative">
                  <Search className="pointer-events-none absolute left-2.5 top-2 h-4 w-4 text-muted-foreground" />
                  <input
                    type="search"
                    placeholder="搜索 IP、内容、创作者..."
                    className="h-8 w-full rounded-md border border-border bg-background pl-8 pr-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                  />
                </div>
              </div>
              <DropdownMenuItem onClick={() => goTo("/")}>二创区</DropdownMenuItem>
              <DropdownMenuItem onClick={() => goTo("/original")}>原创区</DropdownMenuItem>
              <DropdownMenuSeparator />
              {user ? (
                <>
                  <DropdownMenuItem onClick={() => goTo(`/user/${user.id}`)}>个人主页</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => goTo("/dashboard")}>创作者后台</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => goTo("/history")}>浏览历史</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => goTo("/settings")}>账号设置</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => logout()}>退出登录</DropdownMenuItem>
                </>
              ) : (
                <>
                  <DropdownMenuItem onClick={() => goTo("/login")}>登录</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => goTo("/register")}>注册</DropdownMenuItem>
                </>
              )}
            </DropdownMenuContent>
          </DropdownMenu>

          {user ? (
            <DropdownMenu>
              <DropdownMenuTrigger
                className={cn(buttonVariants({ variant: "ghost", size: "sm" }), "hidden h-8 gap-2 px-2 sm:inline-flex")}
              >
                <div className="flex h-6 w-6 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold">
                  {user.username.slice(0, 1).toUpperCase()}
                </div>
                <span className="max-w-[80px] truncate text-sm">{user.username}</span>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-48">
                <div className="px-2 py-1.5 text-sm">
                  <p className="truncate font-medium">{user.username}</p>
                  <p className="truncate text-xs text-muted-foreground">{user.email}</p>
                </div>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => goTo(`/user/${user.id}`)}>
                  <User className="mr-2 h-4 w-4" />
                  个人主页
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => goTo("/dashboard")}>
                  <LayoutDashboard className="mr-2 h-4 w-4" />
                  创作者后台
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => goTo("/history")}>
                  <Clock className="mr-2 h-4 w-4" />
                  浏览历史
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => goTo("/settings")}>
                  <Settings className="mr-2 h-4 w-4" />
                  账号设置
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => goTo("/appeals")}>
                  <Shield className="mr-2 h-4 w-4" />
                  我的申诉
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="cursor-pointer text-destructive focus:text-destructive"
                  onClick={() => logout()}
                >
                  <LogOut className="mr-2 h-4 w-4" />
                  退出登录
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : (
            <div className="hidden items-center gap-2 sm:flex">
              <Link
                href="/login"
                className={cn(buttonVariants({ variant: "ghost", size: "sm" }), "h-8 text-sm")}
              >
                登录
              </Link>
              <Link
                href="/register"
                className={cn(buttonVariants({ size: "sm" }), "h-8 text-sm")}
              >
                注册
              </Link>
            </div>
          )}
        </div>
      </div>
      {/* Mobile expandable search */}
      {mobileSearchOpen && (
        <div className="border-t border-border bg-background px-4 py-2 sm:hidden">
          <div className="relative">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <input
              type="search"
              autoFocus
              placeholder="搜索 IP、内容、创作者..."
              className="w-full rounded-md border border-border bg-muted/40 py-1.5 pl-9 pr-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
        </div>
      )}
    </header>
  );
}
