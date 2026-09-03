"use client";

import { useEffect, useId, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Loader2, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { cn } from "@/lib/utils";

interface GlobalSearchInputProps {
  size?: "sm" | "lg";
  className?: string;
  autoFocus?: boolean;
  /* 受控模式（A-07 搜索页主体输入）：传入 value/onValueChange/onSubmit 后
   * 提交不再自行路由跳转，改调 onSubmit；不传则保持 Header 非受控行为。 */
  value?: string;
  onValueChange?: (value: string) => void;
  onSubmit?: (query: string) => void;
  /** 受控模式占位文案（Header 非受控沿用 common.search）。 */
  placeholder?: string;
  /** 受控模式提交按钮文案；不传则不渲染按钮（Header 形态）。 */
  submitLabel?: string;
  isLoading?: boolean;
  disabled?: boolean;
}

interface SuggestionRow {
  text: string;
  score: number;
}

/**
 * 全站关键词搜索输入（Header 桌面/移动/菜单三处共用 + 搜索页主体输入）：
 * keyword-only，不提供 Agent 模式开关，不承接任何 Agent 状态。
 * 非受控（Header）：提交跳转 `/search?q=<query>`。
 * 受控（搜索页，A-07）：提交调用 onSubmit，由页面执行站内搜索。
 * 输入时展示站内建议下拉（T21：标签/公开内容标题，contains 匹配）。
 */
export function GlobalSearchInput({
  size = "sm",
  className,
  autoFocus,
  value,
  onValueChange,
  onSubmit,
  placeholder,
  submitLabel,
  isLoading = false,
  disabled = false,
}: GlobalSearchInputProps) {
  const t = useTranslations();
  const router = useRouter();
  const controlled = value !== undefined;
  const [internalQuery, setInternalQuery] = useState("");
  const query = controlled ? value : internalQuery;
  const [suggestions, setSuggestions] = useState<SuggestionRow[]>([]);
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const fetchEpochRef = useRef(0);
  /* 同屏可能同时存在 Header 与搜索页两个实例（A-07），listbox id 必须实例唯一。 */
  const listboxId = useId();

  /* 建议下拉：300ms 防抖请求 /search/suggestions；代际计数丢弃陈旧响应，
   * 失败静默（下拉非关键路径）。 */
  useEffect(() => {
    const trimmed = query.trim();
    if (!trimmed) {
      fetchEpochRef.current += 1;
      setSuggestions([]);
      setOpen(false);
      return;
    }
    fetchEpochRef.current += 1;
    const epoch = fetchEpochRef.current;
    const timer = setTimeout(() => {
      api
        .get<{ suggestions?: SuggestionRow[] }>(
          `/api/v1/search/suggestions?q=${encodeURIComponent(trimmed)}&limit=8`,
        )
        .then((data) => {
          if (epoch !== fetchEpochRef.current) return;
          setSuggestions(data.suggestions ?? []);
          setOpen(true);
          setActiveIndex(-1);
        })
        .catch((e) => {
          if (epoch !== fetchEpochRef.current) return;
          silentError(e, { component: "GlobalSearchInput", action: "load suggestions" });
          setSuggestions([]);
        });
    }, 300);
    return () => {
      clearTimeout(timer);
    };
  }, [query]);

  /* 点击组件外部关闭下拉。 */
  useEffect(() => {
    function handlePointerDown(event: PointerEvent) {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("pointerdown", handlePointerDown);
    return () => document.removeEventListener("pointerdown", handlePointerDown);
  }, []);

  function setQueryState(next: string) {
    setInternalQuery(next);
    onValueChange?.(next);
  }

  function go(term: string) {
    const trimmed = term.trim();
    if (!trimmed) return;
    setOpen(false);
    if (onSubmit) {
      onSubmit(trimmed);
      return;
    }
    router.push(`/search?q=${encodeURIComponent(trimmed)}`);
  }

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (activeIndex >= 0 && suggestions[activeIndex]) {
      go(suggestions[activeIndex].text);
      return;
    }
    go(query);
  }

  function handleKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (!open || suggestions.length === 0) return;
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((index) => (index + 1) % suggestions.length);
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((index) => (index <= 0 ? suggestions.length - 1 : index - 1));
    } else if (event.key === "Escape") {
      setOpen(false);
    }
  }

  return (
    <div ref={rootRef} className={cn("relative", className)}>
      <form role="search" onSubmit={handleSubmit} className={cn(submitLabel && "flex items-center gap-2")}>
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="search"
            aria-label={placeholder ?? t("common.search")}
            autoFocus={autoFocus}
            placeholder={placeholder ?? t("common.search")}
            value={query}
            onChange={(event) => setQueryState(event.target.value)}
            onKeyDown={handleKeyDown}
            role="combobox"
            aria-expanded={open && suggestions.length > 0}
            aria-autocomplete="list"
            aria-controls={listboxId}
            aria-busy={isLoading || undefined}
            disabled={disabled || undefined}
            className={cn(
              "w-full rounded-md border pl-9 pr-3 focus:outline-none",
              size === "sm"
                ? "h-8 border-transparent bg-canvas-subtle text-sm placeholder:text-muted-foreground/60 transition-[color,background-color,border-color,box-shadow] duration-150 hover:border-border-strong focus:border-ring focus:bg-background focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background"
                : "h-11 border-input bg-background text-base placeholder:text-muted-foreground/60 focus:border-ring focus:ring-2 focus:ring-ring focus:ring-offset-2",
            )}
          />
        </div>
        {submitLabel && (
          <Button type="submit" size="sm" disabled={disabled || isLoading || !query.trim()} aria-label={submitLabel}>
            {isLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
          </Button>
        )}
      </form>
      {open && suggestions.length > 0 && (
        <ul
          id={listboxId}
          role="listbox"
          aria-label={t("search.suggestionsLabel")}
          className="absolute left-0 right-0 top-full z-50 mt-1 max-h-72 overflow-y-auto rounded-md border border-border bg-card py-1 shadow-[var(--elevation-3)]"
        >
          {suggestions.map((row, index) => (
            <li key={`${row.text}-${index}`} role="option" aria-selected={index === activeIndex}>
              <button
                type="button"
                className={cn(
                  "flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors duration-150",
                  index === activeIndex
                    ? "bg-canvas-subtle text-foreground"
                    : "text-muted-foreground hover:bg-canvas-subtle hover:text-foreground",
                )}
                onMouseEnter={() => setActiveIndex(index)}
                onClick={() => go(row.text)}
              >
                <Search className="h-3.5 w-3.5 shrink-0" />
                <span className="truncate">{row.text}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
