"use client";

import { useEffect } from "react";
import type { ReactNode } from "react";
import { useTranslations } from "next-intl";
import { Brush } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { GuestLanding } from "@/components/landing/GuestLanding";

// `/` 双态门（票 #853 / spec §12.2）：表面由 AuthProvider 恢复完成后的
// 真实身份决定，SSR 读到的 refresh cookie 只是初始提示——「有 cookie」
// 不等于登录成功，也不在 SSR 轮换 refresh。
export function RootAuthGate({
  hasRefreshCookie,
  authenticatedView,
}: {
  /** SSR 侧只读 cookie 存在性（含 __Host- 前缀变体），不消费值。 */
  hasRefreshCookie: boolean;
  /** 已登录表面 = 原二创页（现状零变化），由服务端组件渲染后作为 children 传入。 */
  authenticatedView: ReactNode;
}) {
  const { user, isLoading } = useAuth();
  const tRoot = useTranslations();
  const tLanding = useTranslations("landing");

  // 「登录 `/` 现状零变化」含 <title>：旧 `/` 的标题 = home.heroSubtitle；
  // 匿名/待定态 = landing.metaTitle。客户端登录/登出切换表面时同步翻转。
  useEffect(() => {
    document.title = user ? tRoot("home.heroSubtitle") : tLanding("metaTitle");
  }, [user, tRoot, tLanding]);

  if (user) {
    return <>{authenticatedView}</>;
  }
  // refresh cookie 不存在 ⇒ 恢复必然失败 ⇒ 必为匿名（access token 仅内存值）。
  // 此时恢复期间也直接渲染落地页，避免中性壳闪屏。
  if (!hasRefreshCookie) {
    return <GuestLanding />;
  }
  // 伪/过期 cookie：恢复完成后仍无 user → 归匿名落地页。
  if (isLoading) {
    return <NeutralAuthShell />;
  }
  return <GuestLanding />;
}

// 恢复中 = 中性壳：不闪落地页也不闪二创页；无任何可交互入口。
function NeutralAuthShell() {
  const t = useTranslations("common");
  return (
    <div
      data-testid="root-auth-shell"
      aria-busy="true"
      className="flex min-h-screen items-center justify-center bg-background"
    >
      <Brush className="h-6 w-6 text-primary motion-safe:animate-pulse" aria-hidden="true" />
      <span className="sr-only">{t("loading")}</span>
    </div>
  );
}
