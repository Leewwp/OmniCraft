import { MasonryGrid } from "@/components/content/MasonryGrid";
import { ContentCardData } from "@/components/content/ContentCard";

interface ContentResponse {
  contents?: ContentCardData[];
}

function getApiBase() {
  const raw = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  return `${raw.replace(/\/$/, "")}/api/v1`;
}

async function fetchOriginalContents(apiBase: string): Promise<ContentCardData[]> {
  const params = new URLSearchParams({
    zone: "original",
    sort: "hot",
    time_range: "all",
    page_size: "24",
  });

  try {
    const res = await fetch(`${apiBase}/contents?${params.toString()}`, {
      next: { revalidate: 30 },
    });
    if (!res.ok) {
      return [];
    }
    const data = (await res.json()) as ContentResponse;
    return data.contents || [];
  } catch {
    return [];
  }
}

export default async function OriginalPage() {
  const apiBase = getApiBase();
  const contents = await fetchOriginalContents(apiBase);

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6">
      <section className="space-y-2 rounded-md border border-border bg-card p-4 shadow-none">
        <h1 className="text-2xl font-bold tracking-tight">原创区</h1>
        <p className="text-sm text-muted-foreground">
          浏览创作者发布的原创内容。原创区不包含 IP 归属和 PR 协同修改入口。
        </p>
      </section>

      <section className="space-y-3 rounded-md border border-border bg-card p-4 shadow-none">
        <h2 className="text-base font-semibold">原创内容流</h2>
        <MasonryGrid items={contents} />
      </section>
    </div>
  );
}
