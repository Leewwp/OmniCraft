"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { TagBadge } from "@/components/ui/TagBadge";
import { Button } from "@/components/ui/button";
import { Check, X, Loader2 } from "lucide-react";

interface TagSuggestion {
  id: number;
  tag: string;
  action: string;
  content_item_id: number;
  content_title?: string;
  user_id: number;
  username?: string;
  created_at: string;
}

export default function TagSuggestionsPage() {
  const t = useTranslations();
  const { user } = useAuth();
  const [contentId, setContentId] = useState("");
  const [suggestions, setSuggestions] = useState<TagSuggestion[]>([]);
  const [error, setError] = useState("");
  const [busyId, setBusyId] = useState<number | null>(null);

  const loadSuggestions = useCallback(async () => {
    const cid = parseInt(contentId, 10);
    if (!cid) {
      setSuggestions([]);
      return;
    }
    try {
      const res = await api.get<{ suggestions?: TagSuggestion[] }>(
        `/api/v1/dashboard/tag-suggestions?content_id=${cid}`,
      );
      setSuggestions(res.suggestions ?? []);
    } catch (e) {
      silentError(e, { component: 'TagSuggestionsPage', action: 'loadSuggestions' });
      setError(t(getUserFacingErrorKey(e, "common.loadFailed")));
    }
  }, [contentId, t]);

  useEffect(() => {
    if (!user) return;
    loadSuggestions();
  }, [user, loadSuggestions]);

  async function handleUpdate(id: number, status: "approved" | "rejected") {
    setBusyId(id);
    try {
      await api.patch(`/api/v1/dashboard/tag-suggestions/${id}`, { status });
      await loadSuggestions();
    } catch (e) {
      silentError(e, { component: 'TagSuggestionsPage', action: 'handleUpdate' });
      setError(t(getUserFacingErrorKey(e)));
    } finally {
      setBusyId(null);
    }
  }

  return (
    <div className="mx-auto w-full max-w-4xl space-y-6 px-4 py-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("tagSuggestions.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("tagSuggestions.subtitle")}</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="space-y-1">
        <label className="text-xs font-medium text-muted-foreground">
          {t("tagSuggestions.contentIdLabel")}
        </label>
        <input
          type="number"
          value={contentId}
          onChange={(e) => setContentId(e.target.value)}
          placeholder={t("tagSuggestions.contentIdPlaceholder")}
          className="w-full max-w-xs rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
        />
        <Button size="sm" variant="outline" className="mt-2" onClick={loadSuggestions}>
          {t("tagSuggestions.refresh")}
        </Button>
      </div>

      {suggestions.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-8 text-center ">
          <p className="text-sm text-muted-foreground">{t("tagSuggestions.empty")}</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border bg-muted/40">
              <tr>
                <th className="px-4 py-2.5 text-xs font-medium text-muted-foreground">
                  {t("tagSuggestions.colTag")}
                </th>
                <th className="px-4 py-2.5 text-xs font-medium text-muted-foreground">
                  {t("tagSuggestions.colAction")}
                </th>
                <th className="px-4 py-2.5 text-xs font-medium text-muted-foreground">
                  {t("tagSuggestions.colContent")}
                </th>
                <th className="px-4 py-2.5 text-xs font-medium text-muted-foreground">
                  {t("tagSuggestions.colSuggester")}
                </th>
                <th className="px-4 py-2.5 text-xs font-medium text-muted-foreground">
                  {t("tagSuggestions.colDate")}
                </th>
                <th className="px-4 py-2.5 text-right text-xs font-medium text-muted-foreground">
                  {t("tagSuggestions.colActions")}
                </th>
              </tr>
            </thead>
            <tbody>
              {suggestions.map((s) => (
                <tr key={s.id} className="border-b border-border last:border-0">
                  <td className="px-4 py-2.5">
                    <TagBadge color="orange">{s.tag}</TagBadge>
                  </td>
                  <td className="px-4 py-2.5">
                    <span className={s.action === "add" ? "text-emerald-600" : "text-red-500"}>
                      {s.action === "add" ? "+" : "−"}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 text-muted-foreground">
                    {s.content_title ?? `#${s.content_item_id}`}
                  </td>
                  <td className="px-4 py-2.5 text-muted-foreground">
                    {s.username ?? `#${s.user_id}`}
                  </td>
                  <td className="px-4 py-2.5 text-muted-foreground text-xs">
                    {new Date(s.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-2.5 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-8 w-8 p-0 text-emerald-600 hover:text-emerald-700"
                        disabled={busyId === s.id}
                        onClick={() => handleUpdate(s.id, "approved")}
                        aria-label={t("tagSuggestions.approve")}
                      >
                        {busyId === s.id ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : (
                          <Check className="h-4 w-4" />
                        )}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-8 w-8 p-0 text-red-500 hover:text-red-600"
                        disabled={busyId === s.id}
                        onClick={() => handleUpdate(s.id, "rejected")}
                        aria-label={t("tagSuggestions.reject")}
                      >
                        <X className="h-4 w-4" />
                      </Button>
                    </div>
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
