import { getServerApiBase } from "@/lib/server-api";
import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { getTranslations } from 'next-intl/server';
import { ContentDetailOverlayHost } from "@/components/content/ContentDetailOverlayHost";
import { normalizeContentDetailResponse } from "@/lib/content";

const baseUrl = process.env.NEXT_PUBLIC_SITE_URL || "https://omnicraft.com";

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
  if (!content) return { title: t('content.contentNotFound') };
  const title = `${content.title} — ${t('nav.siteName')}`;
  const desc = content.description?.slice(0, 160) || `${content.title} - ${t('content.viewContent')}`;
  return { title, description: desc, openGraph: { title, description: desc, images: [{ url: content.cover_image_url || `${baseUrl}/og-default.png`, width: 1200, height: 630 }], type: "article" }, twitter: { card: "summary_large_image", title, description: desc, images: [content.cover_image_url || `${baseUrl}/og-default.png`] } };
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
  const sourceOriginal =
    normalized.sourceOriginal ??
    (content.source_original_id ? { id: content.source_original_id, title: "" } : null);

  return (
    <ContentDetailOverlayHost
      content={{ ...content, attachments: normalized.attachments, tags }}
      zone={zone}
      author={content.author ? { id: content.author.id, username: content.author.username } : undefined}
      ip={content.ip?.name ? content.ip : undefined}
      sourceOriginal={sourceOriginal}
    />
  );
}
