import Link from "next/link";
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
  { slug: "", label: "推荐" },
  { slug: "film_tv", label: "影视" },
  { slug: "gaming", label: "游戏" },
  { slug: "literature", label: "文学" },
  { slug: "pet", label: "宠物" },
  { slug: "food", label: "美食" },
  { slug: "beauty_fashion", label: "美妆穿搭" },
  { slug: "home", label: "家居" },
  { slug: "tech_digital", label: "数码科技" },
  { slug: "travel", label: "旅行" },
  { slug: "sports", label: "运动" },
  { slug: "productivity", label: "效率" },
];

const SECONDARY_TYPES = [
  { key: "", label: "全部" },
  { key: "image", label: "图片", contentType: "image" },
  { key: "video", label: "视频", contentType: "video" },
  { key: "audio", label: "音频/乐谱", contentType: "audio,sheet_music" },
  { key: "text", label: "文字", contentType: "article,prompt,text" },
  { key: "template", label: "效率模板", contentType: "template" },
  { key: "model_design", label: "模型与设计", tags: "3D模型" },
  { key: "other", label: "其他", contentType: "other" },
];

const SORTS = [
  { key: "hot", label: "最热门" },
  { key: "newest", label: "最新" },
  { key: "most_views", label: "最多点击" },
  { key: "best_rated", label: "最高好评率" },
];

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
          <h1 className="text-2xl font-bold tracking-tight">原创区</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            浏览创作者发布的原创内容。原创区不包含 IP 归属和 PR 协同修改入口。
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
              {category.label}
            </Link>
          ))}
        </div>
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <h2 className="text-base font-semibold">原创内容流</h2>
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
                {sort.label}
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
              {type.label}
            </Link>
          ))}
        </div>

        <MasonryGrid items={contents} emptyText="暂无原创内容" />
      </section>
    </div>
  );
}
