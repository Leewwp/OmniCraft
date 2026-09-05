"use client";

import { Check } from "lucide-react";
import { useTranslations } from "next-intl";

interface ReputationDetailProps {
  rewardPoints: number;
  completed: boolean;
}

export function ReputationDetail({ rewardPoints, completed }: ReputationDetailProps) {
  const t = useTranslations();

  if (completed) {
    return (
      <span className="inline-flex items-center gap-1 rounded bg-emerald-100 px-2 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400">
        <Check className="h-3 w-3" />
        {t("rehab.completed")}
      </span>
    );
  }

  return (
    <span className="text-xs text-muted-foreground">
      {t("rehab.rewardPoints", { pts: rewardPoints })}
    </span>
  );
}
