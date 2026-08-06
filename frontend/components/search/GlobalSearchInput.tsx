"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Search } from "lucide-react";
import { cn } from "@/lib/utils";

interface GlobalSearchInputProps {
  size?: "sm" | "lg";
  className?: string;
  autoFocus?: boolean;
}

/**
 * 全站关键词搜索输入（Header 桌面/移动/菜单三处共用）：keyword-only，
 * 提交跳转 `/search?q=<query>`；不提供 Agent 模式开关，不承接任何 Agent 状态。
 */
export function GlobalSearchInput({ size = "sm", className, autoFocus }: GlobalSearchInputProps) {
  const t = useTranslations();
  const router = useRouter();
  const [query, setQuery] = useState("");

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    const trimmed = query.trim();
    if (!trimmed) return;
    router.push(`/search?q=${encodeURIComponent(trimmed)}`);
  }

  return (
    <form role="search" onSubmit={handleSubmit} className={cn("relative", className)}>
      <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
      <input
        type="search"
        aria-label={t("common.search")}
        autoFocus={autoFocus}
        placeholder={t("common.search")}
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        className={cn(
          "w-full rounded-md border pl-9 pr-3 focus:outline-none",
          size === "sm"
            ? "h-8 border-transparent bg-canvas-subtle text-sm placeholder:text-muted-foreground/60 transition-[color,background-color,border-color,box-shadow] duration-150 hover:border-border-strong focus:border-ring focus:bg-background focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background"
            : "h-11 border-input bg-background text-base placeholder:text-muted-foreground/60 focus:border-ring focus:ring-2 focus:ring-ring focus:ring-offset-2",
        )}
      />
    </form>
  );
}
