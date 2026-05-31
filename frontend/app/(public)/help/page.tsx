"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { HelpCircle, ChevronRight } from "lucide-react";

const faqSections = [
  {
    categoryKey: "help.registration",
    items: [
      { qKey: "help.regVerifyQ", aKey: "help.regVerifyA" },
      { qKey: "help.regResendQ", aKey: "help.regResendA" },
    ],
  },
  {
    categoryKey: "help.publishing",
    items: [
      { qKey: "help.pubHowQ", aKey: "help.pubHowA" },
      { qKey: "help.pubReviewQ", aKey: "help.pubReviewA" },
    ],
  },
  {
    categoryKey: "help.downloads",
    items: [
      { qKey: "help.dlHowQ", aKey: "help.dlHowA" },
    ],
  },
  {
    categoryKey: "help.favorites",
    items: [
      { qKey: "help.favHowQ", aKey: "help.favHowA" },
    ],
  },
  {
    categoryKey: "help.reports",
    items: [
      { qKey: "help.reportHowQ", aKey: "help.reportHowA" },
    ],
  },
  {
    categoryKey: "help.reputation",
    items: [
      { qKey: "help.repWhatQ", aKey: "help.repWhatA" },
      { qKey: "help.repRestoreQ", aKey: "help.repRestoreA" },
    ],
  },
  {
    categoryKey: "help.agent",
    items: [
      { qKey: "help.agentWhatQ", aKey: "help.agentWhatA" },
    ],
  },
  {
    categoryKey: "help.client",
    items: [
      { qKey: "help.clientWhatQ", aKey: "help.clientWhatA" },
    ],
  },
];

export default function HelpPage() {
  const t = useTranslations();

  return (
    <div className="mx-auto w-full max-w-3xl px-4 py-8">
      <div className="mb-8 flex items-center gap-3">
        <HelpCircle className="h-8 w-8 text-primary" />
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("help.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("help.subtitle")}</p>
        </div>
      </div>

      <div className="space-y-6">
        {faqSections.map((section) => (
          <div key={section.categoryKey} className="rounded-lg border border-border bg-card p-4">
            <h2 className="mb-3 text-sm font-semibold text-primary">{t(section.categoryKey)}</h2>
            <div className="space-y-3">
              {section.items.map((item) => (
                <details key={item.qKey} className="group">
                  <summary className="flex cursor-pointer items-center gap-2 text-sm font-medium hover:text-primary">
                    <ChevronRight className="h-3.5 w-3.5 transition-transform group-open:rotate-90" />
                    {t(item.qKey)}
                  </summary>
                  <p className="mt-2 pl-5 text-sm text-muted-foreground">{t(item.aKey)}</p>
                </details>
              ))}
            </div>
          </div>
        ))}
      </div>

      <div className="mt-8 rounded-lg border border-border bg-canvas-subtle p-4 text-center">
        <p className="text-sm text-muted-foreground">{t("help.moreHelp")}</p>
        <Link href="/feedback" className="mt-2 inline-block text-sm text-primary hover:underline">
          {t("help.submitFeedback")}
        </Link>
      </div>
    </div>
  );
}
