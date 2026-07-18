import { api } from "@/lib/api";

export type SeriesZone = "original" | "fanwork";

export interface SeriesOwner {
  id: number;
  username: string;
}

export interface SeriesContent {
  id: number;
  title: string;
  zone: SeriesZone;
  content_type?: string;
  cover_image_url?: string;
  status?: string;
}

export interface SeriesItem {
  id: number;
  sort_order: number;
  content_item_id: number;
  content: SeriesContent;
}

export interface SeriesSummary {
  id: number;
  title: string;
  description: string;
  zone: SeriesZone;
}

export interface SeriesDetail extends SeriesSummary {
  owner: SeriesOwner;
  cover?: string | null;
  cover_content_id?: number;
  item_count: number;
}

export interface SeriesDetailResponse {
  series: SeriesDetail;
  items: SeriesItem[];
}

interface RawObject {
  [key: string]: unknown;
}

function asObject(value: unknown): RawObject | null {
  return value && typeof value === "object" ? (value as RawObject) : null;
}

function numberValue(value: unknown): number | undefined {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : undefined;
  }
  return undefined;
}

function positiveInteger(value: unknown): number | undefined {
  const parsed = numberValue(value);
  return parsed !== undefined && Number.isInteger(parsed) && parsed > 0 ? parsed : undefined;
}

function nonNegativeInteger(value: unknown): number | undefined {
  const parsed = numberValue(value);
  return parsed !== undefined && Number.isInteger(parsed) && parsed >= 0 ? parsed : undefined;
}

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

function pick(raw: RawObject, snake: string, pascal: string): unknown {
  return raw[snake] ?? raw[pascal];
}

function normalizeZone(value: unknown): SeriesZone | undefined {
  return value === "original" || value === "fanwork" ? value : undefined;
}

function normalizeOwner(value: unknown): SeriesOwner | undefined {
  const raw = asObject(value);
  const id = raw ? positiveInteger(raw.id ?? raw.ID) : undefined;
  const username = raw ? stringValue(raw.username ?? raw.Username) : undefined;
  return id && username?.trim() ? { id, username } : undefined;
}

export function normalizeSeriesSummary(value: unknown): SeriesSummary | null {
  const raw = asObject(value);
  if (!raw) return null;
  const id = positiveInteger(pick(raw, "id", "ID"));
  const title = stringValue(pick(raw, "title", "Title"));
  const zone = normalizeZone(pick(raw, "zone", "Zone"));
  if (!id || !title?.trim() || !zone) return null;
  return {
    id,
    title,
    description: stringValue(pick(raw, "description", "Description")) ?? "",
    zone,
  };
}

function normalizeSeriesDetail(value: unknown): SeriesDetail | null {
  const summary = normalizeSeriesSummary(value);
  const raw = asObject(value);
  const owner = raw ? normalizeOwner(pick(raw, "owner", "Owner")) : undefined;
  const itemCount = raw ? nonNegativeInteger(pick(raw, "item_count", "ItemCount")) : undefined;
  if (!summary || !owner || itemCount === undefined) return null;
  const cover = raw ? stringValue(pick(raw, "cover", "Cover")) : undefined;
  const coverContentID = raw ? positiveInteger(pick(raw, "cover_content_id", "CoverContentID")) : undefined;
  return { ...summary, owner, cover: cover ?? null, cover_content_id: coverContentID, item_count: itemCount };
}

export function normalizeSeriesContent(value: unknown): SeriesContent | null {
  const raw = asObject(value);
  if (!raw) return null;
  const id = positiveInteger(pick(raw, "id", "ID"));
  const title = stringValue(pick(raw, "title", "Title"));
  const zone = normalizeZone(pick(raw, "zone", "Zone"));
  if (!id || !title?.trim() || !zone) return null;
  return {
    id,
    title,
    zone,
    content_type: stringValue(pick(raw, "content_type", "ContentType")),
    cover_image_url: stringValue(pick(raw, "cover_image_url", "CoverImageURL")),
    status: stringValue(pick(raw, "status", "Status")),
  };
}

