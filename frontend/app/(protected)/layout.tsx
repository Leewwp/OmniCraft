"use client";

import { useEffect } from "react";
import { useRouter, usePathname, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import Link from "next/link";

export default function ProtectedLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { user, isLoading } = useAuth();
  const t = useTranslations("auth");
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  useEffect(() => {
    if (!isLoading && !user) {
      // T32（FIX-32）：回跳保参——受保护页常带筛选/预填 query（如
      // /appeals?target_type=account），登录后原样恢复。
      const qs = searchParams.toString();
      const target = qs ? `${pathname}?${qs}` : pathname;
      const redirect = encodeURIComponent(target);
      router.replace(`/login?redirect=${redirect}`);
    }
  }, [user, isLoading, router, pathname, searchParams]);

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="space-y-4 w-full max-w-md px-4">
          <div className="h-6 w-32 bg-muted rounded animate-pulse mx-auto" />
          <div className="h-4 w-48 bg-muted rounded animate-pulse mx-auto" />
          <div className="h-4 w-40 bg-muted rounded animate-pulse mx-auto" />
        </div>
      </div>
    );
  }

  if (!user) {
    return null;
  }

  if (user.is_banned) {
    // T29（FIX-15）：/appeals 是封禁用户唯一申诉出路，放行 children 渲染，
    // 封禁屏不得整页替换（否则申诉链接自循环）；其余受保护页仍封禁屏。
    if (pathname.startsWith("/appeals")) {
      return <>{children}</>;
    }
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center space-y-4">
          <h1 className="text-2xl font-bold text-fg-default">{t("suspendedTitle")}</h1>
          <p className="text-fg-muted">{t("suspendedDescription")}</p>
          <Link
            href="/appeals?target_type=account"
            className="text-accent-emphasis hover:underline"
          >
            {t("submitAppeal")}
          </Link>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
