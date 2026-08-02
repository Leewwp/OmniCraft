"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { TrendingUp } from "lucide-react";
import { cn } from "@/lib/utils";

interface IPCardData {
  id: number;
  name: string;
  slug?: string;
  description?: string;
  cover_url?: string;
  category?: string;
  content_count?: number;
  trend?: number;
}

interface IPCardProps {
  data: IPCardData;
  variant?: "browse" | "list";
  className?: string;
}

interface RecentIPItem { id: number; name: string }
const RECENT_IP_KEY = "recent_ips";

function saveRecentIP(item: RecentIPItem) {
  if (typeof window === "undefined") return;
  const raw = localStorage.getItem(RECENT_IP_KEY);
  const current: RecentIPItem[] = raw ? (JSON.parse(raw) as RecentIPItem[]) : [];
  const deduped = [item, ...current.filter((it) => it.id !== item.id)].slice(0, 6);
  localStorage.setItem(RECENT_IP_KEY, JSON.stringify(deduped));
}

export function IPCard({ data, variant = "browse", className }: IPCardProps) {
  const t = useTranslations();

  // Browse variant: P-01 approved Indigo library card.
  if (variant === "browse") {
    return (
      <Link
        href={`/ip/${data.id}`}
        onClick={() => saveRecentIP({ id: data.id, name: data.name })}
        aria-label={`${t('ip.enterDetail')}: ${data.name}`}
        className={cn(
          "group block w-full min-w-0 overflow-hidden rounded-lg border border-border bg-card shadow-[var(--elevation-1)] transition-[border-color,box-shadow,background-color] duration-150 hover:border-[var(--border-strong)] hover:shadow-[var(--elevation-2)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background motion-reduce:shadow-[var(--elevation-1)] motion-reduce:transition-none",
          className
        )}
      >
        {/* Cover */}
        <div className="aspect-[16/10] bg-muted overflow-hidden">
          {data.cover_url ? (
            <img src={data.cover_url} alt="" className="h-full w-full object-cover transition-transform duration-150 group-hover:scale-[1.015] motion-reduce:transform-none" />
          ) : (
            <div className="flex h-full w-full items-center justify-center text-xl text-muted-foreground/40">
              {data.name.slice(0, 2)}
            </div>
          )}
        </div>

        {/* Info */}
        <div className="space-y-1 bg-card p-3 dark:bg-canvas-subtle">
          <div className="truncate text-sm font-medium text-foreground">
            {data.name}
          </div>
          <div className="flex min-w-0 items-end justify-between gap-1 text-xs text-muted-foreground">
            <span className="min-w-0 truncate">
              {[data.category, data.content_count !== undefined ? t('ip.contentCount', { count: data.content_count }) : ""]
                .filter(Boolean)
                .join(" · ")}
            </span>
            {data.trend !== undefined && data.trend > 0 && (
              <span className="inline-flex shrink-0 items-center gap-0.5 font-medium text-[var(--tag-green-fg)]">
                <TrendingUp className="h-3 w-3" aria-hidden="true" />+{data.trend}%
              </span>
            )}
          </div>
        </div>
      </Link>
    );
  }

  // List variant: larger detail card
  return (
    <Link
      href={`/ip/${data.id}`}
      onClick={() => saveRecentIP({ id: data.id, name: data.name })}
      aria-label={`${t('ip.enterDetail')}: ${data.name}`}
      className={cn(
        "group flex min-w-64 flex-col gap-3 rounded-lg border border-border bg-card p-3 shadow-[var(--elevation-1)] transition-[border-color,box-shadow,background-color] duration-150 hover:border-[var(--border-strong)] hover:bg-canvas-subtle hover:shadow-[var(--elevation-2)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background motion-reduce:transition-none",
        className
      )}
    >
      <div className="flex items-start gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground overflow-hidden">
          {data.cover_url ? (
            <img src={data.cover_url} alt="" className="h-full w-full object-cover" />
          ) : (
            <span className="text-xs font-semibold">{data.name.slice(0, 2)}</span>
          )}
        </div>
        <div className="min-w-0">
          <h3 className="truncate text-sm font-semibold text-foreground">{data.name}</h3>
          <p className="text-xs text-muted-foreground">{data.category || t('ip.uncategorized')}</p>
        </div>
      </div>
      {(data.description || data.content_count) && (
        <p className="line-clamp-2 text-xs text-muted-foreground">
          {data.description || (data.content_count != null ? t('ip.contentCount', { count: data.content_count }) : "")}
        </p>
      )}
      <div className="inline-flex items-center gap-1 text-xs text-muted-foreground">
        {t('ip.enterDetail')}
      </div>
    </Link>
  );
}
