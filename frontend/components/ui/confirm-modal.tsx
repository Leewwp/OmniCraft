"use client";

import { useState, useCallback } from "react";
import { Button } from "@/components/ui/button";

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
  confirmLabel = "确认",
  confirmVariant = "destructive",
  requireReason = false,
  reasonLabel = "操作原因",
  onConfirm,
}: ConfirmModalProps) {
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);

  const handleConfirm = useCallback(async () => {
    if (requireReason && !reason.trim()) return;
    setBusy(true);
    try {
      await onConfirm(reason);
      setReason("");
      onOpenChange(false);
    } finally {
      setBusy(false);
    }
  }, [reason, requireReason, onConfirm, onOpenChange]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div
        className="fixed inset-0 bg-black/50"
        onClick={() => !busy && onOpenChange(false)}
      />
      <div className="relative z-50 w-full max-w-md rounded-lg border border-border bg-card p-6 shadow-md">
        <h3 className="text-lg font-semibold">{title}</h3>
        <p className="mt-2 text-sm text-muted-foreground">{description}</p>

        {requireReason && (
          <div className="mt-4">
            <label className="block text-sm font-medium mb-1">{reasonLabel}</label>
            <textarea
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
              rows={3}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="请输入操作原因..."
              disabled={busy}
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
            取消
          </Button>
          <Button
            variant={confirmVariant}
            size="sm"
            disabled={busy || (requireReason && !reason.trim())}
            onClick={() => void handleConfirm()}
          >
            {busy ? "处理中..." : confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  );
}
