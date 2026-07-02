"use client";

import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";

export const collectionContentTypes = [
  "all",
  "image",
  "article",
  "video",
  "audio",
  "template",
  "sheet_music",
  "mod",
  "prompt",
  "other",
] as const;

export type CollectionContentTypeFilter = (typeof collectionContentTypes)[number];

interface ContentTypeFilterProps {
  value: string;
  counts?: Record<string, number>;
  onChange: (value: string) => void;
  disabled?: boolean;
}

export function ContentTypeFilter({ value, counts, onChange, disabled = false }: ContentTypeFilterProps) {
  const t = useTranslations();
  const selected = isCollectionContentType(value) ? value : "all";

  return (
    <div role="tablist" aria-label={t("collections.filters.label")} className="flex gap-2 overflow-x-auto pb-1">
      {collectionContentTypes.map((type) => {
        const isSelected = selected === type;
        return (
          <button
            key={type}
            type="button"
            role="tab"
            aria-selected={isSelected}
            aria-current={isSelected ? "true" : undefined}
            disabled={disabled}
            onClick={() => onChange(type)}
            className={cn(
              "min-h-11 shrink-0 rounded-md border px-3 text-sm transition-colors md:min-h-9",
              isSelected
                ? "border-primary bg-primary text-primary-foreground"
                : "border-border bg-background text-muted-foreground hover:text-foreground",
            )}
          >
            {t(`collections.filters.${type}`)}
            {counts && type !== "all" && (
              <span className="ml-1 text-xs opacity-80">{counts[type] ?? 0}</span>
            )}
          </button>
        );
      })}
    </div>
  );
}

export function isCollectionContentType(value: string): value is CollectionContentTypeFilter {
  return collectionContentTypes.includes(value as CollectionContentTypeFilter);
}
