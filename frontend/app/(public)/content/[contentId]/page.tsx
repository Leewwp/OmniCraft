import { getServerApiBase } from "@/lib/server-api";
import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { getTranslations } from 'next-intl/server';
import { ContentDetail } from "@/components/content/ContentDetail";
import { ContentSidebar } from "@/components/content/ContentSidebar";
import { VersionHistory } from "@/components/content/VersionHistory";
import { normalizeContentDetailResponse } from "@/lib/content";

interface ContentItem {
  id: number;
  title: string;
  description?: string;
  body?: string;
  content_type?: string;
  author_id?: number;
  author?: { id: number; username: string; avatar_url?: string };
  zone?: string;
  ip?: { id: number; name: string; slug?: string };
  source_original_id?: number;
  cover_image_url?: string;
  allow_copy?: boolean;
  agent_enabled?: boolean;
  view_count?: number;
  like_count?: number;
  dislike_count?: number;
  status?: string;
  created_at?: string;
  updated_at?: string;
}

interface Attachment {
  id: number;
  file_type?: string;
  mime_type?: string;
  oss_key?: string;
  // Preview-only URL for inline renderers; downloads must go through DownloadButton/backend auth.
  oss_url?: string;
  file_size?: number;
  is_primary?: boolean;
}

interface ContentResponse {
  content?: ContentItem;
  attachments?: Attachment[];
  tags?: Array<{ tag: string } | string>;
  source_original?: { id: number; title: string };
}


async function fetchContent(apiBase: string, contentId: string): Promise<ContentResponse | null> {
  try {
    const res = await fetch(`${apiBase}/contents/${contentId}`, { cache: "no-store" });
    if (!res.ok) return null;
    return (await res.json()) as ContentResponse;
  } catch { return null; }
}

const baseUrl = process.env.NEXT_PUBLIC_SITE_URL || "https://omnicraft.com";

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
  const zone = content.zone || "fanwork";
  const sourceOriginal = data.source_original || (content.source_original_id ? { id: content.source_original_id, title: "" } : null);

  return (
    <div className="mx-auto flex w-full max-w-[1280px] gap-6 px-6 py-6">
      {/* Main content area */}
      <div className="flex-1 min-w-0">
        <ContentDetail
          data={{ ...content, attachments: normalized.attachments, tags }}
        />
        {zone === "fanwork" && <VersionHistory contentId={content.id} />}
      </div>

      {/* Right sidebar */}
      <ContentSidebar
        author={content.author ? { id: content.author.id, username: content.author.username } : undefined}
        authorStats={undefined}
        zone={zone}
        ip={content.ip?.name ? content.ip : undefined}
        sourceOriginal={sourceOriginal}
      />
    </div>
  );
}
