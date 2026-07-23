import { getBrowserApiBase, getServerApiBase } from "@/lib/server-api";
import type { Metadata } from "next";
import { getTranslations } from 'next-intl/server';
import { IPBrowseClient } from "@/components/ip/IPBrowseClient";

interface IPItem {
  id: number;
  name: string;
  category?: string;
  description?: string;
  cover_url?: string;
  content_count?: number;
}


export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations();
  return {
    title: `${t('ip.title')} — ${t('nav.siteName')}`,
    description: t('ip.browseDescription'),
  };
}

export default async function IPBrowsePage({
  searchParams,
}: {
  searchParams: Promise<{ category?: string; sort?: string; q?: string }>;
}) {
  const apiBase = getServerApiBase();
  const browserApiBase = getBrowserApiBase();
  const raw = await searchParams;
  const category = raw.category || "";
  const sort = raw.sort || "hot";
  const q = raw.q || "";

  const params = new URLSearchParams();
  if (category) params.set("category", category);
  params.set("sort", sort);
  if (q.trim()) params.set("q", q.trim());
  params.set("page", "1");
  params.set("page_size", "24");

  let initialIPs: IPItem[] = [];
  let initialTotal = 0;

  try {
    const res = await fetch(`${apiBase}/ips?${params.toString()}`, { cache: "no-store" });
    if (res.ok) {
      const data = await res.json() as { ips?: IPItem[]; total?: number };
      initialIPs = data.ips || [];
      initialTotal = data.total || 0;
    }
  } catch { /* use empty */ }

  return (
    <IPBrowseClient
      apiBase={browserApiBase}
      initialIPs={initialIPs}
      initialTotal={initialTotal}
    />
  );
}
