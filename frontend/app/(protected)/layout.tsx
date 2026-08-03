"use client";

import { useEffect } from "react";
import { useRouter, usePathname } from "next/navigation";
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

  useEffect(() => {
    if (!isLoading && !user) {
      const redirect = encodeURIComponent(pathname);
      router.replace(`/login?redirect=${redirect}`);
    }
  }, [user, isLoading, router, pathname]);

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
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center space-y-4">
          <h1 className="text-2xl font-bold text-fg-default">{t("suspendedTitle")}</h1>
          <p className="text-fg-muted">{t("suspendedDescription")}</p>
          <Link href="/appeals" className="text-accent-emphasis hover:underline">
            {t("submitAppeal")}
          </Link>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
