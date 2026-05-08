import Link from "next/link";
import { getTranslations } from 'next-intl/server';
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { buttonVariants } from "@/components/ui/button";
import { normalizeContentList } from "@/lib/content";

interface CategoryItem {
  id: number;
  slug: string;
  name_i18n?: Record<string, string>;
}

interface CategoryResponse {
  categories?: CategoryItem[];
}

interface ContentResponse {
  contents?: unknown[];
}

interface SearchParams {
  category?: string;
  type?: string;
  sort?: string;
}

const PRIMARY_CATEGORIES_FALLBACK = [
  { slug: "", label: "Recommended", i18n: "home.categoryRecommended" },
  { slug: "film_tv", label: "Film & TV", i18n: "home.categoryFilmTv" },
  { slug: "gaming", label: "Gaming", i18n: "home.categoryGaming" },
  { slug: "literature", label: "Literature", i18n: "home.categoryLiterature" },
  { slug: "pet", label: "Pets", i18n: "home.categoryPet" },
  { slug: "food", label: "Food", i18n: "home.categoryFood" },
  { slug: "beauty_fashion", label: "Beauty & Fashion", i18n: "home.categoryBeautyFashion" },
  { slug: "home", label: "Home", i18n: "home.categoryHome" },
  { slug: "tech_digital", label: "Tech & Digital", i18n: "home.categoryTechDigital" },
  { slug: "travel", label: "Travel", i18n: "home.categoryTravel" },
  { slug: "sports", label: "Sports", i18n: "home.categorySports" },
  { slug: "productivity", label: "Productivity", i18n: "home.categoryProductivity" },
];

const PRIMARY_CATEGORY_LABEL_KEY: Record<string, string> = Object.fromEntries(
  PRIMARY_CATEGORIES_FALLBACK.map((c) => [c.slug, c.i18n]),
);

const SECONDARY_TYPES_FALLBACK = [
  { key: "", label: "All", i18n: "content.categoryAll", contentType: "" },
  { key: "image", label: "Image", i18n: "content.categoryImage", contentType: "image" },
  { key: "video", label: "Video", i18n: "content.categoryVideo", contentType: "video" },
  { key: "audio", label: "Audio/Sheet Music", i18n: "content.categoryAudio", contentType: "audio,sheet_music" },
  { key: "text", label: "Text", i18n: "content.categoryText", contentType: "article,prompt,text" },
  { key: "template", label: "Templates", i18n: "content.categoryTemplate", contentType: "template" },
  { key: "model_design", label: "Models & Designs", i18n: "content.categoryModel", tags: "3D模型" },
  { key: "other", label: "Other", i18n: "content.categoryOther", contentType: "other" },
];

const SECONDARY_TYPE_LABEL_KEY: Record<string, string> = Object.fromEntries(
  SECONDARY_TYPES_FALLBACK.map((t) => [t.key, t.i18n]),
);

const SORTS = [
  { key: "hot", label: "Hottest", i18n: "content.sortHottest" },
  { key: "newest", label: "Newest", i18n: "content.sortNewest" },
  { key: "most_views", label: "Most Viewed", i18n: "content.sortMostViewed" },
  { key: "best_rated", label: "Top Rated", i18n: "content.sortTopRated" },
];

const SORT_LABEL_KEY: Record<string, string> = Object.fromEntries(
  SORTS.map((s) => [s.key, s.i18n]),
);

function getApiBase() {
  const raw = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  return `${raw.replace(/\/$/, "")}/api/v1`;
}

function normalizeCategorySlug(slug: string) {
  return slug.endsWith("_orig") ? slug.slice(0, -5) : slug;
}

async function fetchPrimaryCategories(apiBase: string) {
  try {
    const res = await fetch(`${apiBase}/categories?zone=original&level=primary`, { cache: "no-store" });
    if (!res.ok) return PRIMARY_CATEGORIES_FALLBACK;
    const data = (await res.json()) as CategoryResponse;
    const categories = (data.categories || [])
      .map((item) => {
        const slug = normalizeCategorySlug(item.slug);
        const fallback = PRIMARY_CATEGORIES_FALLBACK.find((c) => c.slug === slug);
        return {
          slug,
          label: fallback?.label || item.name_i18n?.zh || item.name_i18n?.en || item.slug,
          i18n: fallback?.i18n || "",
        };
      })
      .filter((item) => PRIMARY_CATEGORIES_FALLBACK.some((c) => c.slug === item.slug));
    return [
      PRIMARY_CATEGORIES_FALLBACK[0],
      ...categories.filter((c) => c.slug !== ""),
    ];
  } catch {
    return PRIMARY_CATEGORIES_FALLBACK;
  }
}

