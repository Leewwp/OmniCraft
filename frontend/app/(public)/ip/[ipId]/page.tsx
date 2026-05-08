import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { getTranslations } from 'next-intl/server';
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { ContentCardData } from "@/components/content/ContentCard";
import { IPDetail } from "@/components/ip/IPDetail";
import { normalizeContentList } from "@/lib/content";

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

const baseUrl = process.env.NEXT_PUBLIC_SITE_URL || "https://omnicraft.com";

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

export async function generateMetadata({
  params,
}: {
  params: Promise<{ ipId: string }>;
}): Promise<Metadata> {
  const { ipId } = await params;
  const t = await getTranslations();
  const apiBase = getApiBase();
  const ip = await fetchIP(apiBase, ipId);
  if (!ip) {
    return { title: t('content.ipNotFound') };
  }
  const title = `${ip.name} — ${t('nav.siteName')}`;
  const description = ip.description?.slice(0, 160) || t('content.browseIpContent', { name: ip.name });
  const ogImage = ip.cover_url || `${baseUrl}/og-default.png`;
  return {
    title,
    description,
    openGraph: {
      title,
      description,
      images: [{ url: ogImage, width: 1200, height: 630 }],
      type: "website",
    },
    twitter: { card: "summary_large_image", title, description, images: [ogImage] },
  };
}

export default async function IPDetailPage({
  params,
  searchParams,
}: {
  params: Promise<{ ipId: string }>;
  searchParams: Promise<{ sort?: string }>;
}) {
  const t = await getTranslations();
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
      <section className="space-y-3 rounded-md border border-border bg-card p-4 ">
        <h2 className="text-base font-semibold">{t('content.allContent')}</h2>
        <MasonryGrid items={contents} />
      </section>
    </div>
  );
}
