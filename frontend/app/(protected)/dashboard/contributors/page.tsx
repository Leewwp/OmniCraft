"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";

interface Contributor {
  user_id: number;
  username: string;
  contribution_count: number;
  blocked: boolean;
}

export default function ContributorsPage() {
  const { user, isLoading } = useAuth();
  const [contributors, setContributors] = useState<Contributor[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!user) return;
    void loadContributors();
  }, [user]);

  async function loadContributors() {
    setError("");
    setLoading(true);
    try {
      const data = await api.get<{ contents?: { id: number }[] }>(
        `/api/v1/contents?author_id=${user!.id}&page=1&page_size=50&sort=newest&time_range=all`
      );
      const contents = data.contents || [];
      const allContributors: Contributor[] = [];
      for (const c of contents) {
        try {
          const prData = await api.get<{ prs?: { submitter_id: number; status: string }[] }>(`/api/v1/contents/${c.id}/prs`);
          for (const pr of prData.prs || []) {
            if (pr.status === "accepted") {
              const existing = allContributors.find((co) => co.user_id === pr.submitter_id);
              if (existing) {
                existing.contribution_count += 1;
              } else {
                allContributors.push({ user_id: pr.submitter_id, username: `用户 #${pr.submitter_id}`, contribution_count: 1, blocked: false });
              }
            }
          }
        } catch { /* skip */ }
      }
      setContributors(allContributors);
    } catch {
      setError("加载贡献者列表失败");
    } finally {
      setLoading(false);
    }
  }

  async function toggleBlock(contributor: Contributor) {
    const action = contributor.blocked ? "解除拉黑" : "拉黑";
    if (!window.confirm(`确认${action}该贡献者吗？`)) return;
    try {
      if (contributor.blocked) {
        await api.delete(`/api/v1/dashboard/contributors/${contributor.user_id}/block`);
      } else {
        await api.post(`/api/v1/dashboard/contributors/${contributor.user_id}/block`, {});
      }
      setContributors((prev) =>
        prev.map((c) => (c.user_id === contributor.user_id ? { ...c, blocked: !c.blocked } : c))
      );
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "操作失败");
    }
  }

  if (isLoading || loading) {
    return <div className="mx-auto w-full max-w-4xl px-4 py-6 text-sm text-muted-foreground">加载中...</div>;
  }

  return (
    <div className="mx-auto w-full max-w-4xl space-y-6 px-4 py-6">
      <div className="rounded-md border border-border bg-card p-4 shadow-none">
        <h1 className="text-2xl font-bold tracking-tight">贡献者管理</h1>
        <p className="mt-1 text-sm text-muted-foreground">管理为你的内容做出贡献的用户</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {contributors.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center shadow-none">
          <p className="text-sm text-muted-foreground">暂无贡献者</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border bg-card shadow-none">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border bg-muted/30 text-xs text-muted-foreground">
              <tr>
                <th className="px-4 py-3 font-medium">用户名</th>
                <th className="px-4 py-3 font-medium">贡献次数</th>
                <th className="px-4 py-3 font-medium">状态</th>
                <th className="px-4 py-3 font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              {contributors.map((c) => (
                <tr key={c.user_id} className="border-b border-border hover:bg-muted/20">
                  <td className="px-4 py-3 font-medium">{c.username}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{c.contribution_count}</td>
                  <td className="px-4 py-3">
                    <span className={c.blocked ? "text-destructive text-xs" : "text-emerald-600 text-xs"}>
                      {c.blocked ? "已拉黑" : "正常"}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <Button size="sm" variant={c.blocked ? "outline" : "destructive"} onClick={() => void toggleBlock(c)}>
                      {c.blocked ? "解除拉黑" : "拉黑"}
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
