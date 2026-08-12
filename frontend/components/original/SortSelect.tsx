"use client";

import { useTranslations } from "next-intl";
import { useRouter, useSearchParams } from "next/navigation";
import { SortSelect as SharedSortSelect } from "@/components/ui/SortSelect";

export function SortSelect() {
  const t = useTranslations();
  const router = useRouter();
  const searchParams = useSearchParams();
  const currentSort = searchParams.get("sort") || "recommended";

  const SORT_OPTIONS = [
    { value: "recommended", label: t('home.categoryRecommended') },
    { value: "hot", label: t('content.sortHottest') },
    { value: "newest", label: t('content.sortNewRelease') },
    { value: "most_views", label: t('content.sortMostViewed') },
  ];

  return (
    <SharedSortSelect
      ariaLabel={t('common.sortLabel')}
      value={currentSort}
      options={SORT_OPTIONS}
      onChange={(val) => {
        const params = new URLSearchParams(searchParams.toString());
        if (val === "recommended") {
          params.delete("sort");
        } else {
          params.set("sort", val);
        }
        const query = params.toString();
        router.push(query ? `/original?${query}` : "/original");
      }}
    />
  );
}