async function fetchSecondaryTypes(apiBase: string) {
  try {
    const res = await fetch(`${apiBase}/categories?zone=original&level=content_type`, { cache: "no-store" });
    if (!res.ok) return SECONDARY_TYPES_FALLBACK;
    const data = (await res.json()) as CategoryResponse;
    const categories = (data.categories || []).map((item) => {
      const fallback = SECONDARY_TYPES_FALLBACK.find((t) => t.key === item.slug);
      return {
        key: item.slug,
        label: fallback?.label || item.name_i18n?.zh || item.name_i18n?.en || item.slug,
        i18n: fallback?.i18n || "",
        contentType: fallback?.contentType || item.slug,
        tags: fallback?.tags,
      };
    });
    return categories.length > 0 ? categories : SECONDARY_TYPES_FALLBACK;
  } catch {
    return SECONDARY_TYPES_FALLBACK;
  }
}

function buildOriginalHref(next: Partial<SearchParams>, current: Required<SearchParams>) {
  const params = new URLSearchParams();
  const merged = { ...current, ...next };
  if (merged.category) params.set("category", merged.category);
  if (merged.type) params.set("type", merged.type);
  if (merged.sort && merged.sort !== "hot") params.set("sort", merged.sort);
  const query = params.toString();
  return query ? `/original?${query}` : "/original";
}

async function fetchOriginalContents(apiBase: string, search: Required<SearchParams>) {
  const params = new URLSearchParams({
    zone: "original",
    sort: search.category === "" && search.sort === "hot" ? "recommended" : search.sort,
    time_range: "all",
    page_size: "24",
  });
  if (search.category) params.set("category", search.category);
  const selectedType = SECONDARY_TYPES_FALLBACK.find((t) => t.key === search.type);
  if (selectedType?.contentType) params.set("content_type", selectedType.contentType);
  if (selectedType?.tags) params.set("tags", selectedType.tags);

  try {
    const res = await fetch(`${apiBase}/contents?${params.toString()}`, { cache: "no-store" });
    if (!res.ok) return [];
    const data = (await res.json()) as ContentResponse;
    return normalizeContentList(data.contents);
  } catch {
    return [];
  }
}

export default async function OriginalPage({ searchParams }: { searchParams: Promise<SearchParams> }) {
  const t = await getTranslations();
  const rawSearch = await searchParams;
  const current = {
    category: rawSearch.category || "",
    type: rawSearch.type || "",
    sort: rawSearch.sort || "hot",
  };
  const apiBase = getApiBase();
  const [categories, secondaryTypes, contents] = await Promise.all([
    fetchPrimaryCategories(apiBase),
    fetchSecondaryTypes(apiBase),
    fetchOriginalContents(apiBase, current),
  ]);

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6">
      {/* Primary category nav */}
      <section className="space-y-3 rounded-md border border-border bg-card p-4 ">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("content.originalZone")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{t("content.originalZoneDesc")}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {categories.map((category) => (
            <Link
              key={category.slug || "recommended"}
              href={buildOriginalHref({ category: category.slug, type: "" }, current)}
              className={buttonVariants({
                size: "sm",
                variant: current.category === category.slug ? "default" : "outline",
              })}
            >
              {category.i18n ? t(category.i18n) : category.label}
            </Link>
          ))}
        </div>
      </section>

      {/* Content stream with secondary nav + sort */}
      <section className="space-y-4 rounded-md border border-border bg-card p-4 ">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <h2 className="text-base font-semibold">{t("home.originalContentStream")}</h2>

          {/* Sort tabs */}
          <div className="flex flex-wrap gap-2">
            {SORTS.map((sort) => (
              <Link
                key={sort.key}
                href={buildOriginalHref({ sort: sort.key }, current)}
                className={buttonVariants({
                  size: "sm",
                  variant: current.sort === sort.key ? "default" : "outline",
                })}
              >
                {t(sort.i18n)}
              </Link>
            ))}
          </div>
        </div>

        {/* Secondary type nav */}
        <div className="flex flex-wrap gap-2">
          {secondaryTypes.map((type) => (
            <Link
              key={type.key || "all"}
              href={buildOriginalHref({ type: type.key }, current)}
              className={buttonVariants({
                size: "sm",
                variant: current.type === type.key ? "default" : "outline",
              })}
            >
              {type.i18n ? t(type.i18n) : type.label}
            </Link>
          ))}
        </div>

        <MasonryGrid items={contents} emptyText={t("home.noOriginalContent")} />
      </section>
    </div>
  );
}
