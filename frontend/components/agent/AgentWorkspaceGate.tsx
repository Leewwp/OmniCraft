"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Bot } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { fetchPublicConfig, type PublicFeatures } from "@/lib/public-config";
import { resolveAgentSurface, type AgentSurface } from "@/lib/agent-guest";
import { AgentFeatureGate } from "@/components/agent/AgentFeatureGate";
import { AgentWorkspace } from "@/components/agent/AgentWorkspace";
import { EmptyState } from "@/components/ui/empty-state";
import { Button } from "@/components/ui/button";
import { useAuthGate } from "@/components/auth/AuthGateProvider";
import { Brush } from "lucide-react";

/**
 * /agent 双态壳（#854）：表面由 AuthProvider 恢复完成后的真实身份与特性
 * 开关决定——登录用户 = 既有受保护工作台（webAgent 门照旧）；游客 =
 * guest_agent_enabled 时的匿名工作台；其余 = 登录引导降级。恢复中渲染
 * 中性壳（与落地页 RootAuthGate 同模式），SSR 不读设备 cookie 判身份。
 */
const disabledFeatures: PublicFeatures = {
  web_agent_enabled: false,
  guest_agent_enabled: false,
  payment_enabled: false,
  creator_support_enabled: false,
  desktop_deploy_enabled: false,
};

export function AgentWorkspaceGate({
  initialConversationId,
  initialQuery,
}: {
  initialConversationId?: number;
  initialQuery?: string;
}) {
  const { user, isLoading } = useAuth();
  const [features, setFeatures] = useState<PublicFeatures>(disabledFeatures);
  const [featuresLoaded, setFeaturesLoaded] = useState(false);

  useEffect(() => {
    let cancelled = false;
    fetchPublicConfig()
      .then((cfg) => {
        if (!cancelled) {
          setFeatures(cfg.features);
          setFeaturesLoaded(true);
        }
      })
      .catch(() => {
        if (!cancelled) setFeaturesLoaded(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const surface: AgentSurface = featuresLoaded
    ? resolveAgentSurface(isLoading, !!user, features)
    : "pending";

  if (surface === "pending") {
    return <NeutralAgentShell />;
  }

  if (surface === "user") {
    return (
      <AgentFeatureGate
        capability="webAgent"
        fallback={<AgentDisabledFallback />}
      >
        <AgentWorkspace
          key={`user-${initialConversationId ?? 0}`}
          initialConversationId={initialConversationId}
          initialQuery={initialQuery}
        />
      </AgentFeatureGate>
    );
  }

  if (surface === "guest") {
    return (
      <AgentWorkspace
        key={`guest-${initialConversationId ?? 0}`}
        initialConversationId={initialConversationId}
        initialQuery={initialQuery}
        variant="guest"
      />
    );
  }

  // 登录引导降级：总闸关闭或游客不可用。已登录但邮箱未验证的用户也落到
  // 这里（webAgent 门拒绝）——CTA 语境按身份切换。
  return <AgentDisabledFallback />;
}

function NeutralAgentShell() {
  // 独立变量名：治理扫描按变量名→命名空间建表，文件内同名 `t` 会互相
  // 覆盖；根命名空间的 loading 键用独立绑定。
  const tShell = useTranslations();
  return (
    <div
      data-testid="agent-auth-shell"
      aria-busy="true"
      className="flex min-h-[calc(100dvh-var(--header-h))] items-center justify-center bg-canvas-default"
    >
      <Brush className="h-6 w-6 text-primary motion-safe:animate-pulse" aria-hidden="true" />
      <span className="sr-only">{tShell("common.loading")}</span>
    </div>
  );
}

/** 总闸关闭/不可用的降级面（消失式入口的另一侧）：游客给登录引导 CTA
 *  （转登录引导），已登录未验证用户沿用 featureDisabled 文案。 */
function AgentDisabledFallback() {
  const t = useTranslations("agent.guest");
  const tAgent = useTranslations("agent");
  const { user } = useAuth();
  const { requireAuth } = useAuthGate();
  return (
    <div className="flex flex-1 items-center justify-center">
      <div className="max-w-md text-center">
        <EmptyState
          icon={Bot}
          title={user ? tAgent("featureDisabledTitle") : t("guideTitle")}
          description={user ? tAgent("featureDisabledDescription") : t("guideDescription")}
        />
        {!user && (
          <Button type="button" size="sm" className="mt-2" onClick={() => requireAuth()}>
            {t("loginCta")}
          </Button>
        )}
      </div>
    </div>
  );
}
