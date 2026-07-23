import { getBrowserApiBase, getServerApiBase } from "@/lib/server-api";
import type { Metadata } from "next";
import { getTranslations } from 'next-intl/server';
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

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations();
  const siteUrl = process.env.NEXT_PUBLIC_SITE_URL || "https://omnicraft.com";
  return {
    title: t('home.heroSubtitle'),
    description: t('home.heroDescription'),
    openGraph: {
      title: t('home.heroSubtitle'),
      description: t('home.heroTitle'),
      images: [{ url: `${siteUrl}/og-default.png`, width: 1200, height: 630 }],
      type: "website",
    },
    twitter: {
      card: "summary_large_image",
      title: `OmniCraft ${t('nav.siteName')}`,
      description: t('home.heroTitle'),
    },
  };
}

export default async function HomePage() {
  const apiBase = getServerApiBase();
  const browserApiBase = getBrowserApiBase();
  const [initialIPs, initialContents] = await Promise.all([
    fetchIPs(apiBase),
    fetchContents(apiBase),
  ]);

  return (
    <HomePageClient
      apiBase={browserApiBase}
      initialIPs={initialIPs}
      initialContents={initialContents}
    />
  );
}
