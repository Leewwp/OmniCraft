"use client";

import React, { use, useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { FolderOpen } from "lucide-react";
import { CollectionInfoCard } from "@/components/content/CollectionInfoCard";
import { ContentTypeFilter, isCollectionContentType } from "@/components/content/ContentTypeFilter";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { EmptyState } from "@/components/ui/empty-state";
import { SkeletonCard } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/Toast";
import { useAuth } from "@/contexts/AuthContext";
import { ApiRequestError } from "@/lib/api";
import { deleteCollection, getCollection, type CollectionDetail, type CollectionItem } from "@/lib/collections";
import { normalizeContentItem } from "@/lib/content";
import { silentError } from "@/lib/error-handler";
import { useTranslations } from "next-intl";

type PageParams = Promise<{ id: string }>;
type PageSearchParams = Promise<Record<string, string | string[] | undefined>>;

interface CollectionDetailPageProps {
  params: PageParams;
  searchParams?: PageSearchParams;
}

type LoadState =
  | { status: "loading" }
  | { status: "ready"; collection: CollectionDetail; items: CollectionItem[]; total: number }
  | { status: "not-found" }
  | { status: "error" };

export default function CollectionDetailPage({ params, searchParams }: CollectionDetailPageProps) {
  const t = useTranslations();
  const router = useRouter();
  const { user } = useAuth();
  const { toast } = useToast();
  const resolvedParams = unwrapMaybePromise(params);
  const resolvedSearchParams = unwrapMaybePromise(searchParams ?? {});
  const collectionId = Number(resolvedParams.id);
  const initialContentType = readContentType(resolvedSearchParams.content_type);
  const [contentType, setContentType] = useState(initialContentType);
  const [state, setState] = useState<LoadState>({ status: "loading" });
  const [confirmDelete, setConfirmDelete] = useState(false);

  const load = useCallback(async () => {
    if (!Number.isFinite(collectionId) || collectionId <= 0) {
      setState({ status: "not-found" });
      return;
    }

    setState((current) => (current.status === "ready" ? current : { status: "loading" }));
    try {
      const response = await getCollection(collectionId, {
        page: 1,
        pageSize: 20,
        contentType: contentType === "all" ? undefined : contentType,
      });
      setState({
        status: "ready",
        collection: response.collection,
        items: response.items ?? [],
        total: response.total ?? response.items?.length ?? 0,
      });
    } catch (error) {
      if (
        error instanceof ApiRequestError &&
        (error.status === 403 || error.status === 404 || error.code === "COLLECTION_NOT_FOUND")
      ) {
        setState({ status: "not-found" });
        return;
      }
      silentError(error, { component: "CollectionDetailPage", action: "load" });
      toast("error", t("common.loadFailed"));
      setState({ status: "error" });
    }
  }, [collectionId, contentType, t, toast]);

  useEffect(() => {
    void load();
  }, [load]);

  function handleFilterChange(nextValue: string) {
    const next = isCollectionContentType(nextValue) ? nextValue : "all";
    setContentType(next);
    const url = new URL(window.location.href);
    if (next === "all") {
      url.searchParams.delete("content_type");
    } else {
      url.searchParams.set("content_type", next);
    }
    window.history.replaceState({}, "", `${url.pathname}${url.search}`);
  }

  async function handleDelete() {
    if (state.status !== "ready" || state.collection.is_default) return;
    await deleteCollection(state.collection.id);
    router.push(`/user/${state.collection.user_id ?? ""}/collections`);
  }

  if (state.status === "loading") {
    return (
      <main className="mx-auto w-full max-w-[1280px] space-y-6 px-4 py-6 md:px-6">
        <div className="h-36 rounded-md border border-border bg-muted/30" />
        <div className="h-11 rounded-md border border-border bg-muted/30" />
        <div className="grid grid-cols-2 gap-4 md:grid-cols-3 xl:grid-cols-4">
          {Array.from({ length: 8 }, (_, index) => (
            <SkeletonCard key={index} />
          ))}
        </div>
      </main>
    );
  }

  if (state.status === "not-found") {
    return (
      <main className="mx-auto w-full max-w-[960px] px-4 py-10">
        <EmptyState
          icon={FolderOpen}
          title={t("collections.detail.error.title")}
          description={t("collections.detail.error.description")}
        />
      </main>
    );
  }

  if (state.status === "error") {
    return (
      <main className="mx-auto w-full max-w-[960px] px-4 py-10">
        <EmptyState
          icon={FolderOpen}
          title={t("common.loadFailed")}
          description={t("collections.detail.error.description")}
          action={
            <Button type="button" variant="outline" onClick={() => void load()}>
              {t("common.retry")}
            </Button>
          }
        />
      </main>
    );
  }

  const isOwner = user?.id === state.collection.user_id || user?.id === state.collection.owner?.id;
  const cards = normalizeCollectionItems(state.items);

  return (
    <main className="mx-auto w-full max-w-[1280px] space-y-6 px-4 py-6 md:px-6">
      <CollectionInfoCard
        collection={{ ...state.collection, item_count: state.collection.item_count ?? state.total }}
        isOwner={isOwner}
        onEdit={() => router.push("/studio/favorites")}
        onDelete={() => setConfirmDelete(true)}
      />

      <ContentTypeFilter value={contentType} onChange={handleFilterChange} />

      <section aria-label={t("collections.detail.a11y.grid")}>
        {cards.length === 0 ? (
          <EmptyState
            icon={FolderOpen}
            title={t("collections.detail.empty.title")}
            description={t("collections.detail.empty.description")}
          />
        ) : (
          <MasonryGrid items={cards} emptyText={t("collections.detail.empty.title")} />
        )}
      </section>

      <ConfirmModal
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title={t("collections.detail.ownerActions.deleteTitle")}
        description={t("collections.detail.ownerActions.deleteDescription", { title: state.collection.title })}
        confirmLabel={t("collections.detail.ownerActions.deleteConfirm")}
        onConfirm={handleDelete}
      />
    </main>
  );
}

function unwrapMaybePromise<T>(value: T | Promise<T>): T {
  if (value && typeof value === "object" && "then" in value && typeof value.then === "function") {
    return use(value as Promise<T>);
  }
  return value as T;
}

function readContentType(raw: string | string[] | undefined): string {
  const value = Array.isArray(raw) ? raw[0] : raw;
  if (value && isCollectionContentType(value) && value !== "all") {
    return value;
  }
  if (typeof window !== "undefined") {
    const fromLocation = new URLSearchParams(window.location.search).get("content_type") ?? "";
    if (isCollectionContentType(fromLocation) && fromLocation !== "all") {
      return fromLocation;
    }
  }
  return "all";
}

function normalizeCollectionItems(items: CollectionItem[]) {
  return items
    .map((item) => normalizeContentItem(item.content_item ?? item.content ?? null))
    .filter((item): item is NonNullable<ReturnType<typeof normalizeContentItem>> => Boolean(item));
}
