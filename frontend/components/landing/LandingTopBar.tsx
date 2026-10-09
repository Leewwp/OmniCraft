"use client";

import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { useTheme } from "next-themes";
import { useEffect, useState } from "react";
import { Brush, Globe, Monitor, Moon, Sun } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import { setLocale } from "@/lib/locale";

// 落地页顶栏（R5）：logo + 章节锚点 + 语言/主题下拉 + 登录 + 主 CTA。
// 语言/主题控件直接复用产品 Header 同款 DropdownMenu 组件形态与文案键
// （Globe 触发 + nav.langZh/langEn、主题三态 nav.themeLight/Dark/System，
// 当前项高亮、点选才切换、点外部关闭、两菜单互斥）——不自创交互、
// 不新建持久化键（locale 走产品 i18n cookie、主题走 next-themes）。

function ThemeIcon({ theme, mounted }: { theme?: string; mounted: boolean }) {
  if (!mounted || theme !== "dark") {
    return <Sun className="h-4 w-4" aria-hidden="true" />;
  }
  return <Moon className="h-4 w-4" aria-hidden="true" />;
}

export function LandingTopBar() {
  const t = useTranslations();
  const locale = useLocale();
  const { theme, resolvedTheme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const activeTheme = theme === "system" ? resolvedTheme : theme;

  async function handleLocaleChange(newLocale: string) {
    // 与产品 Header 同语义：写 NEXT_LOCALE cookie 后整页重载生效。
    await setLocale(newLocale as "zh" | "en");
    window.location.reload();
  }

  return (
    <header className="landing-top" data-testid="landing-topbar">
      <Link href="/recommend" className="landing-logo" aria-label={t("nav.siteName")}>
        <Brush className="h-5 w-5 text-primary" aria-hidden="true" />
        <span>{t("nav.siteName")}</span>
      </Link>

      <nav className="landing-anchors" aria-label={t("landing.surfaces.title")}>
        <a href="#surfaces">{t("landing.nav.surfaces")}</a>
        <a href="#mechanics">{t("landing.nav.mechanics")}</a>
        <a href="#showcase">{t("landing.nav.showcase")}</a>
      </nav>

      <div className="landing-top-spacer" />

      <DropdownMenu>
        <DropdownMenuTrigger
          className={cn(buttonVariants({ variant: "ghost", size: "icon" }), "landing-tbtn")}
          aria-label={t("nav.language")}
        >
          <Globe className="h-4 w-4" aria-hidden="true" />
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

      <DropdownMenu>
        <DropdownMenuTrigger
          className={cn(buttonVariants({ variant: "ghost", size: "icon" }), "landing-tbtn")}
          aria-label={t("nav.themeSwitch")}
        >
          <ThemeIcon theme={activeTheme} mounted={mounted} />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem
            onClick={() => setTheme("light")}
            className={theme === "light" ? "bg-muted" : ""}
          >
            <Sun className="mr-2 h-4 w-4" aria-hidden="true" />
            {t("nav.themeLight")}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => setTheme("dark")}
            className={theme === "dark" ? "bg-muted" : ""}
          >
            <Moon className="mr-2 h-4 w-4" aria-hidden="true" />
            {t("nav.themeDark")}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => setTheme("system")}
            className={theme === "system" ? "bg-muted" : ""}
          >
            <Monitor className="mr-2 h-4 w-4" aria-hidden="true" />
            {t("nav.themeSystem")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Link href="/login" className="landing-login">
        {t("landing.nav.login")}
      </Link>
      <Link href="/recommend" className="landing-cta">
        {t("landing.nav.enter")}
      </Link>
    </header>
  );
}
