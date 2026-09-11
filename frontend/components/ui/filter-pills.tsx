"use client";

import { cn } from "@/lib/utils";

export interface FilterPillOption {
  value: string;
  label: string;
  count?: number;
}

/* #414 O1a：全站筛选药丸矮化统一（2026-09-07 裁决，取代 SP-12「44px+勾号」
   旧基准）——紧凑档（py-1.5，约 28px 高度档）+ 无勾号图标；选中态三线索
   = accent 浅底 + 主色字 + 1px 描边 + semibold，加 aria-pressed 满足
   「不只靠颜色」。selectionMode 支持单选（可选 clearable=点击已选中项清空）
   与多选（点击添加、再次点击移除）。 */

interface FilterPillsBaseProps {
  options: FilterPillOption[];
  ariaLabel: string;
  className?: string;
  loading?: boolean;
  disabled?: boolean;
  /** 侧栏等窄容器改为换行堆叠（默认横向滚动）。 */
  wrap?: boolean;
}

interface FilterPillsSingleProps extends FilterPillsBaseProps {
  selectionMode?: "single";
  value: string;
  onChange: (value: string) => void;
  /** 单选模式：点击已选中项清空（搜索分类、IP 类目等可选场景）。 */
  clearable?: boolean;
}

interface FilterPillsMultipleProps extends FilterPillsBaseProps {
  selectionMode: "multiple";
  value: string[];
  onChange: (value: string[]) => void;
}

export type FilterPillsProps = FilterPillsSingleProps | FilterPillsMultipleProps;

export function FilterPills(props: FilterPillsProps) {
  const { options, ariaLabel, className, loading = false, disabled = false, wrap = false } = props;
  const single = props.selectionMode !== "multiple";

  function isSelected(value: string) {
    return single ? props.value === value : props.value.includes(value);
  }

  function handleClick(option: FilterPillOption) {
    if (single) {
      if (props.clearable && props.value === option.value) {
        props.onChange("");
        return;
      }
      props.onChange(option.value);
      return;
    }
    props.onChange(
      props.value.includes(option.value)
        ? props.value.filter((item) => item !== option.value)
        : [...props.value, option.value],
    );
  }

  return (
    <nav
      aria-label={ariaLabel}
      className={cn(
        wrap ? "flex flex-wrap items-center gap-1.5" : "flex items-center gap-1 overflow-x-auto pb-1",
        (loading || disabled) && "opacity-50",
        className,
      )}
      style={wrap ? undefined : { scrollbarWidth: "none" }}
    >
      {options.map((option) => {
        const active = isSelected(option.value);
        return (
          <button
            key={option.value}
            type="button"
            onClick={() => handleClick(option)}
            aria-pressed={active}
            disabled={disabled}
            className={`inline-flex flex-shrink-0 items-center gap-1 rounded-full border px-3.5 py-1.5 text-xs font-medium transition-colors duration-150 whitespace-nowrap select-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed ${
              active
                ? "border-accent-emphasis bg-accent-subtle text-accent-emphasis font-semibold"
                : "border-transparent text-muted-foreground hover:bg-muted hover:text-foreground"
            }`}
          >
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
