"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { TagBadge } from "@/components/ui/TagBadge";
import { Button } from "@/components/ui/button";
import { Plus, Pencil, Trash2 } from "lucide-react";

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
  const nameInputRef = useRef<HTMLInputElement>(null);
  const tagInputRef = useRef<HTMLInputElement>(null);
  const modalTriggerRef = useRef<HTMLButtonElement | null>(null);
  const modalDialogRef = useRef<HTMLDivElement>(null);
  const deleteTriggerRef = useRef<HTMLButtonElement | null>(null);
  const deleteDialogRef = useRef<HTMLDivElement>(null);
  const deleteCancelRef = useRef<HTMLButtonElement>(null);

  // Delete confirm
  const [deleteTarget, setDeleteTarget] = useState<TagGroup | null>(null);

  const loadGroups = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.get<{ tag_groups?: TagGroup[] }>("/api/v1/users/me/tag-groups");
      setGroups(res.tag_groups ?? []);
    } catch (e) {
      silentError(e, { component: 'TagGroupsPage', action: 'loadGroups' });
      setError(t(getUserFacingErrorKey(e, "common.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    if (!user) return;
    loadGroups();
  }, [user, loadGroups]);

  useEffect(() => {
    if (!modalOpen) return;
    const focusTimer = window.setTimeout(() => nameInputRef.current?.focus(), 0);
    function handleDialogKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && !modalBusy) {
        setModalOpen(false);
        modalTriggerRef.current?.focus();
        return;
      }
      if (event.key !== "Tab") return;
      const focusable = Array.from(
        modalDialogRef.current?.querySelectorAll<HTMLElement>(
          'button:not([disabled]), input:not([disabled]), [href], [tabindex]:not([tabindex="-1"])',
        ) ?? [],
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }
    document.addEventListener("keydown", handleDialogKeyDown);
    return () => {
      window.clearTimeout(focusTimer);
      document.removeEventListener("keydown", handleDialogKeyDown);
    };
  }, [modalBusy, modalOpen]);

  useEffect(() => {
    if (!deleteTarget) return;
    const focusTimer = window.setTimeout(() => deleteCancelRef.current?.focus(), 0);
    function handleDeleteDialogKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && !busy) {
        setDeleteTarget(null);
        deleteTriggerRef.current?.focus();
        return;
      }
      if (event.key !== "Tab") return;
      const focusable = Array.from(
        deleteDialogRef.current?.querySelectorAll<HTMLElement>('button:not([disabled])') ?? [],
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }
    document.addEventListener("keydown", handleDeleteDialogKeyDown);
    return () => {
      window.clearTimeout(focusTimer);
      document.removeEventListener("keydown", handleDeleteDialogKeyDown);
    };
  }, [busy, deleteTarget]);

  function openCreateModal() {
    modalTriggerRef.current = document.activeElement as HTMLButtonElement | null;
    setEditingGroup(null);
    setModalName("");
    setModalTags([]);
    setTagInput("");
    setModalError("");
    setModalOpen(true);
  }

  function openEditModal(g: TagGroup) {
    modalTriggerRef.current = document.activeElement as HTMLButtonElement | null;
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
      if (!modalName.trim()) {
        nameInputRef.current?.focus();
      } else {
        tagInputRef.current?.focus();
      }
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
      modalTriggerRef.current?.focus();
      await loadGroups();
    } catch (e) {
      silentError(e, { component: 'TagGroupsPage', action: 'handleSave' });
      setModalError(t(getUserFacingErrorKey(e, "common.saveFailed")));
    } finally {
      setModalBusy(false);
    }
  }

  async function handleDelete(g: TagGroup) {
    setBusy(true);
    try {
      await api.delete(`/api/v1/users/me/tag-groups/${g.id}`);
      setDeleteTarget(null);
      deleteTriggerRef.current?.focus();
      await loadGroups();
    } catch (e) {
      silentError(e, { component: 'TagGroupsPage', action: 'handleDelete' });
      setError(t(getUserFacingErrorKey(e)));
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

      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}

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
                    className="[@media(pointer:coarse)]:min-h-11 [@media(pointer:coarse)]:min-w-11 p-0"
                    onClick={() => openEditModal(g)}
                    aria-label={`${t("common.edit")}: ${g.name}`}
                  >
                    <Pencil className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="[@media(pointer:coarse)]:min-h-11 [@media(pointer:coarse)]:min-w-11 p-0 text-destructive hover:text-destructive"
                    onClick={(event) => {
                      deleteTriggerRef.current = event.currentTarget;
                      setDeleteTarget(g);
                    }}
                    aria-label={`${t("common.delete")}: ${g.name}`}
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
          <div
            ref={modalDialogRef}
            role="dialog"
            aria-modal="true"
            aria-labelledby="tag-group-dialog-title"
            className="w-full max-w-md rounded-md border border-border bg-card p-6"
          >
            <h2 id="tag-group-dialog-title" className="text-lg font-semibold">
              {editingGroup ? t("tagGroups.editGroup") : t("tagGroups.newGroup")}
            </h2>

            {modalError && (
              <p id="tag-group-error" role="alert" className="mt-2 text-sm text-destructive">
                {modalError}
              </p>
            )}

            <div className="mt-4 space-y-1">
              <label htmlFor="tag-group-name" className="text-xs font-medium text-muted-foreground">
                {t("tagGroups.groupName")}
              </label>
              <input
                id="tag-group-name"
                ref={nameInputRef}
                type="text"
                value={modalName}
                onChange={(e) => setModalName(e.target.value)}
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent aria-invalid:border-destructive aria-invalid:ring-2 aria-invalid:ring-destructive/20"
                placeholder={t("tagGroups.groupNamePlaceholder")}
                aria-invalid={Boolean(modalError && !modalName.trim())}
                aria-describedby={modalError ? "tag-group-error" : undefined}
              />
            </div>

            <div className="mt-4 space-y-1">
              <label htmlFor="tag-group-tag-input" className="text-xs font-medium text-muted-foreground">
                {t("tagGroups.tags")}
              </label>
              <div className="flex gap-1">
                <input
                  id="tag-group-tag-input"
                  ref={tagInputRef}
                  type="text"
                  value={tagInput}
                  onChange={(e) => setTagInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      addTag();
                    }
                  }}
                  className="flex-1 rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent aria-invalid:border-destructive aria-invalid:ring-2 aria-invalid:ring-destructive/20"
                  placeholder={t("tagGroups.tagPlaceholder")}
                  aria-invalid={Boolean(modalError && modalTags.length === 0)}
                  aria-describedby={modalError ? "tag-group-error" : undefined}
                />
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  className="[@media(pointer:coarse)]:min-h-11 [@media(pointer:coarse)]:min-w-11 p-0"
                  onClick={addTag}
                  aria-label={t("tagGroups.addTag")}
                >
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
                onClick={() => {
                  setModalOpen(false);
                  modalTriggerRef.current?.focus();
                }}
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
          <div
            ref={deleteDialogRef}
            role="dialog"
            aria-modal="true"
            aria-labelledby="tag-group-delete-title"
            aria-describedby="tag-group-delete-description"
            className="w-full max-w-sm rounded-md border border-border bg-card p-6"
          >
            <h3 id="tag-group-delete-title" className="text-sm font-semibold">
              {t("tagGroups.deleteConfirm")}
            </h3>
            <p id="tag-group-delete-description" className="mt-1 text-sm text-muted-foreground">
              {t("tagGroups.deleteConfirmDesc", { name: deleteTarget.name })}
            </p>
            <div className="mt-4 flex justify-end gap-2">
              <Button
                ref={deleteCancelRef}
                variant="outline"
                size="sm"
                onClick={() => {
                  setDeleteTarget(null);
                  deleteTriggerRef.current?.focus();
                }}
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
