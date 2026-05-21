"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
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
  const t = useTranslations();
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
      silentError(e, { component: 'AdminUsersPage', action: 'loadUsers' });
      setError(e instanceof ApiRequestError ? e.message : t('admin.users.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [page, t]);

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
      silentError(e, { component: 'AdminUsersPage', action: 'banUser' });
      setError(e instanceof ApiRequestError ? e.message : t('admin.users.banFailed'));
    } finally {
      setBusy(false);
    }
  }

  async function unbanUser(id: number) {
    setBusy(true);
    setError("");
    try {
      await api.post(`/api/v1/admin/users/${id}/unban`, {});
      setUsers((prev) =>
        prev.map((u) => (u.id === id ? { ...u, is_banned: false } : u))
      );
    } catch (e) {
      silentError(e, { component: 'AdminUsersPage', action: 'unbanUser' });
      setError(e instanceof ApiRequestError ? e.message : t('admin.users.unbanFailed'));
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
        <div className="space-y-3 rounded-md border border-border bg-card p-6 ">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-8 w-full animate-pulse rounded bg-muted" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4 ">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('admin.users.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t('admin.users.subtitle', { total })}
          </p>
        </div>
      </div>

      <div className="flex gap-2">
        <input
          type="text"
          className="w-full max-w-sm rounded-md border border-border bg-background px-3 py-2 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          placeholder={t('admin.users.searchPlaceholder')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {filteredUsers.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center ">
          <p className="text-sm text-muted-foreground">{t('admin.users.noMatch')}</p>
        </div>
      ) : (
        <>
          <div className="overflow-x-auto rounded-md border border-border bg-card ">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border bg-muted/30 text-xs text-muted-foreground">
                <tr>
                  <th className="px-4 py-3 font-medium">ID</th>
                  <th className="px-4 py-3 font-medium">{t('admin.users.colUsername')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.users.colEmail')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.users.colReputation')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.users.colRole')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.users.colStatus')}</th>
                  <th className="px-4 py-3 font-medium">{t('admin.users.colActions')}</th>
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
                        {u.is_banned ? t('admin.users.banned') : t('admin.users.normal')}
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
                          {t('admin.users.unban')}
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
                          {t('admin.users.ban')}
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
                {t('common.page', { current: page, total: totalPages })}
              </span>
              <div className="flex gap-2">
                <Button size="sm" variant="outline" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                  {t('common.previous')}
                </Button>
                <Button size="sm" variant="outline" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
                  {t('common.next')}
                </Button>
              </div>
            </div>
          )}
        </>
      )}

      <ConfirmModal
        open={confirmOpen}
        onOpenChange={(v) => { setConfirmOpen(v); if (!v) setConfirmTarget(null); }}
        title={t('admin.users.banTitle')}
        description={confirmTarget ? t('admin.users.banConfirm', { name: `${confirmTarget.username} (${confirmTarget.email})` }) : ""}
        confirmLabel={t('admin.users.confirmBan')}
        confirmVariant="destructive"
        requireReason
        reasonLabel={t('admin.users.banReason')}
        onConfirm={async (reason) => {
          if (confirmTarget) {
            await banUser(confirmTarget.id, reason);
          }
        }}
      />
    </div>
  );
}
