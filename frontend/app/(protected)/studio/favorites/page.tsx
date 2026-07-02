"use client";

import { useEffect, useMemo, useState } from "react";
import { FolderPlus, Loader2, RefreshCw } from "lucide-react";
import { useTranslations } from "next-intl";
import { CollectionCard } from "@/components/content/CollectionCard";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/components/ui/Toast";
import { useAuth } from "@/contexts/AuthContext";
import {
  createCollection,
  deleteCollection,
  listCollections,
  updateCollection,
  type CollectionSummary,
} from "@/lib/collections";

type CollectionZone = "original" | "fanwork";

type FormState = {
  mode: "create" | "edit";
  zone: CollectionZone;
  collection?: CollectionSummary;
  title: string;
  description: string;
  isPublic: boolean;
  sortOrder: number;
};

const zones: CollectionZone[] = ["original", "fanwork"];

export default function StudioFavoritesPage() {
  const t = useTranslations();
  const { user } = useAuth();
  const { toast } = useToast();
  const [collections, setCollections] = useState<CollectionSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(false);
  const [form, setForm] = useState<FormState | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<CollectionSummary | null>(null);

  useEffect(() => {
    if (!user) return;
    void loadCollections();
  }, [user]);

  const grouped = useMemo(
    () => ({
      original: sortCollections(collections.filter((collection) => collection.zone === "original")),
      fanwork: sortCollections(collections.filter((collection) => collection.zone === "fanwork")),
    }),
    [collections],
  );

  async function loadCollections() {
    setLoading(true);
    setError(false);
    try {
      const response = await listCollections();
      setCollections(response.collections);
    } catch {
      setError(true);
      toast("error", t("studio.favorites.toast.loadFailed"));
    } finally {
      setLoading(false);
    }
  }

  function openCreate(zone: CollectionZone) {
    setForm({
      mode: "create",
      zone,
      title: "",
      description: "",
      isPublic: false,
      sortOrder: 0,
    });
  }

  function openEdit(collection: CollectionSummary) {
    setForm({
      mode: "edit",
      zone: collection.zone === "fanwork" ? "fanwork" : "original",
      collection,
      title: collection.title,
      description: collection.description ?? "",
      isPublic: collection.is_public ?? false,
      sortOrder: collection.sort_order ?? 0,
    });
  }

  async function handleSave() {
    if (!form || !form.title.trim()) return;
    setSaving(true);
    try {
      if (form.mode === "create") {
        const created = await createCollection({
          title: form.title.trim(),
          description: form.description.trim(),
          zone: form.zone,
          is_public: form.isPublic,
        });
        setCollections((current) => [
          ...current,
          { ...created, contains_item: false, item_count: created.item_count ?? 0 },
        ]);
        toast("success", t("studio.favorites.toast.created"));
      } else if (form.collection) {
        const updated = await updateCollection(form.collection.id, {
          title: form.title.trim(),
          description: form.description.trim(),
          is_public: form.isPublic,
          sort_order: form.sortOrder,
        });
        setCollections((current) =>
          current.map((collection) =>
            collection.id === form.collection?.id
              ? { ...collection, ...updated, contains_item: collection.contains_item, item_id: collection.item_id }
              : collection,
          ),
        );
        toast("success", t("studio.favorites.toast.updated"));
      }
      setForm(null);
      void loadCollections();
    } catch {
      toast("error", t("studio.favorites.toast.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!deleteTarget || deleteTarget.is_default) return;
    try {
      await deleteCollection(deleteTarget.id);
      setCollections((current) => current.filter((collection) => collection.id !== deleteTarget.id));
      toast("success", t("studio.favorites.toast.deleted"));
      setDeleteTarget(null);
      void loadCollections();
    } catch {
      toast("error", t("studio.favorites.toast.deleteFailed"));
    }
  }

  return (
    <div className="w-full max-w-[1280px] space-y-8">
      <header className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-xl font-bold text-foreground">{t("studio.favorites.header.title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("studio.favorites.header.subtitle")}</p>
        </div>
        <Button type="button" variant="outline" onClick={() => void loadCollections()} disabled={loading}>
          <RefreshCw className="h-4 w-4" />
          {t("studio.favorites.actions.refresh")}
        </Button>
      </header>

      {error && !loading && (
        <EmptyState
          icon={FolderPlus}
          title={t("studio.favorites.error.title")}
          description={t("studio.favorites.error.description")}
          action={
            <Button type="button" variant="outline" onClick={() => void loadCollections()}>
              {t("common.retry")}
            </Button>
          }
        />
      )}

      {!error && (
        <div className="space-y-8">
          {zones.map((zone) => (
            <section key={zone} aria-labelledby={`studio-favorites-${zone}`} className="space-y-3">
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <h2 id={`studio-favorites-${zone}`} className="text-base font-semibold text-foreground">
                    {t(`studio.favorites.zone.${zone}.title`)}
                  </h2>
                  <p className="text-xs text-muted-foreground">{t(`studio.favorites.zone.${zone}.description`)}</p>
                </div>
                <Button type="button" className="w-full sm:w-auto" onClick={() => openCreate(zone)}>
                  <FolderPlus className="h-4 w-4" />
                  {t("studio.favorites.actions.create")}
                </Button>
              </div>

              {loading ? (
                <div className="grid grid-cols-2 gap-4 md:grid-cols-2 xl:grid-cols-3">
                  {[0, 1, 2].map((item) => (
                    <Skeleton key={item} className="h-52 rounded-md border border-border" />
                  ))}
                </div>
              ) : grouped[zone].length === 0 ? (
                <EmptyState
                  icon={FolderPlus}
                  title={t("studio.favorites.empty.zoneTitle")}
                  description={t("studio.favorites.empty.zoneDescription")}
                  action={
                    <Button type="button" onClick={() => openCreate(zone)}>
                      {t("studio.favorites.actions.create")}
                    </Button>
                  }
                />
              ) : (
                <div className="grid grid-cols-2 gap-4 md:grid-cols-2 xl:grid-cols-3" aria-labelledby={`studio-favorites-${zone}`}>
                  {grouped[zone].map((collection) => (
                    <CollectionCard
                      key={collection.id}
                      collection={collection}
                      isOwner
                      onEdit={() => openEdit(collection)}
                      onDelete={() => {
                        if (!collection.is_default) setDeleteTarget(collection);
                      }}
                    />
                  ))}
                </div>
              )}
            </section>
          ))}
        </div>
      )}

      {form && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
          <div role="dialog" aria-modal="true" className="w-full max-w-md rounded-lg border border-border bg-card p-5 shadow-md">
            <h2 className="text-base font-semibold text-foreground">
              {form.mode === "create" ? t("studio.favorites.form.createTitle") : t("studio.favorites.form.editTitle")}
            </h2>
            <div className="mt-4 space-y-3">
              <label className="block space-y-1">
                <span className="text-xs font-medium text-foreground">{t("studio.favorites.form.title")}</span>
                <Input value={form.title} onChange={(event) => setForm({ ...form, title: event.target.value })} disabled={saving} />
              </label>
              <label className="block space-y-1">
                <span className="text-xs font-medium text-foreground">{t("studio.favorites.form.description")}</span>
                <Textarea
                  value={form.description}
                  onChange={(event) => setForm({ ...form, description: event.target.value })}
                  disabled={saving}
                  rows={3}
                />
              </label>
              <label className="flex items-center gap-2 text-xs text-muted-foreground">
                <Checkbox
                  checked={form.isPublic}
                  onChange={(event) => setForm({ ...form, isPublic: event.target.checked })}
                  disabled={saving}
                />
                {t("studio.favorites.form.isPublic")}
              </label>
              {form.mode === "edit" && (
                <label className="block space-y-1">
                  <span className="text-xs font-medium text-foreground">{t("studio.favorites.form.sortOrder")}</span>
                  <Input
                    type="number"
                    value={form.sortOrder}
                    onChange={(event) => setForm({ ...form, sortOrder: Number(event.target.value) })}
                    disabled={saving}
                  />
                </label>
              )}
            </div>
            <div className="mt-5 flex justify-end gap-2">
              <Button type="button" variant="outline" onClick={() => setForm(null)} disabled={saving}>
                {t("common.cancel")}
              </Button>
              <Button type="button" onClick={() => void handleSave()} disabled={saving || !form.title.trim()}>
                {saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                {t("common.save")}
              </Button>
            </div>
          </div>
        </div>
      )}

      <ConfirmModal
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title={t("studio.favorites.delete.title")}
        description={t("studio.favorites.delete.description", { title: deleteTarget?.title ?? "" })}
        confirmLabel={t("studio.favorites.delete.confirm")}
        onConfirm={handleDelete}
      />
    </div>
  );
}

function sortCollections(collections: CollectionSummary[]) {
  return [...collections].sort((left, right) => {
    if (left.is_default !== right.is_default) return left.is_default ? -1 : 1;
    return (left.sort_order ?? 0) - (right.sort_order ?? 0) || left.id - right.id;
  });
}
