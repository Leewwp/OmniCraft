import type { ContentCardData } from "@/components/content/ContentCard";

export interface ContentDetailData extends ContentCardData {
  description?: string;
  body?: string;
  status?: string;
  ip_id?: number;
  ip?: { id?: number; name?: string; slug?: string };
  source_original_id?: number;
  is_public?: boolean;
  allow_copy?: boolean;
  agent_enabled?: boolean;
  dislike_count?: number;
  attachments?: AttachmentData[];
  created_at?: string;
  updated_at?: string;
}

export interface AttachmentData {
  id: number;
  file_type?: string;
  mime_type?: string;
  oss_key?: string;
  oss_url?: string;
  file_size?: number;
  is_primary?: boolean;
}

type RawObject = Record<string, unknown>;

function asObject(value: unknown): RawObject | null {
  return value && typeof value === "object" ? (value as RawObject) : null;
}

function numberValue(value: unknown): number | undefined {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : undefined;
  }
  return undefined;
}

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

function boolValue(value: unknown): boolean | undefined {
  return typeof value === "boolean" ? value : undefined;
}

function pick<T>(raw: RawObject, snake: string, pascal: string): T | undefined {
  return (raw[snake] ?? raw[pascal]) as T | undefined;
}

function normalizeAuthor(value: unknown): ContentCardData["author"] {
  const raw = asObject(value);
  if (!raw) {
    return undefined;
  }
  return {
    id: numberValue(raw.id ?? raw.ID),
    username: stringValue(raw.username ?? raw.Username),
  };
}

function normalizeIP(value: unknown): ContentDetailData["ip"] {
  const raw = asObject(value);
  if (!raw) {
    return undefined;
  }
  const id = numberValue(raw.id ?? raw.ID);
  const name = stringValue(raw.name ?? raw.Name);
  if (!id && !name) {
    return undefined;
  }
  return {
    id,
    name,
    slug: stringValue(raw.slug ?? raw.Slug),
  };
}

function normalizeAttachment(value: unknown): AttachmentData | null {
  const raw = asObject(value);
  if (!raw) {
    return null;
  }
  const id = numberValue(raw.id ?? raw.ID);
  if (!id) {
    return null;
  }
  return {
    id,
    file_type: stringValue(raw.file_type ?? raw.FileType),
    mime_type: stringValue(raw.mime_type ?? raw.MimeType),
    oss_key: stringValue(raw.oss_key ?? raw.OSSKey),
    oss_url: stringValue(raw.oss_url ?? raw.OSSURL),
    file_size: numberValue(raw.file_size ?? raw.FileSize),
    is_primary: boolValue(raw.is_primary ?? raw.IsPrimary),
  };
}

export function normalizeTags(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((item) => {
      if (typeof item === "string") {
        return item;
      }
      const raw = asObject(item);
      return raw ? stringValue(raw.tag ?? raw.Tag) : undefined;
    })
    .filter((item): item is string => Boolean(item));
}

export function normalizeContentItem(value: unknown): ContentDetailData | null {
  const raw = asObject(value);
  if (!raw) {
    return null;
  }

  const id = numberValue(pick(raw, "id", "ID"));
  const title = stringValue(pick(raw, "title", "Title"));
  const zone = stringValue(pick(raw, "zone", "Zone"));
  if (!id || !title || !zone) {
    return null;
  }

  return {
    id,
    title,
    zone,
    description: stringValue(pick(raw, "description", "Description")),
    body: stringValue(pick(raw, "body", "Body")),
    author_id: numberValue(pick(raw, "author_id", "AuthorID")),
    author: normalizeAuthor(pick(raw, "author", "Author")),
    ip_id: numberValue(pick(raw, "ip_id", "IPID")),
    ip: normalizeIP(pick(raw, "ip", "IP")),
    source_original_id: numberValue(pick(raw, "source_original_id", "SourceOriginalID")),
    category: stringValue(pick(raw, "category", "Category")),
    content_type: stringValue(pick(raw, "content_type", "ContentType")),
    cover_image_url: stringValue(pick(raw, "cover_image_url", "CoverImageURL")),
    status: stringValue(pick(raw, "status", "Status")),
    view_count: numberValue(pick(raw, "view_count", "ViewCount")),
    like_count: numberValue(pick(raw, "like_count", "LikeCount")),
    dislike_count: numberValue(pick(raw, "dislike_count", "DislikeCount")),
    comment_count: numberValue(pick(raw, "comment_count", "CommentCount")),
    is_public: boolValue(pick(raw, "is_public", "IsPublic")),
    allow_copy: boolValue(pick(raw, "allow_copy", "AllowCopy")),
    agent_enabled: boolValue(pick(raw, "agent_enabled", "AgentEnabled")),
    created_at: stringValue(pick(raw, "created_at", "CreatedAt")),
    updated_at: stringValue(pick(raw, "updated_at", "UpdatedAt")),
  };
}

export function normalizeContentList(value: unknown): ContentCardData[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((item) => normalizeContentItem(item))
    .filter((item): item is ContentCardData => Boolean(item));
}

export function normalizeAttachments(value: unknown): AttachmentData[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((item) => normalizeAttachment(item))
    .filter((item): item is AttachmentData => Boolean(item));
}
