"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Bookmark, Check, Loader2, Plus, Search, X } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/components/ui/Toast";
import {
  addCollectionItem,
  createCollection,
  listCollections,
  removeCollectionItem,
  type CollectionSummary,
} from "@/lib/collections";
import { cn } from "@/lib/utils";

type CollectionZone = "original" | "fanwork";

interface CollectionPickerProps {
  contentId: number;
  contentTitle: string;
  zone: CollectionZone;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onChanged?: () => void;
}

export function CollectionPicker({
  contentId,
  contentTitle,
  zone,
  open,
  onOpenChange,
  onChanged,
}: CollectionPickerProps) {
  const t = useTranslations();
  const { toast } = useToast();
  const [collections, setCollections] = useState<CollectionSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  const [busyId, setBusyId] = useState<number | null>(null);
  const [creating, setCreating] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [query, setQuery] = useState("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [isPublic, setIsPublic] = useState(false);
  const titleInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!open) return;
    void loadCollections();
  }, [open, zone, contentId]);

  const sameZoneCollections = useMemo(
    () => collections.filter((collection) => collection.zone === zone),
    [collections, zone],
  );

  const filteredCollections = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return sameZoneCollections;
    return sameZoneCollections.filter((collection) => collection.title.toLowerCase().includes(needle));
  }, [query, sameZoneCollections]);

  async function loadCollections() {
    setLoading(true);
    setError(false);
    try {
      const response = await listCollections({ zone, contentItemId: contentId });
      setCollections(response.collections);
    } catch {
      setError(true);
      toast("error", t("collections.picker.errors.load"));
    } finally {
      setLoading(false);
    }
  }

  async function handleAdd(collection: CollectionSummary) {
    if (collection.contains_item) return;
    setBusyId(collection.id);
    try {
      const item = await addCollectionItem(collection.id, contentId);
      setCollections((current) =>
        current.map((entry) =>
          entry.id === collection.id
            ? { ...entry, contains_item: true, item_id: item.id, item_count: (entry.item_count ?? 0) + 1 }
            : entry,
        ),
      );
      onChanged?.();
      toast("success", t("collections.picker.toast.added"));
    } catch {
      toast("error", t("collections.picker.errors.add"));
    } finally {
      setBusyId(null);
    }
  }

  async function handleRemove(collection: CollectionSummary) {
    if (!collection.item_id) return;
    setBusyId(collection.id);
    try {
      await removeCollectionItem(collection.id, collection.item_id);
      setCollections((current) =>
        current.map((entry) =>
          entry.id === collection.id
            ? {
                ...entry,
                contains_item: false,
                item_id: undefined,
                item_count: Math.max(0, (entry.item_count ?? 1) - 1),
              }
            : entry,
        ),
      );
      onChanged?.();
      toast("success", t("collections.picker.toast.removed"));
    } catch {
      toast("error", t("collections.picker.errors.remove"));
    } finally {
      setBusyId(null);
    }
  }

  async function handleCreate() {
    const trimmedTitle = (titleInputRef.current?.value ?? title).trim();
    if (!trimmedTitle) return;
    setCreating(true);
    try {
      const collection = await createCollection({
        title: trimmedTitle,
        description: description.trim(),
        zone,
        is_public: isPublic,
      });
      const item = await addCollectionItem(collection.id, contentId);
      const summary: CollectionSummary = {
        ...collection,
        contains_item: true,
        item_id: item.id,
        item_count: (collection.item_count ?? 0) + 1,
      };
      setCollections((current) => [...current, summary]);
      setTitle("");
      setDescription("");
      setIsPublic(false);
      setShowCreate(false);
      onChanged?.();
      toast("success", t("collections.picker.toast.created"));
    } catch {
      toast("error", t("collections.picker.errors.create"));
    } finally {
      setCreating(false);
    }
  }

  if (!open) return null;

  const showSearch = sameZoneCollections.length >= 10;

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="collection-picker-title"
        className="flex max-h-[85vh] w-full flex-col rounded-t-lg border border-border bg-card shadow-md sm:max-h-[min(70vh,640px)] sm:max-w-[480px] sm:rounded-lg"
      >
        <header className="flex shrink-0 items-start justify-between gap-3 border-b border-border p-4">
          <div className="min-w-0">
            <h2 id="collection-picker-title" className="text-base font-semibold text-foreground">
              {t("collections.picker.title")}
            </h2>
            <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">
              {t("collections.picker.description", { title: contentTitle, zone: t(`collections.picker.zone.${zone}`) })}
            </p>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            onClick={() => onOpenChange(false)}
            aria-label={t("collections.picker.actions.close")}
          >
            <X className="h-4 w-4" />
          </Button>
        </header>

        <div className="min-h-0 flex-1 space-y-3 overflow-y-auto p-4">
          {showSearch && (
            <label className="relative block">
              <span className="sr-only">{t("collections.picker.search.label")}</span>
              <Search className="pointer-events-none absolute left-2 top-2 h-4 w-4 text-muted-foreground" />
              <Input
                role="searchbox"
                aria-label={t("collections.picker.search.label")}
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={t("collections.picker.search.placeholder")}
                className="pl-8"
              />
            </label>
          )}

          {loading && (
            <div className="space-y-2" aria-label={t("collections.picker.states.loading")}>
              {[0, 1, 2, 3, 4].map((item) => (
                <div key={item} className="h-16 rounded-md border border-border bg-muted/30" />
              ))}
            </div>
          )}

          {!loading && error && (
            <div className="rounded-md border border-border bg-card p-4 text-sm">
              <p className="text-muted-foreground">{t("collections.picker.errors.load")}</p>
              <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => void loadCollections()}>
                {t("common.retry")}
              </Button>
            </div>
          )}

          {!loading && !error && filteredCollections.length === 0 && (
            <div className="rounded-md border border-border bg-card p-4 text-sm text-muted-foreground">
              {query ? t("collections.picker.search.empty") : t("collections.picker.states.empty")}
            </div>
          )}

          {!loading && !error && filteredCollections.length > 0 && (
            <div className="space-y-2">
              {filteredCollections.map((collection) => (
                <CollectionPickerRow
                  key={collection.id}
                  collection={collection}
                  busy={busyId === collection.id}
                  onAdd={() => void handleAdd(collection)}
                  onRemove={() => void handleRemove(collection)}
                />
              ))}
            </div>
          )}
        </div>

        <footer className="shrink-0 border-t border-border p-4">
          {showCreate ? (
            <div className="space-y-3">
              <label className="block space-y-1">
                <span className="text-xs font-medium text-foreground">{t("collections.picker.create.title")}</span>
                <input
                  ref={titleInputRef}
                  value={title}
                  onChange={(event) => setTitle(event.target.value)}
                  placeholder={t("collections.picker.create.titlePlaceholder")}
                  disabled={creating}
                  className="h-8 w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-1 text-base outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:bg-input/50 disabled:opacity-50 md:text-sm"
                />
              </label>
              <label className="block space-y-1">
                <span className="text-xs font-medium text-foreground">{t("collections.picker.create.description")}</span>
                <Textarea
                  value={description}
                  onChange={(event) => setDescription(event.target.value)}
                  placeholder={t("collections.picker.create.descriptionPlaceholder")}
                  disabled={creating}
                  rows={2}
                />
              </label>
              <label className="flex items-center gap-2 text-xs text-muted-foreground">
                <Checkbox checked={isPublic} onChange={(event) => setIsPublic(event.target.checked)} disabled={creating} />
                {t("collections.picker.create.isPublic")}
              </label>
              <div className="flex justify-end gap-2">
                <Button type="button" variant="outline" size="sm" onClick={() => setShowCreate(false)} disabled={creating}>
                  {t("common.cancel")}
                </Button>
                <Button type="button" size="sm" onClick={() => void handleCreate()} disabled={creating}>
                  {creating && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                  {t("collections.picker.create.submit")}
                </Button>
              </div>
            </div>
          ) : (
            <Button type="button" variant="outline" className="w-full" onClick={() => setShowCreate(true)}>
              <Plus className="h-4 w-4" />
              {t("collections.picker.actions.new")}
            </Button>
          )}
        </footer>
      </div>
    </div>
  );
}

