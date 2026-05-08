"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";

export default function NewDiscussionPage() {
  const t = useTranslations();
  const router = useRouter();
  const params = useParams<{ ipId: string }>();
  const ipId = parseInt(params.ipId, 10);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit() {
    if (!title.trim()) return;
    setBusy(true);
    setError("");
    try {
      const res = await api.post<{ discussion?: { id: number } }>(
        `/api/v1/ips/${ipId}/discussions`,
        { title: title.trim(), body: body.trim() },
      );
      const discId = res.discussion?.id;
      if (discId) router.push(`/ip/${ipId}/discussions/${discId}`);
      else router.push(`/ip/${ipId}/discussions`);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t("common.operationFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto w-full max-w-2xl space-y-6 px-4 py-6">
      <h1 className="text-2xl font-bold tracking-tight">{t("discussion.newPost")}</h1>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="space-y-4 rounded-md border border-border bg-card p-4 ">
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("discussion.titleLabel")}</label>
          <input
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
            placeholder={t("discussion.titlePlaceholder")}
          />
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("discussion.bodyLabel")}</label>
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={8}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
            placeholder={t("discussion.bodyPlaceholder")}
          />
        </div>
        <Button size="sm" onClick={handleSubmit} disabled={busy || !title.trim()}>
          {busy ? t("common.submitting") : t("common.submit")}
        </Button>
      </div>
    </div>
  );
}
