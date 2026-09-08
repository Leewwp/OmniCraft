"use client";

import { forwardRef, useEffect, useRef } from "react";
import { LoaderCircle, Send, Square } from "lucide-react";
import { cn } from "@/lib/utils";

/* #413 F6a 公共聊天式 Composer：发送按钮内嵌输入框右下角（与内边缘 8px），
   背景融合（非独立悬浮卡片），多行自动增高、上限转内部滚动（泛化自 Agent
   工作台 208px 契约）。文本为按钮预留右/底内边距，任何输入量下不重叠。

   按键语义显式模式（全站现状矩阵，见 #413）：
   - "enter"      Enter 提交、Shift+Enter 换行（私信窗、IP 讨论回复）
   - "ctrl-enter" Ctrl/Cmd+Enter 提交、Enter 换行（顶层评论）
   - "button-only" 仅按钮提交（评论一级回复现状保持）
   所有模式统一 isComposing 防护：输入法选词的 Enter 不得触发提交。 */

export type ComposerKeyMode = "enter" | "ctrl-enter" | "button-only";

/** 自动增高上限：达到后转为内部滚动（沿用 Agent 工作台量级）。 */
export const COMPOSER_MAX_HEIGHT = 208;

interface ComposerProps {
  value: string;
  onChange: (value: string) => void;
  onSubmit: () => void;
  submitLabel: string;
  keyMode: ComposerKeyMode;
  placeholder?: string;
  ariaLabel?: string;
  /** 输入框禁用（加载态等）。 */
  disabled?: boolean;
  /** 发送按钮禁用（空文本/提交中）。 */
  submitDisabled?: boolean;
  /** 提交中（按钮转 spinner）。 */
  submitting?: boolean;
  /** #417 F6b：流式停止——两值齐备时内嵌位渲染停止按钮（Agent 工作台语义）。 */
  stopLabel?: string;
  onStop?: () => void;
  rows?: number;
  maxHeight?: number;
  maxLength?: number;
  className?: string;
}

export const Composer = forwardRef(function Composer({
  value,
  onChange,
  onSubmit,
  submitLabel,
  keyMode,
  placeholder,
  ariaLabel,
  disabled,
  submitDisabled,
  submitting,
  rows = 1,
  maxHeight = COMPOSER_MAX_HEIGHT,
  maxLength,
  className,
  stopLabel,
  onStop,
}: ComposerProps,
ref: React.ForwardedRef<HTMLTextAreaElement>,
) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  /* 自动增高：贴内容长高，maxHeight 封顶转内部滚动。 */
  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, maxHeight)}px`;
  }, [value, maxHeight]);

  function handleKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (event.nativeEvent.isComposing) return; // 输入法选词 Enter 不触发
    if (keyMode === "enter" && event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      onSubmit();
      return;
    }
    if (
      keyMode === "ctrl-enter" &&
      event.key === "Enter" &&
      (event.ctrlKey || event.metaKey)
    ) {
      event.preventDefault();
      onSubmit();
    }
  }

  return (
    <div
      className={cn(
        "relative rounded-md border border-border-default bg-canvas-default focus-within:border-accent-emphasis focus-within:ring-1 focus-within:ring-accent-emphasis disabled:cursor-not-allowed disabled:opacity-60",
        className,
      )}
    >
      <textarea
        ref={(node) => {
          textareaRef.current = node;
          if (typeof ref === "function") ref(node);
          else if (ref) ref.current = node;
        }}
        value={value}
        rows={rows}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={handleKeyDown}
        aria-label={ariaLabel}
        placeholder={placeholder}
        maxLength={maxLength}
        className="block w-full resize-none overflow-y-auto bg-transparent px-3 pb-10 pt-2 pr-12 text-sm text-fg-default placeholder:text-fg-subtle focus:outline-none disabled:cursor-not-allowed"
        style={{ maxHeight }}
      />
      {/* 内嵌动作按钮：右下角 8px、背景融合（透明底，hover 才有 subtle 反馈）。
          #417：流式期渲染停止按钮（同一内嵌位，Square 图标 + stopLabel）。 */}
      {stopLabel && onStop ? (
        <button
          type="button"
          aria-label={stopLabel}
          onClick={onStop}
          className="absolute bottom-2 right-2 inline-flex h-8 w-8 items-center justify-center rounded-md text-fg-muted transition-colors duration-150 hover:bg-canvas-subtle hover:text-destructive focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <Square className="h-3.5 w-3.5 fill-current" aria-hidden="true" />
          <span className="sr-only">{stopLabel}</span>
        </button>
      ) : (
      <button
        type="button"
        aria-label={submitLabel}
        onClick={onSubmit}
        disabled={submitDisabled || disabled}
        className="absolute bottom-2 right-2 inline-flex h-8 w-8 items-center justify-center rounded-md text-fg-muted transition-colors duration-150 hover:bg-canvas-subtle hover:text-accent-emphasis focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-40"
      >
          {submitting ? (
            <LoaderCircle className="h-4 w-4 animate-spin" aria-hidden="true" />
          ) : (
            <Send className="h-4 w-4" aria-hidden="true" />
          )}
          <span className="sr-only">{submitLabel}</span>
        </button>
      )}
    </div>
  );
});