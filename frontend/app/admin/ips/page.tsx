"use client";

import { useEffect, useState, useCallback } from "react";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";

interface IPItem {
  id: number;
  name: string;
  slug: string;
  category: string;
  description: string;
  cover_image_url: string;
  status: string;
  submitter_id: number;
  created_at: string;
}

export default function AdminIPsPage() {
  const [ips, setIps] = useState<IPItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmAction, setConfirmAction] = useState<{
    ipId: number;
    action: "approve" | "reject";
    title: string;
  } | null>(null);

  const pageSize = 20;

  const loadIPs = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await api.get<{ ips: IPItem[]; total: number }>(
        `/api/v1/admin/ips?page=${page}&page_size=${pageSize}`
      );
      setIps(data.ips || []);
      setTotal(data.total || 0);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "加载IP列表失败");
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    void loadIPs();
  }, [loadIPs]);

  async function handleAction(ipId: number, action: "approve" | "reject") {
    setError("");
    try {
      await api.post(`/api/v1/admin/ips/${ipId}/${action}`, {});
      setIps((prev) => prev.filter((ip) => ip.id !== ipId));
      setTotal((t) => t - 1);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "操作失败");
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
          <h1 className="text-2xl font-bold tracking-tight">IP 库管理</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            审核待处理的 IP 提交（共 {total} 个待审）
          </p>
        </div>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {ips.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center shadow-none">
          <p className="text-sm text-muted-foreground">无待审核 IP</p>
        </div>
      ) : (
        <>
          <div className="overflow-x-auto rounded-md border border-border bg-card shadow-none">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border bg-muted/30 text-xs text-muted-foreground">
                <tr>
                  <th className="px-4 py-3 font-medium">名称</th>
                  <th className="px-4 py-3 font-medium">Slug</th>
                  <th className="px-4 py-3 font-medium">分类</th>
                  <th className="px-4 py-3 font-medium">提交人ID</th>
                  <th className="px-4 py-3 font-medium">状态</th>
                  <th className="px-4 py-3 font-medium">操作</th>
                </tr>
              </thead>
              <tbody>
                {ips.map((ip) => (
                  <tr key={ip.id} className="border-b border-border hover:bg-muted/20">
                    <td className="px-4 py-3 font-medium">{ip.name}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{ip.slug}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{ip.category || "-"}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{ip.submitter_id}</td>
                    <td className="px-4 py-3">
                      <span className="rounded bg-amber-50 px-2 py-0.5 text-xs text-amber-700">
                        {ip.status}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex gap-2">
                        <Button
                          size="sm"
                          variant="outline"
                          className="text-emerald-600 hover:bg-emerald-50"
                          onClick={() => {
                            setConfirmAction({ ipId: ip.id, action: "approve", title: ip.name });
                            setConfirmOpen(true);
                          }}
                        >
                          通过
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          className="text-destructive hover:bg-destructive/10"
                          onClick={() => {
                            setConfirmAction({ ipId: ip.id, action: "reject", title: ip.name });
                            setConfirmOpen(true);
                          }}
                        >
                          拒绝
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
        onOpenChange={setConfirmOpen}
        title={confirmAction?.action === "approve" ? "通过 IP 审核" : "拒绝 IP"}
        description={
          confirmAction?.action === "approve"
            ? `确认通过「${confirmAction?.title}」的审核吗？通过后 IP 将进入可发布内容库。`
            : `确认拒绝「${confirmAction?.title}」的审核吗？拒绝后 IP 将被驳回，需重新提交。`
        }
        confirmLabel={confirmAction?.action === "approve" ? "确认通过" : "确认拒绝"}
        confirmVariant={confirmAction?.action === "approve" ? "default" : "destructive"}
        requireReason={confirmAction?.action === "reject"}
        reasonLabel="拒绝原因"
        onConfirm={async (reason) => {
          if (confirmAction) {
            await handleAction(confirmAction.ipId, confirmAction.action);
          }
        }}
      />
    </div>
  );
}
