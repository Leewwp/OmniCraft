"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
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

  // Browse variant: compact cover card (matching design demo)
  if (variant === "browse") {
    return (
      <Link
        href={`/ip/${data.id}`}
        onClick={() => saveRecentIP({ id: data.id, name: data.name })}
        className={cn(
          "group flex-shrink-0 w-[156px] rounded-lg border border-border bg-card overflow-hidden transition-colors hover:border-border/80",
          className
        )}
      >
        {/* Cover */}
        <div className="aspect-[16/10] bg-muted overflow-hidden">
          {data.cover_url ? (
            <img src={data.cover_url} alt={data.name} className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105" />
          ) : (
            <div className="flex h-full w-full items-center justify-center text-xl text-muted-foreground/40">
              {data.name.slice(0, 2)}
            </div>
          )}
        </div>

        {/* Info */}
        <div className="px-2.5 py-2">
          <div className="truncate text-[13px] font-semibold text-foreground">
            {data.name}
          </div>
          <div className="mt-0.5 flex items-center gap-2 text-[11px] text-muted-foreground">
            {data.content_count !== undefined && (
              <span>{data.content_count.toLocaleString()} 内容</span>
            )}
            {data.trend !== undefined && data.trend > 0 && (
              <span className="text-emerald-500 font-medium">↗ {data.trend}%</span>
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
      className={cn(
        "group flex min-w-64 flex-col gap-3 rounded-lg border border-border bg-card p-3 transition-colors hover:bg-muted/30",
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
          {data.description || `${data.content_count?.toLocaleString()} 内容`}
        </p>
      )}
      <div className="inline-flex items-center gap-1 text-xs text-muted-foreground">
        {t('ip.enterDetail')} →
      </div>
    </Link>
  );
}
