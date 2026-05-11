"use client";

import { useTranslations } from "next-intl";
import { useRouter, useSearchParams } from "next/navigation";

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
    <select
      value={currentSort}
      onChange={(e) => {
        const params = new URLSearchParams(searchParams.toString());
        const val = e.target.value;
        if (val === "recommended") {
          params.delete("sort");
        } else {
          params.set("sort", val);
        }
        const query = params.toString();
        router.push(query ? `/original?${query}` : "/original");
      }}
      className="rounded-md border border-border bg-card px-2.5 py-1.5 text-xs text-foreground outline-none focus:ring-1 focus:ring-ring"
    >
      {SORT_OPTIONS.map((opt) => (
        <option key={opt.value} value={opt.value}>
          {opt.label}
        </option>
      ))}
    </select>
  );
}
