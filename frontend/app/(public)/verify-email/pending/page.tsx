"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import CaptchaWidget from "@/components/verification/CaptchaWidget";

function PendingContent() {
  const t = useTranslations();
  const searchParams = useSearchParams();
  const maskedEmail = searchParams.get("email") || "";

  const [email, setEmail] = useState("");
  const [captchaToken, setCaptchaToken] = useState("");
  const [cooldown, setCooldown] = useState(0);
  const [isLoading, setIsLoading] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setTimeout(() => setCooldown(cooldown - 1), 1000);
    return () => clearTimeout(timer);
  }, [cooldown]);

  const handleResend = useCallback(async () => {
    const targetEmail = email.trim();
    if (!targetEmail) {
      setError(t("auth.errorEmailRequired"));
      return;
    }
    setError("");
    setIsLoading(true);
    try {
      await api.post("/api/v1/auth/resend-verification", {
        email: targetEmail,
        captcha_token: captchaToken || undefined,
      });
      setMessage(t("auth.verificationResent"));
      setCooldown(60);
    } catch (err) {
      silentError(err, { component: "VerifyEmailPending", action: "handleResend" });
      setError(err instanceof ApiRequestError ? err.message : t("common.operationFailed"));
    } finally {
      setIsLoading(false);
    }
  }, [email, captchaToken, t]);

  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4 py-12">
      <div className="w-full max-w-sm text-center">
        <div className="mb-8 flex flex-col items-center gap-2">
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth.verifyYourEmail")}</h1>
          <p className="text-sm text-muted-foreground">
            {t("auth.verificationSentTo")} <span className="font-medium">{maskedEmail}</span>
          </p>
        </div>

        <div className="rounded-lg border border-border bg-card p-6">
          <p className="mb-4 text-sm text-muted-foreground">{t("auth.verificationPendingHint")}</p>

          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1.5 text-left">
              <Label htmlFor="resend-email">{t("auth.resendToEmail")}</Label>
              <Input
                id="resend-email"
                type="email"
                placeholder="you@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                disabled={isLoading}
              />
            </div>

            <CaptchaWidget onToken={setCaptchaToken} />

            {error && <p className="text-sm text-destructive" role="alert">{error}</p>}
            {message && <p className="text-sm text-primary">{message}</p>}

            <Button
              onClick={handleResend}
              className="w-full"
              disabled={isLoading || cooldown > 0}
            >
              {cooldown > 0
                ? t("auth.resendCooldown", { seconds: cooldown })
                : isLoading
                  ? t("common.processing")
                  : t("auth.resendVerification")}
            </Button>
          </div>
        </div>

        <div className="mt-4 flex flex-col gap-2 text-sm text-muted-foreground">
          <Link href="/login" className="font-medium text-primary hover:underline">
            {t("auth.backToLogin")}
          </Link>
          <Link href="/register" className="font-medium text-primary hover:underline">
            {t("auth.backToRegister")}
          </Link>
        </div>
      </div>
    </div>
  );
}

export default function VerifyEmailPendingPage() {
  return (
    <Suspense
      fallback={<div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center" />}
    >
      <PendingContent />
    </Suspense>
  );
}
