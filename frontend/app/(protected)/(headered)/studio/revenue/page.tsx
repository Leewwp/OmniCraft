"use client";

import { useTranslations } from "next-intl";
import { DollarSign } from "lucide-react";

export default function StudioRevenuePage() {
  const t = useTranslations();
  return (
    <div>
      <h1 className="mb-1 text-xl font-bold text-foreground">{t('studio.revenue.title')}</h1>
      <p className="mb-6 text-sm text-muted-foreground">{t('studio.revenue.subtitle')}</p>
      <div className="rounded-lg border border-border bg-card p-12 text-center">
        <DollarSign className="mx-auto mb-3 h-8 w-8 text-muted-foreground" />
        <p className="mb-2 text-sm font-medium text-foreground">{t('studio.revenue.comingSoon')}</p>
        <p className="text-sm text-muted-foreground">{t('studio.revenue.comingSoonDesc')}</p>
      </div>
    </div>
  );
}
