"use client";

import { useTranslations } from "next-intl";
import { Shield } from "lucide-react";

export default function PrivacyPage() {
  const t = useTranslations();

  return (
    <div className="mx-auto w-full max-w-3xl px-4 py-8">
      <div className="mb-8 flex items-center gap-3">
        <Shield className="h-8 w-8 text-primary" />
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("privacy.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("privacy.subtitle")}</p>
        </div>
      </div>

      <div className="prose prose-sm max-w-none space-y-6">
        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("privacy.dataProcessing")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("privacy.dataProcessingContent")}</p>
        </section>

        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("privacy.cookies")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("privacy.cookiesContent")}</p>
        </section>

        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("privacy.uploads")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("privacy.uploadsContent")}</p>
        </section>

        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("privacy.logs")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("privacy.logsContent")}</p>
        </section>

        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("privacy.feedbackDiagnostics")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("privacy.feedbackDiagnosticsContent")}</p>
        </section>

        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("privacy.deletion")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("privacy.deletionContent")}</p>
        </section>
      </div>
    </div>
  );
}
