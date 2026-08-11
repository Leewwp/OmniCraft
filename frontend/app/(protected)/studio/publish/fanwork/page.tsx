"use client";

import { Suspense, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { ContentTypeGrid, type ContentType } from "@/components/studio/ContentTypeGrid";
import { PublishForm, type PrefillWarning } from "@/components/studio/PublishForm";

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

function parsePrefillId(value: string | null): { present: boolean; valid: boolean; id?: number } {
  if (value === null) return { present: false, valid: false };
  if (!/^[1-9]\d*$/.test(value.trim())) return { present: true, valid: false };
  const parsed = Number(value);
  if (!Number.isSafeInteger(parsed) || parsed <= 0) return { present: true, valid: false };
  return { present: true, valid: true, id: parsed };
}

function resolvePrefill(searchParams: URLSearchParams): {
  sourceOriginalId?: number;
  sourceFanworkId?: number;
  warnings: PrefillWarning[];
} {
  const original = parsePrefillId(searchParams.get("source_original_id"));
  const fanwork = parsePrefillId(searchParams.get("source_fanwork_id"));
  const warnings = new Set<PrefillWarning>();
  let sourceOriginalId: number | undefined;
  let sourceFanworkId: number | undefined;

  if (original.valid) sourceOriginalId = original.id;
  if (fanwork.present && original.valid) {
    // Both IDs specified: keep source_original_id, clear source_fanwork_id.
    warnings.add("bothSources");
  } else if (fanwork.valid) {
    sourceFanworkId = fanwork.id;
  }
  if (original.present && !original.valid) warnings.add("invalidId");
  if (fanwork.present && !fanwork.valid) warnings.add("invalidId");

  return { sourceOriginalId, sourceFanworkId, warnings: [...warnings] };
}

function PublishFanworkClient() {
  const [selectedType, setSelectedType] = useState<string | null>(null);
  const t = useTranslations("studio.publish");
  const searchParams = useSearchParams();
  const prefill = resolvePrefill(searchParams);

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
          prefillSourceOriginalId={prefill.sourceOriginalId}
          prefillSourceFanworkId={prefill.sourceFanworkId}
          prefillWarnings={prefill.warnings}
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

export default function PublishFanworkPage() {
  return (
    <Suspense fallback={<div className="min-h-[40vh]" />}>
      <PublishFanworkClient />
    </Suspense>
  );
}
