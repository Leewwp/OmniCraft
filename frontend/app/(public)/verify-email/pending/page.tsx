"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { Button } from "@/components/ui/button";
import { Loader2 } from "lucide-react";

function PendingContent() {
  const t = useTranslations();
  const searchParams = useSearchParams();
  const email = searchParams.get("email") || "";
  const maskedEmail = searchParams.get("masked") || email;

  const [cooldown, setCooldown] = useState(60);
  const [isLoading, setIsLoading] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setTimeout(() => setCooldown(cooldown - 1), 1000);
    return () => clearTimeout(timer);
  }, [cooldown]);

  const handleResend = useCallback(async () => {
    if (!email) {
      setError(t("auth.errorEmailRequired"));
      return;
    }
    setError("");
    setMessage("");
    setIsLoading(true);
    try {
      await api.post("/api/v1/auth/resend-verification", {
        email,
      });
      setMessage(t("auth.verificationResent"));
      setCooldown(60);
    } catch (err) {
      silentError(err, { component: "VerifyEmailPending", action: "handleResend" });
      setError(t(getUserFacingErrorKey(err)));
    } finally {
      setIsLoading(false);
    }
  }, [email, t]);

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
            {error && (
              <p className="text-sm text-destructive" role="alert">
                {error}
              </p>
            )}
            {message && <p className="text-sm text-primary">{message}</p>}

            <Button onClick={handleResend} className="w-full" disabled={isLoading || cooldown > 0}>
              {isLoading ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  {t("common.processing")}
                </>
              ) : cooldown > 0 ? (
                t("auth.resendCooldown", { seconds: cooldown })
              ) : (
                t("auth.resendVerification")
              )}
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
    <Suspense fallback={<div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center" />}>
      <PendingContent />
    </Suspense>
  );
}
