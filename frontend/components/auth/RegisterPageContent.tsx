"use client";

import { useState, FormEvent, type ComponentType } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Eye, EyeOff, Loader2 } from "lucide-react";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { useTranslations } from "next-intl";
import { silentError } from "@/lib/error-handler";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { fetchPublicConfig } from "@/lib/public-config";
import { CaptchaWidget, type CaptchaWidgetProps } from "@/components/verification/CaptchaWidget";
import type { User } from "@/contexts/AuthContext";

interface RegisterResponse {
  user: {
    id: number;
    email: string;
    username: string;
  };
  verification_required: boolean;
}
const REGISTER_CAPTCHA_CONTAINER_ID = "register-captcha-container";
const REGISTER_SUBMIT_BUTTON_ID = "register-submit-button";
const REGISTER_CAPTCHA_BUTTON_ID = "register-captcha-button";

interface RegisterPageContentProps {
  user: User | null;
  router: Pick<ReturnType<typeof useRouter>, "push" | "replace">;
  CaptchaComponent?: ComponentType<CaptchaWidgetProps>;
}

export function RegisterPageContent({
  user,
  router,
  CaptchaComponent = CaptchaWidget,
}: RegisterPageContentProps) {
  const t = useTranslations();
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [acceptedTerms, setAcceptedTerms] = useState(false);
  const [acceptedPrivacy, setAcceptedPrivacy] = useState(false);
  const [captchaToken, setCaptchaToken] = useState("");
  const [captchaResetKey, setCaptchaResetKey] = useState(0);
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

  function resetCaptcha() {
    setCaptchaToken("");
    setCaptchaResetKey((key) => key + 1);
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!validate()) return;

    setErrors({});
    setIsLoading(true);
    let submittedCaptcha = false;

    try {
      const config = await fetchPublicConfig();
      if (config.captcha.provider === "aliyun_v2" && !captchaToken) {
        setErrors({ captcha: t("auth.errorCaptchaRequired") });
        return;
      }

      submittedCaptcha = !!captchaToken;
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
        router.push(`/verify-email/pending?email=${encodeURIComponent(email)}&masked=${encodeURIComponent(masked)}`);
      } else {
        router.push("/login");
      }
    } catch (err) {
      if (submittedCaptcha) {
        resetCaptcha();
      }
      silentError(err, { component: "RegisterPage", action: "handleSubmit" });
      if (err instanceof ApiRequestError) {
        if (err.code === "USER_EXISTS") {
          setErrors({ email: t("auth.errorEmailTaken") });
        } else if (err.code === "USERNAME_TAKEN") {
          setErrors({ username: t("auth.errorUsernameTaken") });
        } else if (err.code === "EMAIL_SEND_FAILED") {
          setErrors({ general: t("auth.errorEmailSendFailed") });
        } else if (err.code === "TERMS_VERSION_MISMATCH") {
          setErrors({ terms: t("auth.errorTermsVersionMismatch") });
        } else if (err.code === "PRIVACY_VERSION_MISMATCH") {
          setErrors({ privacy: t("auth.errorPrivacyVersionMismatch") });
        } else {
          setErrors({ general: t(getUserFacingErrorKey(err, "auth.errorRegisterFailed")) });
        }
      } else {
        setErrors({ general: t(getUserFacingErrorKey(err, "auth.errorRegisterFailed")) });
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
            <Field>
              <FieldLabel htmlFor="username">{t("auth.username")}</FieldLabel>
              <Input
                id="username"
                type="text"
                placeholder={t("auth.displayName")}
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                disabled={isLoading}
                aria-invalid={Boolean(errors.username)}
                aria-describedby={errors.username ? "username-error" : undefined}
              />
              {errors.username && <FieldError id="username-error">{errors.username}</FieldError>}
            </Field>

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
                aria-invalid={Boolean(errors.email)}
                aria-describedby={errors.email ? "email-error" : undefined}
              />
              {errors.email && <FieldError id="email-error">{errors.email}</FieldError>}
            </Field>

            <Field>
              <FieldLabel htmlFor="password">{t("auth.password")}</FieldLabel>
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
                  className="pr-10"
                  aria-invalid={Boolean(errors.password)}
                  aria-describedby={errors.password ? "password-error" : undefined}
                />
                <button
                  type="button"
                  aria-label={showPassword ? t("auth.hidePassword") : t("auth.showPassword")}
                  aria-pressed={showPassword}
                  className="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                  onClick={() => setShowPassword(!showPassword)}
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
              {errors.password && <FieldError id="password-error">{errors.password}</FieldError>}
            </Field>

            <Field>
              <FieldLabel htmlFor="confirmPassword">{t("auth.confirmPassword")}</FieldLabel>
              <div className="relative">
                <Input
                  id="confirmPassword"
                  type={showConfirmPassword ? "text" : "password"}
                  placeholder={t("auth.confirmPasswordPlaceholder")}
                  autoComplete="new-password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  required
                  disabled={isLoading}
                  className="pr-10"
                  aria-invalid={Boolean(errors.confirmPassword)}
                  aria-describedby={errors.confirmPassword ? "confirm-password-error" : undefined}
                />
                <button
                  type="button"
                  aria-label={showConfirmPassword ? t("auth.hideConfirmPassword") : t("auth.showConfirmPassword")}
                  aria-pressed={showConfirmPassword}
                  className="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                  onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                >
                  {showConfirmPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
              {errors.confirmPassword && <FieldError id="confirm-password-error">{errors.confirmPassword}</FieldError>}
            </Field>

            <CaptchaComponent
              key={captchaResetKey}
              containerId={REGISTER_CAPTCHA_CONTAINER_ID}
              buttonId={REGISTER_CAPTCHA_BUTTON_ID}
              onToken={(token) => {
                setCaptchaToken(token);
                if (token) {
                  setErrors((currentErrors) => {
                    const nextErrors = { ...currentErrors };
                    delete nextErrors.captcha;
                    return nextErrors;
                  });
                }
              }}
              onError={(error) => {
                setCaptchaToken("");
                setErrors((currentErrors) => ({ ...currentErrors, captcha: error }));
              }}
            />
            <button id={REGISTER_CAPTCHA_BUTTON_ID} type="button" className="hidden" aria-hidden="true" tabIndex={-1} />
            {errors.captcha && <p className="text-xs text-destructive">{errors.captcha}</p>}

            <label className="flex items-start gap-2 text-sm">
              <Checkbox
                checked={acceptedTerms}
                onChange={(e) => setAcceptedTerms(e.target.checked)}
                className="mt-0.5"
                disabled={isLoading}
                aria-invalid={Boolean(errors.terms)}
                aria-describedby={errors.terms ? "terms-error" : undefined}
              />
              <span>
                {t("auth.acceptTerms")}{" "}
                <Link href="/terms" className="text-primary hover:underline" target="_blank">
                  {t("auth.termsOfService")}
                </Link>
              </span>
            </label>
            {errors.terms && <FieldError id="terms-error">{errors.terms}</FieldError>}

            <label className="flex items-start gap-2 text-sm">
              <Checkbox
                checked={acceptedPrivacy}
                onChange={(e) => setAcceptedPrivacy(e.target.checked)}
                className="mt-0.5"
                disabled={isLoading}
                aria-invalid={Boolean(errors.privacy)}
                aria-describedby={errors.privacy ? "privacy-error" : undefined}
              />
              <span>
                {t("auth.acceptPrivacy")}{" "}
                <Link href="/privacy" className="text-primary hover:underline" target="_blank">
                  {t("auth.privacyPolicy")}
                </Link>
              </span>
            </label>
            {errors.privacy && <FieldError id="privacy-error">{errors.privacy}</FieldError>}

            {errors.general && (
              <p className="text-sm text-destructive" role="alert">{errors.general}</p>
            )}

            <Button id={REGISTER_SUBMIT_BUTTON_ID} type="submit" className="w-full mt-1" disabled={isLoading}>
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
