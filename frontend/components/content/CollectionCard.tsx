"use client";

import Link from "next/link";
import { Edit2, Eye, Lock, Trash2, Unlock } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

interface CollectionCardData {
  id: number;
  title: string;
  description?: string | null;
  is_public?: boolean;
  is_default?: boolean;
  item_count?: number;
  cover_url?: string | null;
  created_at?: string;
}

interface CollectionCardProps {
  className?: string;
  collection: CollectionCardData;
  isOwner?: boolean;
  isLoading?: boolean;
  onEdit?: () => void;
  onDelete?: () => void;
}

export function CollectionCard({
  className,
  collection,
  isOwner = false,
  isLoading = false,
  onEdit,
  onDelete,
}: CollectionCardProps) {
  const t = useTranslations();

  if (isLoading) {
    return (
      <div className={cn("rounded-md border border-border bg-card p-3", className)}>
        <Skeleton className="aspect-[3/2] w-full rounded-md" />
        <Skeleton className="mt-3 h-4 w-2/3" />
        <Skeleton className="mt-2 h-3 w-1/2" />
      </div>
    );
  }

  const VisibilityIcon = collection.is_public ? Unlock : Lock;

  return (
    <div className={cn("group relative rounded-md border border-border bg-card transition-colors hover:border-foreground/30", className)}>
      <Link href={`/collections/${collection.id}`} className="block overflow-hidden rounded-md">
        <div className="aspect-[3/2] w-full border-b border-border bg-muted/30">
          {collection.cover_url ? (
            <img src={collection.cover_url} alt="" className="h-full w-full object-cover" />
          ) : (
            <div className="flex h-full w-full items-center justify-center text-muted-foreground">
              <Eye className="h-8 w-8" />
            </div>
          )}
        </div>
        <div className="space-y-2 p-3">
          <div className="flex items-start justify-between gap-2">
            <h3 className="line-clamp-1 text-sm font-medium text-foreground">{collection.title}</h3>
            {collection.is_default && (
              <span className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
                {t("collections.card.default")}
              </span>
            )}
          </div>
          {collection.description && (
            <p className="line-clamp-2 min-h-8 text-xs text-muted-foreground">{collection.description}</p>
          )}
          <div className="flex items-center gap-3 text-xs text-muted-foreground">
            <span>{t("collections.card.itemCount", { count: collection.item_count ?? 0 })}</span>
            <span className="inline-flex items-center gap-1">
              <VisibilityIcon className="h-3 w-3" />
              {collection.is_public ? t("collections.card.public") : t("collections.card.private")}
            </span>
          </div>
        </div>
      </Link>

      {isOwner && (
        <div className="absolute right-2 top-2 flex gap-1 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100">
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              onEdit?.();
            }}
            aria-label={t("collections.card.edit", { title: collection.title })}
          >
            <Edit2 className="h-3.5 w-3.5" />
          </Button>
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            disabled={collection.is_default}
            title={collection.is_default ? t("collections.card.deleteDisabled") : undefined}
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              if (!collection.is_default) onDelete?.();
            }}
            aria-label={t("collections.card.delete", { title: collection.title })}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      )}
    </div>
  );
}
