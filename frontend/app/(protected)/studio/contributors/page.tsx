"use client";

import { useTranslations } from "next-intl";
import { Users } from "lucide-react";

export default function StudioContributorsPage() {
  const t = useTranslations();
  return (
    <div>
      <h1 className="mb-1 text-xl font-bold text-foreground">{t('studio.contributors.title')}</h1>
      <p className="mb-6 text-sm text-muted-foreground">{t('studio.contributors.subtitle')}</p>
      <div className="rounded-lg border border-border bg-card p-12 text-center">
        <Users className="mx-auto mb-3 h-8 w-8 text-muted-foreground" />
        <p className="text-muted-foreground">{t('studio.contributors.empty')}</p>
      </div>
    </div>
  );
}
