"use client";

import { useState, FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Eye, EyeOff, Loader2 } from "lucide-react";
import { api, ApiRequestError } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useTranslations } from "next-intl";
import { silentError } from "@/lib/error-handler";
import { fetchPublicConfig } from "@/lib/public-config";
import CaptchaWidget from "@/components/verification/CaptchaWidget";

interface RegisterResponse {
  user: {
    id: number;
    email: string;
    username: string;
  };
  verification_required: boolean;
}

export default function RegisterPage() {
  const t = useTranslations();
  const router = useRouter();
  const { user } = useAuth();

  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [acceptedTerms, setAcceptedTerms] = useState(false);
  const [acceptedPrivacy, setAcceptedPrivacy] = useState(false);
  const [captchaToken, setCaptchaToken] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  if (user) {
    router.replace("/");
    return null;
  }

  function validate(): boolean {
    const newErrors: Record<string, string> = {};
    if (username.length < 2) newErrors.username = t("auth.errorUsernameTooShort");
    if (username.length > 64) newErrors.username = t("auth.errorUsernameTooLong");
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) newErrors.email = t("auth.errorInvalidEmail");
    if (password.length < 8) newErrors.password = t("auth.errorPasswordTooShort");
    if (password !== confirmPassword) newErrors.confirmPassword = t("auth.errorPasswordMismatch");
    if (!acceptedTerms) newErrors.terms = t("auth.errorTermsRequired");
    if (!acceptedPrivacy) newErrors.privacy = t("auth.errorPrivacyRequired");
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!validate()) return;

    setErrors({});
    setIsLoading(true);

    try {
      const config = await fetchPublicConfig();
      const data = await api.post<RegisterResponse>("/api/v1/auth/register", {
        username,
        email,
        password,
        captcha_token: captchaToken || undefined,
        accepted_terms_version: config.legal.current_terms_version || undefined,
        accepted_privacy_version: config.legal.current_privacy_version || undefined,
      });

      if (data.verification_required) {
        const masked = email.replace(/^(.{2})(.*)(@.*)$/, (_, a, b, c) => a + "*".repeat(Math.min(b.length, 4)) + c);
        router.push(`/verify-email/pending?email=${encodeURIComponent(masked)}`);
      } else {
        router.push("/login");
      }
    } catch (err) {
      silentError(err, { component: "RegisterPage", action: "handleSubmit" });
      if (err instanceof ApiRequestError) {
        if (err.code === "USER_EXISTS") {
          setErrors({ email: t("auth.errorEmailTaken") });
        } else if (err.code === "USERNAME_TAKEN") {
          setErrors({ username: t("auth.errorUsernameTaken") });
        } else if (err.code === "TERMS_VERSION_MISMATCH") {
          setErrors({ terms: t("auth.errorTermsVersionMismatch") });
        } else if (err.code === "PRIVACY_VERSION_MISMATCH") {
          setErrors({ privacy: t("auth.errorPrivacyVersionMismatch") });
        } else {
          setErrors({ general: err.message || t("auth.errorRegisterFailed") });
        }
      } else {
        setErrors({ general: t("auth.errorNetwork") });
      }
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4 py-12">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center gap-2">
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth.joinTitle")}</h1>
          <p className="text-sm text-muted-foreground">{t("auth.registerSubtitle")}</p>
        </div>

        <div className="rounded-lg border border-border bg-card p-6">
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="username">{t("auth.username")}</Label>
              <Input
                id="username"
                type="text"
                placeholder={t("auth.displayName")}
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                disabled={isLoading}
                className={errors.username ? "border-destructive" : ""}
              />
              {errors.username && <p className="text-xs text-destructive">{errors.username}</p>}
            </div>

            <div className="flex flex-col gap-1.5">
              <Label htmlFor="email">{t("auth.email")}</Label>
              <Input
                id="email"
                type="email"
                placeholder="you@example.com"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                disabled={isLoading}
                className={errors.email ? "border-destructive" : ""}
              />
              {errors.email && <p className="text-xs text-destructive">{errors.email}</p>}
            </div>

            <div className="flex flex-col gap-1.5">
              <Label htmlFor="password">{t("auth.password")}</Label>
              <div className="relative">
                <Input
                  id="password"
                  type={showPassword ? "text" : "password"}
                  placeholder={t("auth.passwordMinLength")}
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  disabled={isLoading}
                  className={`pr-10 ${errors.password ? "border-destructive" : ""}`}
                />
                <button
                  type="button"
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                  onClick={() => setShowPassword(!showPassword)}
                  tabIndex={-1}
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
              {errors.password && <p className="text-xs text-destructive">{errors.password}</p>}
            </div>

            <div className="flex flex-col gap-1.5">
              <Label htmlFor="confirmPassword">{t("auth.confirmPassword")}</Label>
              <Input
                id="confirmPassword"
                type={showPassword ? "text" : "password"}
                placeholder={t("auth.confirmPasswordPlaceholder")}
                autoComplete="new-password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                required
                disabled={isLoading}
                className={errors.confirmPassword ? "border-destructive" : ""}
              />
              {errors.confirmPassword && <p className="text-xs text-destructive">{errors.confirmPassword}</p>}
            </div>

            <CaptchaWidget onToken={setCaptchaToken} />

            <label className="flex items-start gap-2 text-sm">
              <input
                type="checkbox"
                checked={acceptedTerms}
                onChange={(e) => setAcceptedTerms(e.target.checked)}
                className="mt-0.5"
                disabled={isLoading}
              />
              <span>
                {t("auth.acceptTerms")}{" "}
                <Link href="/terms" className="text-primary hover:underline" target="_blank">
                  {t("auth.termsOfService")}
                </Link>
              </span>
            </label>
            {errors.terms && <p className="text-xs text-destructive">{errors.terms}</p>}

            <label className="flex items-start gap-2 text-sm">
              <input
                type="checkbox"
                checked={acceptedPrivacy}
                onChange={(e) => setAcceptedPrivacy(e.target.checked)}
                className="mt-0.5"
                disabled={isLoading}
              />
              <span>
                {t("auth.acceptPrivacy")}{" "}
                <Link href="/privacy" className="text-primary hover:underline" target="_blank">
                  {t("auth.privacyPolicy")}
                </Link>
              </span>
            </label>
            {errors.privacy && <p className="text-xs text-destructive">{errors.privacy}</p>}

            {errors.general && (
              <p className="text-sm text-destructive" role="alert">{errors.general}</p>
            )}

            <Button type="submit" className="w-full mt-1" disabled={isLoading}>
              {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("auth.createAccount")}
            </Button>
          </form>
        </div>

        <p className="mt-4 text-center text-sm text-muted-foreground">
          {t("auth.hasAccount")}{" "}
          <Link href="/login" className="font-medium text-primary hover:underline">
            {t("auth.loginNow")}
          </Link>
        </p>
      </div>
    </div>
  );
}
