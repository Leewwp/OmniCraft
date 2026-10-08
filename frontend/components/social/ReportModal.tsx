"use client";

import { useState, useCallback, useEffect, useId, useRef } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { FilterPills, type FilterPillOption } from "@/components/ui/filter-pills";

/* #844：举报预设原因专用弹窗——不动被 20+ 处复用的通用 ConfirmModal。
   交互 = 单选预设 chips（FilterPills，全站筛选选择控件唯一形态）+ 可选补充说明
   （选「其他」时必填）；提交把「预设项（+补充）」拼进现有 reason 字段并保证
   最终值 ≤100 runes（与后端 binding max=100 / DB varchar(100) 对齐）。
   焦点陷阱 / Escape / 背景点击 / busy 语义照搬 ConfirmModal。 */

const REPORT_REASON_MAX = 100;
const PRESET_OTHER = "other";

const PRESET_VALUES = [
  "pornography",
  "illegal",
  "plagiarism",
  "attack",
  "spam",
  "misinformation",
  PRESET_OTHER,
] as const;

type PresetValue = (typeof PRESET_VALUES)[number];

const PRESET_LABEL_KEYS: Record<PresetValue, string> = {
  pornography: "social.reportCategoryPornography",
  illegal: "social.reportCategoryIllegal",
  plagiarism: "social.reportCategoryPlagiarism",
  attack: "social.reportCategoryAttack",
  spam: "social.reportCategorySpam",
  misinformation: "social.reportCategoryMisinformation",
  other: "social.reportCategoryOther",
};

interface ReportModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** 弹窗标题（缺省 social.reportDialogTitle，与旧 ConfirmModal 入口一致）。 */
  title?: string;
  onConfirm: (reason: string) => void | Promise<void>;
}

export function ReportModal({ open, onOpenChange, title, onConfirm }: ReportModalProps) {
  const t = useTranslations();
  const titleId = useId();
  const descriptionId = useId();
  const detailId = useId();
  const dialogRef = useRef<HTMLDivElement>(null);
  const previouslyFocusedRef = useRef<HTMLElement | null>(null);
  const busyRef = useRef(false);
  const onOpenChangeRef = useRef(onOpenChange);
  const [preset, setPreset] = useState("");
  const [detail, setDetail] = useState("");
  const [busy, setBusy] = useState(false);

  const presetOptions: FilterPillOption[] = PRESET_VALUES.map((value) => ({
    value,
    label: t(PRESET_LABEL_KEYS[value]),
  }));
  const presetLabel = PRESET_VALUES.includes(preset as PresetValue)
    ? t(PRESET_LABEL_KEYS[preset as PresetValue])
    : "";
  const isOther = preset === PRESET_OTHER;
  const detailRequired = isOther;
  // 预设标签 + 分隔符占用后剩余可写额度（0 起，防负数）。
  const detailMaxLength = Math.max(REPORT_REASON_MAX - presetLabel.length - 2, 0);
  const detailTooShort = detailRequired && !detail.trim();
  const canSubmit = preset !== "" && !detailTooShort;

  useEffect(() => {
    busyRef.current = busy;
  }, [busy]);

  useEffect(() => {
    onOpenChangeRef.current = onOpenChange;
  }, [onOpenChange]);

  const handleConfirm = useCallback(async () => {
    if (!preset || detailTooShort) return;
    const trimmed = detail.trim();
    const reason = trimmed ? `${presetLabel}: ${trimmed}` : presetLabel;
    // 最终保险：整体仍截到 ≤100 runes（与后端/DB 上限一致）。
    setBusy(true);
    try {
      await onConfirm(Array.from(reason).slice(0, REPORT_REASON_MAX).join(""));
      setPreset("");
      setDetail("");
      onOpenChange(false);
    } catch {
      // 保持弹窗打开，调用方负责失败提示，用户可重试。
    } finally {
      setBusy(false);
    }
  }, [preset, detail, detailTooShort, presetLabel, onConfirm, onOpenChange]);

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
        <h3 id={titleId} className="text-xl font-semibold tracking-tight">
          {title ?? t("social.reportDialogTitle")}
        </h3>
        <p id={descriptionId} className="mt-2 text-sm text-muted-foreground">
          {t("social.reportCategoryLabel")}
        </p>

        <div className="mt-3">
          <FilterPills
            selectionMode="single"
            options={presetOptions}
            value={preset}
            onChange={setPreset}
            wrap
            disabled={busy}
            ariaLabel={t("social.reportCategoryLabel")}
          />
        </div>

        {preset !== "" && (
          <div className="mt-4">
            <Label htmlFor={detailId} className="mb-2">
              {detailRequired
                ? t("social.reportDetailRequiredLabel")
                : t("social.reportDetailOptionalLabel")}
            </Label>
            <Textarea
              id={detailId}
              rows={3}
              value={detail}
              onChange={(e) => setDetail(e.target.value.slice(0, detailMaxLength))}
              placeholder={
                detailRequired
                  ? t("social.reportDetailRequiredLabel")
                  : t("social.reportDetailOptionalLabel")
              }
              maxLength={detailMaxLength}
              disabled={busy}
              required={detailRequired}
              aria-required={detailRequired}
            />
            <p className="mt-1 text-xs text-muted-foreground">{t("social.reportLimitHint")}</p>
          </div>
        )}

        <div className="mt-6 flex justify-end gap-3">
          <Button
            variant="outline"
            size="sm"
            disabled={busy}
            onClick={() => onOpenChange(false)}
          >
            {t("common.cancel")}
          </Button>
          <Button
            variant="destructive"
            size="sm"
            disabled={busy || !canSubmit}
            onClick={() => void handleConfirm()}
          >
            {busy ? t("common.processing") : t("social.report")}
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
