"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";

interface Contributor {
  user_id: number;
  username: string;
  contribution_count: number;
  blocked: boolean;
}

export default function ContributorsPage() {
  const t = useTranslations();
  const { user } = useAuth();
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
                allContributors.push({ user_id: pr.submitter_id, username: t('common.userLabel', { id: pr.submitter_id }), contribution_count: 1, blocked: false });
              }
            }
          }
        } catch (e) { silentError(e, { component: 'ContributorsPage', action: 'loadPRsForContent' }); }
      }
      setContributors(allContributors);
    } catch (e) {
      silentError(e, { component: 'ContributorsPage', action: 'loadContributors' });
      setError(t('dashboard.contributors.loadFailed'));
    } finally {
      setLoading(false);
    }
  }

  async function toggleBlock(contributor: Contributor) {
    const action = contributor.blocked ? t('dashboard.contributors.unblock') : t('dashboard.contributors.block');
    if (!window.confirm(t('dashboard.contributors.confirmAction', { action }))) return;
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
      silentError(e, { component: 'ContributorsPage', action: 'toggleBlock' });
      setError(t(getUserFacingErrorKey(e)));
    }
  }

  if (loading) {
    return <div className="mx-auto w-full max-w-4xl px-4 py-6 text-sm text-muted-foreground">{t('common.loading')}</div>;
  }

  return (
    <div className="mx-auto w-full max-w-4xl space-y-6 px-4 py-6">
      <div className="rounded-md border border-border bg-card p-4 ">
        <h1 className="text-2xl font-bold tracking-tight">{t('dashboard.contributors.title')}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t('dashboard.contributors.subtitle')}</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {contributors.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center ">
          <p className="text-sm text-muted-foreground">{t('dashboard.contributors.noContributors')}</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border bg-card ">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border bg-muted/30 text-xs text-muted-foreground">
              <tr>
                <th className="px-4 py-3 font-medium">{t('dashboard.contributors.colUsername')}</th>
                <th className="px-4 py-3 font-medium">{t('dashboard.contributors.colContributions')}</th>
                <th className="px-4 py-3 font-medium">{t('dashboard.contributors.colStatus')}</th>
                <th className="px-4 py-3 font-medium">{t('dashboard.contributors.colActions')}</th>
              </tr>
            </thead>
            <tbody>
              {contributors.map((c) => (
                <tr key={c.user_id} className="border-b border-border hover:bg-muted/20">
                  <td className="px-4 py-3 font-medium">{c.username}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{c.contribution_count}</td>
                  <td className="px-4 py-3">
                    <span className={c.blocked ? "text-destructive text-xs" : "text-emerald-600 text-xs"}>
                      {c.blocked ? t('dashboard.contributors.blocked') : t('dashboard.contributors.normal')}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <Button size="sm" variant={c.blocked ? "outline" : "destructive"} onClick={() => void toggleBlock(c)}>
                      {c.blocked ? t('dashboard.contributors.unblock') : t('dashboard.contributors.block')}
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
