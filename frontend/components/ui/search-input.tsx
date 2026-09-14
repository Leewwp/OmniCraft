"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { Search, X } from "lucide-react";
import { cn } from "@/lib/utils";

/* SP-19 G1-2：全站统一搜索输入框（唯一基准 = IP 库公式）：
   rounded-full + border-border + bg-muted 起底（边框与背景色区分，易注意到）、
   左侧搜索图标、focus:bg-background + 2px focus ring、可选清除按钮（X）。
   两档高度：sm = 36px（min-h-9）/ lg = 44px（min-h-11）。受控/非受控两用；
   token 用主家族（border / muted / background），不逐页保旧 canvas 系 token。 */

interface SearchInputProps
  extends Omit<React.ComponentProps<"input">, "size" | "value" | "onChange"> {
  size?: "sm" | "lg";
  /** 受控值；不传则组件内部维护（非受控）。 */
  value?: string;
  onValueChange?: (value: string) => void;
  /** 显示清除按钮（默认 true）。清除时先置空值再回调（供触发重新搜索等副作用）。 */
  clearable?: boolean;
  onClear?: () => void;
}

export function SearchInput({
  size = "sm",
  value,
  onValueChange,
  clearable = true,
  onClear,
  className,
  ...props
}: SearchInputProps) {
  const t = useTranslations();
  const [internalValue, setInternalValue] = React.useState(
    typeof props.defaultValue === "string" ? props.defaultValue : "",
  );
  const controlled = value !== undefined;
  const current = controlled ? value : internalValue;

  function handleChange(event: React.ChangeEvent<HTMLInputElement>) {
    const next = event.target.value;
    if (!controlled) setInternalValue(next);
    onValueChange?.(next);
  }

  const showClear = clearable && current.length > 0;

  return (
    <div className="relative w-full min-w-0">
      <Search
        className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
        aria-hidden="true"
      />
      <input
        type="search"
        data-slot="search-input"
        value={current}
        onChange={handleChange}
        className={cn(
          "w-full rounded-full border border-border bg-muted pl-9 text-sm text-foreground transition-[background-color,border-color,box-shadow] duration-150 placeholder:text-muted-foreground/60 hover:border-border-strong focus:border-ring focus:bg-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50 [&::-webkit-search-cancel-button]:appearance-none",
          showClear ? "pr-9" : "pr-4",
          size === "sm" ? "min-h-9" : "min-h-11",
          className,
        )}
        {...props}
      />
      {showClear && (
        <button
          type="button"
          aria-label={t("common.clearSearch")}
          onClick={() => {
            if (!controlled) setInternalValue("");
            onValueChange?.("");
            onClear?.();
          }}
          className="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <X className="h-3.5 w-3.5" aria-hidden="true" />
        </button>
      )}
    </div>
  );
}
