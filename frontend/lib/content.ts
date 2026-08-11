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
  series_memberships?: SeriesMembership[];
  created_at?: string;
  updated_at?: string;
}

export interface AttachmentData {
  id: number;
  file_type?: string;
  mime_type?: string;
  oss_key?: string;
  // Preview-only URL for inline renderers; downloads must go through DownloadButton/backend auth.
  oss_url?: string;
  file_size?: number;
  is_primary?: boolean;
  // #83 媒体元数据：自然宽高与稳定排序（sort_order ASC NULLS LAST, id ASC）。
  // 历史/旧行宽高可能为 NULL，渲染侧使用防御性默认比例。
  width?: number;
  height?: number;
  sort_order?: number;
}

export interface SeriesContentSummary {
  id: number;
  title: string;
}

export interface SeriesMembership {
  series_id: number;
  series_title: string;
  series_zone?: "original" | "fanwork";
  current_index: number;
  total: number;
  previous?: SeriesContentSummary;
  next?: SeriesContentSummary;
}

export interface NormalizedContentDetailResponse {
  content: ContentDetailData | null;
  attachments: AttachmentData[];
  tags: string[];
  series_memberships: SeriesMembership[];
  sourceOriginal?: { id: number; title: string };
  sourceFanwork?: { id: number; title: string };
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

function positiveInteger(value: unknown): number | undefined {
  const parsed = numberValue(value);
  return parsed !== undefined && Number.isInteger(parsed) && parsed > 0 ? parsed : undefined;
}

function nonNegativeInteger(value: unknown): number | undefined {
  const parsed = numberValue(value);
  return parsed !== undefined && Number.isInteger(parsed) && parsed >= 0 ? parsed : undefined;
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

export function normalizeAttachment(value: unknown): AttachmentData | null {
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
    width: positiveInteger(raw.width ?? raw.Width),
    height: positiveInteger(raw.height ?? raw.Height),
    sort_order: nonNegativeInteger(raw.sort_order ?? raw.SortOrder),
  };
}

function normalizeSeriesContentSummary(value: unknown): SeriesContentSummary | undefined {
  const raw = asObject(value);
  if (!raw) {
    return undefined;
  }
  const id = positiveInteger(raw.id ?? raw.ID);
  const title = stringValue(raw.title ?? raw.Title);
  if (!id || !title?.trim()) {
    return undefined;
  }
  return { id, title };
}

function normalizeSeriesMembership(value: unknown): SeriesMembership | null {
  const raw = asObject(value);
  if (!raw) {
    return null;
  }
  const seriesId = positiveInteger(raw.series_id ?? raw.SeriesID);
  const seriesTitle = stringValue(raw.series_title ?? raw.SeriesTitle);
  const rawZone = raw.series_zone ?? raw.SeriesZone;
  const seriesZone = rawZone === "original" || rawZone === "fanwork" ? rawZone : undefined;
  const currentIndex = positiveInteger(raw.current_index ?? raw.CurrentIndex);
  const total = positiveInteger(raw.total ?? raw.Total);
  if (!seriesId || !seriesTitle?.trim() || !currentIndex || !total || currentIndex > total) {
    return null;
  }
  return {
    series_id: seriesId,
    series_title: seriesTitle,
    ...(seriesZone ? { series_zone: seriesZone } : {}),
    current_index: currentIndex,
    total,
    previous: normalizeSeriesContentSummary(raw.previous ?? raw.Previous),
    next: normalizeSeriesContentSummary(raw.next ?? raw.Next),
  };
}

export function normalizeSeriesMemberships(value: unknown): SeriesMembership[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((item) => normalizeSeriesMembership(item))
    .filter((item): item is SeriesMembership => Boolean(item));
}

function normalizeSourceSummary(value: unknown): { id: number; title: string } | undefined {
  const raw = asObject(value);
  if (!raw) {
    return undefined;
  }
  const id = positiveInteger(raw.id ?? raw.ID);
  const title = stringValue(raw.title ?? raw.Title);
  if (!id || !title?.trim()) {
    return undefined;
  }
  return { id, title };
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
    series_memberships: normalizeSeriesMemberships(pick(raw, "series_memberships", "SeriesMemberships")),
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

export function normalizeContentListResponse(value: unknown): ContentCardData[] {
  if (Array.isArray(value)) {
    return normalizeContentList(value);
  }
  const raw = asObject(value);
  if (!raw) {
    return [];
  }
  return normalizeContentList(raw.contents ?? raw.Contents ?? raw.items ?? raw.Items ?? raw.data ?? raw.Data);
}

export function normalizeAttachments(value: unknown): AttachmentData[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((item) => normalizeAttachment(item))
    .filter((item): item is AttachmentData => Boolean(item));
}

export function normalizeContentDetailResponse(value: unknown): NormalizedContentDetailResponse {
  const raw = asObject(value);
  if (!raw) {
    return { content: null, attachments: [], tags: [], series_memberships: [] };
  }

  const content = normalizeContentItem(pick(raw, "content", "Content"));
  const topLevelMemberships = pick<unknown>(raw, "series_memberships", "SeriesMemberships");
  const seriesMemberships =
    topLevelMemberships === undefined
      ? (content?.series_memberships ?? [])
      : normalizeSeriesMemberships(topLevelMemberships);
  const sourceOriginal = normalizeSourceSummary(pick(raw, "source_original", "SourceOriginal"));
  const sourceFanwork = normalizeSourceSummary(pick(raw, "source_fanwork", "SourceFanwork"));

  return {
    content: content ? { ...content, series_memberships: seriesMemberships } : null,
    attachments: normalizeAttachments(pick(raw, "attachments", "Attachments")),
    tags: normalizeTags(pick(raw, "tags", "Tags")),
    series_memberships: seriesMemberships,
    ...(sourceOriginal ? { sourceOriginal } : {}),
    ...(sourceFanwork ? { sourceFanwork } : {}),
  };
}
