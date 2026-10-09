"use client";

import { useTranslations } from "next-intl";
import { LogIn } from "lucide-react";
import { Button } from "@/components/ui/button";

/**
 * #854（Q14）游客轮数用尽转化卡：替换工作台输入区下方形态，CTA 走 SP-17
 * 登录浮窗（由调用方注入 onLogin = requireAuth 续做）。文案 agent.guest.*，
 * 双语；卡片不展示轮数计数之外的服务端内部状态。
 */
export function AgentGuestExhaustedCard({ onLogin }: { onLogin: () => void }) {
  const t = useTranslations("agent.guest");
  return (
    <div
      data-testid="guest-exhausted-card"
      role="status"
      className="mb-3 rounded-lg border border-border-default bg-card p-4 text-left"
    >
      <p className="text-sm font-semibold text-fg-default">{t("exhaustedTitle")}</p>
      <p className="mt-1 text-sm text-fg-muted">{t("exhaustedDescription")}</p>
      <Button type="button" size="sm" className="mt-3" onClick={onLogin}>
        <LogIn className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
        {t("loginCta")}
      </Button>
    </div>
  );
}
