"use client";

import { useEffect, useState } from "react";
import { fetchPublicConfig, type PublicFeatures } from "@/lib/public-config";
import { useAuth } from "@/contexts/AuthContext";
import { useTranslations } from "next-intl";

const disabledFeatures: PublicFeatures = {
  web_agent_enabled: false,
  payment_enabled: false,
  creator_support_enabled: false,
  desktop_deploy_enabled: false,
};

export function AgentFeatureGate({
  capability,
  children,
  fallback,
}: {
  capability: "webAgent" | "desktopDeploy";
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

  let allowed = false;

  if (capability === "webAgent") {
    allowed = features.web_agent_enabled && !!user && !!user.email_verified_at;
  }

  if (capability === "desktopDeploy") {
    allowed = features.desktop_deploy_enabled;
  }

  if (!allowed) return <>{fallback}</>;

  return <>{children}</>;
}
