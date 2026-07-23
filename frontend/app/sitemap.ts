import { getServerApiBase } from "@/lib/server-api";
import type { MetadataRoute } from "next";


const baseUrl = process.env.NEXT_PUBLIC_SITE_URL || "https://omnicraft.com";

interface ContentItem {
  id: number;
  zone?: string;
  updated_at?: string;
}

interface IPItem {
  slug: string;
  updated_at?: string;
}

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const apiBase = getServerApiBase();
  const entries: MetadataRoute.Sitemap = [];

  // Static pages
  entries.push(
    { url: baseUrl, lastModified: new Date(), changeFrequency: "daily", priority: 1.0 },
    { url: `${baseUrl}/original`, lastModified: new Date(), changeFrequency: "daily", priority: 0.9 },
    { url: `${baseUrl}/home`, lastModified: new Date(), changeFrequency: "weekly", priority: 0.5 },
  );

  // Published content pages
  try {
    const res = await fetch(`${apiBase}/contents?zone=original&page=1&page_size=1000&sort=newest`);
    if (res.ok) {
      const data = await res.json();
      const contents = (data.contents || []) as ContentItem[];
      for (const c of contents) {
        entries.push({
          url: `${baseUrl}/original/${c.id}`,
          lastModified: c.updated_at ? new Date(c.updated_at) : new Date(),
          changeFrequency: "weekly" as const,
          priority: 0.8,
        });
      }
    }
  } catch { /* skip on build — will be regenerated on next deploy */ }

  try {
    const res2 = await fetch(`${apiBase}/contents?zone=fanwork&page=1&page_size=1000&sort=newest`);
    if (res2.ok) {
      const data2 = await res2.json();
      const fanworks = (data2.contents || []) as ContentItem[];
      for (const c of fanworks) {
        entries.push({
          url: `${baseUrl}/content/${c.id}`,
          lastModified: c.updated_at ? new Date(c.updated_at) : new Date(),
          changeFrequency: "weekly" as const,
          priority: 0.8,
        });
      }
    }
  } catch { /* skip */ }

  // IP pages
  try {
    const res3 = await fetch(`${apiBase}/ips?page=1&page_size=500`);
    if (res3.ok) {
      const data3 = await res3.json();
      const ips = (data3.ips || []) as IPItem[];
      for (const ip of ips) {
        entries.push({
          url: `${baseUrl}/ip/${ip.slug}`,
          lastModified: ip.updated_at ? new Date(ip.updated_at) : new Date(),
          changeFrequency: "weekly" as const,
          priority: 0.7,
        });
      }
    }
  } catch { /* skip */ }

  return entries;
}
