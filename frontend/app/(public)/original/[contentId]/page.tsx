import { getServerApiBase } from "@/lib/server-api";
import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { getTranslations } from 'next-intl/server';
import { ContentDetail } from "@/components/content/ContentDetail";
import { ContentSidebar } from "@/components/content/ContentSidebar";
import { normalizeContentDetailResponse } from "@/lib/content";

interface ContentResponse { content?: unknown; attachments?: unknown[]; tags?: unknown[]; }
interface RelatedResponse { total?: number; }

const baseUrl = process.env.NEXT_PUBLIC_SITE_URL || "https://omnicraft.com";


async function fetchContent(apiBase: string, contentId: string): Promise<ContentResponse | null> {
  try {
    const res = await fetch(`${apiBase}/contents/${contentId}`, { cache: "no-store" });
    if (!res.ok) return null;
    return (await res.json()) as ContentResponse;
  } catch { return null; }
}

async function fetchRelatedCount(apiBase: string, contentId: string) {
  try {
    const res = await fetch(`${apiBase}/contents/${contentId}/related-fanworks?page=1&page_size=1`, { cache: "no-store" });
    if (!res.ok) return 0;
    return ((await res.json()) as RelatedResponse).total || 0;
  } catch { return 0; }
}

export async function generateMetadata({ params }: { params: Promise<{ contentId: string }> }): Promise<Metadata> {
  const { contentId } = await params;
  const t = await getTranslations();
  const rawData = await fetchContent(getServerApiBase(), contentId);
  const content = normalizeContentDetailResponse(rawData).content;
  if (!content || content.zone !== "original") return { title: `${t('content.originalZone')} — OmniCraft` };
  const title = `${content.title} — ${t('content.originalZone')}`;
  const desc = content.description?.slice(0, 160) || `${content.title} - ${t('content.viewOriginal')}`;
  return { title, description: desc, openGraph: { title, description: desc, images: [{ url: content.cover_image_url || `${baseUrl}/og-default.png`, width: 1200, height: 630 }], type: "article" }, twitter: { card: "summary_large_image", title, description: desc, images: [content.cover_image_url || `${baseUrl}/og-default.png`] } };
}

export default async function OriginalDetailPage({ params }: { params: Promise<{ contentId: string }> }) {
  const t = await getTranslations();
  const { contentId } = await params;
  const apiBase = getServerApiBase();
  const [rawData, relatedCount] = await Promise.all([fetchContent(apiBase, contentId), fetchRelatedCount(apiBase, contentId)]);
  const normalized = normalizeContentDetailResponse(rawData);
  const content = normalized.content;
  if (!content || content.zone !== "original") notFound();

  return (
    <div className="mx-auto flex w-full max-w-[1280px] gap-6 px-6 py-6">
      {/* Main content */}
      <div className="flex-1 min-w-0">
        <ContentDetail data={{ ...content, attachments: normalized.attachments, tags: normalized.tags }} />
      </div>

      {/* Right sidebar */}
      <ContentSidebar
        author={content.author ? { id: content.author.id, username: content.author.username } : undefined}
        zone="original"
        originalId={content.id}
        relatedFanworksCount={relatedCount}
      />
    </div>
  );
}
