import Link from "next/link";
import { notFound } from "next/navigation";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { ContentCardData } from "@/components/content/ContentCard";
import { IPCategoryTabs } from "@/components/ip/IPCategoryTabs";
import { getCategoryLabel } from "@/components/ip/ipCategory";
import { normalizeContentList } from "@/lib/content";

interface IPItem {
  id: number;
  name: string;
}

interface IPResponse {
  ip?: IPItem;
}

interface ContentResponse {
  contents?: ContentCardData[];
}

function getApiBase() {
  const raw = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  return `${raw.replace(/\/$/, "")}/api/v1`;
}

async function fetchIP(apiBase: string, ipId: string): Promise<IPItem | null> {
  try {
    const res = await fetch(`${apiBase}/ips/${ipId}`, { cache: "no-store" });
    if (!res.ok) {
      return null;
    }
    const data = (await res.json()) as IPResponse;
    return data.ip || null;
  } catch {
    return null;
  }
}

async function fetchCategoryContents(
  apiBase: string,
  ipId: string,
  category: string,
  sort: string,
  page: string
) {
  const params = new URLSearchParams({
    ip_id: ipId,
    zone: "fanwork",
    content_type: category,
    sort,
    time_range: "all",
    page,
    page_size: "24",
  });

  try {
    const res = await fetch(`${apiBase}/contents?${params.toString()}`, {
      cache: "no-store",
    });
    if (!res.ok) {
      return [] as ContentCardData[];
    }
    const data = (await res.json()) as ContentResponse;
    return normalizeContentList(data.contents);
  } catch {
    return [] as ContentCardData[];
  }
}

export default async function IPCategoryPage({
  params,
  searchParams,
}: {
  params: Promise<{ ipId: string; category: string }>;
  searchParams: Promise<{ sort?: string; page?: string }>;
}) {
  const { ipId, category } = await params;
  const query = await searchParams;
  const sort = query.sort || "hot";
  const page = query.page || "1";

  const apiBase = getApiBase();
  const [ip, contents] = await Promise.all([
    fetchIP(apiBase, ipId),
    fetchCategoryContents(apiBase, ipId, category, sort, page),
  ]);

  if (!ip) {
    notFound();
  }

  const pageNum = Number.isNaN(Number(page)) ? 1 : Math.max(1, Number(page));

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6">
      <section className="space-y-3 rounded-md border border-border bg-card p-4 shadow-none">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h1 className="text-xl font-bold">{ip.name}</h1>
            <p className="text-sm text-muted-foreground">类目：{getCategoryLabel(category)}</p>
          </div>
          <Link
            href={`/ip/${ipId}/discussions`}
            className="rounded-md border border-border px-3 py-2 text-xs hover:bg-muted"
          >
            讨论区
          </Link>
        </div>

        <IPCategoryTabs ipId={ipId} activeCategory={category} />

        <div className="flex items-center justify-between">
          <select
            defaultValue={sort}
            className="h-9 rounded-md border border-border bg-background px-3 text-sm"
          >
            <option value="hot">最热门</option>
            <option value="most_views">最多点击</option>
            <option value="newest">最新发布</option>
            <option value="best_rated">最高好评率</option>
          </select>
          <span className="text-xs text-muted-foreground">第 {pageNum} 页</span>
        </div>

        <MasonryGrid items={contents} />

        <div className="flex items-center justify-end gap-2 pt-2">
          {pageNum > 1 ? (
            <Link
              href={`/ip/${ipId}/${category}?sort=${sort}&page=${pageNum - 1}`}
              className="rounded-md border border-border px-3 py-2 text-xs hover:bg-muted"
            >
              上一页
            </Link>
          ) : null}
          <Link
            href={`/ip/${ipId}/${category}?sort=${sort}&page=${pageNum + 1}`}
            className="rounded-md border border-border px-3 py-2 text-xs hover:bg-muted"
          >
            下一页
          </Link>
        </div>
      </section>
    </div>
  );
}
