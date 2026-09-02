"use client";

import { Check } from "lucide-react";
import { cn } from "@/lib/utils";

export interface FilterPillOption {
  value: string;
  label: string;
  count?: number;
}

interface FilterPillsProps {
  options: FilterPillOption[];
  value: string;
  onChange: (value: string) => void;
  ariaLabel: string;
  className?: string;
  loading?: boolean;
  disabled?: boolean;
}

// 全站筛选/类目选择唯一形态（ui-spec §Component: FilterPills）：
// 药丸 + 44px 触控高度 + aria-pressed；选中态 = accent-subtle 底 +
// accent-emphasis 字 + 描边 + Check 图标三线索；切换由接入方就地处理。
export function FilterPills({ options, value, onChange, ariaLabel, className, loading = false, disabled = false }: FilterPillsProps) {
  return (
    <nav
      aria-label={ariaLabel}
      className={cn(
        "flex items-center gap-1 overflow-x-auto pb-1",
        (loading || disabled) && "opacity-50",
        className,
      )}
      style={{ scrollbarWidth: "none" }}
    >
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            onClick={() => onChange(option.value)}
            aria-pressed={active}
            disabled={disabled}
            className={`inline-flex min-h-11 flex-shrink-0 items-center gap-1 rounded-full border px-3 text-xs font-medium transition-colors duration-150 whitespace-nowrap focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed ${
              active
                ? "border-accent-emphasis bg-accent-subtle text-accent-emphasis font-semibold"
                : "border-transparent text-muted-foreground hover:bg-muted hover:text-foreground"
            }`}
          >
            {active && <Check className="h-3.5 w-3.5" aria-hidden="true" />}
            {option.label}
            {option.count != null && (
              <span className="ml-0.5 text-[11px] tabular-nums opacity-70">{option.count}</span>
            )}
          </button>
        );
      })}
    </nav>
  );
}
