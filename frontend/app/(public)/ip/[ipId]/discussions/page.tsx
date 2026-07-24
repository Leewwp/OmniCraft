"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { DiscussionCard } from "@/components/social/DiscussionCard";
import { Button } from "@/components/ui/button";
import { Plus, Search } from "lucide-react";
import { silentError } from "@/lib/error-handler";

interface DiscussionData {
  id: number;
  title: string;
  ip_id?: number;
  author?: { id?: number; username?: string };
  reply_count?: number;
  last_active_at?: string;
  created_at?: string;
}

export default function DiscussionsPage() {
  const t = useTranslations();
  const params = useParams<{ ipId: string }>();
  const ipId = parseInt(params.ipId, 10);
  const { user } = useAuth();
  const [discussions, setDiscussions] = useState<DiscussionData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const searchRef = useRef(search);
  searchRef.current = search;

  const load = useCallback(async (searchQuery?: string) => {
    setLoading(true);
    setError("");
    try {
      const q = (searchQuery ?? searchRef.current).trim();
      const path = q ? `/search?q=${encodeURIComponent(q)}` : "";
      const res = await api.get<{ discussions?: DiscussionData[] }>(
        `/api/v1/ips/${ipId}/discussions${path}`,
      );
      setDiscussions(res.discussions ?? []);
    } catch (e) {
      silentError(e, { component: 'DiscussionsPage', action: 'load' });
      setError(t(getUserFacingErrorKey(e, "common.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [ipId, t]);

  useEffect(() => { load(); }, [load]);

  return (
    <div className="mx-auto w-full max-w-3xl space-y-6 px-4 py-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("discussion.title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("discussion.subtitle")}</p>
        </div>
        {user && (
          <Link href={`/ip/${ipId}/discussions/new`}>
            <Button size="sm"><Plus className="mr-1 h-4 w-4" />{t("discussion.newPost")}</Button>
          </Link>
        )}
      </div>

      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("discussion.searchPlaceholder")}
            className="w-full rounded-md border border-border bg-background py-2 pl-10 pr-4 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
          />
        </div>
        <Button size="sm" variant="outline" onClick={() => load()}>{t("discussion.search")}</Button>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {loading ? (
        <div className="text-sm text-muted-foreground text-center py-8">{t("common.loading")}</div>
      ) : discussions.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-8 text-center text-sm text-muted-foreground ">
          {t("discussion.empty")}
        </div>
      ) : (
        <div className="space-y-3">
          {discussions.map((d) => (
            <DiscussionCard key={d.id} data={d} />
          ))}
        </div>
      )}
    </div>
  );
}
