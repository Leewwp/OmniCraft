import { api } from "@/lib/api";

export interface CollectionOwner {
  id: number;
  username: string;
  avatar_url?: string | null;
}

export interface CollectionSummary {
  id: number;
  user_id?: number;
  title: string;
  description?: string | null;
  zone?: string;
  is_public?: boolean;
  is_default?: boolean;
  item_count?: number;
  sort_order?: number;
  cover_url?: string | null;
  created_at?: string;
  updated_at?: string;
  owner?: CollectionOwner;
  contains_item: boolean;
  item_id?: number;
}

export interface CollectionDetail extends Omit<CollectionSummary, "contains_item" | "item_id"> {
  zone: string;
  is_public: boolean;
  is_default: boolean;
  item_count?: number;
  owner?: CollectionOwner;
}

export interface CollectionItem {
  id: number;
  collection_id?: number;
  content_id?: number;
  content_item_id?: number;
  note?: string | null;
  added_at?: string;
  content?: unknown;
  content_item?: unknown;
}

export interface CollectionListParams {
  zone?: string;
  contentItemId?: number;
  ownerId?: number;
}

export interface CollectionListResponse {
  collections: CollectionSummary[];
  total?: number;
  page?: number;
  page_size?: number;
  [key: string]: unknown;
}

export interface CreateCollectionInput {
  title: string;
  description?: string;
  zone: string;
  is_public: boolean;
}

export interface CollectionDetailParams {
  page?: number;
  pageSize?: number;
  contentType?: string;
}

export interface CollectionDetailResponse {
  collection: CollectionDetail;
  items?: CollectionItem[];
  total?: number;
  page?: number;
  page_size?: number;
  [key: string]: unknown;
}

export interface UpdateCollectionInput {
  title?: string;
  description?: string;
  is_public?: boolean;
  sort_order?: number;
}

type RawCollectionSummary = Omit<CollectionSummary, "contains_item" | "item_id"> & {
  contains_item?: unknown;
  item_id?: unknown;
};

type RawCollectionListResponse =
  | RawCollectionSummary[]
  | (Omit<CollectionListResponse, "collections"> & {
      collections?: RawCollectionSummary[];
      items?: RawCollectionSummary[];
    });

type RawCollectionEnvelope = CollectionDetail | { collection: CollectionDetail };
type RawCollectionItemEnvelope = CollectionItem | { item: CollectionItem };

export async function listCollections(params: CollectionListParams = {}): Promise<CollectionListResponse> {
  const path = `/api/v1/collections${queryString([
    ["zone", params.zone],
    ["content_item_id", params.contentItemId],
    ["owner_id", params.ownerId],
  ])}`;
  const response = await api.get<RawCollectionListResponse>(path);
  const collections = Array.isArray(response) ? response : response.collections ?? response.items ?? [];

  if (Array.isArray(response)) {
    return { collections: collections.map(normalizeCollectionSummary) };
  }

  const { items: _items, collections: _collections, ...rest } = response;
  return {
    ...rest,
    collections: collections.map(normalizeCollectionSummary),
  };
}

export async function createCollection(input: CreateCollectionInput): Promise<CollectionDetail> {
  const response = await api.post<RawCollectionEnvelope>("/api/v1/collections", input);
  return unwrapCollection(response);
}

export function getCollection(id: number, params: CollectionDetailParams = {}): Promise<CollectionDetailResponse> {
  const path = `/api/v1/collections/${id}${queryString([
    ["page", params.page],
    ["page_size", params.pageSize],
    ["content_type", params.contentType],
  ])}`;
  return api.get<CollectionDetailResponse>(path);
}

export async function addCollectionItem(
  collectionId: number,
  contentItemId: number,
  note?: string
): Promise<CollectionItem> {
  const response = await api.post<RawCollectionItemEnvelope>(`/api/v1/collections/${collectionId}/items`, {
    content_item_id: contentItemId,
    ...(note !== undefined ? { note } : {}),
  });
  return unwrapCollectionItem(response);
}

export function removeCollectionItem(collectionId: number, itemId: number): Promise<void> {
  return api.delete<void>(`/api/v1/collections/${collectionId}/items/${itemId}`);
}

export async function updateCollection(id: number, patch: UpdateCollectionInput): Promise<CollectionDetail> {
  const response = await api.put<RawCollectionEnvelope>(`/api/v1/collections/${id}`, patch);
  return unwrapCollection(response);
}

export function deleteCollection(id: number): Promise<void> {
  return api.delete<void>(`/api/v1/collections/${id}`);
}

function queryString(entries: Array<[string, string | number | undefined]>): string {
  const params = new URLSearchParams();
  for (const [key, value] of entries) {
    if (value !== undefined && value !== "") {
      params.set(key, String(value));
    }
  }

  const serialized = params.toString();
  return serialized ? `?${serialized}` : "";
}

function normalizeCollectionSummary(raw: RawCollectionSummary): CollectionSummary {
  const { contains_item: rawContainsItem, item_id: rawItemId, ...summary } = raw;
  const itemId = typeof rawItemId === "number" && Number.isFinite(rawItemId) ? rawItemId : undefined;

  return {
    ...summary,
    contains_item: rawContainsItem === true || rawContainsItem === 1 || itemId !== undefined,
    ...(itemId !== undefined ? { item_id: itemId } : {}),
  };
}

function unwrapCollection(response: RawCollectionEnvelope): CollectionDetail {
  if ("collection" in response) {
    return response.collection;
  }
  return response;
}

function unwrapCollectionItem(response: RawCollectionItemEnvelope): CollectionItem {
  if ("item" in response) {
    return response.item;
  }
  return response;
}
