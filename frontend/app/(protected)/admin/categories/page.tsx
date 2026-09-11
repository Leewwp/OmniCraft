"use client";

import { FormEvent, useEffect, useRef, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";

interface Category {
  id: number;
  zone: string;
  level: string;
  parent_id: number | null;
  name_i18n: Record<string, string>;
  slug: string;
  sort_order: number;
  is_active: boolean;
}

export default function AdminCategoriesPage() {
  const t = useTranslations();
  const [categories, setCategories] = useState<Category[]>([]);
  const [parentCategories, setParentCategories] = useState<Category[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const [editingId, setEditingId] = useState<number | null>(null);
  const [editValues, setEditValues] = useState<Record<string, string>>({});
  const [showCreate, setShowCreate] = useState(false);
  const [createAttempted, setCreateAttempted] = useState(false);
  const [editAttempted, setEditAttempted] = useState(false);
  const createNameRef = useRef<HTMLInputElement>(null);
  const createSlugRef = useRef<HTMLInputElement>(null);
  const [createValues, setCreateValues] = useState({
    zone: "fanwork",
    level: "category",
    parent_id: "",
    name_zh: "",
    name_en: "",
    slug: "",
    sort_order: "0",
  });

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmAction, setConfirmAction] = useState<{
    type: "delete";
    id: number;
    name: string;
  } | null>(null);

  const loadCategories = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [data, parentsData] = await Promise.all([
        api.get<{ categories: Category[] }>(
          "/api/v1/categories?zone=fanwork&level=category"
        ),
        api.get<{ categories: Category[] }>("/api/v1/categories"),
      ]);
      setCategories(data.categories || []);
      setParentCategories(parentsData.categories || []);
    } catch (e) {
      silentError(e, { component: 'AdminCategoriesPage', action: 'loadCategories' });
      setError(t(getUserFacingErrorKey(e, "admin.categories.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadCategories();
  }, [loadCategories]);

  async function createCategory(event?: FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    setCreateAttempted(true);
    if (!createValues.name_zh.trim() || !createValues.slug.trim()) {
      if (!createValues.name_zh.trim()) {
        createNameRef.current?.focus();
      } else {
        createSlugRef.current?.focus();
      }
      return;
    }
    setBusy(true);
    setError("");
    try {
      await api.post("/api/v1/admin/categories", {
        zone: createValues.zone,
        level: createValues.level,
        parent_id: createValues.parent_id ? Number(createValues.parent_id) : null,
        name_i18n: { zh: createValues.name_zh, en: createValues.name_en || createValues.name_zh },
        slug: createValues.slug,
        sort_order: Number(createValues.sort_order),
      });
      setShowCreate(false);
      setCreateAttempted(false);
      setCreateValues({ zone: "fanwork", level: "category", parent_id: "", name_zh: "", name_en: "", slug: "", sort_order: "0" });
      await loadCategories();
    } catch (e) {
      silentError(e, { component: 'AdminCategoriesPage', action: 'createCategory' });
      setError(t(getUserFacingErrorKey(e, "admin.categories.createFailed")));
    } finally {
      setBusy(false);
    }
  }

  async function updateCategory(id: number) {
    setEditAttempted(true);
    if (!editValues.name_zh?.trim()) {
      document.getElementById(`category-edit-name-zh-${id}`)?.focus();
      return;
    }
    setBusy(true);
    setError("");
    try {
      await api.patch(`/api/v1/admin/categories/${id}`, {
        name_i18n: { zh: editValues.name_zh, en: editValues.name_en || editValues.name_zh },
        slug: editValues.slug,
        sort_order: Number(editValues.sort_order),
      });
      setEditingId(null);
      setEditAttempted(false);
      await loadCategories();
    } catch (e) {
      silentError(e, { component: 'AdminCategoriesPage', action: 'updateCategory' });
      setError(t(getUserFacingErrorKey(e, "admin.categories.updateFailed")));
    } finally {
      setBusy(false);
    }
  }

  async function deleteCategory(id: number) {
    setBusy(true);
    setError("");
    try {
      await api.delete(`/api/v1/admin/categories/${id}`);
      await loadCategories();
    } catch (e) {
      silentError(e, { component: 'AdminCategoriesPage', action: 'deleteCategory' });
      setError(t(getUserFacingErrorKey(e, "admin.categories.deleteFailed")));
    } finally {
      setBusy(false);
    }
  }

  async function moveUp(index: number) {
    if (index <= 0) return;
    const newOrder = [...categories];
    [newOrder[index - 1], newOrder[index]] = [newOrder[index], newOrder[index - 1]];
    setCategories(newOrder);
    await saveReorder(newOrder.map((c) => c.id));
  }

  async function moveDown(index: number) {
    if (index >= categories.length - 1) return;
    const newOrder = [...categories];
    [newOrder[index], newOrder[index + 1]] = [newOrder[index + 1], newOrder[index]];
    setCategories(newOrder);
    await saveReorder(newOrder.map((c) => c.id));
  }

  async function saveReorder(ids: number[]) {
    try {
      await api.put("/api/v1/admin/categories/reorder", { ids });
    } catch (e) {
      silentError(e, { component: 'AdminCategoriesPage', action: 'saveReorder' });
      setError(t(getUserFacingErrorKey(e, "admin.categories.sortFailed")));
      await loadCategories();
    }
  }

  function startEdit(cat: Category) {
    setEditingId(cat.id);
    setEditAttempted(false);
    setEditValues({
      name_zh: (cat.name_i18n as Record<string, string>)?.zh || "",
      name_en: (cat.name_i18n as Record<string, string>)?.en || "",
      slug: cat.slug,
      sort_order: String(cat.sort_order),
    });
  }

  function cancelEdit() {
    setEditingId(null);
    setEditAttempted(false);
    setEditValues({});
  }

  const parentOptions = parentCategories.filter(
    (cat) => cat.zone === createValues.zone && cat.level === "category",
  );
  const createNameInvalid = createAttempted && !createValues.name_zh.trim();
  const createSlugInvalid = createAttempted && !createValues.slug.trim();

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
      <div className="flex flex-col gap-3 rounded-md border border-border bg-card p-4 sm:flex-row sm:items-center sm:justify-between ">
        <div className="min-w-0">
          <h1 className="text-2xl font-bold tracking-tight">{t('admin.categories.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t('admin.categories.subtitle', { count: categories.length })}
          </p>
        </div>
        <Button size="sm" className="self-start sm:self-auto" disabled={showCreate} onClick={() => setShowCreate(true)}>
          {t('admin.categories.newCategory')}
        </Button>
      </div>

      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}

      {/* Create form */}
      {showCreate && (
        <form className="rounded-md border border-border bg-card p-4" onSubmit={createCategory} noValidate>
          <h3 className="mb-3 text-sm font-semibold">{t('admin.categories.newCategory')}</h3>
          {(createNameInvalid || createSlugInvalid) && (
            <p id="category-create-error" role="alert" className="mb-3 text-sm text-destructive">
              {[createNameInvalid ? t('admin.categories.nameRequired') : "", createSlugInvalid ? t('admin.categories.slugRequired') : ""]
                .filter(Boolean)
                .join(" ")}
            </p>
          )}
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <label htmlFor="category-zone" className="block text-xs font-medium mb-1">{t('admin.categories.colZone')} (zone)</label>
              <Select
                id="category-zone"
                className="px-2 py-1.5 text-sm"
                value={createValues.zone}
                onChange={(e) => setCreateValues((v) => ({ ...v, zone: e.target.value, parent_id: "" }))}
              >
                <option value="fanwork">{t('admin.categories.zoneFanwork')}</option>
                <option value="original">{t('admin.categories.zoneOriginal')}</option>
              </Select>
            </div>
            <div>
              <label htmlFor="category-level" className="block text-xs font-medium mb-1">{t('admin.categories.colLevel')} (level)</label>
              <Select
                id="category-level"
                className="px-2 py-1.5 text-sm"
                value={createValues.level}
                onChange={(e) => setCreateValues((v) => ({ ...v, level: e.target.value }))}
              >
                <option value="category">{t('admin.categories.levelCategory')}</option>
                <option value="content_type">{t('admin.categories.levelContentType')}</option>
              </Select>
            </div>
            <div>
              <label htmlFor="category-name-zh" className="block text-xs font-medium mb-1">{t('admin.categories.nameZh')}</label>
              <input
                id="category-name-zh"
                ref={createNameRef}
                type="text"
                className="w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-ring aria-invalid:border-destructive aria-invalid:ring-2 aria-invalid:ring-destructive/20"
                value={createValues.name_zh}
                onChange={(e) => setCreateValues((v) => ({ ...v, name_zh: e.target.value }))}
                placeholder={t('home.hottest')}
                aria-invalid={createNameInvalid}
                aria-describedby={createNameInvalid ? "category-create-error" : undefined}
              />
            </div>
            <div>
              <label htmlFor="category-name-en" className="block text-xs font-medium mb-1">{t('admin.categories.nameEn')}</label>
              <input
                id="category-name-en"
                type="text"
                className="w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-ring aria-invalid:border-destructive aria-invalid:ring-2 aria-invalid:ring-destructive/20"
                value={createValues.name_en}
                onChange={(e) => setCreateValues((v) => ({ ...v, name_en: e.target.value }))}
                placeholder="Recommended"
              />
            </div>
            <div>
              <label htmlFor="category-slug" className="block text-xs font-medium mb-1">Slug</label>
              <input
                id="category-slug"
                ref={createSlugRef}
                type="text"
                className="w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-ring aria-invalid:border-destructive aria-invalid:ring-2 aria-invalid:ring-destructive/20"
                value={createValues.slug}
                onChange={(e) => setCreateValues((v) => ({ ...v, slug: e.target.value }))}
                placeholder="recommended"
                aria-invalid={createSlugInvalid}
                aria-describedby={createSlugInvalid ? "category-create-error" : undefined}
              />
            </div>
            <div>
              <label htmlFor="category-sort" className="block text-xs font-medium mb-1">{t('admin.categories.sort')}</label>
              <Input
                id="category-sort"
                type="text"
                inputMode="numeric"
                pattern="[0-9]*"
                value={createValues.sort_order}
                onChange={(e) => setCreateValues((v) => ({ ...v, sort_order: e.target.value.replace(/\D/g, '') }))}
              />
            </div>
            <div>
              <label htmlFor="category-parent" className="block text-xs font-medium mb-1">{t('admin.categories.parentId')}</label>
              <Select
                id="category-parent"
                className="px-2 py-1.5 text-sm"
                value={createValues.parent_id}
                onChange={(e) => setCreateValues((v) => ({ ...v, parent_id: e.target.value }))}
              >
                <option value="">{t('admin.categories.parentHint')}</option>
                {parentOptions.map((cat) => (
                  <option key={cat.id} value={String(cat.id)}>
                    {(cat.name_i18n as Record<string, string>)?.zh || cat.slug}
                  </option>
                ))}
              </Select>
            </div>
          </div>
          <div className="mt-4 flex gap-2">
            <Button type="submit" size="sm" disabled={busy}>
              {busy ? t('admin.categories.creating') : t('admin.categories.create')}
            </Button>
            <Button type="button" size="sm" variant="outline" onClick={() => { setShowCreate(false); setCreateAttempted(false); }}>
              {t('common.cancel')}
            </Button>
          </div>
        </form>
      )}

      {categories.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center ">
          <p className="text-sm text-muted-foreground">{t('admin.categories.noCategories')}</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border bg-card ">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border bg-muted/30 text-xs text-muted-foreground">
              <tr>
                <th className="px-4 py-3 font-medium w-8">{t('admin.categories.colSort')}</th>
                <th className="px-4 py-3 font-medium">{t('admin.categories.colName')}</th>
                <th className="px-4 py-3 font-medium">Slug</th>
                <th className="px-4 py-3 font-medium">{t('admin.categories.colZone')}</th>
                <th className="px-4 py-3 font-medium">{t('admin.categories.colLevel')}</th>
                <th className="px-4 py-3 font-medium">{t('admin.categories.colParent')}</th>
                <th className="px-4 py-3 font-medium">{t('admin.categories.colStatus')}</th>
                <th className="px-4 py-3 font-medium">{t('admin.categories.colActions')}</th>
              </tr>
            </thead>
            <tbody>
              {categories.map((cat, idx) => (
                <tr key={cat.id} className="border-b border-border hover:bg-muted/20">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1">
                      {editingId === cat.id && (
                        <Input
                          type="text"
                          inputMode="numeric"
                          pattern="[0-9]*"
                          aria-label={t('admin.categories.sort')}
                          className="h-8 w-16 px-1.5 py-1 text-xs"
                          value={editValues.sort_order || ""}
                          onChange={(e) => setEditValues((v) => ({ ...v, sort_order: e.target.value.replace(/\D/g, '') }))}
                        />
                      )}
                      <button
                        type="button"
                        className="[@media(pointer:coarse)]:min-h-11 [@media(pointer:coarse)]:min-w-11 text-xs text-muted-foreground hover:text-foreground disabled:opacity-30"
                        disabled={idx === 0}
                        onClick={() => void moveUp(idx)}
                        aria-label={t('admin.categories.moveUp', { name: (cat.name_i18n as Record<string, string>)?.zh || cat.slug })}
                      >
                        ▲
                      </button>
                      <button
                        type="button"
                        className="[@media(pointer:coarse)]:min-h-11 [@media(pointer:coarse)]:min-w-11 text-xs text-muted-foreground hover:text-foreground disabled:opacity-30"
                        disabled={idx === categories.length - 1}
                        onClick={() => void moveDown(idx)}
                        aria-label={t('admin.categories.moveDown', { name: (cat.name_i18n as Record<string, string>)?.zh || cat.slug })}
                      >
                        ▼
                      </button>
                    </div>
                  </td>
                  {editingId === cat.id ? (
                    <>
                      <td className="px-4 py-3">
                        <div className="flex gap-1">
                          <input
                            id={`category-edit-name-zh-${cat.id}`}
                            type="text"
                            className="w-20 rounded border border-border bg-background px-1.5 py-1 text-xs focus:outline-none focus:ring-2 focus:ring-ring aria-invalid:border-destructive aria-invalid:ring-2 aria-invalid:ring-destructive/20"
                            value={editValues.name_zh || ""}
                            onChange={(e) => setEditValues((v) => ({ ...v, name_zh: e.target.value }))}
                            aria-label={`${t('admin.categories.nameZh')}: ${(cat.name_i18n as Record<string, string>)?.zh || cat.slug}`}
                            aria-invalid={editAttempted && !editValues.name_zh?.trim()}
                            aria-describedby={editAttempted && !editValues.name_zh?.trim() ? `category-edit-error-${cat.id}` : undefined}
                          />
                          <input
                            type="text"
                            className="w-20 rounded border border-border bg-background px-1.5 py-1 text-xs focus:outline-none focus:ring-2 focus:ring-ring"
                            value={editValues.name_en || ""}
                            onChange={(e) => setEditValues((v) => ({ ...v, name_en: e.target.value }))}
                            placeholder="EN"
                            aria-label={`${t('admin.categories.nameEn')}: ${(cat.name_i18n as Record<string, string>)?.en || cat.slug}`}
                          />
                          {editAttempted && !editValues.name_zh?.trim() && (
                            <span id={`category-edit-error-${cat.id}`} role="alert" className="sr-only">
                              {t('admin.categories.nameRequired')}
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        <input
                          type="text"
                          className="w-24 rounded border border-border bg-background px-1.5 py-1 text-xs focus:outline-none focus:ring-2 focus:ring-ring"
                          value={editValues.slug || ""}
                          onChange={(e) => setEditValues((v) => ({ ...v, slug: e.target.value }))}
                          aria-label={t('admin.categories.slugFor', { name: (cat.name_i18n as Record<string, string>)?.zh || cat.slug })}
                        />
                      </td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{cat.zone}</td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{cat.level}</td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{cat.parent_id ?? "-"}</td>
                      <td className="px-4 py-3">
                        <span
                          className={`rounded-full px-2 py-0.5 text-xs ${
                            cat.is_active ? "bg-emerald-50 text-emerald-700" : "bg-muted text-muted-foreground"
                          }`}
                        >
                          {cat.is_active ? t('admin.categories.enable') : t('admin.categories.disable')}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex gap-1">
                          <Button size="sm" disabled={busy} onClick={() => void updateCategory(cat.id)}>
                            {t('admin.categories.saveSort')}
                          </Button>
                          <Button size="sm" variant="outline" onClick={cancelEdit}>
                            {t('common.cancel')}
                          </Button>
                        </div>
                      </td>
                    </>
                  ) : (
                    <>
                      <td className="px-4 py-3 font-medium">
                        {(cat.name_i18n as Record<string, string>)?.zh || cat.slug}
                        <span className="ml-1 text-xs text-muted-foreground">
                          ({(cat.name_i18n as Record<string, string>)?.en || ""})
                        </span>
                      </td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{cat.slug}</td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{cat.zone}</td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{cat.level}</td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{cat.parent_id ?? "-"}</td>
                      <td className="px-4 py-3">
                        <span
                          className={`rounded-full px-2 py-0.5 text-xs ${
                            cat.is_active ? "bg-emerald-50 text-emerald-700" : "bg-muted text-muted-foreground"
                          }`}
                        >
                          {cat.is_active ? t('admin.categories.enable') : t('admin.categories.disable')}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex gap-1">
                          <Button size="sm" variant="outline" onClick={() => startEdit(cat)}>
                            {t('common.edit')}
                          </Button>
                          <Button
                            size="sm"
                            variant="destructive"
                            disabled={busy}
                            onClick={() => {
                              setConfirmAction({
                                type: "delete",
                                id: cat.id,
                                name: (cat.name_i18n as Record<string, string>)?.zh || cat.slug,
                              });
                              setConfirmOpen(true);
                            }}
                          >
                            {t('common.delete')}
                          </Button>
                        </div>
                      </td>
                    </>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmModal
        open={confirmOpen}
        onOpenChange={(v) => { setConfirmOpen(v); if (!v) setConfirmAction(null); }}
        title={t('admin.categories.deleteTitle')}
        description={confirmAction ? t('admin.categories.deleteConfirm', { name: confirmAction.name }) : ""}
        confirmLabel={t('admin.categories.confirmDelete')}
        confirmVariant="destructive"
        onConfirm={async () => {
          if (confirmAction) {
            await deleteCategory(confirmAction.id);
          }
        }}
      />
    </div>
  );
}
