"use client";

import { Suspense, useState, FormEvent } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { Button } from "@/components/ui/button";
import { Field, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

function getPasswordStrength(password: string): { label: string; color: string } {
  if (password.length < 8) return { label: "weak", color: "bg-destructive" };
  const hasUpper = /[A-Z]/.test(password);
  const hasLower = /[a-z]/.test(password);
  const hasDigit = /\d/.test(password);
  const hasSpecial = /[^A-Za-z0-9]/.test(password);
  const score = [hasUpper, hasLower, hasDigit, hasSpecial].filter(Boolean).length;
  if (score >= 3 && password.length >= 10) return { label: "strong", color: "bg-primary" };
  if (score >= 2) return { label: "medium", color: "bg-yellow-500" };
  return { label: "weak", color: "bg-destructive" };
}

function ResetPasswordContent() {
  const t = useTranslations();
  const searchParams = useSearchParams();
  const token = searchParams.get("token") || "";

  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);

  const strength = getPasswordStrength(newPassword);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (newPassword !== confirmPassword) {
      // #380/F-A006: auth 域只有 errorPasswordMismatch——曾引用不存在的
      // auth.passwordMismatch，把原始键名当错误文案展示给用户。
      setError(t("auth.errorPasswordMismatch"));
      return;
    }
    if (newPassword.length < 8) {
      setError(t("auth.errorPasswordTooShort"));
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
      if (err instanceof ApiRequestError) {
        if (err.code === "PASSWORD_TOO_SHORT") {
          setError(t("auth.errorPasswordTooShort"));
        } else if (err.code === "INVALID_TOKEN") {
          setError(t("auth.invalidResetToken"));
        } else {
          setError(t(getUserFacingErrorKey(err)));
        }
      } else {
        setError(t(getUserFacingErrorKey(err)));
      }
    } finally {
      setIsLoading(false);
    }
  }

  if (success) {
    return (
      <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4 py-12">
        <div className="w-full max-w-sm text-center">
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth.passwordResetSuccess")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{t("auth.passwordResetSuccessHint")}</p>
          <div className="mt-6">
            <Link href="/login" className="font-medium text-primary hover:underline">
              {t("auth.backToLogin")}
            </Link>
          </div>
        </div>
      </div>
    );
  }

  if (!token) {
    return (
      <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4 py-12">
        <div className="w-full max-w-sm text-center">
          <p className="text-sm text-muted-foreground">{t("auth.invalidResetToken")}</p>
          <div className="mt-4">
            <Link href="/forgot-password" className="font-medium text-primary hover:underline">
              {t("auth.requestNewLink")}
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
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth.resetPassword")}</h1>
          <p className="text-sm text-muted-foreground">{t("auth.resetPasswordHint")}</p>
        </div>

        <div className="rounded-lg border border-border bg-card p-6">
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <Field>
              <FieldLabel htmlFor="newPassword">{t("auth.newPassword")}</FieldLabel>
              <Input
                id="newPassword"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                required
                disabled={isLoading}
                minLength={8}
                aria-invalid={Boolean(error)}
                aria-describedby={error ? "reset-password-error" : undefined}
              />
              {newPassword.length > 0 && (
                <div className="flex items-center gap-2">
                  <div className="h-1.5 flex-1 rounded-full bg-muted overflow-hidden">
                    <div className={`h-full rounded-full transition-all ${strength.color}`} style={{ width: strength.label === "weak" ? "33%" : strength.label === "medium" ? "66%" : "100%" }} />
                  </div>
                  <span className="text-xs text-muted-foreground">
                    {t(`auth.strength_${strength.label}`)}
                  </span>
                </div>
              )}
            </Field>
            <Field>
              <FieldLabel htmlFor="confirmPassword">{t("auth.confirmPassword")}</FieldLabel>
              <Input
                id="confirmPassword"
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                required
                disabled={isLoading}
                minLength={8}
                aria-invalid={Boolean(error)}
                aria-describedby={error ? "reset-password-error" : undefined}
              />
            </Field>
            {error && <FieldError id="reset-password-error" className="text-sm">{error}</FieldError>}
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
