"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { Button, buttonVariants } from "@/components/ui/button";

// T48 (FIX-22b): the contributor-facing PR creation panel. Opened from the
// content page entry (?content_id&create=1): pick a base version, describe
// the change, provide the new text, and submit to POST /api/v1/pr.

interface ContentVersionOption {
  id: number;
  version_number: number;
  status: string;
}

interface CreatePRPanelProps {
  contentId: number;
}

export function CreatePRPanel({ contentId }: CreatePRPanelProps) {
  const t = useTranslations();
  const [versions, setVersions] = useState<ContentVersionOption[]>([]);
  const [loading, setLoading] = useState(true);
  const [baseVersionId, setBaseVersionId] = useState<number | null>(null);
  const [message, setMessage] = useState("");
  const [newText, setNewText] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);
  const [validation, setValidation] = useState("");

  useEffect(() => {
    void (async () => {
      setLoading(true);
      setError("");
      try {
        const data = await api.get<{ versions?: ContentVersionOption[] }>(
          `/api/v1/contents/${contentId}/versions?page=1&page_size=50`
        );
        const list = data.versions || [];
        setVersions(list);
        // 默认选最新（列表首行，后端 created_at/version_number 序）
        if (list.length > 0) {
          setBaseVersionId(list[0].id);
        }
      } catch (e) {
        silentError(e, { component: "CreatePRPanel", action: "loadVersions" });
        setError(t(getUserFacingErrorKey(e, "studio.pr.create.loadFailed")));
      } finally {
        setLoading(false);
      }
    })();
  }, [contentId, t]);

  const canSubmit = useMemo(
    () => baseVersionId !== null && newText.trim().length > 0 && !busy,
    [baseVersionId, newText, busy]
  );

  async function handleSubmit() {
    if (baseVersionId === null) {
      setValidation(t("studio.pr.create.selectVersionRequired"));
      return;
    }
    setValidation("");
    setBusy(true);
    setError("");
    try {
      await api.post("/api/v1/pr", {
        content_item_id: contentId,
        base_version_id: baseVersionId,
        message: message.trim(),
        new_text: newText,
      });
      setSuccess(true);
    } catch (e) {
      silentError(e, { component: "CreatePRPanel", action: "handleSubmit" });
      setError(t(getUserFacingErrorKey(e, "dashboard.pr.submitFailed")));
    } finally {
      setBusy(false);
    }
  }

  if (success) {
    return (
      <div className="space-y-4 rounded-md border border-border bg-card p-6">
        <p role="status" className="text-sm font-medium text-emerald-600">
          {t("studio.pr.create.success")}
        </p>
        <Link
          href="/studio/pr-requests"
          className={buttonVariants({ size: "sm", variant: "outline" })}
        >
          {t("studio.pr.create.backToList")}
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-4 rounded-md border border-border bg-card p-4">
      <h2 className="text-lg font-semibold tracking-tight">{t("studio.pr.create.title")}</h2>

      {error ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}

      {loading ? (
        <div className="space-y-3" aria-busy="true">
          <div className="h-8 w-full animate-pulse rounded bg-muted" />
          <div className="h-24 w-full animate-pulse rounded bg-muted" />
        </div>
      ) : (
        <>
          <div>
            <label htmlFor="pr-base-version" className="mb-1.5 block text-sm font-medium text-foreground">
              {t("studio.pr.create.baseVersionLabel")}
            </label>
            <select
              id="pr-base-version"
              value={baseVersionId ?? ""}
              onChange={(e) => setBaseVersionId(Number(e.target.value))}
              className="w-full max-w-xs rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
            >
              {versions.map((v) => (
                <option key={v.id} value={v.id}>
                  v{v.version_number}
                </option>
              ))}
            </select>
            {validation ? (
              <p role="alert" className="mt-1 text-xs text-destructive">
                {validation}
              </p>
            ) : null}
          </div>

          <div>
            <label htmlFor="pr-message" className="mb-1.5 block text-sm font-medium text-foreground">
              {t("studio.pr.create.messageLabel")}
            </label>
            <textarea
              id="pr-message"
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              placeholder={t("studio.pr.create.messagePlaceholder")}
              rows={3}
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
            />
          </div>

          <div>
            <label htmlFor="pr-new-text" className="mb-1.5 block text-sm font-medium text-foreground">
              {t("studio.pr.create.newTextLabel")}
            </label>
            <textarea
              id="pr-new-text"
              value={newText}
              onChange={(e) => setNewText(e.target.value)}
              placeholder={t("studio.pr.create.newTextPlaceholder")}
              rows={10}
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
            />
          </div>

          <div className="flex items-center gap-3">
            <Button size="sm" disabled={!canSubmit} onClick={() => void handleSubmit()}>
              {busy ? t("studio.pr.create.submitting") : t("studio.pr.create.submit")}
            </Button>
            <Link
              href="/studio/pr-requests"
              className={buttonVariants({ size: "sm", variant: "ghost" })}
            >
              {t("studio.pr.create.backToList")}
            </Link>
          </div>
        </>
      )}
    </div>
  );
}
