"use client";

import { use, useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { BookOpen, Layers } from "lucide-react";
import { useTranslations } from "next-intl";

import { EmptyState } from "@/components/ui/empty-state";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/Toast";
import { ApiRequestError } from "@/lib/api";
import { getSeriesDetail, type SeriesDetailResponse } from "@/lib/series";
import { silentError } from "@/lib/error-handler";

type PageParams = Promise<{ id: string }>;

interface SeriesDetailPageProps {
  params: PageParams;
}

type LoadState =
  | { status: "loading" }
  | { status: "ready"; detail: SeriesDetailResponse }
  | { status: "not-found" }
  | { status: "error" };

export default function SeriesDetailPage({ params }: SeriesDetailPageProps) {
  const t = useTranslations();
  const { toast } = useToast();
  const resolvedParams = unwrapMaybePromise(params);
  const seriesID = Number(resolvedParams.id);
  const [state, setState] = useState<LoadState>({ status: "loading" });

  const load = useCallback(async () => {
    if (!Number.isInteger(seriesID) || seriesID <= 0) {
      setState({ status: "not-found" });
      return;
    }
    setState((current) => (current.status === "ready" ? current : { status: "loading" }));
    try {
      const detail = await getSeriesDetail(seriesID);
      setState({ status: "ready", detail });
    } catch (error) {
      if (error instanceof ApiRequestError && (error.status === 404 || error.code === "SERIES_NOT_FOUND")) {
        setState({ status: "not-found" });
        return;
      }
      silentError(error, { component: "SeriesDetailPage", action: "load" });
      toast("error", t("series.detail.error.loadFailed"));
      setState({ status: "error" });
    }
  }, [seriesID, t, toast]);

  useEffect(() => {
    void load();
  }, [load]);

  if (state.status === "loading") {
    return (
      <main className="mx-auto w-full max-w-[1080px] space-y-6 px-4 py-6 md:px-6">
        <div className="flex flex-col gap-4 rounded-lg border border-border-default bg-card p-4 sm:flex-row">
          <Skeleton className="h-[140px] w-full rounded-md sm:w-[220px]" />
          <div className="flex-1 space-y-3">
            <Skeleton className="h-7 w-2/3" />
            <Skeleton className="h-4 w-1/3" />
            <Skeleton className="h-16 w-full" />
          </div>
        </div>
        <div className="space-y-3" aria-hidden="true">
          {Array.from({ length: 6 }, (_, index) => <Skeleton key={index} className="h-14 w-full" />)}
        </div>
      </main>
    );
  }

  if (state.status === "not-found") {
    return (
      <main className="mx-auto w-full max-w-[1080px] px-4 py-10 md:px-6">
        <EmptyState icon={BookOpen} title={t("series.detail.error.title")} description={t("series.detail.error.description")} />
      </main>
    );
  }

  if (state.status === "error") {
    return (
      <main className="mx-auto w-full max-w-[1080px] px-4 py-10 md:px-6">
        <EmptyState
          icon={BookOpen}
          title={t("series.detail.error.loadFailed")}
          description={t("series.detail.error.description")}
          action={<Button type="button" variant="outline" onClick={() => void load()}>{t("common.retry")}</Button>}
        />
      </main>
    );
  }

  const { series, items } = state.detail;
  const visibleItems = items.filter((item) => !item.content.status || item.content.status === "published");
  const zoneLabel = series.zone === "fanwork" ? t("series.detail.header.zoneFanwork") : t("series.detail.header.zoneOriginal");

  return (
    <main className="mx-auto w-full max-w-[1080px] space-y-6 px-4 py-6 md:px-6">
      <header className="flex flex-col gap-4 rounded-lg border border-border-default bg-card p-4 sm:flex-row">
        {series.cover ? (
          <img src={series.cover} alt={t("series.detail.a11y.cover", { title: series.title })} className="h-auto aspect-video w-full rounded-md border border-border-default object-cover sm:h-[140px] sm:w-[220px] sm:aspect-auto" />
        ) : (
          <div className="flex aspect-video w-full items-center justify-center rounded-md border border-border-default bg-canvas-subtle text-fg-muted sm:h-[140px] sm:w-[220px] sm:aspect-auto" aria-hidden="true">
            <Layers className="h-8 w-8" />
          </div>
        )}
        <div className="min-w-0 flex-1">
          <div className="mb-2 flex flex-wrap items-center gap-2">
            <span className="rounded border border-border-default bg-canvas-subtle px-2 py-0.5 text-xs text-fg-muted">{zoneLabel}</span>
            <span className="text-xs text-fg-muted">{t("series.detail.header.itemCount", { count: series.item_count })}</span>
          </div>
          <h1 className="text-2xl font-semibold text-fg-default">{series.title}</h1>
          <Link href={`/user/${series.owner.id}`} className="mt-2 inline-flex min-h-11 items-center text-sm text-accent-emphasis underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-emphasis">
            {t("series.detail.header.owner", { username: series.owner.username })}
          </Link>
          {series.description && <p className="mt-3 whitespace-pre-wrap text-sm leading-6 text-fg-muted">{series.description}</p>}
        </div>
      </header>

      <section aria-label={t("series.detail.items.ariaLabel")}>
        <h2 className="mb-3 text-base font-semibold text-fg-default">{t("series.detail.items.title")}</h2>
        {visibleItems.length === 0 ? (
          <EmptyState icon={BookOpen} title={t("series.detail.empty.title")} description={t("series.detail.empty.description")} />
        ) : (
          <ol className="divide-y divide-border-default rounded-lg border border-border-default bg-card" aria-label={t("series.detail.items.ariaLabel")}>
            {visibleItems.map((item, index) => (
              <li key={item.id}>
                <Link href={item.content.zone === "original" ? `/original/${item.content.id}` : `/content/${item.content.id}`} className="flex min-h-11 items-center gap-3 px-4 py-3 text-sm transition-colors hover:bg-canvas-subtle focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-accent-emphasis">
                  <span className="w-12 shrink-0 text-xs text-fg-muted">{t("series.detail.items.itemLabel", { index: index + 1 })}</span>
                  <span className="min-w-0 flex-1 font-medium text-fg-default">{item.content.title}</span>
                  <span className="shrink-0 text-xs text-fg-muted">{item.content.zone === "fanwork" ? t("series.detail.header.zoneFanwork") : t("series.detail.header.zoneOriginal")}</span>
                </Link>
              </li>
            ))}
          </ol>
        )}
      </section>
    </main>
  );
}

function unwrapMaybePromise<T>(value: T | Promise<T>): T {
  if (value && typeof value === "object" && "then" in value && typeof value.then === "function") {
    return use(value as Promise<T>);
  }
  return value as T;
}
