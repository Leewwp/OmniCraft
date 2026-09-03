"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { ContentTypeGrid, applyTypeOrder, type ContentType } from "@/components/studio/ContentTypeGrid";
import { PublishForm } from "@/components/studio/PublishForm";
import { fetchPublicConfig } from "@/lib/public-config";
import { silentError } from "@/lib/error-handler";

const CONTENT_TYPE_KEYS = [
  { value: "image", icon: "🖼️" },
  { value: "video", icon: "🎬" },
  { value: "article", icon: "📝" },
  { value: "audio", icon: "🎵" },
  { value: "sheet_music", icon: "🎼" },
  { value: "template", icon: "📋" },
  { value: "prompt", icon: "🤖" },
  { value: "other", icon: "📦" },
] as const;

export default function PublishOriginalPage() {
  const [selectedType, setSelectedType] = useState<string | null>(null);
  /* T25：类型清单与顺序跟随 /config/public 下发的运营配置 */
  const [typeOrder, setTypeOrder] = useState<string[] | null>(null);
  const t = useTranslations("studio.publish");

  useEffect(() => {
    let active = true;
    fetchPublicConfig()
      .then((config) => {
        if (active) setTypeOrder(config.publish?.type_order_original ?? null);
      })
      .catch((error) => {
        silentError(error, { component: "PublishOriginalPage", action: "fetchTypeOrder" });
      });
    return () => {
      active = false;
    };
  }, []);

  const contentTypes: ContentType[] = applyTypeOrder(CONTENT_TYPE_KEYS, typeOrder).map(({ value, icon }) => ({
    value,
    icon,
    label: t(`typeLabel.${value}`),
    description: t(`typeDescOriginal.${value}`),
  }));

  if (selectedType) {
    return (
      <div className="max-w-2xl">
        <h1 className="mb-6 text-xl font-bold text-foreground">{t("originalTitle")}</h1>
        <PublishForm
          zone="original"
          contentType={selectedType}
          onBack={() => setSelectedType(null)}
        />
      </div>
    );
  }

  return (
    <div>
      <h1 className="mb-1 text-xl font-bold text-foreground">{t("originalTitle")}</h1>
      <p className="mb-6 text-sm text-muted-foreground">{t("selectType")}</p>
      <ContentTypeGrid
        types={contentTypes}
        selected={selectedType}
        onSelect={setSelectedType}
      />
    </div>
  );
}