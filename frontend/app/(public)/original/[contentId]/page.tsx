import Link from "next/link";
import { notFound } from "next/navigation";
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

export default async function OriginalDetailPage({
  params,
}: {
  params: Promise<{ contentId: string }>;
}) {
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
      <div className="flex flex-wrap items-center gap-2 rounded-md border border-border bg-card p-3 shadow-none">
        {relatedCount > 0 ? (
          <Link
            href={`/original/${content.id}/fanworks`}
            className={buttonVariants({ size: "sm", variant: "default" })}
          >
            相关二创
            <ArrowRight className="h-3.5 w-3.5" />
          </Link>
        ) : null}
        <Link
          href={`/publish?zone=fanwork&source_original_id=${content.id}`}
          className={buttonVariants({ size: "sm", variant: "outline" })}
        >
          <GitBranchPlus className="h-3.5 w-3.5" />
          基于此原创发布二创
        </Link>
      </div>

      <ContentDetail data={detailData} />
    </div>
  );
}
