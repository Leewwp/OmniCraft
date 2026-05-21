"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { ContentTypeGrid, type ContentType } from "@/components/studio/ContentTypeGrid";
import { PublishForm } from "@/components/studio/PublishForm";

const CONTENT_TYPE_KEYS = [
  { value: "image", icon: "🖼️" },
  { value: "video", icon: "🎬" },
  { value: "article", icon: "📝" },
  { value: "audio", icon: "🎵" },
  { value: "sheet_music", icon: "🎼" },
  { value: "mod", icon: "🧩" },
  { value: "prompt", icon: "🤖" },
  { value: "other", icon: "📦" },
] as const;

export default function PublishFanworkPage() {
  const [selectedType, setSelectedType] = useState<string | null>(null);
  const t = useTranslations("studio.publish");

  const contentTypes: ContentType[] = CONTENT_TYPE_KEYS.map(({ value, icon }) => ({
    value,
    icon,
    label: t(`typeLabel.${value}`),
    description: t(`typeDescFanwork.${value}`),
  }));

  if (selectedType) {
    return (
      <div className="max-w-2xl">
        <h1 className="mb-6 text-xl font-bold text-foreground">{t("fanworkTitle")}</h1>
        <PublishForm
          zone="fanwork"
          contentType={selectedType}
          onBack={() => setSelectedType(null)}
        />
      </div>
    );
  }

  return (
    <div>
      <h1 className="mb-1 text-xl font-bold text-foreground">{t("fanworkTitle")}</h1>
      <p className="mb-6 text-sm text-muted-foreground">{t("selectType")}</p>
      <ContentTypeGrid
        types={contentTypes}
        selected={selectedType}
        onSelect={setSelectedType}
      />
    </div>
  );
}