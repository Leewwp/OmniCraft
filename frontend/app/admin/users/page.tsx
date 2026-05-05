"use client";

import { useEffect, useState, useCallback } from "react";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";

interface UserItem {
  id: number;
  username: string;
  email: string;
  role: string;
  is_banned: boolean;
  reputation: number;
  created_at: string;
}

export default function AdminUsersPage() {
  const [users, setUsers] = useState<UserItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmTarget, setConfirmTarget] = useState<UserItem | null>(null);

  const pageSize = 20;

  const loadUsers = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await api.get<{ users: UserItem[]; total: number }>(
        `/api/v1/admin/users?page=${page}&page_size=${pageSize}`
      );
      setUsers(data.users || []);
      setTotal(data.total || 0);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "加载用户列表失败");
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    void loadUsers();
  }, [loadUsers]);

  async function banUser(id: number, reason: string) {
    setBusy(true);
    setError("");
    try {
      await api.post(`/api/v1/admin/users/${id}/ban`, { reason });
      setUsers((prev) =>
        prev.map((u) => (u.id === id ? { ...u, is_banned: true } : u))
      );
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "封禁失败");
    } finally {
      setBusy(false);
    }
  }

  async function unbanUser(id: number) {
    setBusy(true);
    setError("");
    try {
      await api.patch(`/api/v1/users/${id}`, { is_banned: false });
      setUsers((prev) =>
        prev.map((u) => (u.id === id ? { ...u, is_banned: false } : u))
      );
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "解封失败");
    } finally {
      setBusy(false);
    }
  }

  const filteredUsers = search
    ? users.filter(
        (u) =>
          u.username.toLowerCase().includes(search.toLowerCase()) ||
          u.email.toLowerCase().includes(search.toLowerCase())
      )
    : users;

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
          <h1 className="text-2xl font-bold tracking-tight">用户管理</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            管理所有注册用户（共 {total} 人）
          </p>
        </div>
      </div>

      <div className="flex gap-2">
        <input
          type="text"
          className="w-full max-w-sm rounded-md border border-border bg-background px-3 py-2 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          placeholder="搜索用户名或邮箱..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {filteredUsers.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center shadow-none">
          <p className="text-sm text-muted-foreground">无匹配用户</p>
        </div>
      ) : (
        <>
          <div className="overflow-x-auto rounded-md border border-border bg-card shadow-none">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border bg-muted/30 text-xs text-muted-foreground">
                <tr>
                  <th className="px-4 py-3 font-medium">ID</th>
                  <th className="px-4 py-3 font-medium">用户名</th>
                  <th className="px-4 py-3 font-medium">邮箱</th>
                  <th className="px-4 py-3 font-medium">信誉分</th>
                  <th className="px-4 py-3 font-medium">角色</th>
                  <th className="px-4 py-3 font-medium">状态</th>
                  <th className="px-4 py-3 font-medium">操作</th>
                </tr>
              </thead>
              <tbody>
                {filteredUsers.map((u) => (
                  <tr key={u.id} className="border-b border-border hover:bg-muted/20">
                    <td className="px-4 py-3 text-xs text-muted-foreground">{u.id}</td>
                    <td className="px-4 py-3 font-medium">{u.username}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{u.email}</td>
                    <td className="px-4 py-3">
                      <span
                        className={`rounded px-2 py-0.5 text-xs font-medium ${
                          u.reputation < 3
                            ? "bg-red-50 text-red-700"
                            : u.reputation < 10
                              ? "bg-amber-50 text-amber-700"
                              : "bg-emerald-50 text-emerald-700"
                        }`}
                      >
                        {u.reputation}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{u.role}</td>
                    <td className="px-4 py-3">
                      <span
                        className={`rounded px-2 py-0.5 text-xs ${
                          u.is_banned
                            ? "bg-red-50 text-red-700"
                            : "bg-emerald-50 text-emerald-700"
                        }`}
                      >
                        {u.is_banned ? "已封禁" : "正常"}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      {u.is_banned ? (
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={busy || u.role === "admin"}
                          onClick={() => void unbanUser(u.id)}
                        >
                          解封
                        </Button>
                      ) : (
                        <Button
                          size="sm"
                          variant="destructive"
                          disabled={busy || u.role === "admin"}
                          onClick={() => {
                            setConfirmTarget(u);
                            setConfirmOpen(true);
                          }}
                        >
                          封禁
                        </Button>
                      )}
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
        title="封禁用户"
        description={confirmTarget ? `确认封禁「${confirmTarget.username} (${confirmTarget.email})」吗？封禁后该用户将无法登录和使用平台。` : ""}
        confirmLabel="确认封禁"
        confirmVariant="destructive"
        requireReason
        reasonLabel="封禁原因"
        onConfirm={async (reason) => {
          if (confirmTarget) {
            await banUser(confirmTarget.id, reason);
          }
        }}
      />
    </div>
  );
}
