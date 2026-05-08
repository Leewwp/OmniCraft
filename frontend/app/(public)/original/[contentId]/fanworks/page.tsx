import Link from "next/link";
import { notFound } from "next/navigation";
import { getTranslations } from 'next-intl/server';
import { ArrowLeft } from "lucide-react";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { buttonVariants } from "@/components/ui/button";
import { normalizeContentItem, normalizeContentList } from "@/lib/content";

interface ContentResponse {
  content?: unknown;
}

interface RelatedResponse {
  contents?: unknown[];
  total?: number;
}

interface SearchParams {
  type?: string;
  sort?: string;
}

const TYPE_FILTERS = [
  { key: "", label: "All", contentType: "text,article,prompt" },
  { key: "text", label: "Text", contentType: "text,article,prompt" },
  { key: "image", label: "Image", contentType: "image" },
  { key: "video", label: "Video", contentType: "video" },
  { key: "audio", label: "Audio/Sheet Music", contentType: "audio,sheet_music" },
  { key: "mod", label: "Mod", contentType: "mod" },
  { key: "other", label: "Other", contentType: "other" },
];

const TYPE_FILTER_LABEL_KEY: Record<string, string> = {
  "": "content.categoryAll",
  text: "content.categoryText",
  image: "content.categoryImage",
  video: "content.categoryVideo",
  audio: "content.categoryAudio",
  mod: "content.categoryMod",
  other: "content.categoryOther",
};

const SORTS = [
  { key: "newest", label: "Newest" },
  { key: "hot", label: "Hottest" },
  { key: "most_views", label: "Most Viewed" },
  { key: "best_rated", label: "Top Rated" },
];

const SORT_LABEL_KEY: Record<string, string> = {
  newest: "content.sortNewest",
  hot: "content.sortHottest",
  most_views: "content.sortMostViewed",
  best_rated: "content.sortTopRated",
};

function getApiBase() {
  const raw = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  return `${raw.replace(/\/$/, "")}/api/v1`;
}

function buildHref(contentId: string, next: Partial<SearchParams>, current: Required<SearchParams>) {
  const merged = { ...current, ...next };
  const params = new URLSearchParams();
  if (merged.type) params.set("type", merged.type);
  if (merged.sort && merged.sort !== "newest") params.set("sort", merged.sort);
  const query = params.toString();
  return query ? `/original/${contentId}/fanworks?${query}` : `/original/${contentId}/fanworks`;
}

async function fetchOriginal(apiBase: string, contentId: string) {
  try {
    const res = await fetch(`${apiBase}/contents/${contentId}`, { cache: "no-store" });
    if (!res.ok) return null;
    const data = (await res.json()) as ContentResponse;
    return normalizeContentItem(data.content);
  } catch {
    return null;
  }
}

async function fetchRelatedFanworks(apiBase: string, contentId: string, search: Required<SearchParams>) {
  const params = new URLSearchParams({
    page: "1",
    page_size: "24",
    sort: search.sort,
  });
  const selectedType = TYPE_FILTERS.find((item) => item.key === search.type);
  if (selectedType?.contentType) {
    params.set("content_type", selectedType.contentType);
  }

  try {
    const res = await fetch(`${apiBase}/contents/${contentId}/related-fanworks?${params.toString()}`, {
      cache: "no-store",
    });
    if (!res.ok) return { contents: [], total: 0 };
    const data = (await res.json()) as RelatedResponse;
    return {
      contents: normalizeContentList(data.contents),
      total: data.total || 0,
    };
  } catch {
    return { contents: [], total: 0 };
  }
}

export default async function RelatedFanworksPage({
  params,
  searchParams,
}: {
  params: Promise<{ contentId: string }>;
  searchParams: Promise<SearchParams>;
}) {
  const t = await getTranslations();
  const { contentId } = await params;
  const rawSearch = await searchParams;
  const current = {
    type: rawSearch.type || "",
    sort: rawSearch.sort || "newest",
  };
  const apiBase = getApiBase();
  const [original, related] = await Promise.all([
    fetchOriginal(apiBase, contentId),
    fetchRelatedFanworks(apiBase, contentId, current),
  ]);

  if (!original || original.zone !== "original") {
    notFound();
  }

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6">
      <section className="space-y-3 rounded-md border border-border bg-card p-4 ">
        <Link
          href={`/original/${original.id}`}
          className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          {t('content.backToOriginal')}
        </Link>
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('content.fanworksTitle')}</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {t('content.fanworksDesc', { title: original.title, total: related.total })}
          </p>
        </div>
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 ">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <h2 className="text-base font-semibold">{t('content.fanworkStream')}</h2>
          <div className="flex flex-wrap gap-2">
            {SORTS.map((sort) => (
              <Link
                key={sort.key}
                href={buildHref(contentId, { sort: sort.key }, current)}
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
          {TYPE_FILTERS.map((type) => (
            <Link
              key={type.key || "all"}
              href={buildHref(contentId, { type: type.key }, current)}
              className={buttonVariants({
                size: "sm",
                variant: current.type === type.key ? "default" : "outline",
              })}
            >
              {t(TYPE_FILTER_LABEL_KEY[type.key])}
            </Link>
          ))}
        </div>

        <MasonryGrid items={related.contents} emptyText={t('content.noFanworks')} />
      </section>
    </div>
  );
}
