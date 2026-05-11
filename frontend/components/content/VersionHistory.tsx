"use client";

import { useTranslations, useLocale } from "next-intl";
import { useEffect, useState } from "react";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { ChevronDown, ChevronUp } from "lucide-react";

interface Version {
  id: number;
  version_number: number;
  content_item_id: number;
  author_id: number;
  storage_type: string;
  message?: string;
  is_latest: boolean;
  created_at: string;
}

interface VersionContent {
  content?: string;
}

interface VersionHistoryProps {
  contentId: number;
  isAuthor?: boolean;
}

export function VersionHistory({ contentId, isAuthor }: VersionHistoryProps) {
  const t = useTranslations();
  const locale = useLocale();
  const [versions, setVersions] = useState<Version[]>([]);
  const [loading, setLoading] = useState(true);
  const [mobileExpanded, setMobileExpanded] = useState(false);
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<{ version: Version; content: string } | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void loadVersions();
  }, [contentId]);

  async function loadVersions() {
    setError("");
    setLoading(true);
    try {
      const data = await api.get<{ versions?: Version[] }>(`/api/v1/contents/${contentId}/versions`);
      setVersions(data.versions || []);
    } catch (e) {
      setError(e instanceof ApiRequestError ? `${e.code}: ${e.message}` : t('content.versionLoadFailed'));
    } finally {
      setLoading(false);
    }
  }

  async function previewVersion(version: Version) {
    setBusy(true);
    try {
      const data = await api.get<VersionContent>(`/api/v1/versions/${version.id}`);
      setPreview({ version, content: data.content || t('common.noData') });
    } catch {
      setError(t('content.versionContentLoadFailed'));
    } finally {
      setBusy(false);
    }
  }

  async function rollbackToVersion(version: Version) {
    if (!window.confirm(t('content.restoreConfirm', { version: version.version_number }))) {
      return;
    }
    setBusy(true);
    try {
      await api.post(`/api/v1/contents/${contentId}/versions`, {
        base_version_id: version.id,
      });
      await loadVersions();
    } catch {
      setError(t('content.versionRestoreFailed'));
    } finally {
      setBusy(false);
    }
  }

  if (loading) {
    return <div className="space-y-2 rounded-md border border-border bg-card p-4 "><div className="h-8 w-48 animate-pulse rounded bg-muted" /><div className="h-4 w-64 animate-pulse rounded bg-muted" /></div>;
  }

  if (error) {
    return <p className="text-sm text-destructive">{error}</p>;
  }

  if (versions.length === 0) {
    return <p className="text-sm text-muted-foreground">{t('content.noVersionHistory')}</p>;
  }

  return (
    <div className="space-y-3 rounded-md border border-border bg-card p-4 ">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">{t('content.versionHistory')}</h3>
        <button
          type="button"
          className="rounded p-1 hover:bg-muted md:hidden"
          onClick={() => setMobileExpanded(!mobileExpanded)}
        >
          {mobileExpanded ? (
            <ChevronDown className="h-4 w-4" />
          ) : (
            <ChevronUp className="h-4 w-4" />
          )}
        </button>
      </div>

      <div className={`relative ml-2 border-l-2 border-border pl-6 ${mobileExpanded ? "" : "hidden md:block"}`}>
        {versions.map((v) => (
          <div key={v.id} className="relative mb-4">
            <div className="absolute -left-[26px] top-1 h-3 w-3 rounded-full border-2 border-border bg-card" />
            <div className="space-y-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">v{v.version_number}</span>
                {v.is_latest && (
                  <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">{t('content.latest')}</span>
                )}
              </div>
              <p className="text-xs text-muted-foreground">
                {new Date(v.created_at).toLocaleDateString(locale === "en" ? "en-US" : "zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })}
              </p>
              {v.message && <p className="text-xs text-foreground/80">{v.message}</p>}
              <div className="flex gap-2 pt-1">
                <Button size="sm" variant="outline" disabled={busy} onClick={() => void previewVersion(v)}>
                  {t('content.preview')}
                </Button>
                {isAuthor && !v.is_latest && (
                  <Button size="sm" variant="outline" disabled={busy} onClick={() => void rollbackToVersion(v)}>
                    {t('content.restoreVersion')}
                  </Button>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>

      {preview && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={() => setPreview(null)}>
          <div className="max-h-[80vh] w-full max-w-2xl overflow-auto rounded-md border border-border bg-card p-6 shadow-md" onClick={(e) => e.stopPropagation()}>
            <div className="mb-3 flex items-center justify-between">
              <h4 className="text-sm font-semibold">{t('content.versionPreview', { version: preview.version.version_number })}</h4>
              <Button size="sm" variant="outline" onClick={() => setPreview(null)}>{t('common.close')}</Button>
            </div>
            <pre className="whitespace-pre-wrap rounded border border-border bg-muted/20 p-3 text-xs">{preview.content}</pre>
          </div>
        </div>
      )}
    </div>
  );
}
