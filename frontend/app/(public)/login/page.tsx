"use client";

import { Suspense, useState, FormEvent } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Brush, Eye, EyeOff, Loader2 } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { ApiRequestError } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { useTranslations } from "next-intl";
import { silentError } from "@/lib/error-handler";

function LoginPageContent() {
  const t = useTranslations();
  const router = useRouter();
  const searchParams = useSearchParams();
  const { login } = useAuth();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [rememberMe, setRememberMe] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  // T29（FIX-15）：USER_BANNED 登录拒绝时展示申诉/工单指引（产品内唯一出路入口）。
  const [bannedGuidance, setBannedGuidance] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBannedGuidance(false);
    setIsLoading(true);

    try {
      await login(email, password, rememberMe);
      const redirect = searchParams.get("redirect") || "/";
      router.push(redirect);
    } catch (err) {
      silentError(err, { component: "LoginPage", action: "handleSubmit" });
      if (err instanceof ApiRequestError && err.code === "USER_BANNED") {
        setBannedGuidance(true);
      }
      setError(t(getUserFacingErrorKey(err, "auth.errorLoginFailed")));
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4 py-12">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center gap-2">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10">
            <Brush className="h-6 w-6 text-primary" />
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth.welcomeBack")}</h1>
          <p className="text-sm text-muted-foreground">{t("auth.loginTitle")}</p>
        </div>

        <div className="rounded-lg border border-border bg-card p-6">
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <Field>
              <FieldLabel htmlFor="email">{t("auth.email")}</FieldLabel>
              <Input
                id="email"
                type="email"
                placeholder="you@example.com"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                disabled={isLoading}
              />
            </Field>

            <Field>
              <FieldLabel htmlFor="password">{t("auth.password")}</FieldLabel>
              <div className="relative">
                <Input
                  id="password"
                  type={showPassword ? "text" : "password"}
                  placeholder={t("auth.passwordPlaceholder")}
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  disabled={isLoading}
                  className="pr-10"
                />
                <button
                  type="button"
                  aria-label={showPassword ? t("auth.hidePassword") : t("auth.showPassword")}
                  aria-pressed={showPassword}
                  className="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                  onClick={() => setShowPassword(!showPassword)}
                >
                  {showPassword ? (
                    <EyeOff className="h-4 w-4" />
                  ) : (
                    <Eye className="h-4 w-4" />
                  )}
                </button>
              </div>
            </Field>

            <label className="flex cursor-pointer items-center gap-2 text-sm text-muted-foreground">
              <Checkbox
                checked={rememberMe}
                onChange={(e) => setRememberMe(e.target.checked)}
                className="h-3.5 w-3.5 rounded border-border accent-primary"
              />
              {t("auth.rememberMe")}
            </label>

            {error && (
              <div className="space-y-2" role="alert">
                <p className="text-sm text-destructive">{error}</p>
                {bannedGuidance && (
                  <p className="text-sm text-muted-foreground">
                    {t("auth.bannedFeedbackHint")}{" "}
                    <Link
                      href="/feedback"
                      className="font-medium text-primary hover:underline"
                    >
                      {t("auth.bannedFeedbackEntry")}
                    </Link>
                  </p>
                )}
              </div>
            )}

            <Button type="submit" className="mt-1 w-full" disabled={isLoading}>
              {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("auth.loginButton")}
            </Button>
          </form>
        </div>

        <p className="mt-4 text-center text-sm text-muted-foreground">
          {t("auth.noAccount")}{" "}
          <Link href="/register" className="font-medium text-primary hover:underline">
            {t("auth.registerNow")}
          </Link>
        </p>
        <p className="mt-2 text-center text-sm text-muted-foreground">
          <Link href="/forgot-password" className="font-medium text-primary hover:underline">
            {t("auth.forgotPassword")}
          </Link>
        </p>
      </div>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={<div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center" />}>
      <LoginPageContent />
    </Suspense>
  );
}
