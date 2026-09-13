"use client";

import { useEffect, useId, useRef, type ComponentType, type KeyboardEvent, type RefObject } from "react";
import { createPortal } from "react-dom";
import Link from "next/link";
import { X } from "lucide-react";
import { useTranslations } from "next-intl";
import { LoginForm } from "@/components/auth/LoginForm";
import type { CaptchaWidgetProps } from "@/components/verification/CaptchaWidget";

export interface LoginModalProps {
  open: boolean;
  onClose: () => void;
  /** 登录成功（表单 onSuccess）；关闭+续做由 AuthGateProvider 编排。 */
  onSuccess: () => void;
  /** 非空时渲染进该容器（原生 dialog top layer 场景），否则 portal 到 body。 */
  portalContainer?: HTMLElement | null;
  captchaComponent?: ComponentType<CaptchaWidgetProps>;
  /** 关闭时焦点归还目标（requireAuth 触发瞬间捕获；事件桥触发时为 null）。 */
  restoreFocusRef?: RefObject<HTMLElement | null>;
}

// SP-17/T2 (#491)：登录浮窗——confirm-modal 背板+焦点陷阱同模式。
// 层级：普通页面 portal body（z-50）；详情浮窗（top layer）内触发时由
// AuthGateProvider 提供容器，渲染进该 dialog 内部（top layer 子树随绘）。

export function LoginModal({
  open,
  onClose,
  onSuccess,
  portalContainer,
  captchaComponent,
  restoreFocusRef,
}: LoginModalProps) {
  const t = useTranslations();
  const titleId = useId();
  const dialogRef = useRef<HTMLDivElement>(null);
  const previouslyFocusedRef = useRef<HTMLElement | null>(null);
  const openRef = useRef(open);

  useEffect(() => {
    openRef.current = open;
  }, [open]);

  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    if (!open) return;

    previouslyFocusedRef.current =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;

    const focusTimer = window.setTimeout(() => {
      const emailInput = dialogRef.current?.querySelector<HTMLInputElement>("input[data-testid='login-email']");
      (emailInput ?? dialogRef.current)?.focus();
    }, 0);

    function handleKeyDown(event: KeyboardEvent | globalThis.KeyboardEvent) {
      if (!dialogRef.current || !openRef.current) return;

      if (event.key === "Escape") {
        event.preventDefault();
        // 详情浮窗 dialog 的 cancel 由 isGateOpen 门让位（见 ContentDetailOverlay）。
        event.stopPropagation();
        onCloseRef.current();
        return;
      }

      if (event.key !== "Tab") return;

      const focusable = getFocusableElements(dialogRef.current);
      if (focusable.length === 0) {
        event.preventDefault();
        dialogRef.current.focus();
        return;
      }

      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const active = document.activeElement;

      if (event.shiftKey && active === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && active === last) {
        event.preventDefault();
        first.focus();
      }
    }

    document.addEventListener("keydown", handleKeyDown);

    return () => {
      window.clearTimeout(focusTimer);
      document.removeEventListener("keydown", handleKeyDown);
      (restoreFocusRef?.current ?? previouslyFocusedRef.current)?.focus();
    };
  }, [open]);

  if (!open || typeof document === "undefined") return null;

  const content = (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div
        className="fixed inset-0 bg-black/50 backdrop-blur-sm"
        data-testid="login-modal-backdrop"
        onClick={onClose}
      />
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className="relative z-50 w-full max-w-sm rounded-lg border border-border bg-card p-6 shadow-md"
        data-testid="login-modal"
      >
        <div className="mb-4 flex items-start justify-between gap-2">
          <div>
            <h2 id={titleId} className="text-lg font-semibold tracking-tight">{t("auth.welcomeBack")}</h2>
            <p className="mt-1 text-sm text-muted-foreground">{t("auth.loginRequiredDescription")}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label={t("common.close")}
            className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          >
            <X className="h-4 w-4" aria-hidden="true" />
          </button>
        </div>

        <LoginForm onSuccess={onSuccess} autoFocusEmail CaptchaComponent={captchaComponent} />

        <p className="mt-4 text-center text-sm text-muted-foreground">
          {t("auth.noAccount")}{" "}
          <Link href="/register" className="font-medium text-primary hover:underline">
            {t("auth.registerNow")}
          </Link>
        </p>
      </div>
    </div>
  );

  return createPortal(content, portalContainer ?? document.body);
}

function getFocusableElements(root: HTMLElement | null): HTMLElement[] {
  if (!root) return [];

  return Array.from(
    root.querySelectorAll<HTMLElement>(
      [
        "a[href]",
        "button:not([disabled])",
        "textarea:not([disabled])",
        "input:not([disabled])",
        "select:not([disabled])",
        '[tabindex]:not([tabindex="-1"])',
      ].join(","),
    ),
  );
}
