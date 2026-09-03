"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Bot } from "lucide-react";
import { Header } from "@/components/layout/Header";
import { AgentFeatureGate } from "@/components/agent/AgentFeatureGate";
import { AgentWorkspace } from "@/components/agent/AgentWorkspace";
import { EmptyState } from "@/components/ui/empty-state";

/* A-07：搜索页「问 AI 助手」入口经 /agent?q= 预填首轮问题（仅首挂载生效）。
 * useSearchParams 需要 Suspense 边界（CSR bailout），故拆出内部组件。 */
function AgentWorkspacePanel() {
  const searchParams = useSearchParams();
  const initialQuery = (searchParams.get("q") || "").trim();
  return <AgentWorkspace initialQuery={initialQuery || undefined} />;
}

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
        <Suspense fallback={null}>
          <AgentWorkspacePanel />
        </Suspense>
      </AgentFeatureGate>
    </div>
  );
}
