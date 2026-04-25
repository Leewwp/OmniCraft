import { HomePageClient } from "@/components/home/HomePageClient";
import { ContentCardData } from "@/components/content/ContentCard";

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
      next: { revalidate: 30 },
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
        next: { revalidate: 30 },
      }
    );
    if (!res.ok) {
      return [];
    }
    const data = (await res.json()) as ContentResponse;
    return data.contents || [];
  } catch {
    return [];
  }
}

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
