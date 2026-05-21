"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { TagBadge } from "@/components/ui/TagBadge";
import { Button } from "@/components/ui/button";
import { Plus, Pencil, Trash2, X } from "lucide-react";

interface TagGroup {
  id: number;
  name: string;
  tags: string[];
}

export default function TagGroupsPage() {
  const t = useTranslations();
  const { user } = useAuth();
  const [groups, setGroups] = useState<TagGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  // Modal state
  const [modalOpen, setModalOpen] = useState(false);
  const [editingGroup, setEditingGroup] = useState<TagGroup | null>(null);
  const [modalName, setModalName] = useState("");
  const [modalTags, setModalTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");
  const [modalBusy, setModalBusy] = useState(false);
  const [modalError, setModalError] = useState("");

  // Delete confirm
  const [deleteTarget, setDeleteTarget] = useState<TagGroup | null>(null);

  const loadGroups = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.get<{ tag_groups?: TagGroup[] }>("/api/v1/users/me/tag-groups");
      setGroups(res.tag_groups ?? []);
    } catch (e) {
      silentError(e, { component: 'TagGroupsPage', action: 'loadGroups' });
      setError(e instanceof ApiRequestError ? e.message : t("common.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    if (!user) return;
    loadGroups();
  }, [user, loadGroups]);

  function openCreateModal() {
    setEditingGroup(null);
    setModalName("");
    setModalTags([]);
    setTagInput("");
    setModalError("");
    setModalOpen(true);
  }

  function openEditModal(g: TagGroup) {
    setEditingGroup(g);
    setModalName(g.name);
    setModalTags([...g.tags]);
    setTagInput("");
    setModalError("");
    setModalOpen(true);
  }

  function addTag() {
    const trimmed = tagInput.trim();
    if (!trimmed) return;
    if (modalTags.includes(trimmed)) {
      setTagInput("");
      return;
    }
    setModalTags((prev) => [...prev, trimmed]);
    setTagInput("");
  }

  function removeTag(idx: number) {
    setModalTags((prev) => prev.filter((_, i) => i !== idx));
  }

  async function handleSave() {
    if (!modalName.trim() || modalTags.length === 0) {
      setModalError(t("tagGroups.fillAllFields"));
      return;
    }
    setModalBusy(true);
    setModalError("");
    try {
      if (editingGroup) {
        await api.patch(`/api/v1/users/me/tag-groups/${editingGroup.id}`, {
          name: modalName.trim(),
          tags: modalTags,
        });
      } else {
        await api.post("/api/v1/users/me/tag-groups", {
          name: modalName.trim(),
          tags: modalTags,
        });
      }
      setModalOpen(false);
      await loadGroups();
    } catch (e) {
      silentError(e, { component: 'TagGroupsPage', action: 'handleSave' });
      setModalError(e instanceof ApiRequestError ? e.message : t("common.saveFailed"));
    } finally {
      setModalBusy(false);
    }
  }

  async function handleDelete(g: TagGroup) {
    setBusy(true);
    try {
      await api.delete(`/api/v1/users/me/tag-groups/${g.id}`);
      setDeleteTarget(null);
      await loadGroups();
    } catch (e) {
      silentError(e, { component: 'TagGroupsPage', action: 'handleDelete' });
      setError(e instanceof ApiRequestError ? e.message : t("common.operationFailed"));
    } finally {
      setBusy(false);
    }
  }

  if (loading) {
    return (
      <div className="mx-auto w-full max-w-[720px] px-4 py-6 text-sm text-muted-foreground">
        {t("common.loading")}
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-[720px] space-y-6 px-4 py-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("tagGroups.title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("tagGroups.subtitle")}</p>
        </div>
        <Button size="sm" onClick={openCreateModal}>
          <Plus className="mr-1 h-4 w-4" />
          {t("tagGroups.newGroup")}
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {groups.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-8 text-center ">
          <p className="text-sm text-muted-foreground">{t("tagGroups.empty")}</p>
          <Button size="sm" variant="outline" className="mt-3" onClick={openCreateModal}>
            {t("tagGroups.createFirst")}
          </Button>
        </div>
      ) : (
        <div className="space-y-3">
          {groups.map((g) => (
            <div
              key={g.id}
              className="rounded-md border border-border bg-card p-4 "
            >
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0 flex-1">
                  <h3 className="text-sm font-semibold">{g.name}</h3>
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    {g.tags.map((tag) => (
                      <TagBadge key={tag} color="blue">
                        {tag}
                      </TagBadge>
                    ))}
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-1">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-8 w-8 p-0"
                    onClick={() => openEditModal(g)}
                    aria-label={t("common.edit")}
                  >
                    <Pencil className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-8 w-8 p-0 text-destructive hover:text-destructive"
                    onClick={() => setDeleteTarget(g)}
                    aria-label={t("common.delete")}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Create/Edit Modal */}
      {modalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30">
          <div className="w-full max-w-md rounded-md border border-border bg-card p-6 ">
            <h2 className="text-lg font-semibold">
              {editingGroup ? t("tagGroups.editGroup") : t("tagGroups.newGroup")}
            </h2>

            {modalError && <p className="mt-2 text-sm text-destructive">{modalError}</p>}

            <div className="mt-4 space-y-1">
              <label className="text-xs font-medium text-muted-foreground">
                {t("tagGroups.groupName")}
              </label>
              <input
                type="text"
                value={modalName}
                onChange={(e) => setModalName(e.target.value)}
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
                placeholder={t("tagGroups.groupNamePlaceholder")}
              />
            </div>

            <div className="mt-4 space-y-1">
              <label className="text-xs font-medium text-muted-foreground">
                {t("tagGroups.tags")}
              </label>
              <div className="flex gap-1">
                <input
                  type="text"
                  value={tagInput}
                  onChange={(e) => setTagInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      addTag();
                    }
                  }}
                  className="flex-1 rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
                  placeholder={t("tagGroups.tagPlaceholder")}
                />
                <Button type="button" size="sm" variant="outline" onClick={addTag}>
                  <Plus className="h-4 w-4" />
                </Button>
              </div>
              {modalTags.length > 0 && (
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {modalTags.map((tag, idx) => (
                    <TagBadge key={`${tag}-${idx}`} color="purple" onRemove={() => removeTag(idx)}>
                      {tag}
                    </TagBadge>
                  ))}
                </div>
              )}
            </div>

            <div className="mt-6 flex justify-end gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setModalOpen(false)}
                disabled={modalBusy}
              >
                {t("common.cancel")}
              </Button>
              <Button size="sm" onClick={handleSave} disabled={modalBusy}>
                {modalBusy ? t("common.saving") : t("common.save")}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirm Modal */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30">
          <div className="w-full max-w-sm rounded-md border border-border bg-card p-6 ">
            <h3 className="text-sm font-semibold">{t("tagGroups.deleteConfirm")}</h3>
            <p className="mt-1 text-sm text-muted-foreground">
              {t("tagGroups.deleteConfirmDesc", { name: deleteTarget.name })}
            </p>
            <div className="mt-4 flex justify-end gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setDeleteTarget(null)}
                disabled={busy}
              >
                {t("common.cancel")}
              </Button>
              <Button
                size="sm"
                variant="destructive"
                onClick={() => handleDelete(deleteTarget)}
                disabled={busy}
              >
                {busy ? t("common.processing") : t("common.delete")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
