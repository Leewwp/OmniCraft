import Link from "next/link";
import { MessageSquareText } from "lucide-react";
import { IPCategoryTabs } from "@/components/ip/IPCategoryTabs";

interface IPItem {
  id: number;
  name: string;
  description?: string;
  category?: string;
  cover_url?: string;
}

interface IPDetailProps {
  ip: IPItem;
}

export function IPDetail({ ip }: IPDetailProps) {
  return (
    <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
      <div className="flex flex-col gap-4 md:flex-row">
        <div className="flex h-36 w-full items-center justify-center rounded-md border border-border bg-muted/40 md:w-52">
          {ip.cover_url ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={ip.cover_url} alt={ip.name} className="h-full w-full rounded-md object-cover" />
          ) : (
            <span className="text-sm text-muted-foreground">IP 封面</span>
          )}
        </div>

        <div className="flex flex-1 flex-col gap-2">
          <h1 className="text-2xl font-bold tracking-tight">{ip.name}</h1>
          <p className="text-sm text-muted-foreground">分类：{ip.category || "未分类"}</p>
          <p className="text-sm leading-relaxed text-foreground/90">
            {ip.description || "暂无 IP 介绍"}
          </p>
          <div>
            <Link
              href={`/ip/${ip.id}/discussions`}
              className="inline-flex items-center gap-2 rounded-md border border-border px-3 py-2 text-xs hover:bg-muted"
            >
              <MessageSquareText className="h-3.5 w-3.5" />
              进入讨论区
            </Link>
          </div>
        </div>
      </div>

      <IPCategoryTabs ipId={String(ip.id)} activeCategory="all" />
    </section>
  );
}
