"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { api, ApiRequestError } from "@/lib/api";
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
      setError(e instanceof ApiRequestError ? e.message : t('admin.categories.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadCategories();
  }, [loadCategories]);

  async function createCategory() {
    if (!createValues.name_zh.trim() || !createValues.slug.trim()) return;
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
      setCreateValues({ zone: "fanwork", level: "category", parent_id: "", name_zh: "", name_en: "", slug: "", sort_order: "0" });
      await loadCategories();
    } catch (e) {
      silentError(e, { component: 'AdminCategoriesPage', action: 'createCategory' });
      setError(e instanceof ApiRequestError ? e.message : t('admin.categories.createFailed'));
    } finally {
      setBusy(false);
    }
  }

  async function updateCategory(id: number) {
    if (!editValues.name_zh?.trim()) return;
    setBusy(true);
    setError("");
    try {
      await api.patch(`/api/v1/admin/categories/${id}`, {
        name_i18n: { zh: editValues.name_zh, en: editValues.name_en || editValues.name_zh },
        slug: editValues.slug,
        sort_order: Number(editValues.sort_order),
      });
      setEditingId(null);
      await loadCategories();
    } catch (e) {
      silentError(e, { component: 'AdminCategoriesPage', action: 'updateCategory' });
      setError(e instanceof ApiRequestError ? e.message : t('admin.categories.updateFailed'));
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
      setError(e instanceof ApiRequestError ? e.message : t('admin.categories.deleteFailed'));
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
      setError(e instanceof ApiRequestError ? e.message : t('admin.categories.sortFailed'));
      await loadCategories();
    }
  }

  function startEdit(cat: Category) {
    setEditingId(cat.id);
    setEditValues({
      name_zh: (cat.name_i18n as Record<string, string>)?.zh || "",
      name_en: (cat.name_i18n as Record<string, string>)?.en || "",
      slug: cat.slug,
      sort_order: String(cat.sort_order),
    });
  }

  function cancelEdit() {
    setEditingId(null);
    setEditValues({});
  }

  const parentOptions = parentCategories.filter(
    (cat) => cat.zone === createValues.zone && cat.level === "category",
  );

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
          <h1 className="text-2xl font-bold tracking-tight">{t('admin.categories.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t('admin.categories.subtitle', { count: categories.length })}
          </p>
        </div>
        <Button size="sm" disabled={showCreate} onClick={() => setShowCreate(true)}>
          {t('admin.categories.newCategory')}
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {/* Create form */}
      {showCreate && (
        <div className="rounded-md border border-border bg-card p-4 ">
          <h3 className="mb-3 text-sm font-semibold">{t('admin.categories.newCategory')}</h3>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <label className="block text-xs font-medium mb-1">{t('admin.categories.colZone')} (zone)</label>
              <Select
                aria-label={t('admin.categories.colZone')}
                className="px-2 py-1.5 text-sm"
                value={createValues.zone}
                onChange={(e) => setCreateValues((v) => ({ ...v, zone: e.target.value, parent_id: "" }))}
              >
                <option value="fanwork">{t('admin.categories.zoneFanwork')}</option>
                <option value="original">{t('admin.categories.zoneOriginal')}</option>
              </Select>
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">{t('admin.categories.colLevel')} (level)</label>
              <Select
                aria-label={t('admin.categories.colLevel')}
                className="px-2 py-1.5 text-sm"
                value={createValues.level}
                onChange={(e) => setCreateValues((v) => ({ ...v, level: e.target.value }))}
              >
                <option value="category">{t('admin.categories.levelCategory')}</option>
                <option value="content_type">{t('admin.categories.levelContentType')}</option>
              </Select>
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">{t('admin.categories.nameZh')}</label>
              <input
                type="text"
                className="w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                value={createValues.name_zh}
                onChange={(e) => setCreateValues((v) => ({ ...v, name_zh: e.target.value }))}
                placeholder={t('home.hottest')}
              />
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">{t('admin.categories.nameEn')}</label>
              <input
                type="text"
                className="w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                value={createValues.name_en}
                onChange={(e) => setCreateValues((v) => ({ ...v, name_en: e.target.value }))}
                placeholder="Recommended"
              />
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">Slug</label>
              <input
                type="text"
                className="w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                value={createValues.slug}
                onChange={(e) => setCreateValues((v) => ({ ...v, slug: e.target.value }))}
                placeholder="recommended"
              />
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">{t('admin.categories.sort')}</label>
              <Input
                type="text"
                inputMode="numeric"
                pattern="[0-9]*"
                aria-label={t('admin.categories.sort')}
                value={createValues.sort_order}
                onChange={(e) => setCreateValues((v) => ({ ...v, sort_order: e.target.value.replace(/\D/g, '') }))}
              />
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">{t('admin.categories.parentId')}</label>
              <Select
                aria-label={t('admin.categories.parentId')}
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
            <Button size="sm" disabled={busy} onClick={() => void createCategory()}>
              {busy ? t('admin.categories.creating') : t('admin.categories.create')}
            </Button>
            <Button size="sm" variant="outline" onClick={() => setShowCreate(false)}>
              {t('common.cancel')}
            </Button>
          </div>
        </div>
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
                        className="text-xs text-muted-foreground hover:text-foreground disabled:opacity-30"
                        disabled={idx === 0}
                        onClick={() => void moveUp(idx)}
                      >
                        ▲
                      </button>
                      <button
                        className="text-xs text-muted-foreground hover:text-foreground disabled:opacity-30"
                        disabled={idx === categories.length - 1}
                        onClick={() => void moveDown(idx)}
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
                            type="text"
                            className="w-20 rounded border border-border bg-background px-1.5 py-1 text-xs focus:outline-none focus:ring-2 focus:ring-ring"
                            value={editValues.name_zh || ""}
                            onChange={(e) => setEditValues((v) => ({ ...v, name_zh: e.target.value }))}
                          />
                          <input
                            type="text"
                            className="w-20 rounded border border-border bg-background px-1.5 py-1 text-xs focus:outline-none focus:ring-2 focus:ring-ring"
                            value={editValues.name_en || ""}
                            onChange={(e) => setEditValues((v) => ({ ...v, name_en: e.target.value }))}
                            placeholder="EN"
                          />
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        <input
                          type="text"
                          className="w-24 rounded border border-border bg-background px-1.5 py-1 text-xs focus:outline-none focus:ring-2 focus:ring-ring"
                          value={editValues.slug || ""}
                          onChange={(e) => setEditValues((v) => ({ ...v, slug: e.target.value }))}
                        />
                      </td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{cat.zone}</td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{cat.level}</td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{cat.parent_id ?? "-"}</td>
                      <td className="px-4 py-3">
                        <span
                          className={`rounded px-2 py-0.5 text-xs ${
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
                          className={`rounded px-2 py-0.5 text-xs ${
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
