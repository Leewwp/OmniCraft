"use client";

import type { ReactNode } from "react";
import { ChevronDown, Loader2, type LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

/**
 * #719 回合元数据三卡（已深度思考 / N 个工具步骤 / N 条参考来源）共享的
 * 轻量小胶囊触发器（DeepSeek「思考结束」形态参考）：一行高、大圆角、极浅
 * 边框、图标 + 文字 + 计数 + 右侧 chevron 展开指示；宽度随文字自然伸缩，
 * 窄视口（390px）由父容器 flex-wrap 换行不溢出。
 *
 * 只统一外观——流式顺序、自动折叠、点击行为（思考/工具原地展开、参考
 * 来源开侧栏）由各使用方维持不变。
 */
interface AgentMetaPillProps {
  /** 图标组件（streaming 时替换为旋转 Loader2）。 */
  icon: LucideIcon;
  label: string;
  count?: number;
  /** 展开指示（chevron 旋转 + aria-expanded）。 */
  expanded?: boolean;
  /** toggle 语义（参考来源入口 aria-pressed）。 */
  pressed?: boolean;
  streaming?: boolean;
  onClick: () => void;
  /** 显式可达名称（如工具块仅有计数字面时）。 */
  ariaLabel?: string;
  className?: string;
  children?: ReactNode;
}

export function AgentMetaPill({
  icon: Icon,
  label,
  count,
  expanded,
  pressed,
  streaming,
  onClick,
  ariaLabel,
  className,
  children,
}: AgentMetaPillProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-expanded={expanded}
      aria-pressed={pressed}
      aria-label={ariaLabel}
      className={cn(
        "inline-flex min-h-7 items-center gap-1.5 rounded-full border border-border-default/50 bg-canvas-subtle px-3 text-xs text-fg-muted transition-colors hover:bg-canvas-subtle/80 hover:text-fg-default focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        className,
      )}
    >
      {streaming ? (
        <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin" aria-hidden="true" />
      ) : (
        <Icon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
      )}
      <span className="font-medium">{label}</span>
      {count !== undefined && <span className="tabular-nums">{count}</span>}
      {children}
      <ChevronDown
        className={cn(
          "h-3.5 w-3.5 shrink-0 transition-transform duration-150",
          expanded && "rotate-180",
        )}
        aria-hidden="true"
      />
    </button>
  );
}
