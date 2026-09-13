"use client";

import { useId, useState, type ComponentType, type FormEvent } from "react";
import Link from "next/link";
import { Eye, EyeOff, Loader2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { ApiRequestError } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { CaptchaWidget, type CaptchaWidgetProps } from "@/components/verification/CaptchaWidget";

interface LoginFormProps {
  /** 登录成功回调（页面跳转 / 浮窗续做由调用方决定）。 */
  onSuccess: () => void;
  showRememberMe?: boolean;
  autoFocusEmail?: boolean;
  /** 测试注入口（RegisterPageContent 先例）。 */
  CaptchaComponent?: ComponentType<CaptchaWidgetProps>;
}

// SP-17/T2 (#491)：登录表单唯一实现，/login 页与 LoginModal 共用。
// CAPTCHA_REQUIRED 动态插入验证码（后端失败计数无预检接口）；CAPTCHA_FAILED
// 重置重试；USER_BANNED 申诉指引对齐既有登录页特判。

export function LoginForm({
  onSuccess,
  showRememberMe = false,
  autoFocusEmail = false,
  CaptchaComponent = CaptchaWidget,
}: LoginFormProps) {
  const t = useTranslations();
  const { login } = useAuth();
  const instanceId = useId();
  const emailId = `${instanceId}-email`;
  const passwordId = `${instanceId}-password`;
  const captchaContainerId = `login-captcha-container-${instanceId}`;
  const captchaButtonId = `login-captcha-button-${instanceId}`;

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [rememberMe, setRememberMe] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const [bannedGuidance, setBannedGuidance] = useState(false);
  const [captchaVisible, setCaptchaVisible] = useState(false);
  const [captchaToken, setCaptchaToken] = useState("");
  const [captchaAttempt, setCaptchaAttempt] = useState(0);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (isLoading) return;
    setError("");
    setBannedGuidance(false);
    if (captchaVisible && !captchaToken) {
      setError(t("auth.captchaRequired"));
      return;
    }
    setIsLoading(true);
    try {
      await login(email, password, rememberMe, captchaVisible ? captchaToken : undefined);
      onSuccess();
    } catch (err) {
      silentError(err, { component: "LoginForm", action: "handleSubmit" });
      if (err instanceof ApiRequestError && err.code === "CAPTCHA_REQUIRED") {
        setCaptchaVisible(true);
        setError(t("auth.captchaRequired"));
      } else if (err instanceof ApiRequestError && err.code === "CAPTCHA_FAILED") {
        setCaptchaVisible(true);
        setCaptchaToken("");
        setCaptchaAttempt((attempt) => attempt + 1);
        setError(t("auth.captchaFailed"));
      } else if (err instanceof ApiRequestError && err.code === "USER_BANNED") {
        setBannedGuidance(true);
        setError(t(getUserFacingErrorKey(err, "auth.errorLoginFailed")));
      } else {
        setError(t(getUserFacingErrorKey(err, "auth.errorLoginFailed")));
      }
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <Field>
        <FieldLabel htmlFor={emailId}>{t("auth.email")}</FieldLabel>
        <Input
          id={emailId}
          type="email"
          placeholder="you@example.com"
          autoComplete="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          disabled={isLoading}
          autoFocus={autoFocusEmail}
          data-testid="login-email"
        />
      </Field>

      <Field>
        <FieldLabel htmlFor={passwordId}>{t("auth.password")}</FieldLabel>
        <div className="relative">
          <Input
            id={passwordId}
            type={showPassword ? "text" : "password"}
            placeholder={t("auth.passwordPlaceholder")}
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            disabled={isLoading}
            className="pr-10"
            data-testid="login-password"
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
      </Field>

      {showRememberMe && (
        <label className="flex cursor-pointer items-center gap-2 text-sm text-muted-foreground">
          <Checkbox
            checked={rememberMe}
            onChange={(e) => setRememberMe(e.target.checked)}
            className="h-3.5 w-3.5 rounded border-border accent-primary"
          />
          {t("auth.rememberMe")}
        </label>
      )}

      {captchaVisible && (
        <div data-testid="login-captcha">
          <CaptchaComponent
            key={captchaAttempt}
            containerId={captchaContainerId}
            buttonId={captchaButtonId}
            onToken={(token) => {
              setCaptchaToken(token);
              if (token) setError("");
            }}
            onError={() => {
              setCaptchaToken("");
              setError(t("auth.captchaFailed"));
            }}
          />
          {/* 阿里云 embed 模式绑定的隐藏触发按钮（RegisterPageContent 同款） */}
          <button id={captchaButtonId} type="button" className="hidden" aria-hidden="true" tabIndex={-1} />
        </div>
      )}

      {error && (
        <div className="space-y-2" role="alert">
          <p className="text-sm text-destructive">{error}</p>
          {bannedGuidance && (
            <p className="text-sm text-muted-foreground">
              {t("auth.bannedFeedbackHint")}{" "}
              <Link href="/feedback" className="font-medium text-primary hover:underline">
                {t("auth.bannedFeedbackEntry")}
              </Link>
            </p>
          )}
        </div>
      )}

      <Button type="submit" className="mt-1 w-full" disabled={isLoading} data-testid="login-submit">
        {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
        {t("auth.loginButton")}
      </Button>
    </form>
  );
}
