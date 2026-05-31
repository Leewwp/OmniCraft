"use client";

import { useTranslations } from "next-intl";
import FeedbackForm from "@/components/feedback/FeedbackForm";
import { MessageSquarePlus } from "lucide-react";

export default function FeedbackPage() {
  const t = useTranslations();

  return (
    <div className="mx-auto w-full max-w-2xl px-4 py-8">
      <div className="mb-6 flex items-center gap-3">
        <MessageSquarePlus className="h-8 w-8 text-primary" />
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("feedback.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("feedback.subtitle")}</p>
        </div>
      </div>

      <div className="rounded-lg border border-border bg-card p-4">
        <FeedbackForm />
      </div>

      <p className="mt-4 text-center text-xs text-muted-foreground">
        {t("feedback.anonymousNote")}
      </p>
    </div>
  );
}
