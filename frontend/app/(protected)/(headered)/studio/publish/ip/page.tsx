"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { IPPublishForm } from "@/components/studio/IPPublishForm";

export default function PublishIPPage() {
  const t = useTranslations("studio.publishIP");
  const router = useRouter();

  return (
    <div className="max-w-2xl">
      <h1 className="mb-6 text-xl font-bold text-foreground">{t("title")}</h1>
      <IPPublishForm onBack={() => router.push("/studio")} />
    </div>
  );
}
