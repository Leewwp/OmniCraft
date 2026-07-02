"use client";

import React, { use, useCallback, useEffect, useMemo, useState } from "react";
import { FolderOpen, FolderPlus, Loader2, RefreshCw } from "lucide-react";
import { useRouter } from "next/navigation";
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
import { silentError } from "@/lib/error-handler";

type PageParams = Promise<{ userId: string }> | { userId: string };
type CollectionZone = "original" | "fanwork";

interface UserCollectionsPageProps {
  params: PageParams;
}

type LoadState =
  | { status: "loading" }
  | { status: "ready"; collections: CollectionSummary[]; total: number }
  | { status: "error" };

type FormState = {
  mode: "create" | "edit";
  zone: CollectionZone;
  collection?: CollectionSummary;
  title: string;
  description: string;
  isPublic: boolean;
};

export default function UserCollectionsPage({ params }: UserCollectionsPageProps) {
  const t = useTranslations();
  const router = useRouter();
  const { user } = useAuth();
  const { toast } = useToast();
  const resolvedParams = unwrapMaybePromise(params);
  const ownerId = Number(resolvedParams.userId);
  const [state, setState] = useState<LoadState>({ status: "loading" });
  const [form, setForm] = useState<FormState | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<CollectionSummary | null>(null);
  const isOwner = user?.id === ownerId;
  const ownerName = isOwner && user?.username ? user.username : t("common.userLabel", { id: ownerId || "-" });

  const load = useCallback(async () => {
    if (!Number.isFinite(ownerId) || ownerId <= 0) {
      setState({ status: "error" });
      return;
    }

    setState((current) => (current.status === "ready" ? current : { status: "loading" }));
    try {
      const response = await listCollections({ ownerId });
      setState({
        status: "ready",
        collections: sortCollections(response.collections),
        total: response.total ?? response.collections.length,
      });
    } catch (error) {
      silentError(error, { component: "UserCollectionsPage", action: "load" });
      toast("error", t("collections.userList.toast.loadFailed"));
      setState({ status: "error" });
    }
  }, [ownerId, t, toast]);

  useEffect(() => {
    void load();
  }, [load]);

  const visibleCount = state.status === "ready" ? state.collections.length : 0;
  const headerCount = state.status === "ready" ? state.total : visibleCount;
  const grouped = useMemo(() => {
    if (state.status !== "ready") return [];
    return state.collections;
  }, [state]);

  function openCreate() {
    setForm({
      mode: "create",
      zone: "original",
      title: "",
      description: "",
      isPublic: false,
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
    });
  }

  async function handleSave() {
    if (!form || !form.title.trim()) return;

    setSaving(true);
    try {
      if (form.mode === "create") {
        await createCollection({
          title: form.title.trim(),
          description: form.description.trim(),
          zone: form.zone,
          is_public: form.isPublic,
        });
        toast("success", t("collections.userList.toast.created"));
      } else if (form.collection) {
        await updateCollection(form.collection.id, {
          title: form.title.trim(),
          description: form.description.trim(),
          is_public: form.isPublic,
        });
        toast("success", t("collections.userList.toast.updated"));
      }
      setForm(null);
      void load();
    } catch (error) {
      silentError(error, { component: "UserCollectionsPage", action: "save" });
      toast("error", t("collections.userList.toast.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!deleteTarget || deleteTarget.is_default) return;

    try {
      await deleteCollection(deleteTarget.id);
      toast("success", t("collections.userList.toast.deleted"));
      setDeleteTarget(null);
      void load();
    } catch (error) {
      silentError(error, { component: "UserCollectionsPage", action: "delete" });
      toast("error", t("collections.userList.toast.deleteFailed"));
    }
  }

  return (
    <main className="mx-auto w-full max-w-[960px] space-y-6 px-4 py-4 md:max-w-[840px] md:px-6 md:py-6 xl:max-w-[960px]">
      <header className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full border border-border bg-muted text-base font-semibold text-muted-foreground">
            {ownerName.slice(0, 1).toUpperCase()}
          </div>
          <div className="min-w-0">
            <h1 className="truncate text-xl font-semibold text-foreground">
              {t("collections.userList.header.title", { name: ownerName })}
            </h1>
            <p className="mt-1 text-sm text-muted-foreground">
              {t("collections.userList.header.subtitle", { count: headerCount })}
            </p>
          </div>
        </div>
        <div className="flex w-full gap-2 md:w-auto">
          <Button type="button" variant="outline" className="flex-1 md:flex-none" onClick={() => void load()}>
            <RefreshCw className="h-4 w-4" />
            {t("collections.userList.actions.refresh")}
          </Button>
          {isOwner && (
            <Button type="button" className="flex-1 md:flex-none" onClick={openCreate}>
              <FolderPlus className="h-4 w-4" />
              {t("collections.userList.actions.create")}
            </Button>
          )}
        </div>
      </header>

      {state.status === "loading" && (
        <div className="grid grid-cols-2 gap-4 md:grid-cols-3">
          {Array.from({ length: 6 }, (_, index) => (
            <Skeleton key={index} className="h-52 rounded-md border border-border" />
          ))}
        </div>
      )}

      {state.status === "error" && (
        <EmptyState
          icon={FolderOpen}
          title={t("collections.userList.error.title")}
          description={t("collections.userList.error.description")}
          action={
            <Button type="button" variant="outline" onClick={() => void load()}>
              {t("common.retry")}
            </Button>
          }
        />
      )}

      {state.status === "ready" && grouped.length === 0 && (
        <EmptyState
          icon={FolderOpen}
          title={isOwner ? t("collections.userList.empty.ownerTitle") : t("collections.userList.empty.visitorTitle")}
          description={
            isOwner
              ? t("collections.userList.empty.ownerDescription")
              : t("collections.userList.empty.visitorDescription")
          }
          action={
            isOwner ? (
              <Button type="button" onClick={openCreate}>
                <FolderPlus className="h-4 w-4" />
                {t("collections.userList.actions.create")}
              </Button>
            ) : undefined
          }
        />
      )}

      {state.status === "ready" && grouped.length > 0 && (
        <section aria-label={t("collections.userList.a11y.grid")}>
          <div className="grid grid-cols-2 gap-4 md:grid-cols-3">
            {grouped.map((collection) => (
              <CollectionCard
                key={collection.id}
                collection={collection}
                isOwner={isOwner}
                onEdit={() => openEdit(collection)}
                onDelete={() => {
                  if (!collection.is_default) setDeleteTarget(collection);
                }}
              />
            ))}
          </div>
        </section>
      )}

      {form && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
          <div role="dialog" aria-modal="true" className="w-full max-w-md rounded-lg border border-border bg-card p-5">
            <h2 className="text-base font-semibold text-foreground">
              {form.mode === "create"
                ? t("collections.userList.form.createTitle")
                : t("collections.userList.form.editTitle")}
            </h2>
            <div className="mt-4 space-y-3">
              {form.mode === "create" && (
                <label className="block space-y-1">
                  <span className="text-xs font-medium text-foreground">{t("collections.userList.form.zone")}</span>
                  <select
                    value={form.zone}
                    onChange={(event) => setForm({ ...form, zone: event.target.value === "fanwork" ? "fanwork" : "original" })}
                    disabled={saving}
                    className="min-h-11 w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring md:min-h-9"
                  >
                    <option value="original">{t("collections.userList.form.original")}</option>
                    <option value="fanwork">{t("collections.userList.form.fanwork")}</option>
                  </select>
                </label>
              )}
              <label className="block space-y-1">
                <span className="text-xs font-medium text-foreground">{t("collections.userList.form.title")}</span>
                <Input value={form.title} onChange={(event) => setForm({ ...form, title: event.target.value })} disabled={saving} />
              </label>
              <label className="block space-y-1">
                <span className="text-xs font-medium text-foreground">{t("collections.userList.form.description")}</span>
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
                {t("collections.userList.form.isPublic")}
              </label>
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
        title={t("collections.userList.delete.title")}
        description={t("collections.userList.delete.description", { title: deleteTarget?.title ?? "" })}
        confirmLabel={t("collections.userList.delete.confirm")}
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

function sortCollections(collections: CollectionSummary[]) {
  return [...collections].sort((left, right) => {
    if (left.is_default !== right.is_default) return left.is_default ? -1 : 1;
    return (left.sort_order ?? 0) - (right.sort_order ?? 0) || left.id - right.id;
  });
}