function CollectionPickerRow({
  collection,
  busy,
  onAdd,
  onRemove,
}: {
  collection: CollectionSummary;
  busy: boolean;
  onAdd: () => void;
  onRemove: () => void;
}) {
  const t = useTranslations();
  const status = collection.is_public ? t("collections.picker.states.public") : t("collections.picker.states.private");

  return (
    <div className="flex min-h-16 items-center gap-3 rounded-md border border-border bg-card p-3">
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-border bg-muted/30">
        <Bookmark className="h-4 w-4 text-muted-foreground" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <p className="truncate text-sm font-medium text-foreground">{collection.title}</p>
          {collection.is_default && (
            <span className="rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
              {t("collections.picker.states.default")}
            </span>
          )}
          {collection.contains_item && (
            <span className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-[11px] text-primary">
              <Check className="h-3 w-3" />
              {t("collections.picker.states.added")}
            </span>
          )}
        </div>
        <p className="mt-1 text-xs text-muted-foreground">
          {status} · {t("collections.picker.states.itemCount", { count: collection.item_count ?? 0 })}
        </p>
      </div>
      {collection.contains_item ? (
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={busy || !collection.item_id}
          onClick={onRemove}
          aria-label={t("collections.picker.actions.removeFrom", { title: collection.title })}
        >
          {busy && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
          {t("collections.picker.actions.remove")}
        </Button>
      ) : (
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={busy}
          onClick={onAdd}
          className={cn(busy && "opacity-80")}
        >
          {busy && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
          {t("collections.picker.actions.add")}
        </Button>
      )}
    </div>
  );
}
