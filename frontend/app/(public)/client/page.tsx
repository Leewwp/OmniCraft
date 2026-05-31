"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { MonitorSmartphone } from "lucide-react";

export default function ClientPage() {
  const t = useTranslations();

  return (
    <div className="mx-auto w-full max-w-3xl px-4 py-8">
      <div className="mb-8 flex items-center gap-3">
        <MonitorSmartphone className="h-8 w-8 text-primary" />
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("client.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("client.subtitle")}</p>
        </div>
      </div>

      <div className="rounded-lg border border-amber-200 bg-amber-50/30 p-6 text-center dark:border-amber-900/30 dark:bg-amber-950/10">
        <MonitorSmartphone className="mx-auto h-12 w-12 text-amber-500" />
        <h2 className="mt-4 text-lg font-semibold">{t("client.unavailableTitle")}</h2>
        <p className="mt-2 text-sm text-muted-foreground">{t("client.unavailableDesc")}</p>
      </div>

      <div className="mt-6 space-y-4">
        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("client.whatIsTitle")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("client.whatIsDesc")}</p>
        </section>

        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("client.featuresTitle")}</h2>
          <ul className="mt-2 list-inside list-disc space-y-1 text-sm text-muted-foreground">
            <li>{t("client.feature1")}</li>
            <li>{t("client.feature2")}</li>
            <li>{t("client.feature3")}</li>
          </ul>
        </section>

        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-semibold">{t("client.feedbackTitle")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{t("client.feedbackDesc")}</p>
          <Link href="/feedback" className="mt-2 inline-block text-sm text-primary hover:underline">
            {t("feedback.submit")}
          </Link>
        </section>
      </div>
    </div>
  );
}
