"use client";

import { Suspense } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Brush } from "lucide-react";
import { useTranslations } from "next-intl";
import { LoginForm } from "@/components/auth/LoginForm";

// SP-17/T2 (#491)：/login 页与 LoginModal 共用 LoginForm；顺带补齐此前缺失
// 的验证码分支（CAPTCHA_REQUIRED 动态插入 CaptchaWidget，超阈值不再卡死）。

function LoginPageContent() {
  const t = useTranslations();
  const router = useRouter();
  const searchParams = useSearchParams();

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
          <LoginForm
            showRememberMe
            onSuccess={() => {
              const redirect = searchParams.get("redirect") || "/";
              router.push(redirect);
            }}
          />
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
