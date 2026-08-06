"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useTheme } from "next-themes";
import { useTranslations, useLocale } from "next-intl";
import {
  Bell,
  Brush,
  Clock,
  Globe,
  LayoutDashboard,
  LogOut,
  Menu,
  Monitor,
  Moon,
  Plus,
  Search,
  Settings,
  Shield,
  Sun,
  User,
  X,
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
import { NotificationDropdown } from "@/components/social/NotificationDropdown";

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
  const pathname = usePathname();
  const [mobileSearchOpen, setMobileSearchOpen] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!mobileMenuOpen) return;
    function handleEscape(event: KeyboardEvent) {
      if (event.key === "Escape") setMobileMenuOpen(false);
    }
    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [mobileMenuOpen]);

  const activeTheme = theme === "system" ? resolvedTheme : theme;

  function goTo(path: string) {
    router.push(path);
    setMobileMenuOpen(false);
  }

  function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = searchQuery.trim();
    if (!trimmed) return;
    router.push(`/search?q=${encodeURIComponent(trimmed)}`);
    setMobileSearchOpen(false);
    setMobileMenuOpen(false);
  }

  async function handleLocaleChange(newLocale: string) {
    await setLocale(newLocale as "zh" | "en", user?.id);
    window.location.reload();
  }

  return (
    <>
      <header className="sticky top-0 z-40 h-[var(--header-h)] w-full border-b border-border-default bg-canvas-default shadow-none">
        <div className="mx-auto flex h-full max-w-7xl items-center gap-3 px-4">
        <Link
          href="/"
          className="flex items-center gap-2 font-semibold text-foreground transition-opacity hover:opacity-80"
        >
          <Brush className="h-5 w-5 text-primary" />
          <span className="text-base">{t("nav.siteName")}</span>
        </Link>

        <nav className="hidden h-full items-center gap-1 min-[701px]:flex">
          <Link
            href="/recommend"
            aria-current={pathname.startsWith("/recommend") ? "page" : undefined}
            className={cn(
              "relative flex h-full items-center px-3 text-sm font-medium text-fg-muted transition-colors duration-150 after:absolute after:inset-x-3 after:bottom-0 after:h-0.5 after:bg-primary after:opacity-0 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset",
              pathname.startsWith("/recommend") && "font-semibold text-foreground after:opacity-100",
            )}
          >
            {t("nav.recommend")}
          </Link>
          <Link
            href="/"
            aria-current={pathname === "/" ? "page" : undefined}
            className={cn(
              "relative flex h-full items-center px-3 text-sm font-medium text-fg-muted transition-colors duration-150 after:absolute after:inset-x-3 after:bottom-0 after:h-0.5 after:bg-primary after:opacity-0 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset",
              pathname === "/" && "font-semibold text-foreground after:opacity-100",
            )}
          >
            {t("nav.fanworkZone")}
          </Link>
          <Link
            href="/original"
            aria-current={pathname.startsWith("/original") ? "page" : undefined}
            className={cn(
              "relative flex h-full items-center px-3 text-sm font-medium text-fg-muted transition-colors duration-150 after:absolute after:inset-x-3 after:bottom-0 after:h-0.5 after:bg-primary after:opacity-0 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset",
              pathname.startsWith("/original") && "font-semibold text-foreground after:opacity-100",
            )}
          >
            {t("nav.originalZone")}
          </Link>
        </nav>

        <div className="flex flex-1 items-center gap-2">
          {/* Desktop search */}
          <form onSubmit={handleSearch} className="relative hidden max-w-[480px] flex-1 items-center min-[701px]:flex">
            <Search className="pointer-events-none absolute left-3 h-4 w-4 text-muted-foreground" />
            <input
              type="search"
              aria-label={t("common.search")}
              placeholder={t("common.search")}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="h-8 w-full rounded-md border border-transparent bg-canvas-subtle pl-9 pr-3 text-sm placeholder:text-muted-foreground/60 transition-[color,background-color,border-color,box-shadow] duration-150 hover:border-border-strong focus:border-ring focus:bg-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background"
            />
          </form>
          {/* Mobile search toggle */}
          <div className="flex flex-1 justify-end min-[701px]:hidden">
            <button
              type="button"
              aria-label={t("common.search")}
              className="inline-flex size-11 items-center justify-center rounded-md hover:bg-canvas-subtle focus-visible:ring-2 focus-visible:ring-ring"
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
              className={cn(buttonVariants({ variant: "ghost", size: "icon" }), "hidden min-[701px]:inline-flex")}
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
              className={cn(buttonVariants({ variant: "ghost", size: "icon" }), "hidden min-[701px]:inline-flex")}
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

          {/* Publish button */}
          <Link
            href={user ? "/studio/publish/original" : "/login?redirect=/studio/publish/original"}
            className={cn(buttonVariants({ size: "sm" }), "hidden gap-1.5 text-sm min-[701px]:inline-flex")}
          >
            <Plus className="h-4 w-4" />
            {t("nav.publish")}
          </Link>

          {/* Notification bell with dropdown */}
          <NotificationDropdown />

          <button
            type="button"
            aria-label={t("nav.openMenu")}
            aria-expanded={mobileMenuOpen}
            aria-controls="mobile-primary-navigation"
            onClick={() => setMobileMenuOpen(true)}
            className={cn(buttonVariants({ variant: "ghost", size: "icon" }), "min-[701px]:hidden")}
          >
            <Menu className="size-4" />
          </button>

          {/* Desktop user menu */}
          {user ? (
            <DropdownMenu>
              <DropdownMenuTrigger
                className={cn(buttonVariants({ variant: "ghost", size: "sm" }), "hidden gap-2 px-2 min-[701px]:inline-flex")}
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
                <DropdownMenuItem onClick={() => goTo("/studio/overview")}>
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
            <div className="hidden items-center gap-2 min-[701px]:flex">
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
          <form onSubmit={handleSearch} className="absolute inset-x-0 top-full border-b border-border bg-canvas-default px-4 py-2 min-[701px]:hidden">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <input
                type="search"
                aria-label={t("common.search")}
                autoFocus
                placeholder={t("common.search")}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="h-11 w-full rounded-md border border-input bg-background pl-9 pr-3 text-base placeholder:text-muted-foreground/60 focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
              />
            </div>
          </form>
        )}
      </header>

      {mobileMenuOpen && (
        <div className="fixed inset-0 z-[60] min-[701px]:hidden">
          <button
            type="button"
            aria-label={t("studio.sidebar.collapse")}
            className="absolute inset-0 bg-black/50"
            onClick={() => setMobileMenuOpen(false)}
          />
          <aside
            id="mobile-primary-navigation"
            role="dialog"
            aria-modal="true"
            aria-label={t("nav.openMenu")}
            className="relative flex h-full w-[85vw] max-w-[320px] flex-col overflow-y-auto border-r border-border bg-canvas-default p-4 shadow-md"
          >
            <div className="flex items-center justify-between">
              <Link
                href="/"
                onClick={() => setMobileMenuOpen(false)}
                className="flex items-center gap-2 font-semibold text-foreground"
              >
                <Brush className="size-5 text-primary" />
                <span>{t("nav.siteName")}</span>
              </Link>
              <button
                type="button"
                aria-label={t("studio.sidebar.collapse")}
                title={t("studio.sidebar.collapse")}
                onClick={() => setMobileMenuOpen(false)}
                className="inline-flex size-11 items-center justify-center rounded-md text-fg-muted hover:bg-canvas-subtle hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
              >
                <X className="size-5" />
              </button>
            </div>

            <form onSubmit={handleSearch} className="relative mt-4">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <input
                type="search"
                aria-label={t("common.search")}
                placeholder={t("common.search")}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="h-11 w-full rounded-md border border-input bg-background pl-9 pr-3 text-base focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
              />
            </form>

            <nav className="mt-4 flex flex-col gap-1">
              <Link
                href="/recommend"
                aria-current={pathname.startsWith("/recommend") ? "page" : undefined}
                onClick={() => setMobileMenuOpen(false)}
                className={cn(
                  "flex min-h-11 items-center rounded-md px-3 text-sm font-medium text-fg-muted hover:bg-canvas-subtle hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring",
                  pathname.startsWith("/recommend") && "bg-accent-subtle font-semibold text-accent-emphasis",
                )}
              >
                {t("nav.recommend")}
              </Link>
              <Link
                href="/"
                aria-current={pathname === "/" ? "page" : undefined}
                onClick={() => setMobileMenuOpen(false)}
                className={cn(
                  "flex min-h-11 items-center rounded-md px-3 text-sm font-medium text-fg-muted hover:bg-canvas-subtle hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring",
                  pathname === "/" && "bg-accent-subtle font-semibold text-accent-emphasis",
                )}
              >
                {t("nav.fanworkZone")}
              </Link>
              <Link
                href="/original"
                aria-current={pathname.startsWith("/original") ? "page" : undefined}
                onClick={() => setMobileMenuOpen(false)}
                className={cn(
                  "flex min-h-11 items-center rounded-md px-3 text-sm font-medium text-fg-muted hover:bg-canvas-subtle hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring",
                  pathname.startsWith("/original") && "bg-accent-subtle font-semibold text-accent-emphasis",
                )}
              >
                {t("nav.originalZone")}
              </Link>
            </nav>

            <div className="my-4 h-px bg-border" />
            <div className="flex flex-col gap-1">
              {user ? (
                <>
                  <button type="button" onClick={() => goTo(`/user/${user.id}`)} className="min-h-11 rounded-md px-3 text-left text-sm hover:bg-canvas-subtle">{t("nav.profile")}</button>
                  <button type="button" onClick={() => goTo("/studio/overview")} className="min-h-11 rounded-md px-3 text-left text-sm hover:bg-canvas-subtle">{t("nav.dashboard")}</button>
                  <button type="button" onClick={() => goTo("/history")} className="min-h-11 rounded-md px-3 text-left text-sm hover:bg-canvas-subtle">{t("nav.history")}</button>
                  <button type="button" onClick={() => goTo("/settings")} className="min-h-11 rounded-md px-3 text-left text-sm hover:bg-canvas-subtle">{t("nav.settings")}</button>
                  <button type="button" onClick={() => goTo("/appeals")} className="min-h-11 rounded-md px-3 text-left text-sm hover:bg-canvas-subtle">{t("nav.appeals")}</button>
                  <button
                    type="button"
                    onClick={() => {
                      setMobileMenuOpen(false);
                      logout();
                    }}
                    className="min-h-11 rounded-md px-3 text-left text-sm text-destructive hover:bg-canvas-subtle"
                  >
                    {t("nav.logout")}
                  </button>
                </>
              ) : (
                <>
                  <button type="button" onClick={() => goTo("/login")} className="min-h-11 rounded-md px-3 text-left text-sm hover:bg-canvas-subtle">{t("nav.login")}</button>
                  <button type="button" onClick={() => goTo("/register")} className="min-h-11 rounded-md bg-primary px-3 text-left text-sm font-medium text-primary-foreground hover:bg-accent-hover">{t("nav.register")}</button>
                </>
              )}
            </div>
          </aside>
        </div>
      )}
    </>
  );
}
