"use client";

import { Suspense, useState, useEffect } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { Button } from "@/components/ui/button";
import { Field, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import CaptchaWidget from "@/components/verification/CaptchaWidget";

function ForgotPasswordContent() {
  const t = useTranslations();
  const [email, setEmail] = useState("");
  const [captchaToken, setCaptchaToken] = useState("");
  const [captchaResetKey, setCaptchaResetKey] = useState(0);
  const [isLoading, setIsLoading] = useState(false);
  const [sent, setSent] = useState(false);
  const [cooldown, setCooldown] = useState(0);
  const [error, setError] = useState("");

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setTimeout(() => setCooldown(cooldown - 1), 1000);
    return () => clearTimeout(timer);
  }, [cooldown]);

  function resetCaptcha() {
    setCaptchaToken("");
    setCaptchaResetKey((key) => key + 1);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setIsLoading(true);
    let submittedCaptcha = false;
    try {
      submittedCaptcha = !!captchaToken;
      await api.post("/api/v1/auth/forgot-password", {
        email: email.trim(),
        captcha_token: captchaToken || undefined,
      });
      setSent(true);
      setCooldown(60);
    } catch (err) {
      if (submittedCaptcha) {
        resetCaptcha();
      }
      silentError(err, { component: "ForgotPasswordPage", action: "handleSubmit" });
      setError(t(getUserFacingErrorKey(err)));
    } finally {
      setIsLoading(false);
    }
  }

  if (sent) {
    return (
      <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4 py-12">
        <div className="w-full max-w-sm text-center">
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth.checkEmail")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{t("auth.resetLinkSent")}</p>
          {cooldown > 0 && (
            <p className="mt-2 text-xs text-muted-foreground">
              {t("auth.resendCooldown", { seconds: cooldown })}
            </p>
          )}
          <div className="mt-6">
            <Link href="/login" className="font-medium text-primary hover:underline">
              {t("auth.backToLogin")}
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4 py-12">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center gap-2">
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth.forgotPassword")}</h1>
          <p className="text-sm text-muted-foreground">{t("auth.forgotPasswordHint")}</p>
        </div>

        <div className="rounded-lg border border-border bg-card p-6">
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <Field>
              <FieldLabel htmlFor="email">{t("auth.email")}</FieldLabel>
              <Input
                id="email"
                type="email"
                placeholder="you@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                disabled={isLoading}
                aria-invalid={Boolean(error)}
                aria-describedby={error ? "email-error" : undefined}
              />
              {error && <FieldError id="email-error">{error}</FieldError>}
            </Field>

            <CaptchaWidget key={captchaResetKey} onToken={setCaptchaToken} />

            <Button type="submit" className="w-full" disabled={isLoading}>
              {isLoading ? t("common.processing") : t("auth.sendResetLink")}
            </Button>
          </form>
        </div>

        <p className="mt-4 text-center text-sm text-muted-foreground">
          <Link href="/login" className="font-medium text-primary hover:underline">
            {t("auth.backToLogin")}
          </Link>
        </p>
      </div>
    </div>
  );
}

export default function ForgotPasswordPage() {
  return (
    <Suspense
      fallback={<div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center" />}
    >
      <ForgotPasswordContent />
    </Suspense>
  );
}
