"use client";

import { useTranslations } from "next-intl";
import { Tags } from "lucide-react";

export default function StudioTagSuggestionsPage() {
  const t = useTranslations();
  return (
    <div>
      <h1 className="mb-1 text-xl font-bold text-foreground">{t('studio.tags.title')}</h1>
      <p className="mb-6 text-sm text-muted-foreground">{t('studio.tags.subtitle')}</p>
      <div className="rounded-lg border border-border bg-card p-12 text-center">
        <Tags className="mx-auto mb-3 h-8 w-8 text-muted-foreground" />
        <p className="text-muted-foreground">{t('studio.tags.empty')}</p>
      </div>
    </div>
  );
}
