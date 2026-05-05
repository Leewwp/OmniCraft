import { notFound } from "next/navigation";
import { ContentDetail } from "@/components/content/ContentDetail";
import { VersionHistory } from "@/components/content/VersionHistory";

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
  oss_url?: string;
  file_size?: number;
  is_primary?: boolean;
}

interface ContentResponse {
  content?: ContentItem;
  attachments?: Attachment[];
  tags?: Array<{ tag: string } | string>;
}

function getApiBase() {
  const raw = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  return `${raw.replace(/\/$/, "")}/api/v1`;
}

function normalizeTags(tags: ContentResponse["tags"]): string[] {
  if (!tags) return [];
  return tags
    .map((tag) => (typeof tag === "string" ? tag : tag.tag))
    .filter((tag): tag is string => Boolean(tag));
}

async function fetchContent(apiBase: string, contentId: string): Promise<ContentResponse | null> {
  try {
    const res = await fetch(`${apiBase}/contents/${contentId}`, {
      next: { revalidate: 30 },
    });
    if (!res.ok) return null;
    return (await res.json()) as ContentResponse;
  } catch {
    return null;
  }
}

export default async function FanworkContentDetailPage({
  params,
}: {
  params: Promise<{ contentId: string }>;
}) {
  const { contentId } = await params;
  const apiBase = getApiBase();
  const data = await fetchContent(apiBase, contentId);

  if (!data?.content) {
    notFound();
  }

  // Redirect original content to the original detail page
  if (data.content.zone === "original") {
    notFound();
  }

  const content = data.content;
  const tags = normalizeTags(data.tags);
  const contentIdNum = content.id;
  const zone = content.zone || "fanwork";

  // Build the content detail data object
  const detailData = {
    ...content,
    attachments: data.attachments || [],
    tags,
  };

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-6 px-4 py-6">
      <ContentDetail data={detailData} />

      {/* Version History - only for fanwork zone */}
      {zone === "fanwork" && (
        <VersionHistory contentId={contentIdNum} />
      )}
    </div>
  );
}
