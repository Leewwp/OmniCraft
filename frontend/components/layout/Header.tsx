"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import { Sun, Moon, Monitor, Search, User, LogOut, LayoutDashboard, Brush, Clock, Settings, Shield } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

export function Header() {
  const { user, logout } = useAuth();
  const { theme, setTheme } = useTheme();
  const router = useRouter();

  return (
    <header className="sticky top-0 z-50 w-full border-b border-border bg-background shadow-none">
      <div className="mx-auto flex h-14 max-w-7xl items-center gap-4 px-4">
        {/* Logo */}
        <Link
          href="/"
          className="flex items-center gap-2 font-semibold text-foreground hover:opacity-80 transition-opacity"
        >
          <Brush className="h-5 w-5 text-primary" />
          <span className="text-base">万象工坊</span>
        </Link>

        {/* Nav */}
        <nav className="hidden sm:flex items-center gap-1">
          <Link
            href="/"
            className="rounded-md px-3 py-1.5 text-sm text-foreground/80 hover:bg-muted hover:text-foreground transition-colors"
          >
            二创区
          </Link>
          <Link
            href="/original"
            className="rounded-md px-3 py-1.5 text-sm text-foreground/80 hover:bg-muted hover:text-foreground transition-colors"
          >
            原创区
          </Link>
        </nav>

        {/* Search */}
        <div className="flex flex-1 items-center gap-2">
          <div className="relative hidden sm:flex flex-1 max-w-sm items-center">
            <Search className="absolute left-3 h-4 w-4 text-muted-foreground pointer-events-none" />
            <input
              type="search"
              placeholder="搜索 IP、内容、创作者..."
              className="w-full rounded-md border border-border bg-muted/40 py-1.5 pl-9 pr-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
        </div>

        {/* Right side */}
        <div className="flex items-center gap-2">
          {/* Theme toggle */}
          <DropdownMenu>
            <DropdownMenuTrigger
              className={cn(buttonVariants({ variant: "ghost", size: "icon" }), "h-8 w-8")}
            >
              {theme === "dark" ? (
                <Moon className="h-4 w-4" />
              ) : theme === "light" ? (
                <Sun className="h-4 w-4" />
              ) : (
                <Monitor className="h-4 w-4" />
              )}
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

          {/* Auth */}
          {user ? (
            <DropdownMenu>
              <DropdownMenuTrigger
                className={cn(buttonVariants({ variant: "ghost", size: "sm" }), "gap-2 h-8 px-2")}
              >
                <div className="h-6 w-6 rounded-full bg-primary/10 flex items-center justify-center text-xs font-semibold">
                  {user.username.slice(0, 1).toUpperCase()}
                </div>
                <span className="hidden sm:inline text-sm max-w-[80px] truncate">
                  {user.username}
                </span>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-48">
                <div className="px-2 py-1.5 text-sm">
                  <p className="font-medium truncate">{user.username}</p>
                  <p className="text-xs text-muted-foreground truncate">{user.email}</p>
                </div>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => router.push(`/user/${user.id}`)}>
                  <User className="mr-2 h-4 w-4" />
                  个人主页
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => router.push("/dashboard")}>
                  <LayoutDashboard className="mr-2 h-4 w-4" />
                  创作者后台
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => router.push("/history")}>
                  <Clock className="mr-2 h-4 w-4" />
                  浏览历史
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => router.push("/settings")}>
                  <Settings className="mr-2 h-4 w-4" />
                  账号设置
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => router.push("/appeals")}>
                  <Shield className="mr-2 h-4 w-4" />
                  我的申诉
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="text-destructive focus:text-destructive cursor-pointer"
                  onClick={() => logout()}
                >
                  <LogOut className="mr-2 h-4 w-4" />
                  退出登录
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : (
            <div className="flex items-center gap-2">
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
    </header>
  );
}
