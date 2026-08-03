"use client";

import { useEffect, useState, type ComponentType } from "react";
import { useTranslations } from "next-intl";
import { CaptchaWidget, type CaptchaWidgetProps } from "@/components/verification/CaptchaWidget";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";

interface VerificationReminderCardProps {
  email: string;
  CaptchaComponent?: ComponentType<CaptchaWidgetProps>;
}

export function VerificationReminderCard({
  email,
  CaptchaComponent = CaptchaWidget,
}: VerificationReminderCardProps) {
  const t = useTranslations();
  const [resendBusy, setResendBusy] = useState(false);
  const [resendCooldown, setResendCooldown] = useState(0);
  const [captchaToken, setCaptchaToken] = useState<string | null>(null);
  const [resendCaptchaResetKey, setResendCaptchaResetKey] = useState(0);
  const [resendSuccess, setResendSuccess] = useState(false);
  const [resendError, setResendError] = useState("");

  useEffect(() => {
    if (resendCooldown <= 0) return;
    const timer = setInterval(() => setResendCooldown((seconds) => seconds - 1), 1000);
    return () => clearInterval(timer);
  }, [resendCooldown]);

  function resetResendCaptcha() {
    setCaptchaToken(null);
    setResendCaptchaResetKey((key) => key + 1);
  }

  async function handleResendVerification() {
    if (!captchaToken) return;
    setResendBusy(true);
    setResendSuccess(false);
    setResendError("");
    try {
      await api.post("/api/v1/auth/resend-verification", {
        email,
        captcha_token: captchaToken,
      });
      setResendSuccess(true);
      setResendCooldown(60);
    } catch (error) {
      silentError(error, { component: "VerificationReminderCard", action: "handleResendVerification" });
      setResendError(t(getUserFacingErrorKey(error)));
    } finally {
      resetResendCaptcha();
      setResendBusy(false);
    }
  }

  return (
    <div className="space-y-3 rounded-md border border-amber-200 bg-amber-50/30 p-4 dark:border-amber-900/30 dark:bg-amber-950/10">
      <h3 className="text-sm font-semibold text-amber-700 dark:text-amber-400">{t("settings.verifyEmailTitle")}</h3>
      <p className="text-xs text-amber-600/80 dark:text-amber-400/70">{t("settings.verifyEmailDesc")}</p>
      {resendSuccess && (
        <p className="text-xs text-emerald-600">{t("auth.verificationResent")}</p>
      )}
      {resendError && <p className="text-xs text-destructive">{resendError}</p>}
      <CaptchaComponent key={resendCaptchaResetKey} onToken={setCaptchaToken} onError={() => setCaptchaToken(null)} />
      <Button
        size="sm"
        variant="outline"
        disabled={resendBusy || resendCooldown > 0 || !captchaToken}
        onClick={() => void handleResendVerification()}
      >
        {resendCooldown > 0
          ? t("auth.resendCooldown", { seconds: resendCooldown })
          : resendBusy
            ? t("common.processing")
            : t("auth.resendVerification")}
      </Button>
    </div>
  );
}
