"use client";

import { FormEvent, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { MarkdownEditor } from "@/components/content/MarkdownEditor";
import { MarkdownRenderer } from "@/components/content/MarkdownRenderer";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { useToast } from "@/components/ui/Toast";

const TITLE_LIMIT = 120;
const BODY_LIMIT = 5000;

interface BroadcastResponse {
  data: {
    recipient_count: number;
    broadcast_at: string;
  };
}

export default function AdminNotificationsPage() {
  const t = useTranslations("adminNotifications");
  const locale = useLocale();
  const { toast } = useToast();
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [titleTouched, setTitleTouched] = useState(false);
  const [bodyTouched, setBodyTouched] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [sending, setSending] = useState(false);

  const validation = useMemo(() => {
    const next: { title?: string; body?: string } = {};
    if (!title.trim()) {
      next.title = t("validation.titleRequired");
    } else if (title.length > TITLE_LIMIT) {
      next.title = t("validation.titleTooLong");
    }

    if (!body.trim()) {
      next.body = t("validation.bodyRequired");
    } else if (body.length > BODY_LIMIT) {
      next.body = t("validation.bodyTooLong");
    }

    return next;
  }, [body, t, title]);

  const isValid = !validation.title && !validation.body;
  const trimmedTitle = title.trim();
  const trimmedBody = body.trim();
  const hasPreview = Boolean(trimmedTitle || trimmedBody);
  const titleError = titleTouched || title.length > TITLE_LIMIT ? validation.title : "";
  const bodyError = bodyTouched || body.length > BODY_LIMIT ? validation.body : "";

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setTitleTouched(true);
    setBodyTouched(true);
    if (!isValid || sending) return;
    setConfirmOpen(true);
  }

  async function handleConfirm() {
    setSending(true);
    try {
      const response = await api.post<BroadcastResponse>("/api/v1/admin/notifications/broadcast", {
        title: trimmedTitle,
        body: trimmedBody,
        channel: "broadcast",
      });
      toast(
        "success",
        t("toast.success", {
          count: response.data.recipient_count,
          time: formatBroadcastTime(response.data.broadcast_at, locale),
        }),
      );
    } catch (error) {
      silentError(error, { component: "AdminNotificationsPage", action: "handleConfirm" });
      toast("error", t("toast.failure"));
    } finally {
      setSending(false);
    }
  }

  return (
    <div className="mx-auto w-full max-w-[1180px] space-y-6 p-4 sm:p-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </header>

      <div className="grid gap-6 xl:grid-cols-[minmax(420px,1fr)_minmax(360px,0.9fr)]">
        <form className="space-y-5 rounded-lg border border-border bg-card p-4 sm:p-5" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <div className="flex items-end justify-between gap-3">
              <label htmlFor="broadcast-title" className="text-sm font-medium text-foreground">
                {t("form.titleLabel")}
              </label>
              <span className="text-xs text-muted-foreground">
                {t("form.titleCount", { count: title.length })}
              </span>
            </div>
            <input
              id="broadcast-title"
              className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm outline-none transition-colors placeholder:text-muted-foreground focus:border-ring focus:ring-2 focus:ring-ring/25 disabled:opacity-60"
              value={title}
              onBlur={() => setTitleTouched(true)}
              onChange={(event) => setTitle(event.currentTarget.value)}
              placeholder={t("form.titlePlaceholder")}
              aria-invalid={Boolean(titleError)}
              aria-describedby="broadcast-title-hint broadcast-title-error"
              disabled={sending}
            />
            <div className="min-h-5">
              {titleError ? (
                <p id="broadcast-title-error" className="text-xs text-destructive">
                  {titleError}
                </p>
              ) : (
                <p id="broadcast-title-hint" className="text-xs text-muted-foreground">
                  {t("form.titleHint")}
                </p>
              )}
            </div>
          </div>

          <div className="space-y-2">
            <div className="flex items-end justify-between gap-3">
              <label htmlFor="broadcast-body" className="text-sm font-medium text-foreground">
                {t("form.bodyLabel")}
              </label>
              <span className="text-xs text-muted-foreground">
                {t("form.bodyCount", { count: body.length })}
              </span>
            </div>
            <div onBlur={() => setBodyTouched(true)}>
              <MarkdownEditor
                id="broadcast-body"
                value={body}
                onChange={setBody}
                disabled={sending}
                ariaDescribedBy="broadcast-body-hint broadcast-body-error"
                ariaInvalid={Boolean(bodyError)}
              />
            </div>
            <div className="min-h-5">
              {bodyError ? (
                <p id="broadcast-body-error" className="text-xs text-destructive">
                  {bodyError}
                </p>
              ) : (
                <p id="broadcast-body-hint" className="text-xs text-muted-foreground">
                  {t("form.bodyHint")}
                </p>
              )}
            </div>
          </div>

          <div className="flex justify-end">
            <Button type="submit" className="min-h-11 w-full sm:w-auto" disabled={!isValid || sending}>
              {sending ? t("form.sending") : t("form.send")}
            </Button>
          </div>
        </form>

        <section
          aria-label={t("a11y.preview")}
          aria-live="polite"
          className="space-y-4 rounded-lg border border-border bg-card p-4 sm:p-5"
        >
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-base font-semibold text-foreground">{t("preview.title")}</h2>
          </div>

          {hasPreview ? (
            <article className="space-y-4 rounded-md border border-border bg-background p-4">
              {trimmedTitle && <h3 className="text-lg font-semibold text-foreground">{trimmedTitle}</h3>}
              {trimmedBody && <MarkdownRenderer content={body} />}
            </article>
          ) : (
            <div className="rounded-md border border-dashed border-border bg-background p-6 text-center">
              <p className="text-sm font-medium text-foreground">{t("preview.emptyTitle")}</p>
              <p className="mt-1 text-sm text-muted-foreground">{t("preview.emptyDescription")}</p>
            </div>
          )}
        </section>
      </div>

      <ConfirmModal
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t("confirm.title")}
        description={t("confirm.description")}
        confirmLabel={t("confirm.confirm")}
        confirmVariant="default"
        onConfirm={handleConfirm}
      />
    </div>
  );
}

function formatBroadcastTime(value: string, locale: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}
