"use client";

import { Suspense, useState, FormEvent } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Brush } from "lucide-react";

function ResetPasswordContent() {
  const t = useTranslations();
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get("token") || "";

  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (newPassword !== confirmPassword) {
      setError(t("auth.passwordMismatch"));
      return;
    }
    if (newPassword.length < 6) {
      setError(t("auth.passwordTooShort"));
      return;
    }
    if (!token) {
      setError(t("auth.invalidResetToken"));
      return;
    }
    setIsLoading(true);
    try {
      await api.post("/api/v1/auth/reset-password", { token, new_password: newPassword });
      setSuccess(true);
    } catch (err) {
      silentError(err, { component: "ResetPasswordPage", action: "handleSubmit" });
      setError(err instanceof ApiRequestError ? err.message : t("common.operationFailed"));
    } finally {
      setIsLoading(false);
    }
  }

  if (success) {
    return (
      <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4 py-12">
        <div className="w-full max-w-sm text-center">
          <div className="mb-8 flex flex-col items-center gap-2">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10">
              <Brush className="h-6 w-6 text-primary" />
            </div>
            <h1 className="text-2xl font-semibold tracking-tight">{t("auth.passwordResetSuccess")}</h1>
          </div>
          <div className="rounded-lg border border-border bg-card p-6 shadow-sm">
            <p className="text-sm text-muted-foreground">{t("auth.passwordResetSuccessHint")}</p>
          </div>
          <p className="mt-4 text-sm text-muted-foreground">
            <Link href="/login" className="font-medium text-primary hover:underline">
              {t("auth.backToLogin")}
            </Link>
          </p>
        </div>
      </div>
    );
  }

  if (!token) {
    return (
      <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4 py-12">
        <div className="w-full max-w-sm text-center">
          <div className="rounded-lg border border-border bg-card p-6 shadow-sm">
            <p className="text-sm text-muted-foreground">{t("auth.invalidResetToken")}</p>
          </div>
          <p className="mt-4 text-sm text-muted-foreground">
            <Link href="/forgot-password" className="font-medium text-primary hover:underline">
              {t("auth.requestNewLink")}
            </Link>
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4 py-12">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center gap-2">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10">
            <Brush className="h-6 w-6 text-primary" />
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth.resetPassword")}</h1>
          <p className="text-sm text-muted-foreground">{t("auth.resetPasswordHint")}</p>
        </div>

        <div className="rounded-lg border border-border bg-card p-6 shadow-sm">
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="newPassword">{t("auth.newPassword")}</Label>
              <Input
                id="newPassword"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                required
                disabled={isLoading}
                minLength={6}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="confirmPassword">{t("auth.confirmPassword")}</Label>
              <Input
                id="confirmPassword"
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                required
                disabled={isLoading}
                minLength={6}
              />
            </div>
            {error && <p className="text-sm text-destructive" role="alert">{error}</p>}
            <Button type="submit" className="w-full" disabled={isLoading}>
              {isLoading ? t("common.processing") : t("auth.resetPassword")}
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

export default function ResetPasswordPage() {
  return (
    <Suspense
      fallback={<div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center" />}
    >
      <ResetPasswordContent />
    </Suspense>
  );
}