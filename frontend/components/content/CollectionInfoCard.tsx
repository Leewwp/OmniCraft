"use client";

import { Edit2, Folder, Lock, Trash2, Unlock } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type CollectionInfoOwner = {
  id?: number;
  username?: string;
  avatar_url?: string | null;
};

export interface CollectionInfoData {
  id: number;
  user_id?: number;
  title: string;
  description?: string | null;
  zone: string;
  is_public: boolean;
  is_default: boolean;
  item_count?: number;
  owner?: CollectionInfoOwner;
  cover_url?: string | null;
}

interface CollectionInfoCardProps {
  collection: CollectionInfoData;
  isOwner?: boolean;
  onEdit?: () => void;
  onDelete?: () => void;
}

export function CollectionInfoCard({
  collection,
  isOwner = false,
  onEdit,
  onDelete,
}: CollectionInfoCardProps) {
  const t = useTranslations();
  const ownerName =
    collection.owner?.username || t("common.userLabel", { id: collection.owner?.id ?? collection.user_id ?? "-" });
  const VisibilityIcon = collection.is_public ? Unlock : Lock;
  const zoneKey = collection.zone === "fanwork" ? "fanwork" : "original";

  return (
    <section className="grid gap-4 rounded-md border border-border bg-card p-4 md:grid-cols-[132px_minmax(0,1fr)_auto] xl:grid-cols-[160px_minmax(0,1fr)_auto]">
      <div className="aspect-video overflow-hidden rounded-md border border-border bg-muted/40 md:aspect-[3/2]">
        {collection.cover_url ? (
          <img src={collection.cover_url} alt={collection.title} className="h-full w-full object-cover" />
        ) : (
          <div aria-hidden="true" className="flex h-full w-full items-center justify-center text-muted-foreground">
            <Folder className="h-8 w-8" />
          </div>
        )}
      </div>

      <div className="min-w-0 space-y-3">
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="min-w-0 text-xl font-semibold text-foreground md:text-2xl">{collection.title}</h1>
            {collection.is_default && (
              <span className="rounded-full border border-border bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                {t("collections.info.default")}
              </span>
            )}
          </div>
          <p className="line-clamp-2 text-sm text-muted-foreground">
            {collection.description || t("collections.info.noDescription")}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
          <span>{t("collections.info.author", { name: ownerName })}</span>
          <span>{t(`collections.info.${zoneKey}`)}</span>
          <span className="inline-flex items-center gap-1">
            <VisibilityIcon className="h-3.5 w-3.5" />
            {collection.is_public ? t("collections.info.public") : t("collections.info.private")}
          </span>
          <span>{t("collections.info.itemCount", { count: collection.item_count ?? 0 })}</span>
        </div>
      </div>

      {isOwner && (
        <div className="flex items-start gap-2 md:justify-end">
          <Button type="button" variant="outline" size="sm" onClick={onEdit} aria-label={t("collections.detail.ownerActions.edit")}>
            <Edit2 className="h-3.5 w-3.5" />
            {t("common.edit")}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={collection.is_default}
            title={collection.is_default ? t("collections.info.defaultDeleteDisabled") : undefined}
            onClick={onDelete}
            aria-label={t("collections.detail.ownerActions.delete")}
            className={cn(!collection.is_default && "text-destructive")}
          >
            <Trash2 className="h-3.5 w-3.5" />
            {t("common.delete")}
          </Button>
        </div>
      )}
    </section>
  );
}
