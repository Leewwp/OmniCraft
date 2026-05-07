"use client";

import { useCallback, useEffect, useState } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { Clock, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { api, ApiRequestError } from "@/lib/api";

interface ContentItem {
  id: number;
  title: string;
  content_type: string;
  cover_image_url?: string;
}

interface HistoryRecord {
  id: number;
  content_item: ContentItem;
  viewed_at: string;
}

interface HistoryGroup {
  label: string;
  records: HistoryRecord[];
}

function groupByDate(records: HistoryRecord[]): HistoryGroup[] {
  const now = new Date();
  const todayStr = now.toDateString();
  const yesterdayStr = new Date(now.getTime() - 86400000).toDateString();

  const groups: Record<string, HistoryRecord[]> = {};
  for (const record of records) {
    const d = new Date(record.viewed_at);
    const ds = d.toDateString();
    let label = ds;
    if (ds === todayStr) label = "今天";
    else if (ds === yesterdayStr) label = "昨天";
    else label = `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日`;

    if (!groups[label]) groups[label] = [];
    groups[label].push(record);
  }

  return Object.entries(groups).map(([label, records]) => ({ label, records }));
}

export default function HistoryPage() {
  const router = useRouter();
  const [groups, setGroups] = useState<HistoryGroup[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [confirmClear, setConfirmClear] = useState(false);
  const [clearing, setClearing] = useState(false);

  const load = useCallback(
    async (p: number) => {
      try {
        setLoading(true);
        const data = await api.get<{ history: HistoryRecord[]; total: number }>(
          `/api/v1/users/me/history?page=${p}&page_size=40`
        );
        setTotal(data.total ?? 0);
        if (p === 1) {
          setGroups(groupByDate(data.history ?? []));
        } else {
          setGroups((prev) => {
            const allRecords = [
              ...prev.flatMap((g) => g.records),
              ...(data.history ?? []),
            ];
            return groupByDate(allRecords);
          });
        }
      } catch (e) {
        if (e instanceof ApiRequestError && e.status === 401) {
          router.push("/login");
        }
      } finally {
        setLoading(false);
      }
    },
    [router]
  );

  useEffect(() => {
    load(page);
  }, [load, page]);

  async function handleClear() {
    try {
      setClearing(true);
      await api.delete("/api/v1/users/me/history");
      setGroups([]);
      setTotal(0);
    } finally {
      setClearing(false);
      setConfirmClear(false);
    }
  }

  const loadedCount = groups.reduce((s, g) => s + g.records.length, 0);

  return (
    <div className="max-w-4xl mx-auto px-4 py-8">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-2">
          <Clock className="w-5 h-5 text-muted-foreground" />
          <h1 className="text-2xl font-semibold">浏览历史</h1>
          {total > 0 && (
            <span className="text-sm text-muted-foreground ml-1">({total})</span>
          )}
        </div>
        {total > 0 && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => setConfirmClear(true)}
            className="text-destructive hover:text-destructive"
          >
            <Trash2 className="w-4 h-4 mr-1" />
            清除全部
          </Button>
        )}
      </div>

      {loading && page === 1 ? (
        <div className="space-y-8">
          {[...Array(2)].map((_, i) => (
            <div key={i}>
              <div className="h-4 w-12 bg-muted rounded animate-pulse mb-3" />
              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
                {[...Array(4)].map((_, j) => (
                  <div
                    key={j}
                    className="aspect-[3/4] bg-muted rounded-lg animate-pulse"
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      ) : groups.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-24 text-muted-foreground gap-3">
          <Clock className="w-12 h-12 opacity-30" />
          <p className="text-lg">暂无浏览记录</p>
        </div>
      ) : (
        <div className="space-y-8">
          {groups.map((group) => (
            <div key={group.label}>
              <h2 className="text-sm font-medium text-muted-foreground mb-3 sticky top-0 bg-background py-1">
                {group.label}
              </h2>
              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
                {group.records.map((record) => (
                  <div
                    key={record.id}
                    className="group cursor-pointer rounded-lg overflow-hidden border border-border hover:border-accent-foreground/30 transition-colors"
                    onClick={() =>
                      router.push(`/content/${record.content_item?.id}`)
                    }
                  >
                    <div className="aspect-[3/4] bg-muted relative overflow-hidden">
                      {record.content_item?.cover_image_url ? (
                        <Image
                          src={record.content_item.cover_image_url}
                          alt={record.content_item.title}
                          fill
                          className="object-cover group-hover:scale-105 transition-transform duration-200"
                          sizes="(max-width: 640px) 50vw, (max-width: 768px) 33vw, 25vw"
                        />
                      ) : (
                        <div className="w-full h-full flex items-center justify-center text-muted-foreground/50 text-xs">
                          {record.content_item?.content_type ?? "内容"}
                        </div>
                      )}
                    </div>
                    <div className="p-2">
                      <p className="text-xs font-medium line-clamp-2 leading-tight">
                        {record.content_item?.title ?? "未知内容"}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}

          {loadedCount < total && (
            <div className="text-center pt-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p) => p + 1)}
                disabled={loading}
              >
                {loading ? "加载中..." : "加载更多"}
              </Button>
            </div>
          )}
        </div>
      )}

      {confirmClear && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-background rounded-xl p-6 max-w-sm w-full mx-4 shadow-xl border border-border">
            <h3 className="text-lg font-semibold mb-2">确认清除历史</h3>
            <p className="text-sm text-muted-foreground mb-5">
              将清除全部浏览历史记录，此操作不可撤销。
            </p>
            <div className="flex gap-3 justify-end">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setConfirmClear(false)}
                disabled={clearing}
              >
                取消
              </Button>
              <Button
                variant="destructive"
                size="sm"
                onClick={handleClear}
                disabled={clearing}
              >
                {clearing ? "清除中..." : "确认清除"}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
