"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { FolderOpen, Layers3 } from "lucide-react";
import { cn } from "@/lib/utils";

interface IPCardData {
  id: number;
  name: string;
  slug?: string;
  description?: string;
  cover_url?: string;
  category?: string;
}

interface IPCardProps {
  data: IPCardData;
  className?: string;
}

interface RecentIPItem {
  id: number;
  name: string;
}

const RECENT_IP_KEY = "recent_ips";

function saveRecentIP(item: RecentIPItem) {
  if (typeof window === "undefined") {
    return;
  }
  const raw = window.localStorage.getItem(RECENT_IP_KEY);
  const current: RecentIPItem[] = raw ? (JSON.parse(raw) as RecentIPItem[]) : [];
  const deduped = [item, ...current.filter((it) => it.id !== item.id)].slice(0, 5);
  window.localStorage.setItem(RECENT_IP_KEY, JSON.stringify(deduped));
}

export function IPCard({ data, className }: IPCardProps) {
  const t = useTranslations();
  return (
    <Link
      href={`/ip/${data.id}`}
      onClick={() => saveRecentIP({ id: data.id, name: data.name })}
      className={cn(
        "group flex min-w-64 flex-col gap-3 rounded-md border border-border bg-card p-3 transition-colors hover:bg-muted/30",
        className
      )}
    >
      <div className="flex items-start gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md border border-border bg-muted/40 text-muted-foreground">
          <Layers3 className="h-4 w-4" />
        </div>
        <div className="space-y-1">
          <h3 className="line-clamp-1 text-sm font-semibold text-foreground">{data.name}</h3>
          <p className="text-xs text-muted-foreground">{data.category || t('ip.uncategorized')}</p>
        </div>
      </div>

      <p className="line-clamp-2 text-xs text-muted-foreground">
        {data.description || t('ip.noDescriptionShort')}
      </p>

      <div className="inline-flex items-center gap-1 text-xs text-muted-foreground">
        <FolderOpen className="h-3.5 w-3.5" />
        {t('ip.enterDetail')}
      </div>
    </Link>
  );
}
