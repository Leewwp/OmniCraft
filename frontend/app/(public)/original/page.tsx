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

const PRIMARY_CATEGORIES = [
  { slug: "", label: "Recommended" },
  { slug: "film_tv", label: "Film & TV" },
  { slug: "gaming", label: "Gaming" },
  { slug: "literature", label: "Literature" },
  { slug: "pet", label: "Pets" },
  { slug: "food", label: "Food" },
  { slug: "beauty_fashion", label: "Beauty & Fashion" },
  { slug: "home", label: "Home" },
  { slug: "tech_digital", label: "Tech & Digital" },
  { slug: "travel", label: "Travel" },
  { slug: "sports", label: "Sports" },
  { slug: "productivity", label: "Productivity" },
];

const PRIMARY_CATEGORY_LABEL_KEY: Record<string, string> = {
  "": "home.categoryRecommended",
  film_tv: "home.categoryFilmTv",
  gaming: "home.categoryGaming",
  literature: "home.categoryLiterature",
  pet: "home.categoryPet",
  food: "home.categoryFood",
  beauty_fashion: "home.categoryBeautyFashion",
  home: "home.categoryHome",
  tech_digital: "home.categoryTechDigital",
  travel: "home.categoryTravel",
  sports: "home.categorySports",
  productivity: "home.categoryProductivity",
};

const SECONDARY_TYPES = [
  { key: "", label: "All", contentType: "" },
  { key: "image", label: "Image", contentType: "image" },
  { key: "video", label: "Video", contentType: "video" },
  { key: "audio", label: "Audio/Sheet Music", contentType: "audio,sheet_music" },
  { key: "text", label: "Text", contentType: "article,prompt,text" },
  { key: "template", label: "Templates", contentType: "template" },
  { key: "model_design", label: "Models & Designs", tags: "3D模型" },
  { key: "other", label: "Other", contentType: "other" },
];

const SECONDARY_TYPE_LABEL_KEY: Record<string, string> = {
  "": "content.categoryAll",
  image: "content.categoryImage",
  video: "content.categoryVideo",
  audio: "content.categoryAudio",
  text: "content.categoryText",
  template: "content.categoryTemplate",
  model_design: "content.categoryModel",
  other: "content.categoryOther",
};

const SORTS = [
  { key: "hot", label: "Hottest" },
  { key: "newest", label: "Newest" },
  { key: "most_views", label: "Most Viewed" },
  { key: "best_rated", label: "Top Rated" },
];

const SORT_LABEL_KEY: Record<string, string> = {
  hot: "content.sortHottest",
  newest: "content.sortNewest",
  most_views: "content.sortMostViewed",
  best_rated: "content.sortTopRated",
};

function getApiBase() {
  const raw = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  return `${raw.replace(/\/$/, "")}/api/v1`;
}

function normalizeCategorySlug(slug: string) {
  return slug.endsWith("_orig") ? slug.slice(0, -5) : slug;
}

async function fetchPrimaryCategories(apiBase: string) {
  try {
    const res = await fetch(`${apiBase}/categories?zone=original&level=primary`, {
      cache: "no-store",
    });
    if (!res.ok) {
      return PRIMARY_CATEGORIES;
    }
    const data = (await res.json()) as CategoryResponse;
    const categories = (data.categories || [])
      .map((item) => {
        const slug = normalizeCategorySlug(item.slug);
        const fallback = PRIMARY_CATEGORIES.find((category) => category.slug === slug);
        return {
          slug,
          label: fallback?.label || item.name_i18n?.zh || item.name_i18n?.en || item.slug,
        };
      })
      .filter((item) => PRIMARY_CATEGORIES.some((category) => category.slug === item.slug));
    return [PRIMARY_CATEGORIES[0], ...categories];
  } catch {
    return PRIMARY_CATEGORIES;
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
    sort: search.category ? search.sort : search.sort === "hot" ? "recommended" : search.sort,
    time_range: "all",
    page_size: "24",
  });
  if (search.category) {
    params.set("category", search.category);
  }
  const selectedType = SECONDARY_TYPES.find((item) => item.key === search.type);
  if (selectedType?.contentType) {
    params.set("content_type", selectedType.contentType);
  }
  if (selectedType?.tags) {
    params.set("tags", selectedType.tags);
  }

  try {
    const res = await fetch(`${apiBase}/contents?${params.toString()}`, {
      cache: "no-store",
    });
    if (!res.ok) {
      return [];
    }
    const data = (await res.json()) as ContentResponse;
    return normalizeContentList(data.contents);
  } catch {
    return [];
  }
}

export default async function OriginalPage({
  searchParams,
}: {
  searchParams: Promise<SearchParams>;
}) {
  const t = await getTranslations();
  const rawSearch = await searchParams;
  const current = {
    category: rawSearch.category || "",
    type: rawSearch.type || "",
    sort: rawSearch.sort || "hot",
  };
  const apiBase = getApiBase();
  const [categories, contents] = await Promise.all([
    fetchPrimaryCategories(apiBase),
    fetchOriginalContents(apiBase, current),
  ]);

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6">
      <section className="space-y-3 rounded-md border border-border bg-card p-4 shadow-none">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('content.originalZone')}</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {t('content.originalZoneDesc')}
          </p>
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
              {t(PRIMARY_CATEGORY_LABEL_KEY[category.slug] || category.label)}
            </Link>
          ))}
        </div>
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <h2 className="text-base font-semibold">{t('content.originalContentStream')}</h2>
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
                {t(SORT_LABEL_KEY[sort.key])}
              </Link>
            ))}
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          {SECONDARY_TYPES.map((type) => (
            <Link
              key={type.key || "all"}
              href={buildOriginalHref({ type: type.key }, current)}
              className={buttonVariants({
                size: "sm",
                variant: current.type === type.key ? "default" : "outline",
              })}
            >
              {t(SECONDARY_TYPE_LABEL_KEY[type.key])}
            </Link>
          ))}
        </div>

        <MasonryGrid items={contents} emptyText={t('home.noOriginalContent')} />
      </section>
    </div>
  );
}
