"use client";

import { useState, useCallback, useEffect, useId, useRef } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

interface ConfirmModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  confirmLabel?: string;
  confirmVariant?: "default" | "destructive";
  requireReason?: boolean;
  reasonLabel?: string;
  onConfirm: (reason: string) => void | Promise<void>;
}

export function ConfirmModal({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  confirmVariant = "destructive",
  requireReason = false,
  reasonLabel,
  onConfirm,
}: ConfirmModalProps) {
  const t = useTranslations();
  const titleId = useId();
  const descriptionId = useId();
  const reasonId = useId();
  const dialogRef = useRef<HTMLDivElement>(null);
  const previouslyFocusedRef = useRef<HTMLElement | null>(null);
  const busyRef = useRef(false);
  const onOpenChangeRef = useRef(onOpenChange);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);

  const effectiveConfirmLabel = confirmLabel ?? t('common.confirm');
  const effectiveReasonLabel = reasonLabel ?? t('common.reason');

  useEffect(() => {
    busyRef.current = busy;
  }, [busy]);

  useEffect(() => {
    onOpenChangeRef.current = onOpenChange;
  }, [onOpenChange]);

  const handleConfirm = useCallback(async () => {
    if (requireReason && !reason.trim()) return;
    setBusy(true);
    try {
      await onConfirm(reason);
      setReason("");
      onOpenChange(false);
    } catch {
      // Keep the dialog open so the caller can show the failure and the user can retry.
    } finally {
      setBusy(false);
    }
  }, [reason, requireReason, onConfirm, onOpenChange]);

  useEffect(() => {
    if (!open) return;

    previouslyFocusedRef.current =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;

    const focusTimer = window.setTimeout(() => {
      const focusable = getFocusableElements(dialogRef.current);
      (focusable[0] ?? dialogRef.current)?.focus();
    }, 0);

    function handleKeyDown(event: KeyboardEvent) {
      if (!dialogRef.current) return;

      if (event.key === "Escape" && !busyRef.current) {
        event.preventDefault();
        onOpenChangeRef.current(false);
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
  }, [open]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div
        className="fixed inset-0 bg-black/50 backdrop-blur-sm"
        onClick={() => !busy && onOpenChange(false)}
      />
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={descriptionId}
        tabIndex={-1}
        className="relative z-50 w-full max-w-md rounded-lg border border-border bg-card p-6 shadow-md"
      >
        <h3 id={titleId} className="text-xl font-semibold tracking-tight">{title}</h3>
        <p id={descriptionId} className="mt-2 text-sm text-muted-foreground">{description}</p>

        {requireReason && (
          <div className="mt-4">
            <Label htmlFor={reasonId} className="mb-2">
              {effectiveReasonLabel}
            </Label>
            <Textarea
              id={reasonId}
              rows={3}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder={effectiveReasonLabel}
              disabled={busy}
              required={requireReason}
              aria-required={requireReason}
            />
          </div>
        )}

        <div className="mt-6 flex justify-end gap-3">
          <Button
            variant="outline"
            size="sm"
            disabled={busy}
            onClick={() => onOpenChange(false)}
          >
            {t('common.cancel')}
          </Button>
          <Button
            variant={confirmVariant}
            size="sm"
            disabled={busy || (requireReason && !reason.trim())}
            onClick={() => void handleConfirm()}
          >
            {busy ? t('common.processing') : effectiveConfirmLabel}
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
