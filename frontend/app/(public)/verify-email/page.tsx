"use client";

import { Suspense, useState, useEffect } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { Loader2 } from "lucide-react";

function VerifyEmailContent() {
  const t = useTranslations();
  const searchParams = useSearchParams();
  const token = searchParams.get("token") || "";

  const [isLoading, setIsLoading] = useState(true);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token) {
      setError(t("auth.invalidVerifyToken"));
      setIsLoading(false);
      return;
    }

    async function verify() {
      try {
        await api.post("/api/v1/auth/verify-email", { token });
        setSuccess(true);
      } catch (err) {
        silentError(err, { component: "VerifyEmailPage", action: "autoVerify" });
        setError(err instanceof ApiRequestError ? err.message : t("common.operationFailed"));
      } finally {
        setIsLoading(false);
      }
    }

    verify();
  }, [token, t]);

  if (isLoading) {
    return (
      <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4 py-12">
        <div className="flex flex-col items-center gap-3">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
          <p className="text-sm text-muted-foreground">{t("auth.verifyingEmail")}</p>
        </div>
      </div>
    );
  }

  if (success) {
    return (
      <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4 py-12">
        <div className="w-full max-w-sm text-center">
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth.verifyEmailSuccess")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{t("auth.verifyEmailSuccessHint")}</p>
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
      <div className="w-full max-w-sm text-center">
        <h1 className="text-2xl font-semibold tracking-tight">{t("auth.verifyEmailFailed")}</h1>
        <p className="mt-2 text-sm text-destructive">{error || t("auth.invalidVerifyToken")}</p>
        <div className="mt-6 flex flex-col gap-2">
          <Link href="/verify-email/pending" className="font-medium text-primary hover:underline">
            {t("auth.resendVerification")}
          </Link>
          <Link href="/login" className="font-medium text-primary hover:underline">
            {t("auth.backToLogin")}
          </Link>
        </div>
      </div>
    </div>
  );
}

export default function VerifyEmailPage() {
  return (
    <Suspense
      fallback={<div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center" />}
    >
      <VerifyEmailContent />
    </Suspense>
  );
}
