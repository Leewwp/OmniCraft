import Link from "next/link";
import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { getTranslations } from 'next-intl/server';
import { ArrowRight, GitBranchPlus } from "lucide-react";
import { ContentDetail } from "@/components/content/ContentDetail";
import { buttonVariants } from "@/components/ui/button";
import {
  normalizeAttachments,
  normalizeContentItem,
  normalizeTags,
  type ContentDetailData,
} from "@/lib/content";

interface ContentResponse {
  content?: unknown;
  attachments?: unknown[];
  tags?: unknown[];
}

interface RelatedResponse {
  total?: number;
}

const baseUrl = process.env.NEXT_PUBLIC_SITE_URL || "https://omnicraft.com";

function getApiBase() {
  const raw = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  return `${raw.replace(/\/$/, "")}/api/v1`;
}

async function fetchContent(apiBase: string, contentId: string): Promise<ContentResponse | null> {
  try {
    const res = await fetch(`${apiBase}/contents/${contentId}`, {
      cache: "no-store",
    });
    if (!res.ok) {
      return null;
    }
    return (await res.json()) as ContentResponse;
  } catch {
    return null;
  }
}

async function fetchRelatedCount(apiBase: string, contentId: string) {
  try {
    const res = await fetch(`${apiBase}/contents/${contentId}/related-fanworks?page=1&page_size=1`, {
      cache: "no-store",
    });
    if (!res.ok) {
      return 0;
    }
    const data = (await res.json()) as RelatedResponse;
    return data.total || 0;
  } catch {
    return 0;
  }
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ contentId: string }>;
}): Promise<Metadata> {
  const { contentId } = await params;
  const t = await getTranslations();
  const apiBase = getApiBase();
  const rawData = await fetchContent(apiBase, contentId);
  const content = normalizeContentItem(rawData?.content);
  if (!content || content.zone !== "original") {
    return { title: `${t('content.originalZone')} — OmniCraft` };
  }
  const title = `${content.title} — ${t('content.originalZone')}`;
  const description = content.description?.slice(0, 160) || `${content.title} - ${t('content.viewOriginal')}`;
  const ogImage = content.cover_image_url || `${baseUrl}/og-default.png`;
  return {
    title,
    description,
    openGraph: {
      title,
      description,
      images: [{ url: ogImage, width: 1200, height: 630 }],
      type: "article",
    },
    twitter: { card: "summary_large_image", title, description, images: [ogImage] },
  };
}

export default async function OriginalDetailPage({
  params,
}: {
  params: Promise<{ contentId: string }>;
}) {
  const t = await getTranslations();
  const { contentId } = await params;
  const apiBase = getApiBase();
  const [rawData, relatedCount] = await Promise.all([
    fetchContent(apiBase, contentId),
    fetchRelatedCount(apiBase, contentId),
  ]);
  const content = normalizeContentItem(rawData?.content);

  if (!content || content.zone !== "original") {
    notFound();
  }

  const detailData: ContentDetailData = {
    ...content,
    attachments: normalizeAttachments(rawData?.attachments),
    tags: normalizeTags(rawData?.tags),
  };

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-6 px-4 py-6">
      <div className="flex flex-wrap items-center gap-2 rounded-md border border-border bg-card p-3 ">
        {relatedCount > 0 ? (
          <Link
            href={`/original/${content.id}/fanworks`}
            className={buttonVariants({ size: "sm", variant: "default" })}
          >
            {t('content.relatedFanworks')}
            <ArrowRight className="h-3.5 w-3.5" />
          </Link>
        ) : null}
        <Link
          href={`/publish?zone=fanwork&source_original_id=${content.id}`}
          className={buttonVariants({ size: "sm", variant: "outline" })}
        >
          <GitBranchPlus className="h-3.5 w-3.5" />
          {t('content.createFanwork')}
        </Link>
      </div>

      <ContentDetail data={detailData} />
    </div>
  );
}
