"use client";

import { useTranslations } from "next-intl";
import { FileText } from "lucide-react";

export default function TermsPage() {
  const t = useTranslations();

  return (
    <div className="mx-auto w-full max-w-3xl px-4 py-8">
      <div className="mb-8 flex items-center gap-3">
        <FileText className="h-8 w-8 text-primary" />
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("terms.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("terms.subtitle")}</p>
        </div>
      </div>

      <div className="prose prose-sm max-w-none space-y-6">
        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("terms.acceptance")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("terms.acceptanceContent")}</p>
        </section>

        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("terms.accountResponsibility")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("terms.accountResponsibilityContent")}</p>
        </section>

        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("terms.contentRules")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("terms.contentRulesContent")}</p>
        </section>

        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("terms.communityGuidelines")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("terms.communityGuidelinesContent")}</p>
        </section>

        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("terms.ipRights")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("terms.ipRightsContent")}</p>
        </section>

        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("terms.liability")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("terms.liabilityContent")}</p>
        </section>

        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("terms.changes")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("terms.changesContent")}</p>
        </section>
      </div>
    </div>
  );
}
