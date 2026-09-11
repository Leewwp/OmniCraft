"use client";

import { FormEvent, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { Button } from "@/components/ui/button";
import { silentError } from "@/lib/error-handler";

export default function NewDiscussionPage() {
  const t = useTranslations();
  const router = useRouter();
  const params = useParams<{ ipId: string }>();
  const ipId = parseInt(params.ipId, 10);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [submitted, setSubmitted] = useState(false);
  const titleInputRef = useRef<HTMLInputElement>(null);
  const titleInvalid = submitted && !title.trim();

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitted(true);
    if (!title.trim()) {
      titleInputRef.current?.focus();
      return;
    }
    setBusy(true);
    setError("");
    try {
      const res = await api.post<{ discussion?: { id: number } }>(
        `/api/v1/ips/${ipId}/discussions`,
        { title: title.trim(), body: body.trim() },
      );
      const discId = res.discussion?.id;
      // #290：讨论列表并入 /ip/[ipId] Hub 的 discussions tab（帖详情为浮层），
      // 创建后直接回到该 tab，避免经旧路由多跳一次 301。
      router.push(discId ? `/ip/${ipId}?tab=discussions&d=${discId}` : `/ip/${ipId}?tab=discussions`);
    } catch (e) {
      silentError(e, { component: 'NewDiscussionPage', action: 'handleSubmit' });
      setError(t(getUserFacingErrorKey(e)));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto w-full max-w-2xl space-y-6 px-4 py-6">
      <h1 className="text-2xl font-bold tracking-tight">{t("discussion.newPost")}</h1>

      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}

      <form className="space-y-4 rounded-md border border-border bg-card p-4" onSubmit={handleSubmit}>
        <div className="space-y-1">
          <label htmlFor="discussion-title" className="text-xs font-medium text-muted-foreground">
            {t("discussion.titleLabel")}
          </label>
          <input
            id="discussion-title"
            ref={titleInputRef}
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent aria-invalid:border-destructive aria-invalid:ring-2 aria-invalid:ring-destructive/20"
            placeholder={t("discussion.titlePlaceholder")}
            aria-invalid={titleInvalid}
            aria-describedby={titleInvalid ? "discussion-title-error" : undefined}
          />
          {titleInvalid && (
            <p id="discussion-title-error" role="alert" className="text-xs text-destructive">
              {t("discussion.titleRequired")}
            </p>
          )}
        </div>
        <div className="space-y-1">
          <label htmlFor="discussion-body" className="text-xs font-medium text-muted-foreground">
            {t("discussion.bodyLabel")}
          </label>
          <textarea
            id="discussion-body"
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={8}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
            placeholder={t("discussion.bodyPlaceholder")}
          />
        </div>
        <Button type="submit" size="sm" className="[@media(pointer:coarse)]:min-h-11" disabled={busy}>
          {busy ? t("common.submitting") : t("common.submit")}
        </Button>
      </form>
    </div>
  );
}
