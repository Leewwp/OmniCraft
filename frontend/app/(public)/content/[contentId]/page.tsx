import { notFound } from "next/navigation";
import { Badge } from "@/components/ui/badge";
import { SubmitPREntry } from "@/components/pr/SubmitPREntry";
import { ContentDetailClient } from "@/components/content/ContentDetailClient";

interface ContentItem {
  ID?: number;
  id?: number;
  Title?: string;
  title?: string;
  Description?: string;
  description?: string;
  ContentType?: string;
  content_type?: string;
  AuthorID?: number;
  author_id?: number;
  CoverImageURL?: string;
  cover_image_url?: string;
  AllowCopy?: boolean;
  allow_copy?: boolean;
  Zone?: string;
  zone?: string;
}

interface Attachment {
  id: number;
  file_type?: string;
  mime_type?: string;
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
  if (!tags) {
    return [];
  }
  return tags
    .map((tag) => (typeof tag === "string" ? tag : tag.tag))
    .filter((tag): tag is string => Boolean(tag));
}

async function fetchContent(apiBase: string, contentId: string): Promise<ContentResponse | null> {
  try {
    const res = await fetch(`${apiBase}/contents/${contentId}`, {
      next: { revalidate: 30 },
    });
    if (!res.ok) {
      return null;
    }
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

  const zone = data.content.Zone ?? data.content.zone;
  if (zone === "original") {
    notFound();
  }

  const tags = normalizeTags(data.tags);

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-6 px-4 py-6">
      <article className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <header className="space-y-2">
          <h1 className="text-2xl font-bold tracking-tight">{data.content.Title ?? data.content.title}</h1>
          <p className="text-sm text-muted-foreground">创作者：用户 #{(data.content.AuthorID ?? data.content.author_id) ?? "-"}</p>
          <p className="text-xs text-muted-foreground">类型：{(data.content.ContentType ?? data.content.content_type) || "other"}</p>
        </header>

        {(data.content.CoverImageURL ?? data.content.cover_image_url) ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={data.content.CoverImageURL ?? data.content.cover_image_url}
            alt={data.content.Title ?? data.content.title}
            className="max-h-96 w-full rounded-md border border-border object-cover"
          />
        ) : null}

        <section className="rounded-md border border-border bg-muted/30 p-3">
          <p className="text-sm leading-relaxed text-foreground/90">
            {(data.content.Description ?? data.content.description) || "该内容暂无文字说明。"}
          </p>
        </section>

        {tags.length > 0 ? (
          <section className="flex flex-wrap gap-2">
            {tags.map((tag) => (
              <Badge key={tag} variant="secondary">
                {tag}
              </Badge>
            ))}
          </section>
        ) : null}

        <div className="flex items-center justify-between gap-3 border-t border-border pt-3">
          <p className="text-xs text-muted-foreground">二创内容支持 PR 协同修改流程。</p>
          <SubmitPREntry
            contentId={data.content.ID ?? data.content.id ?? 0}
            zone={data.content.Zone ?? data.content.zone ?? ""}
            allowCopy={data.content.AllowCopy ?? data.content.allow_copy}
            authorId={data.content.AuthorID ?? data.content.author_id ?? 0}
          />
        </div>

        {data.attachments && data.attachments.length > 0 ? (
          <section className="space-y-2 rounded-md border border-border bg-muted/20 p-3">
            <h2 className="text-sm font-semibold">附件</h2>
            <ul className="space-y-1 text-xs text-muted-foreground">
              {data.attachments.map((attachment) => (
                <li key={attachment.id}>
                  {attachment.file_type || "file"} · {attachment.mime_type || "unknown"}
                </li>
              ))}
            </ul>
          </section>
        ) : null}

        <ContentDetailClient contentId={data.content.ID ?? data.content.id ?? 0} authorId={data.content.AuthorID ?? data.content.author_id ?? 0} />
      </article>
    </div>
  );
}
