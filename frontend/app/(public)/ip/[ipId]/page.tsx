import { getServerApiBase, getBrowserApiBase } from "@/lib/server-api";
import { absoluteUrl } from "@/lib/site-url";
import { buildCreativeWorkJsonLd, canonicalAlternates, serializeJsonLd } from "@/lib/seo";
import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { getTranslations } from 'next-intl/server';
import { IPHubClient } from "@/components/ip/hub/IPHubClient";

interface IPItem {
  id: number;
  name: string;
  description?: string;
  category?: string;
  cover_url?: string;
  tags?: string[];
  follower_count?: number;
  is_following?: boolean;
  /** 票 #950：JSON-LD dateModified 用（GET /ips/:id 已随 model.IP 返回）。 */
  updated_at?: string;
}

interface IPResponse {
  ip?: IPItem;
  stats?: { follower_count?: number; discussion_count?: number; work_count?: number };
}

const baseUrl = absoluteUrl(""); // SP-05 低-41：占位域出清，env 缺失走相对路径

async function fetchIP(apiBase: string, ipId: string): Promise<IPResponse | null> {
  try {
    const res = await fetch(`${apiBase}/ips/${ipId}`, { cache: "no-store" });
    if (!res.ok) {
      return null;
    }
    return (await res.json()) as IPResponse;
  } catch {
    return null;
  }
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ ipId: string }>;
}): Promise<Metadata> {
  const { ipId } = await params;
  const t = await getTranslations();
  const apiBase = getServerApiBase();
  const data = await fetchIP(apiBase, ipId);
  const ip = data?.ip;
  // 票 #950：canonical 取数字 id 形式（/ips/:id 仅收数字 id，站内链接同为
  // /ip/{id}；sitemap 的 slug URL 是既存坏链，不在本票范围）。
  const alternates = canonicalAlternates(`/ip/${ip?.id ?? ipId}`);
  if (!ip) {
    return { title: t('content.ipNotFound'), alternates };
  }
  const title = `${ip.name} — ${t('nav.siteName')}`;
  const description = ip.description?.slice(0, 160) || t('content.browseIpContent', { name: ip.name });
  const ogImage = ip.cover_url || `${baseUrl}/og-default.png`;
  return {
    title,
    description,
    alternates,
    openGraph: {
      title,
      description,
      images: [{ url: ogImage, width: 1200, height: 630 }],
      type: "website",
    },
    twitter: { card: "summary_large_image", title, description, images: [ogImage] },
  };
}

// /ip/[ipId] 贴吧式社区枢纽（#290）：单页三模块（分享/讨论/提案）+ IP 内搜索。
// SSR 仅提供身份区与统计首屏；模块列表由 IPHubClient 客户端拉取。
export default async function IPDetailPage({
  params,
}: {
  params: Promise<{ ipId: string }>;
}) {
  const { ipId } = await params;
  const apiBase = getServerApiBase();
  const data = await fetchIP(apiBase, ipId);
  const ip = data?.ip;
  if (!ip) {
    notFound();
  }

  const stats = {
    follower_count: data?.stats?.follower_count ?? ip.follower_count ?? 0,
    discussion_count: data?.stats?.discussion_count ?? 0,
    work_count: data?.stats?.work_count ?? 0,
  };

  // 票 #950：结构化数据取页面已有数据（不新增接口调用）
  const jsonLd = serializeJsonLd(
    buildCreativeWorkJsonLd({
      name: ip.name,
      url: absoluteUrl(`/ip/${ip.id}`),
      dateModified: ip.updated_at,
    }),
  );

  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: jsonLd }} />
      <IPHubClient ip={ip} stats={stats} apiBase={getBrowserApiBase()} />
    </>
  );
}
