// 首页（二创区）服务端取数 —— 自 app/(public)/page.tsx 原样抽出（T1 #853）：
// `/fanworks` 与 `/`（登录态）共用同一实现，不复制业务逻辑。
// 注意：本模块保持无 next-intl/server 依赖，使其可在单测环境直接导入。
import type { ContentCardData } from "@/components/content/ContentCard";
import { normalizeContentList } from "@/lib/content";
import { getServerApiBase } from "@/lib/server-api";

export interface IPItem {
  id: number;
  name: string;
  category?: string;
  description?: string;
}

export interface HomePageData {
  ips: IPItem[];
  contents: ContentCardData[];
  total: number | null;
}

interface IPResponse {
  ips?: IPItem[];
}

interface ContentResponse {
  contents?: ContentCardData[];
  total?: number;
}

export async function fetchIPs(apiBase: string): Promise<IPItem[]> {
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

export async function fetchContents(apiBase: string): Promise<{ items: ContentCardData[]; total: number | null }> {
  try {
    /* #410 F2：首屏 = 每页 = 12 条（2026-09-07 全局裁决）。 */
    const res = await fetch(
      `${apiBase}/contents?zone=fanwork&sort=hot&time_range=all&page=1&page_size=12`,
      {
        cache: "no-store",
      }
    );
    if (!res.ok) {
      return { items: [], total: null };
    }
    const data = (await res.json()) as ContentResponse;
    return {
      items: normalizeContentList(data.contents),
      total: typeof data.total === "number" ? data.total : null,
    };
  } catch {
    return { items: [], total: null };
  }
}

export async function fetchHomePageData(): Promise<HomePageData> {
  const apiBase = getServerApiBase();
  const [initialIPs, firstPage] = await Promise.all([
    fetchIPs(apiBase),
    fetchContents(apiBase),
  ]);
  return { ips: initialIPs, contents: firstPage.items, total: firstPage.total };
}
