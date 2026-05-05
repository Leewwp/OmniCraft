"use client";

import { useEffect, useState, useCallback } from "react";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import Link from "next/link";

interface ContentItem {
  id: number;
  title: string;
  content_type: string;
  zone: string;
  author_id: number;
  status: string;
  report_count?: number;
  view_count: number;
  created_at: string;
}

export default function AdminContentsPage() {
  const [contents, setContents] = useState<ContentItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmTarget, setConfirmTarget] = useState<ContentItem | null>(null);

  const pageSize = 20;

  const loadContents = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await api.get<{ contents: ContentItem[]; total: number }>(
        `/api/v1/admin/contents?page=${page}&page_size=${pageSize}`
      );
      setContents(data.contents || []);
      setTotal(data.total || 0);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "加载待审内容失败");
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    void loadContents();
  }, [loadContents]);

  async function banContent(id: number) {
    setBusy(true);
    setError("");
    try {
      await api.post(`/api/v1/admin/contents/${id}/ban`, {});
      setContents((prev) => prev.filter((c) => c.id !== id));
      setTotal((t) => t - 1);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "封禁失败");
    } finally {
      setBusy(false);
    }
  }

  const totalPages = Math.ceil(total / pageSize);

  if (loading) {
    return (
      <div className="space-y-4 p-6">
        <div className="space-y-3 rounded-md border border-border bg-card p-6 shadow-none">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-8 w-full animate-pulse rounded bg-muted" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4 shadow-none">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">内容终审</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            审核被举报或 AI 标记的可疑内容（共 {total} 个待审）
          </p>
        </div>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {contents.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center shadow-none">
          <p className="text-sm text-muted-foreground">无待审内容</p>
        </div>
      ) : (
        <>
          <div className="overflow-x-auto rounded-md border border-border bg-card shadow-none">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border bg-muted/30 text-xs text-muted-foreground">
                <tr>
                  <th className="px-4 py-3 font-medium">标题</th>
                  <th className="px-4 py-3 font-medium">类型</th>
                  <th className="px-4 py-3 font-medium">分区</th>
                  <th className="px-4 py-3 font-medium">作者ID</th>
                  <th className="px-4 py-3 font-medium">浏览</th>
                  <th className="px-4 py-3 font-medium">状态</th>
                  <th className="px-4 py-3 font-medium">操作</th>
                </tr>
              </thead>
              <tbody>
                {contents.map((c) => (
                  <tr key={c.id} className="border-b border-border hover:bg-muted/20">
                    <td className="max-w-[200px] truncate px-4 py-3 font-medium">{c.title}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{c.content_type}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{c.zone}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{c.author_id}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{c.view_count}</td>
                    <td className="px-4 py-3">
                      <span className="rounded bg-amber-50 px-2 py-0.5 text-xs text-amber-700">
                        {c.status}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex gap-2">
                        <Link href={`/content/${c.id}`} target="_blank">
                          <Button size="sm" variant="outline">查看</Button>
                        </Link>
                        <Button
                          size="sm"
                          variant="destructive"
                          disabled={busy}
                          onClick={() => {
                            setConfirmTarget(c);
                            setConfirmOpen(true);
                          }}
                        >
                          封禁
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-between">
              <span className="text-xs text-muted-foreground">
                第 {page} / {totalPages} 页
              </span>
              <div className="flex gap-2">
                <Button size="sm" variant="outline" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                  上一页
                </Button>
                <Button size="sm" variant="outline" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
                  下一页
                </Button>
              </div>
            </div>
          )}
        </>
      )}

      <ConfirmModal
        open={confirmOpen}
        onOpenChange={(v) => { setConfirmOpen(v); if (!v) setConfirmTarget(null); }}
        title="封禁内容"
        description={confirmTarget ? `确认封禁「${confirmTarget.title}」吗？封禁后该内容将不再对用户可见，作者信誉分 -3。` : ""}
        confirmLabel="确认封禁"
        confirmVariant="destructive"
        requireReason
        reasonLabel="封禁原因"
        onConfirm={async (_reason) => {
          if (confirmTarget) {
            await banContent(confirmTarget.id);
          }
        }}
      />
    </div>
  );
}
