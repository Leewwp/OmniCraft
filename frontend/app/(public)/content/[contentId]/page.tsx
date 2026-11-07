import { getServerApiBase } from "@/lib/server-api";
import { absoluteUrl } from "@/lib/site-url";
import { buildCreativeWorkJsonLd, canonicalAlternates, serializeJsonLd } from "@/lib/seo";
import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { getTranslations } from 'next-intl/server';
import { ContentDetailOverlayHost } from "@/components/content/ContentDetailOverlayHost";
import { normalizeContentDetailResponse } from "@/lib/content";

const baseUrl = absoluteUrl(""); // SP-05 低-41：占位域出清，env 缺失走相对路径

async function fetchContent(apiBase: string, contentId: string): Promise<unknown> {
  try {
    const res = await fetch(`${apiBase}/contents/${contentId}`, { cache: "no-store" });
    if (!res.ok) return null;
    return (await res.json()) as unknown;
  } catch { return null; }
}

export async function generateMetadata({ params }: { params: Promise<{ contentId: string }> }): Promise<Metadata> {
  const { contentId } = await params;
  const t = await getTranslations();
  const data = await fetchContent(getServerApiBase(), contentId);
  const content = normalizeContentDetailResponse(data).content;
  const alternates = canonicalAlternates(`/content/${contentId}`);
  if (!content) return { title: t('content.contentNotFound'), alternates };
  const title = `${content.title} — ${t('nav.siteName')}`;
  const desc = content.description?.slice(0, 160) || `${content.title} - ${t('content.viewContent')}`;
  return { title, description: desc, alternates, openGraph: { title, description: desc, images: [{ url: content.cover_image_url || `${baseUrl}/og-default.png`, width: 1200, height: 630 }], type: "article" }, twitter: { card: "summary_large_image", title, description: desc, images: [content.cover_image_url || `${baseUrl}/og-default.png`] } };
}

export default async function FanworkContentDetailPage({ params }: { params: Promise<{ contentId: string }> }) {
  const { contentId } = await params;
  const apiBase = getServerApiBase();
  const data = await fetchContent(apiBase, contentId);
  const normalized = normalizeContentDetailResponse(data);
  const content = normalized.content;
  if (!data || !content) notFound();
  if (content.zone === "original") notFound();

  const tags = normalized.tags;
  const zone: "fanwork" = "fanwork";
  // 票 #950：结构化数据取页面已有数据（不新增接口调用）
  const jsonLd = serializeJsonLd(
    buildCreativeWorkJsonLd({
      name: content.title,
      url: absoluteUrl(`/content/${contentId}`),
      dateModified: content.updated_at,
    }),
  );
  const sourceOriginal =
    normalized.sourceOriginal ??
    (content.source_original_id ? { id: content.source_original_id, title: "" } : null);
  const sourceFanwork = normalized.sourceFanwork ?? null;

  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: jsonLd }} />
      <ContentDetailOverlayHost
      content={{ ...content, attachments: normalized.attachments, tags }}
      zone={zone}
      author={
        content.author
          ? {
              id: content.author.id,
              username: content.author.username,
              avatar_url: content.author.avatar_url,
              is_following: content.author.is_following,
            }
          : undefined
      }
      ip={content.ip?.name ? content.ip : undefined}
      sourceOriginal={sourceOriginal}
      sourceFanwork={sourceFanwork}
      relatedFanworks={{
        sourceContentId: content.id,
        sourceZone: "fanwork",
        titleKey: "relatedFanworks.derivatives.title",
        createHref: `/studio/publish/fanwork?source_fanwork_id=${content.id}`,
      }}
    />
    </>
  );
}
