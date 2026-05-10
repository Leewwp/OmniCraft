"use client";

import { useRouter, useSearchParams } from "next/navigation";

const SORT_OPTIONS = [
  { value: "recommended", label: "推荐" },
  { value: "hot", label: "最热门" },
  { value: "newest", label: "最新发布" },
  { value: "most_views", label: "最多点击" },
];

export function SortSelect() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const currentSort = searchParams.get("sort") || "recommended";

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
