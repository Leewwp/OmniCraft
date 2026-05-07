"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useTheme } from "next-themes";
import { useTranslations, useLocale } from "next-intl";
import {
  Brush,
  Clock,
  Globe,
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
import { setLocale } from "@/lib/locale";

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
  const t = useTranslations();
  const locale = useLocale();
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

  async function handleLocaleChange(newLocale: string) {
    await setLocale(newLocale as "zh" | "en", user?.id);
    window.location.reload();
  }

  return (
    <header className="sticky top-0 z-50 w-full border-b border-border bg-background shadow-none">
      <div className="mx-auto flex h-14 max-w-7xl items-center gap-4 px-4">
        <Link
          href="/"
          className="flex items-center gap-2 font-semibold text-foreground transition-opacity hover:opacity-80"
        >
          <Brush className="h-5 w-5 text-primary" />
          <span className="text-base">{t("nav.siteName")}</span>
        </Link>

        <nav className="hidden items-center gap-1 sm:flex">
          <Link
            href="/"
            className="rounded-md px-3 py-1.5 text-sm text-foreground/80 transition-colors hover:bg-muted hover:text-foreground"
          >
            {t("nav.fanworkZone")}
          </Link>
          <Link
            href="/original"
            className="rounded-md px-3 py-1.5 text-sm text-foreground/80 transition-colors hover:bg-muted hover:text-foreground"
          >
            {t("nav.originalZone")}
          </Link>
        </nav>

        <div className="flex flex-1 items-center gap-2">
          {/* Desktop search */}
          <div className="relative hidden max-w-sm flex-1 items-center sm:flex">
            <Search className="pointer-events-none absolute left-3 h-4 w-4 text-muted-foreground" />
            <input
              type="search"
              placeholder={t("common.search")}
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
          {/* Language switcher */}
          <DropdownMenu>
            <DropdownMenuTrigger
              className={cn(buttonVariants({ variant: "ghost", size: "icon" }), "h-8 w-8")}
            >
              <Globe className="h-4 w-4" />
              <span className="sr-only">{t("nav.language")}</span>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                onClick={() => handleLocaleChange("zh")}
                className={locale === "zh" ? "bg-muted" : ""}
              >
                {t("nav.langZh")}
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => handleLocaleChange("en")}
                className={locale === "en" ? "bg-muted" : ""}
              >
                {t("nav.langEn")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          {/* Theme switcher */}
          <DropdownMenu>
            <DropdownMenuTrigger
              className={cn(buttonVariants({ variant: "ghost", size: "icon" }), "h-8 w-8")}
            >
              <ThemeIcon theme={activeTheme} mounted={mounted} />
              <span className="sr-only">{t("nav.themeSwitch")}</span>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => setTheme("light")}>
                <Sun className="mr-2 h-4 w-4" />
                {t("nav.themeLight")}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => setTheme("dark")}>
                <Moon className="mr-2 h-4 w-4" />
                {t("nav.themeDark")}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => setTheme("system")}>
                <Monitor className="mr-2 h-4 w-4" />
                {t("nav.themeSystem")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          {/* Mobile menu */}
          <DropdownMenu>
            <DropdownMenuTrigger
              className={cn(buttonVariants({ variant: "ghost", size: "icon" }), "h-8 w-8 sm:hidden")}
            >
              <Menu className="h-4 w-4" />
              <span className="sr-only">{t("nav.openMenu")}</span>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-64">
              <div className="p-1.5">
                <div className="relative">
                  <Search className="pointer-events-none absolute left-2.5 top-2 h-4 w-4 text-muted-foreground" />
                  <input
                    type="search"
                    placeholder={t("common.search")}
                    className="h-8 w-full rounded-md border border-border bg-background pl-8 pr-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                  />
                </div>
              </div>
              <DropdownMenuItem onClick={() => goTo("/")}>{t("nav.fanworkZone")}</DropdownMenuItem>
              <DropdownMenuItem onClick={() => goTo("/original")}>{t("nav.originalZone")}</DropdownMenuItem>
              <DropdownMenuSeparator />
              {user ? (
                <>
                  <DropdownMenuItem onClick={() => goTo(`/user/${user.id}`)}>{t("nav.profile")}</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => goTo("/dashboard")}>{t("nav.dashboard")}</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => goTo("/history")}>{t("nav.history")}</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => goTo("/settings")}>{t("nav.settings")}</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => goTo("/appeals")}>{t("nav.appeals")}</DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={() => logout()}>{t("nav.logout")}</DropdownMenuItem>
                </>
              ) : (
                <>
                  <DropdownMenuItem onClick={() => goTo("/login")}>{t("nav.login")}</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => goTo("/register")}>{t("nav.register")}</DropdownMenuItem>
                </>
              )}
            </DropdownMenuContent>
          </DropdownMenu>

          {/* Desktop user menu */}
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
                  {t("nav.profile")}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => goTo("/dashboard")}>
                  <LayoutDashboard className="mr-2 h-4 w-4" />
                  {t("nav.dashboard")}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => goTo("/history")}>
                  <Clock className="mr-2 h-4 w-4" />
                  {t("nav.history")}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => goTo("/settings")}>
                  <Settings className="mr-2 h-4 w-4" />
                  {t("nav.settings")}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => goTo("/appeals")}>
                  <Shield className="mr-2 h-4 w-4" />
                  {t("nav.appeals")}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="cursor-pointer text-destructive focus:text-destructive"
                  onClick={() => logout()}
                >
                  <LogOut className="mr-2 h-4 w-4" />
                  {t("nav.logout")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : (
            <div className="hidden items-center gap-2 sm:flex">
              <Link
                href="/login"
                className={cn(buttonVariants({ variant: "ghost", size: "sm" }), "h-8 text-sm")}
              >
                {t("nav.login")}
              </Link>
              <Link
                href="/register"
                className={cn(buttonVariants({ size: "sm" }), "h-8 text-sm")}
              >
                {t("nav.register")}
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
              placeholder={t("common.search")}
              className="w-full rounded-md border border-border bg-muted/40 py-1.5 pl-9 pr-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
        </div>
      )}
    </header>
  );
}