export function normalizeSeriesItem(value: unknown): SeriesItem | null {
  const raw = asObject(value);
  if (!raw) return null;
  const id = positiveInteger(pick(raw, "id", "ID"));
  const sortOrder = nonNegativeInteger(pick(raw, "sort_order", "SortOrder"));
  const contentItemID = positiveInteger(pick(raw, "content_item_id", "ContentItemID"));
  const content = normalizeSeriesContent(pick(raw, "content", "Content"));
  if (!id || sortOrder === undefined || !contentItemID || !content) return null;
  return { id, sort_order: sortOrder, content_item_id: contentItemID, content };
}

export function normalizeSeriesDetailResponse(value: unknown): SeriesDetailResponse | null {
  const raw = asObject(value);
  if (!raw) return null;
  const series = normalizeSeriesDetail(pick(raw, "series", "Series"));
  const rawItems = pick(raw, "items", "Items");
  if (!series || !Array.isArray(rawItems)) return null;
  const items = rawItems
    .map((item) => normalizeSeriesItem(item))
    .filter((item): item is SeriesItem => Boolean(item));
  return { series, items };
}

export async function getSeriesDetail(seriesID: number, options: { manage?: boolean } = {}): Promise<SeriesDetailResponse> {
  if (!Number.isInteger(seriesID) || seriesID <= 0) {
    throw new Error("Invalid series id");
  }
  const response = await api.get<unknown>(`/api/v1/series/${seriesID}${options.manage ? "?manage=true" : ""}`);
  const normalized = normalizeSeriesDetailResponse(response);
  if (!normalized) throw new Error("Invalid series response");
  return normalized;
}

export async function listOwnedSeries(zone?: SeriesZone): Promise<SeriesSummary[]> {
  const query = zone ? `?zone=${encodeURIComponent(zone)}` : "";
  const response = await api.get<unknown>(`/api/v1/series${query}`);
  const raw = asObject(response);
  const items = raw ? pick(raw, "items", "Items") : undefined;
  if (!Array.isArray(items)) return [];
  return items.map(normalizeSeriesSummary).filter((item): item is SeriesSummary => Boolean(item));
}

export async function listSeriesCandidates(zone: SeriesZone, search = ""): Promise<SeriesContent[]> {
  const params = new URLSearchParams({ zone });
  if (search.trim()) params.set("q", search.trim());
  const response = await api.get<unknown>(`/api/v1/series/candidates?${params.toString()}`);
  const raw = asObject(response);
  const items = raw ? pick(raw, "items", "Items") : undefined;
  if (!Array.isArray(items)) return [];
  return items.map(normalizeSeriesContent).filter((item): item is SeriesContent => Boolean(item));
}

export async function createSeries(input: { title: string; description: string; zone: SeriesZone }) {
  return api.post<{ series: SeriesSummary }>("/api/v1/series", input);
}

export async function updateSeries(seriesID: number, patch: { title?: string; description?: string; cover_content_id?: number | null }) {
  return api.put<{ series: SeriesSummary }>(`/api/v1/series/${seriesID}`, patch);
}

export async function deleteSeries(seriesID: number) {
  return api.delete<{ message: string }>(`/api/v1/series/${seriesID}`);
}

export async function addSeriesItem(seriesID: number, contentItemID: number) {
  return api.post<{ item: SeriesItem }>(`/api/v1/series/${seriesID}/items`, { content_item_id: contentItemID });
}

export async function removeSeriesItem(seriesID: number, itemID: number) {
  return api.delete<{ message: string }>(`/api/v1/series/${seriesID}/items/${itemID}`);
}

export async function reorderSeriesItems(seriesID: number, itemIDs: number[]) {
  return api.put<{ message: string }>(`/api/v1/series/${seriesID}/items/reorder`, { item_ids: itemIDs });
}
