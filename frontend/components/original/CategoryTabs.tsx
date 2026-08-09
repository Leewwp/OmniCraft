"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";

interface CategoryTab {
  slug: string;
  i18n: string;
  name_i18n?: Record<string, string>;
}

interface CategoryTabsProps {
  categories: CategoryTab[];
  currentCategory: string;
}

export function CategoryTabs({ categories, currentCategory }: CategoryTabsProps) {
  const t = useTranslations();
  const router = useRouter();
  const searchParams = useSearchParams();

  function navigate(slug: string) {
    const params = new URLSearchParams(searchParams.toString());
    if (slug) {
      params.set("category", slug);
    } else {
      params.delete("category");
    }
    if (params.get("sort") === "recommended") {
      params.delete("sort");
    }
    const query = params.toString();
    router.push(query ? `/original?${query}` : "/original");
  }

  return (
    <div
      role="tablist"
      aria-label={t("content.originalZone")}
      className="flex flex-1 items-center gap-1 overflow-x-auto"
      style={{ scrollbarWidth: "none" }}
    >
      {categories.map((cat) => {
        const active = currentCategory === cat.slug;
        return (
          <button
            key={cat.slug || "recommended"}
            type="button"
            role="tab"
            aria-selected={active}
            aria-pressed={active}
            onClick={() => navigate(cat.slug)}
            className={`flex-shrink-0 rounded-full border px-4 py-2 text-sm font-medium transition-all duration-150 whitespace-nowrap select-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
              active
                ? "border-accent-emphasis bg-accent-subtle text-accent-emphasis font-semibold"
                : "border-transparent text-fg-muted hover:bg-canvas-subtle hover:text-foreground cursor-pointer"
            }`}
          >
            {cat.i18n ? t(cat.i18n) : cat.name_i18n?.zh || cat.name_i18n?.en || cat.slug}
          </button>
        );
      })}
    </div>
  );
}
