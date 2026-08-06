"use client";

import { useTranslations } from "next-intl";
import { Bot } from "lucide-react";
import { Header } from "@/components/layout/Header";
import { AgentFeatureGate } from "@/components/agent/AgentFeatureGate";
import { AgentWorkspace } from "@/components/agent/AgentWorkspace";
import { EmptyState } from "@/components/ui/empty-state";

export default function AgentPage() {
  const t = useTranslations();

  return (
    <div className="flex min-h-dvh flex-col">
      <Header />
      <AgentFeatureGate
        capability="webAgent"
        fallback={
          <div className="flex flex-1 items-center justify-center">
            <EmptyState
              icon={Bot}
              title={t("agent.featureDisabledTitle")}
              description={t("agent.featureDisabledDescription")}
            />
          </div>
        }
      >
        <AgentWorkspace />
      </AgentFeatureGate>
    </div>
  );
}
