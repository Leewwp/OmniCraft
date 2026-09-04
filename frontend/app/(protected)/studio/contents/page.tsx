"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { Eye, Heart, MessageCircle, Edit, Trash2, FileText, ShieldQuestion } from "lucide-react";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { DataList } from "@/components/ui/data-list";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { ContentStatusBadge } from "@/components/studio/ContentStatusBadge";
import { useToast } from "@/components/ui/Toast";

interface StudioContentRow {
  id: number; title: string; zone: string; content_type: string;
  view_count: number; like_count: number; comment_count: number;
  status: string;
  cover_image_url?: string;
  is_public?: boolean;
  allow_copy?: boolean;
  ban_reason?: string;
}

export default function StudioContentsPage() {
  const t = useTranslations();
  const { toast } = useToast();
  const [contents, setContents] = useState<StudioContentRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const contentsRef = useRef(contents);
  contentsRef.current = contents;

  // 编辑弹层（FIX-14）：PATCH 契约字段 title/cover/is_public/allow_copy。
  const [editing, setEditing] = useState<StudioContentRow | null>(null);
  const [editForm, setEditForm] = useState({ title: "", cover_image_url: "", is_public: true, allow_copy: true });
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<StudioContentRow | null>(null);

  const load = useCallback(async (nextPage = 1, append = false) => {
    if (append) setLoadingMore(true); else setLoading(contentsRef.current.length === 0);
    setError("");
    setPage(nextPage);
    try {
      const res = await api.get(`/api/v1/users/me/contents?page=${nextPage}&page_size=20`) as Record<string, unknown>;
      const rawItems = (res?.contents ?? res?.data) as Array<Record<string, unknown>> | undefined;
      const incoming = (rawItems || []).map((c) => ({
        id: c.id as number, title: c.title as string, zone: c.zone as string,
        content_type: c.content_type as string, view_count: c.view_count as number,
        like_count: c.like_count as number, comment_count: c.comment_count as number,
        status: c.status as string,
        cover_image_url: typeof c.cover_image_url === "string" ? c.cover_image_url : undefined,
        is_public: typeof c.is_public === "boolean" ? c.is_public : undefined,
        allow_copy: typeof c.allow_copy === "boolean" ? c.allow_copy : undefined,
        ban_reason: typeof c.ban_reason === "string" ? c.ban_reason : undefined,
      }));
      const meta = res?.meta as Record<string, unknown> | undefined;
      const total = (res?.total as number) ?? (meta?.total as number) ?? incoming.length;
      const pageSize = (res?.page_size as number) ?? (meta?.page_size as number) ?? 20;
      setContents((current) => append
        ? [...current, ...incoming.filter((item) => !current.some((existing) => existing.id === item.id))]
        : incoming);
      setPage(nextPage);
      setHasMore(total > nextPage * pageSize);
    } catch {
      setError(t("common.loadFailed"));
    } finally {
      setLoadingMore(false);
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { void load(); }, [load]);

  function openEdit(item: StudioContentRow) {
    setEditing(item);
    setEditForm({
      title: item.title,
      cover_image_url: item.cover_image_url ?? "",
      is_public: item.is_public ?? true,
      allow_copy: item.allow_copy ?? true,
    });
  }

  async function saveEdit() {
    if (!editing) return;
    setSaving(true);
    try {
      await api.patch(`/api/v1/contents/${editing.id}`, {
        title: editForm.title,
        cover_image_url: editForm.cover_image_url,
        is_public: editForm.is_public,
        allow_copy: editForm.allow_copy,
      });
      toast("success", t("studio.contents.editSaved"));
      setEditing(null);
      await load(page, page > 1);
    } catch (e) {
      silentError(e, { component: "StudioContentsPage", action: "saveEdit" });
      toast("error", t(getUserFacingErrorKey(e, "studio.contents.editFailed")));
    } finally {
      setSaving(false);
    }
  }

  async function confirmDelete() {
    if (!deleting) return;
    try {
      await api.delete(`/api/v1/contents/${deleting.id}`);
      toast("success", t("studio.contents.deleted"));
      setDeleting(null);
      await load(page, page > 1);
    } catch (e) {
      silentError(e, { component: "StudioContentsPage", action: "confirmDelete" });
      toast("error", t(getUserFacingErrorKey(e, "studio.contents.deleteFailed")));
    }
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-foreground">{t('studio.contents.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t('studio.contents.subtitle')}</p>
        </div>
        <Link href="/studio/publish/original">
          <Button size="sm">{t('studio.contents.publishNew')}</Button>
        </Link>
      </div>

      <DataList
        items={contents}
        loading={loading}
        error={error}
        onRetry={() => void load(page, page > 1)}
        hasMore={hasMore}
        loadingMore={loadingMore}
        onLoadMore={() => load(page + 1, true)}
        empty={
          <EmptyState
            icon={FileText}
            title={t('studio.contents.noContent')}
            action={<Link href="/studio/publish/original"><Button>{t('studio.contents.startCreating')}</Button></Link>}
          />
        }
        loadingState={<div className="space-y-3">{[1, 2, 3].map((i) => <Skeleton key={i} className="h-20 rounded-lg" />)}</div>}
        getKey={(item) => item.id}
        renderItem={(item) => (
            <div
              key={item.id}
              className="flex items-center gap-4 rounded-lg border border-border bg-card p-4"
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <Link
                    href={`/content/${item.id}`}
                    className="text-sm font-medium text-foreground hover:text-primary truncate"
                  >
                    {item.title}
                  </Link>
                  <Badge variant="secondary" className="text-[10px]">{item.zone}</Badge>
                  <Badge variant="outline" className="text-[10px]">{item.content_type}</Badge>
                  <ContentStatusBadge status={item.status} />
                </div>
                {item.status === "banned" && item.ban_reason && (
                  <p className="mt-1 text-xs text-destructive">{t('studio.contents.banReason')}: {item.ban_reason}</p>
                )}
                <div className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
                  <span className="inline-flex items-center gap-1"><Eye className="h-3 w-3" /> {item.view_count}</span>
                  <span className="inline-flex items-center gap-1"><Heart className="h-3 w-3" /> {item.like_count}</span>
                  <span className="inline-flex items-center gap-1"><MessageCircle className="h-3 w-3" /> {item.comment_count}</span>
                </div>
              </div>
              <div className="flex items-center gap-1">
                {item.status === "banned" && (
                  <Link
                    href={`/appeals?target_type=content&target_id=${item.id}`}
                    className="mr-1"
                  >
                    <Button variant="outline" size="sm" className="h-7 gap-1 text-xs">
                      <ShieldQuestion className="h-3.5 w-3.5" />
                      {t('studio.contents.appeal')}
                    </Button>
                  </Link>
                )}
                <Button
                  variant="ghost"
                  size="icon-sm"
                  title={item.status === "banned" ? t('studio.contents.editBannedDisabled') : t('studio.contents.edit')}
                  disabled={item.status === "banned"}
                  onClick={() => openEdit(item)}
                >
                  <Edit className="h-3.5 w-3.5" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  title={t('studio.contents.delete')}
                  onClick={() => setDeleting(item)}
                >
                  <Trash2 className="h-3.5 w-3.5 text-destructive" />
                </Button>
              </div>
            </div>
        )}
      />

      {editing && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label={t('studio.contents.editDialogTitle')}
          className="fixed inset-0 z-50 flex items-center justify-center bg-foreground/40 p-4"
          onClick={(e) => { if (e.target === e.currentTarget) setEditing(null); }}
        >
          <div className="w-full max-w-md rounded-lg border border-border bg-card p-5 shadow-lg">
            <h2 className="text-base font-semibold text-foreground">{t('studio.contents.editDialogTitle')}</h2>
            <p className="mt-1 text-xs text-muted-foreground">{t('studio.contents.editDialogHint')}</p>
            <div className="mt-4 space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="edit-title">{t('studio.contents.fieldTitle')}</Label>
                <Input
                  id="edit-title"
                  value={editForm.title}
                  onChange={(e) => setEditForm((f) => ({ ...f, title: e.target.value }))}
                  maxLength={200}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="edit-cover">{t('studio.contents.fieldCover')}</Label>
                <Input
                  id="edit-cover"
                  value={editForm.cover_image_url}
                  onChange={(e) => setEditForm((f) => ({ ...f, cover_image_url: e.target.value }))}
                  placeholder="https://cdn.example.test/uploads/..."
                />
              </div>
              <div className="flex items-center justify-between">
                <Label htmlFor="edit-public">{t('studio.contents.fieldPublic')}</Label>
                <Switch
                  id="edit-public"
                  checked={editForm.is_public}
                  onCheckedChange={(v) => setEditForm((f) => ({ ...f, is_public: v }))}
                />
              </div>
              <div className="flex items-center justify-between">
                <Label htmlFor="edit-allow-copy">{t('studio.contents.fieldAllowCopy')}</Label>
                <Switch
                  id="edit-allow-copy"
                  checked={editForm.allow_copy}
                  onCheckedChange={(v) => setEditForm((f) => ({ ...f, allow_copy: v }))}
                />
              </div>
            </div>
            <div className="mt-5 flex justify-end gap-2">
              <Button variant="outline" size="sm" onClick={() => setEditing(null)} disabled={saving}>
                {t('common.cancel')}
              </Button>
              <Button size="sm" onClick={() => void saveEdit()} disabled={saving || !editForm.title.trim()}>
                {t('studio.contents.save')}
              </Button>
            </div>
          </div>
        </div>
      )}

      <ConfirmModal
        open={deleting !== null}
        onOpenChange={(open) => { if (!open) setDeleting(null); }}
        title={t('studio.contents.deleteTitle')}
        description={deleting ? t('studio.contents.deleteDescription', { title: deleting.title }) : ""}
        confirmLabel={t('studio.contents.delete')}
        confirmVariant="destructive"
        onConfirm={() => void confirmDelete()}
      />
    </div>
  );
}
