"use client";

import { useTranslations } from "next-intl";
import { FilterPills } from "@/components/ui/filter-pills";

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

// 收敛至全站筛选形态基准 FilterPills（ui-spec §Component: FilterPills 收敛注记）：
// 药丸 + aria-pressed + accent 选中态；就地 onChange 语义保持不变。
export function ContentTypeFilter({ value, counts, onChange, disabled = false }: ContentTypeFilterProps) {
  const t = useTranslations();
  const selected = isCollectionContentType(value) ? value : "all";

  return (
    <FilterPills
      ariaLabel={t("collections.filters.label")}
      options={collectionContentTypes.map((type) => ({
        value: type,
        label: t(`collections.filters.${type}`),
        count: counts && type !== "all" ? counts[type] ?? 0 : undefined,
      }))}
      value={selected}
      onChange={onChange}
      disabled={disabled}
    />
  );
}

export function isCollectionContentType(value: string): value is CollectionContentTypeFilter {
  return collectionContentTypes.includes(value as CollectionContentTypeFilter);
}
