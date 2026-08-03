"use client";

import { Fragment, useEffect, useState, type ReactNode } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

interface DataListProps<T> {
  items: readonly T[];
  loading: boolean;
  renderItem: (item: T, index: number) => ReactNode;
  empty: ReactNode;
  error?: string;
  onRetry?: () => void;
  loadingState?: ReactNode;
  hasMore?: boolean;
  onLoadMore?: () => void | Promise<void>;
  loadingMore?: boolean;
  endOfList?: ReactNode;
  getKey?: (item: T, index: number) => string | number;
  className?: string;
}

function DataList<T>({
  items,
  loading,
  renderItem,
  empty,
  error,
  onRetry,
  loadingState,
  hasMore = false,
  onLoadMore,
  loadingMore = false,
  endOfList,
  getKey,
  className,
}: DataListProps<T>) {
  const t = useTranslations();
  const [requestingMore, setRequestingMore] = useState(false);
  const hasItems = items.length > 0;
  const isLoadingMore = loadingMore || requestingMore;

  useEffect(() => {
    if (!loadingMore) setRequestingMore(false);
  }, [loadingMore]);

  function loadMore() {
    if (!onLoadMore || !hasMore || isLoadingMore) return;
    setRequestingMore(true);
    Promise.resolve(onLoadMore()).finally(() => setRequestingMore(false));
  }

  if (loading && !hasItems) {
    return (
      <div className={cn("space-y-3", className)} aria-busy="true" aria-live="polite">
        <span className="sr-only" role="status">{t("common.loading")}</span>
        {loadingState ?? <Skeleton className="h-20 w-full" />}
      </div>
    );
  }

  if (!hasItems && error) {
    return (
      <div className={cn("rounded-md border border-destructive/50 bg-destructive/5 p-6 text-center", className)} role="alert">
        <p className="text-sm text-destructive">{error}</p>
        {onRetry && <Button type="button" variant="outline" className="mt-4" onClick={onRetry}>{t("common.retry")}</Button>}
      </div>
    );
  }

  if (!hasItems) {
    return <div className={className}>{empty}</div>;
  }

  return (
    <div className={cn("space-y-4", className)} aria-busy={isLoadingMore || undefined}>
      {error && (
        <div className="flex items-center justify-between gap-3 rounded-md border border-destructive/30 bg-destructive/5 p-3" role="alert">
          <p className="text-sm text-destructive">{error}</p>
          {onRetry && <Button type="button" variant="outline" size="sm" onClick={onRetry}>{t("common.retry")}</Button>}
        </div>
      )}
      <div data-slot="data-list-items" className="space-y-3">
        {items.map((item, index) => <Fragment key={getKey?.(item, index) ?? index}>{renderItem(item, index)}</Fragment>)}
      </div>
      {hasMore && onLoadMore ? (
        <div className="flex justify-center pt-2">
          <Button type="button" variant="outline" onClick={loadMore} disabled={isLoadingMore}>
            {isLoadingMore ? t("common.processing") : t("common.next")}
          </Button>
        </div>
      ) : endOfList ? (
        <div className="text-center text-xs text-muted-foreground" aria-live="polite">{endOfList}</div>
      ) : null}
    </div>
  );
}

export { DataList };
export type { DataListProps };
