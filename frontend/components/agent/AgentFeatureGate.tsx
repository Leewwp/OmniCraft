"use client";

import { useEffect, useState } from "react";
import { fetchPublicConfig, type PublicFeatures } from "@/lib/public-config";
import { useAuth } from "@/contexts/AuthContext";
import { useTranslations } from "next-intl";

const disabledFeatures: PublicFeatures = {
  web_agent_enabled: false,
  guest_agent_enabled: false,
  payment_enabled: false,
  creator_support_enabled: false,
  desktop_deploy_enabled: false,
};

export function AgentFeatureGate({
  capability,
  children,
  fallback,
}: {
  /** webAgent = 登录工作台能力；agentEntry = #854 /agent 入口可见性
   *  （登录看 webAgent 门，游客看 guest 总闸）；desktopDeploy 照旧。 */
  capability: "webAgent" | "agentEntry" | "desktopDeploy";
  children: React.ReactNode;
  fallback?: React.ReactNode;
}) {
  const { user } = useAuth();
  const t = useTranslations();
  const [features, setFeatures] = useState<PublicFeatures>(disabledFeatures);
  const [loaded, setLoaded] = useState(false);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setFailed(false);
    fetchPublicConfig()
      .then((cfg) => {
        if (!cancelled) {
          setFeatures(cfg.features);
          setLoaded(true);
        }
      })
      .catch(() => {
        // A fetch failure must not silently render the "disabled" fallback —
        // the real feature state is unknown, so surface an explicit error + retry.
        if (!cancelled) {
          setFailed(true);
          setLoaded(true);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [attempt]);

  if (!loaded) return null;

  if (failed) {
    return (
      <div className="flex flex-col items-start gap-2 rounded-lg border border-border-destructive bg-card p-4 text-sm">
        <p className="font-medium">{t("agent.gate.loadFailed")}</p>
        <button
          type="button"
          className="rounded-md border border-border bg-card px-3 py-1.5 text-sm transition-colors hover:bg-muted"
          onClick={() => setAttempt((n) => n + 1)}
        >
          {t("common.retry")}
        </button>
      </div>
    );
  }

  const hasVerifiedUser = !!user && !!user.email_verified_at;
  let allowed = false;

  if (capability === "webAgent") {
    allowed = features.web_agent_enabled && !!user && !!user.email_verified_at;
  }

  if (capability === "agentEntry") {
    /* #854（Q21）：入口可见性——登录用户沿用 verified webAgent 门；游客由
       guest_agent_enabled 决定（服务端投影已含 web_agent_enabled 联动）。
       入口只决定 /agent 可达，不在入口展示任何轮数。 */
    allowed = features.web_agent_enabled && (hasVerifiedUser || features.guest_agent_enabled);
  }

  if (capability === "desktopDeploy") {
    allowed = features.desktop_deploy_enabled;
  }

  if (!allowed) return <>{fallback}</>;

  return <>{children}</>;
}
