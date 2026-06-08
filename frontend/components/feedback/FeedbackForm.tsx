"use client";

import { useState, useEffect } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import CaptchaWidget from "@/components/verification/CaptchaWidget";
import { Check, Upload, X, Loader2 } from "lucide-react";

const CATEGORIES = [
  "web_bug",
  "desktop_deploy",
  "content_or_community",
  "account_or_security",
  "agent_quality",
  "feature_request",
  "other",
] as const;

interface FeedbackFormProps {
  onSuccess?: (ticketId: number) => void;
}

export default function FeedbackForm({ onSuccess }: FeedbackFormProps) {
  const t = useTranslations();
  const { user } = useAuth();

  const [category, setCategory] = useState<string>("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [contactEmail, setContactEmail] = useState(user?.email || "");
  const [captchaToken, setCaptchaToken] = useState<string | null>(null);
  const [captchaResetKey, setCaptchaResetKey] = useState(0);
  const [includeDiagnostics, setIncludeDiagnostics] = useState(false);
  const [screenshotFile, setScreenshotFile] = useState<File | null>(null);
  const [screenshotUploading, setScreenshotUploading] = useState(false);
  const [screenshotGrant, setScreenshotGrant] = useState<{ grant_id: string; oss_key: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);

  useEffect(() => {
    if (user?.email) {
      setContactEmail(user.email);
    }
  }, [user]);

  function resetAnonymousCaptcha() {
    if (!user) {
      setCaptchaToken(null);
      setCaptchaResetKey((key) => key + 1);
    }
  }

  async function handleScreenshotSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    if (file.size > 20 * 1024 * 1024) {
      setError(t("feedback.screenshotTooLarge"));
      return;
    }
    setScreenshotFile(file);
    setScreenshotGrant(null);
    setError("");

    setScreenshotUploading(true);
    try {
      const presignRes = (await api.post("/api/v1/feedback/attachments/presign", {
        file_name: file.name,
        mime_type: file.type,
        size_bytes: file.size,
        captcha_token: user ? undefined : captchaToken,
      })) as { grant_id: string; oss_key: string; upload_url: string };
      resetAnonymousCaptcha();

      const uploadRes = await fetch(presignRes.upload_url, {
        method: "PUT",
        body: file,
        headers: { "Content-Type": file.type },
      });
      if (!uploadRes.ok) {
        throw new Error("feedback screenshot upload failed");
      }

      setScreenshotGrant({ grant_id: presignRes.grant_id, oss_key: presignRes.oss_key });
    } catch (e) {
      silentError(e, { component: "FeedbackForm", action: "handleScreenshotSelect" });
      setError(e instanceof ApiRequestError ? e.message : t("feedback.screenshotUploadFailed"));
      setScreenshotFile(null);
      resetAnonymousCaptcha();
    } finally {
      setScreenshotUploading(false);
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    if (!category) {
      setError(t("feedback.categoryRequired"));
      return;
    }
    if (!title.trim() || !description.trim()) {
      setError(t("feedback.titleAndDescriptionRequired"));
      return;
    }
    if (!user && !contactEmail.trim()) {
      setError(t("feedback.contactEmailRequired"));
      return;
    }
    if (!user && !captchaToken) {
      setError(t("feedback.captchaRequired"));
      return;
    }

    setBusy(true);
    try {
      const diagnosticSummary: Record<string, string> = {};
      if (includeDiagnostics) {
        diagnosticSummary.app_version = "web-beta";
        diagnosticSummary.platform = navigator.userAgent;
        diagnosticSummary.route = window.location.pathname;
      }

      const payload: Record<string, unknown> = {
        category,
        title: title.trim(),
        description: description.trim(),
        diagnostic_summary: diagnosticSummary,
        captcha_token: user ? undefined : captchaToken,
        contact_email: user ? undefined : contactEmail.trim(),
        attachment_grants: screenshotGrant ? [screenshotGrant] : [],
      };

      const res = (await api.post("/api/v1/feedback", payload)) as { id: number };
      setSuccess(true);
      onSuccess?.(res.id);
    } catch (e) {
      silentError(e, { component: "FeedbackForm", action: "handleSubmit" });
      setError(e instanceof ApiRequestError ? e.message : t("common.operationFailed"));
    } finally {
      setBusy(false);
    }
  }

  if (success) {
    return (
      <div className="rounded-lg border border-emerald-200 bg-emerald-50/30 p-6 text-center dark:border-emerald-900/30 dark:bg-emerald-950/10">
        <p className="text-sm font-medium text-emerald-700 dark:text-emerald-400">{t("feedback.submitSuccess")}</p>
        <p className="mt-1 text-xs text-muted-foreground">{t("feedback.submitSuccessDesc")}</p>
      </div>
    );
  }

  return (
    <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="space-y-1">
        <label className="text-xs font-medium text-muted-foreground">{t("feedback.category")}</label>
        <select
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
          required
        >
          <option value="">{t("feedback.selectCategory")}</option>
          {CATEGORIES.map((cat) => (
            <option key={cat} value={cat}>
              {t(`feedback.cat_${cat}`)}
            </option>
          ))}
        </select>
      </div>

      <div className="space-y-1">
        <label className="text-xs font-medium text-muted-foreground">{t("feedback.title")}</label>
        <Input
          type="text"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          maxLength={160}
          required
        />
      </div>

      <div className="space-y-1">
        <label className="text-xs font-medium text-muted-foreground">{t("feedback.description")}</label>
        <Textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={5}
          required
        />
      </div>

      {!user && (
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("feedback.contactEmail")}</label>
          <Input
            type="email"
            value={contactEmail}
            onChange={(e) => setContactEmail(e.target.value)}
            required
          />
        </div>
      )}

      <div className="space-y-1">
        <label className="text-xs font-medium text-muted-foreground">{t("feedback.screenshot")}</label>
        <div className="flex items-center gap-2">
          {screenshotFile ? (
            <div className="flex items-center gap-2 rounded-md border border-border bg-canvas-subtle px-3 py-1.5 text-sm">
              <span className="max-w-[200px] truncate">{screenshotFile.name}</span>
              {screenshotUploading ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />
              ) : screenshotGrant ? (
                <Check className="h-3.5 w-3.5 text-emerald-600" aria-hidden="true" />
              ) : null}
              <button
                type="button"
                onClick={() => { setScreenshotFile(null); setScreenshotGrant(null); }}
                className="text-muted-foreground hover:text-foreground"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          ) : (
            <Button type="button" size="sm" variant="outline" onClick={() => document.getElementById("feedback-screenshot")?.click()}>
              <Upload className="mr-1 h-3.5 w-3.5" />
              {t("feedback.uploadScreenshot")}
            </Button>
          )}
          <input
            id="feedback-screenshot"
            type="file"
            accept="image/*"
            className="hidden"
            onChange={(e) => void handleScreenshotSelect(e)}
          />
        </div>
        <p className="text-[11px] text-muted-foreground">{t("feedback.screenshotHint")}</p>
      </div>

      <label className="flex items-center gap-2 text-xs text-muted-foreground">
        <input
          type="checkbox"
          checked={includeDiagnostics}
          onChange={(e) => setIncludeDiagnostics(e.target.checked)}
          className="h-3.5 w-3.5"
        />
        {t("feedback.includeDiagnostics")}
      </label>
      {includeDiagnostics && (
        <p className="text-[11px] text-muted-foreground">{t("feedback.diagnosticsHint")}</p>
      )}

      {!user && (
        <div className="space-y-2">
          <CaptchaWidget key={captchaResetKey} onToken={setCaptchaToken} onError={() => setCaptchaToken(null)} />
        </div>
      )}

      <Button type="submit" size="sm" disabled={busy || (!user && !captchaToken)}>
        {busy ? t("common.processing") : t("feedback.submit")}
      </Button>
    </form>
  );
}
