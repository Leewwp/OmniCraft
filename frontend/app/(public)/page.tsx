import type { Metadata } from "next";
import { HomePageClient } from "@/components/home/HomePageClient";
import { ContentCardData } from "@/components/content/ContentCard";
import { normalizeContentList } from "@/lib/content";

interface IPItem {
  id: number;
  name: string;
  category?: string;
  description?: string;
}

interface IPResponse {
  ips?: IPItem[];
}

interface ContentResponse {
  contents?: ContentCardData[];
}

function getApiBase() {
  const raw = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  return `${raw.replace(/\/$/, "")}/api/v1`;
}

async function fetchIPs(apiBase: string): Promise<IPItem[]> {
  try {
    const res = await fetch(`${apiBase}/ips?sort=hot&page_size=20`, {
      cache: "no-store",
    });
    if (!res.ok) {
      return [];
    }
    const data = (await res.json()) as IPResponse;
    return data.ips || [];
  } catch {
    return [];
  }
}

async function fetchContents(apiBase: string): Promise<ContentCardData[]> {
  try {
    const res = await fetch(
      `${apiBase}/contents?zone=fanwork&sort=hot&time_range=all&page_size=24`,
      {
        cache: "no-store",
      }
    );
    if (!res.ok) {
      return [];
    }
    const data = (await res.json()) as ContentResponse;
    return normalizeContentList(data.contents);
  } catch {
    return [];
  }
}

export const metadata: Metadata = {
  title: "OmniCraft 万象工坊 — 全民创意分享平台",
  description: "二创区、原创区 — 发现、分享、共创你的热爱。Mod 下载与部署、乐谱分享、AI 辅助创作。",
  openGraph: {
    title: "OmniCraft 万象工坊 — 全民创意分享平台",
    description: "发现、分享、共创你的热爱。",
    images: [{ url: `${process.env.NEXT_PUBLIC_SITE_URL || "https://omnicraft.com"}/og-default.png`, width: 1200, height: 630 }],
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "OmniCraft 万象工坊",
    description: "发现、分享、共创你的热爱。",
  },
};

export default async function HomePage() {
  const apiBase = getApiBase();
  const [initialIPs, initialContents] = await Promise.all([
    fetchIPs(apiBase),
    fetchContents(apiBase),
  ]);

  return (
    <HomePageClient
      apiBase={apiBase}
      initialIPs={initialIPs}
      initialContents={initialContents}
    />
  );
}
