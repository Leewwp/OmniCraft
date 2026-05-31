"use client";

import { useEffect, useState } from "react";
import { fetchPublicConfig, type PublicFeatures } from "@/lib/public-config";
import { useAuth } from "@/contexts/AuthContext";

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
  const [features, setFeatures] = useState<PublicFeatures>(disabledFeatures);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let cancelled = false;
    fetchPublicConfig()
      .then((cfg) => {
        if (!cancelled) {
          setFeatures(cfg.features);
          setLoaded(true);
        }
      })
      .catch(() => {
        if (!cancelled) setLoaded(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (!loaded) return null;

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
