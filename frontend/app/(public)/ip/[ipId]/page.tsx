import { notFound } from "next/navigation";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { ContentCardData } from "@/components/content/ContentCard";
import { IPDetail } from "@/components/ip/IPDetail";

interface IPItem {
  id: number;
  name: string;
  description?: string;
  category?: string;
  cover_url?: string;
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
    const res = await fetch(`${apiBase}/ips/${ipId}`, { next: { revalidate: 30 } });
    if (!res.ok) {
      return null;
    }
    const data = (await res.json()) as IPResponse;
    return data.ip || null;
  } catch {
    return null;
  }
}

async function fetchContents(apiBase: string, ipId: string, sort: string) {
  const params = new URLSearchParams({
    ip_id: ipId,
    zone: "fanwork",
    sort,
    time_range: "all",
    page_size: "24",
  });
  try {
    const res = await fetch(`${apiBase}/contents?${params.toString()}`, {
      next: { revalidate: 30 },
    });
    if (!res.ok) {
      return [] as ContentCardData[];
    }
    const data = (await res.json()) as ContentResponse;
    return data.contents || [];
  } catch {
    return [] as ContentCardData[];
  }
}

export default async function IPDetailPage({
  params,
  searchParams,
}: {
  params: Promise<{ ipId: string }>;
  searchParams: Promise<{ sort?: string }>;
}) {
  const { ipId } = await params;
  const query = await searchParams;
  const sort = query.sort || "hot";

  const apiBase = getApiBase();
  const [ip, contents] = await Promise.all([
    fetchIP(apiBase, ipId),
    fetchContents(apiBase, ipId, sort),
  ]);

  if (!ip) {
    notFound();
  }

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6">
      <IPDetail ip={ip} />
      <section className="space-y-3 rounded-md border border-border bg-card p-4 shadow-none">
        <h2 className="text-base font-semibold">全部内容</h2>
        <MasonryGrid items={contents} />
      </section>
    </div>
  );
}
