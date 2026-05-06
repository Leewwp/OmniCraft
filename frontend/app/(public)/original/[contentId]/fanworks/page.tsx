import Link from "next/link";
import { notFound } from "next/navigation";
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
  { key: "", label: "全部" },
  { key: "text", label: "文字", contentType: "text,article,prompt" },
  { key: "image", label: "图片", contentType: "image" },
  { key: "video", label: "视频", contentType: "video" },
  { key: "audio", label: "音频/乐谱", contentType: "audio,sheet_music" },
  { key: "mod", label: "Mod", contentType: "mod" },
  { key: "other", label: "其他", contentType: "other" },
];

const SORTS = [
  { key: "newest", label: "最新" },
  { key: "hot", label: "最热门" },
  { key: "most_views", label: "最多点击" },
  { key: "best_rated", label: "最高好评率" },
];

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
      <section className="space-y-3 rounded-md border border-border bg-card p-4 shadow-none">
        <Link
          href={`/original/${original.id}`}
          className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          返回原创详情
        </Link>
        <div>
          <h1 className="text-2xl font-bold tracking-tight">相关二创</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {original.title} 的相关二创内容，共 {related.total} 个。
          </p>
        </div>
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <h2 className="text-base font-semibold">二创内容流</h2>
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
                {sort.label}
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
              {type.label}
            </Link>
          ))}
        </div>

        <MasonryGrid items={related.contents} emptyText="暂无相关二创" />
      </section>
    </div>
  );
}
