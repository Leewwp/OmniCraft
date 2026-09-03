"use client";

import { useCallback, useEffect, useId, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Loader2, MessageCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/components/ui/Toast";
import { useAuth, interactionDenialKey } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";

/**
 * 用户主页「发私信」入口（FIX-30①/T35，纯前端）：打开撰写弹窗发送首条私信。
 * 走既有 POST /messages 冷启动 guard 语义——每个方向首条天然放行，连续第二条
 * 由后端 DM_REPLY_REQUIRED 拦截（唯一防骚扰机制，本组件只如实转达，不做任何
 * 预放宽）。未登录跳 /login；禁言用户禁用并提示原因（FollowButton 同模式）。
 */
interface MessageComposeButtonProps {
  userId: number;
  displayName: string;
}

const MAX_DM_LENGTH = 2000;

export function MessageComposeButton({ userId, displayName }: MessageComposeButtonProps) {
  const t = useTranslations();
  const router = useRouter();
  const { user, capabilities } = useAuth();
  const { toast } = useToast();
  const [open, setOpen] = useState(false);
  const interactionBlocked = !!user && !capabilities.can_interact;

  function openCompose() {
    if (!user) {
      router.push("/login");
      return;
    }
    setOpen(true);
  }

  return (
    <>
      <Button
        size="sm"
        variant="outline"
        className="gap-1"
        onClick={openCompose}
        disabled={interactionBlocked}
        title={interactionBlocked ? t(interactionDenialKey(capabilities.interaction_denial_reason)) : undefined}
      >
        <MessageCircle className="h-3.5 w-3.5" />
        {t("user.sendMessage")}
      </Button>
      {open && (
        <MessageComposeDialog
          userId={userId}
          displayName={displayName}
          onClose={() => setOpen(false)}
        />
      )}
    </>
  );
}

function MessageComposeDialog({
  userId,
  displayName,
  onClose,
}: {
  userId: number;
  displayName: string;
  onClose: () => void;
}) {
  const t = useTranslations();
  const router = useRouter();
  const { toast } = useToast();
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const titleId = useId();
  const descriptionId = useId();
  const textareaId = useId();
  const dialogRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const previouslyFocusedRef = useRef<HTMLElement | null>(null);
  const busyRef = useRef(false);
  const onCloseRef = useRef(onClose);

  useEffect(() => {
    busyRef.current = busy;
  }, [busy]);

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  const closeDialog = useCallback(() => {
    if (!busyRef.current) onCloseRef.current();
  }, []);

  /* 焦点管理：打开时聚焦输入框，Esc 关闭，Tab 循环，关闭后焦点回入口（ConfirmModal 同模式）。 */
  useEffect(() => {
    previouslyFocusedRef.current =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;

    const focusTimer = window.setTimeout(() => {
      textareaRef.current?.focus();
    }, 0);

    function handleKeyDown(event: KeyboardEvent) {
      if (!dialogRef.current) return;
      if (event.key === "Escape" && !busyRef.current) {
        event.preventDefault();
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
      previouslyFocusedRef.current?.focus();
    };
  }, []);

  async function send() {
    const body = text.trim();
    if (!body || busy) return;
    setBusy(true);
    setError("");
    try {
      await api.post("/api/v1/messages", { recipient_id: userId, text: body });
      toast("success", t("messages.compose.success"));
      onClose();
      router.push("/messages");
    } catch (e) {
      /* 失败保持弹窗打开以便重试；guard/审核错误给专用文案。 */
      if (e instanceof ApiRequestError && e.code === "DM_REPLY_REQUIRED") {
        /* 顶层 messages.dmReplyRequired 为既有 key（messages.chat.replyRequired 不存在）。 */
        setError(t("messages.dmReplyRequired"));
      } else if (e instanceof ApiRequestError && e.code === "CONTENT_BLOCKED") {
        setError(t("messages.compose.blocked"));
      } else {
        setError(t(getUserFacingErrorKey(e, "messages.error.send")));
      }
      silentError(e, { component: "MessageComposeButton", action: "send" });
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="fixed inset-0 bg-black/50 backdrop-blur-sm" onClick={closeDialog} />
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={descriptionId}
        tabIndex={-1}
        className="relative z-50 w-full max-w-md rounded-lg border border-border bg-card p-6 shadow-md"
      >
        <h3 id={titleId} className="text-xl font-semibold tracking-tight">
          {t("messages.compose.title", { name: displayName })}
        </h3>
        <p id={descriptionId} className="mt-2 text-sm text-muted-foreground">
          {t("messages.compose.description")}
        </p>

        <div className="mt-4">
          <Label htmlFor={textareaId} className="mb-2">
            {t("messages.compose.label")}
          </Label>
          <Textarea
            id={textareaId}
            ref={textareaRef}
            rows={4}
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder={t("messages.compose.placeholder")}
            maxLength={MAX_DM_LENGTH}
            disabled={busy}
            aria-required="true"
            onKeyDown={(e) => {
              /* Enter 发送、Shift+Enter 换行（ChatWindow 输入同语义）。 */
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                void send();
              }
            }}
          />
        </div>

        {error && (
          <p className="mt-2 text-sm text-destructive" role="alert">
            {error}
          </p>
        )}

        <div className="mt-6 flex justify-end gap-3">
          <Button variant="outline" size="sm" disabled={busy} onClick={closeDialog}>
            {t("common.cancel")}
          </Button>
          <Button size="sm" disabled={busy || !text.trim()} onClick={() => void send()}>
            {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : t("messages.chat.send")}
          </Button>
        </div>
      </div>
    </div>
  );
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
