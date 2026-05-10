"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { Eye, Heart, MessageCircle, Edit, Trash2 } from "lucide-react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";

export default function StudioContentsPage() {
  const t = useTranslations();
  const [contents, setContents] = useState<Array<{
    id: number; title: string; zone: string; content_type: string;
    view_count: number; like_count: number; comment_count: number;
    status: string;
  }>>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.get("/api/v1/my/contents?limit=50")
      .then((res) => setContents(((res as Record<string, unknown>)?.data as Array<Record<string, unknown>> || []).map((c) => ({
        id: c.id as number, title: c.title as string, zone: c.zone as string,
        content_type: c.content_type as string, view_count: c.view_count as number,
        like_count: c.like_count as number, comment_count: c.comment_count as number,
        status: c.status as string,
      }))))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-foreground">我的内容</h1>
          <p className="mt-1 text-sm text-muted-foreground">管理你发布的所有内容</p>
        </div>
        <Link href="/studio/publish/original">
          <Button size="sm">发布新内容</Button>
        </Link>
      </div>

      {loading ? (
        <div className="space-y-3">
          {[1, 2, 3].map((i) => <Skeleton key={i} className="h-20 rounded-lg" />)}
        </div>
      ) : contents.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-12 text-center">
          <p className="mb-4 text-muted-foreground">你还没有发布任何内容</p>
          <Link href="/studio/publish/original">
            <Button>开始创作</Button>
          </Link>
        </div>
      ) : (
        <div className="rounded-lg border border-border bg-card">
          {contents.map((item) => (
            <div
              key={item.id}
              className="flex items-center gap-4 border-b border-border p-4 last:border-b-0"
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <Link
                    href={`/content/${item.id}`}
                    className="text-sm font-medium text-foreground hover:text-primary truncate"
                  >
                    {item.title}
                  </Link>
                  <Badge variant="secondary" className="text-[10px]">{item.zone}</Badge>
                  <Badge variant="outline" className="text-[10px]">{item.content_type}</Badge>
                </div>
                <div className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
                  <span className="inline-flex items-center gap-1"><Eye className="h-3 w-3" /> {item.view_count}</span>
                  <span className="inline-flex items-center gap-1"><Heart className="h-3 w-3" /> {item.like_count}</span>
                  <span className="inline-flex items-center gap-1"><MessageCircle className="h-3 w-3" /> {item.comment_count}</span>
                </div>
              </div>
              <div className="flex items-center gap-1">
                <Button variant="ghost" size="icon-sm" title="编辑">
                  <Edit className="h-3.5 w-3.5" />
                </Button>
                <Button variant="ghost" size="icon-sm" title="删除">
                  <Trash2 className="h-3.5 w-3.5 text-destructive" />
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
