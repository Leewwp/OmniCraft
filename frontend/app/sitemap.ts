import { getServerApiBase } from "@/lib/server-api";
import { absoluteUrl } from "@/lib/site-url";
import type { MetadataRoute } from "next";

interface ContentItem {
  id: number;
  zone?: string;
  updated_at?: string;
}

interface IPItem {
  slug: string;
  updated_at?: string;
}

/**
 * SP-25 低-41：列表端点在 repo 层把 page_size 钳制到 ≤100（超限静默重置为
 * 20），此前 sitemap 只拉一页 1000/500 → 每类实收 20 条且无告警。现按
 * page_size=100（钳制上限内）以响应 total 为准翻页拉全量；60 页保险上限
 * 防御 total 异常值。
 */
async function fetchAllPages<T>(path: string, listKey: string): Promise<T[]> {
  const apiBase = getServerApiBase();
  const out: T[] = [];
  let page = 1;
  for (;;) {
    const sep = path.includes("?") ? "&" : "?";
    let res: Response;
    try {
      res = await fetch(`${apiBase}${path}${sep}page=${page}&page_size=100`);
    } catch {
      return out;
    }
    if (!res.ok) {
      return out;
    }
    const data = await res.json();
    const items = (data[listKey] || []) as T[];
    out.push(...items);
    const total = typeof data.total === "number" ? data.total : out.length;
    if (items.length === 0 || out.length >= total || page >= 60) {
      return out;
    }
    page++;
  }
}

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const entries: MetadataRoute.Sitemap = [];

  // Static pages
  entries.push(
    { url: absoluteUrl("/"), lastModified: new Date(), changeFrequency: "daily", priority: 1.0 },
    { url: absoluteUrl("/original"), lastModified: new Date(), changeFrequency: "daily", priority: 0.9 },
    { url: absoluteUrl("/home"), lastModified: new Date(), changeFrequency: "weekly", priority: 0.5 },
  );

  // Published original content pages (paged to total, see fetchAllPages)
  const originals = await fetchAllPages<ContentItem>("/contents?zone=original&sort=newest", "contents");
  for (const c of originals) {
    entries.push({
      url: absoluteUrl(`/original/${c.id}`),
      lastModified: c.updated_at ? new Date(c.updated_at) : new Date(),
      changeFrequency: "weekly" as const,
      priority: 0.8,
    });
  }

  // Published fanwork pages
  const fanworks = await fetchAllPages<ContentItem>("/contents?zone=fanwork&sort=newest", "contents");
  for (const c of fanworks) {
    entries.push({
      url: absoluteUrl(`/content/${c.id}`),
      lastModified: c.updated_at ? new Date(c.updated_at) : new Date(),
      changeFrequency: "weekly" as const,
      priority: 0.8,
    });
  }

  // IP pages
  const ips = await fetchAllPages<IPItem>("/ips", "ips");
  for (const ip of ips) {
    entries.push({
      url: absoluteUrl(`/ip/${ip.slug}`),
      lastModified: ip.updated_at ? new Date(ip.updated_at) : new Date(),
      changeFrequency: "weekly" as const,
      priority: 0.7,
    });
  }

  return entries;
}
