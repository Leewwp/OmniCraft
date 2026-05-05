"use client";

import { useEffect, useState, useCallback } from "react";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";

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
  const [categories, setCategories] = useState<Category[]>([]);
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
      const data = await api.get<{ categories: Category[] }>(
        "/api/v1/categories?zone=fanwork&level=category"
      );
      setCategories(data.categories || []);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "加载分类失败");
    } finally {
      setLoading(false);
    }
  }, []);

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
      setError(e instanceof ApiRequestError ? e.message : "创建分类失败");
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
      setError(e instanceof ApiRequestError ? e.message : "更新分类失败");
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
      setError(e instanceof ApiRequestError ? e.message : "删除分类失败");
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
      setError(e instanceof ApiRequestError ? e.message : "排序保存失败");
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

  if (loading) {
    return (
      <div className="space-y-4 p-6">
        <div className="space-y-3 rounded-md border border-border bg-card p-6 shadow-none">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-8 w-full animate-pulse rounded bg-muted" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4 shadow-none">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">分类与标签管理</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            管理内容分类（共 {categories.length} 个分类）
          </p>
        </div>
        <Button size="sm" disabled={showCreate} onClick={() => setShowCreate(true)}>
          新建分类
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {/* Create form */}
      {showCreate && (
        <div className="rounded-md border border-border bg-card p-4 shadow-none">
          <h3 className="mb-3 text-sm font-semibold">新建分类</h3>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <label className="block text-xs font-medium mb-1">分区 (zone)</label>
              <select
                className="w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                value={createValues.zone}
                onChange={(e) => setCreateValues((v) => ({ ...v, zone: e.target.value }))}
              >
                <option value="fanwork">fanwork (二创区)</option>
                <option value="original">original (原创区)</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">层级 (level)</label>
              <select
                className="w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                value={createValues.level}
                onChange={(e) => setCreateValues((v) => ({ ...v, level: e.target.value }))}
              >
                <option value="category">category (一级分类)</option>
                <option value="content_type">content_type (二级分类)</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">中文名称</label>
              <input
                type="text"
                className="w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                value={createValues.name_zh}
                onChange={(e) => setCreateValues((v) => ({ ...v, name_zh: e.target.value }))}
                placeholder="推荐"
              />
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">英文名称</label>
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
              <label className="block text-xs font-medium mb-1">排序</label>
              <input
                type="number"
                className="w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                value={createValues.sort_order}
                onChange={(e) => setCreateValues((v) => ({ ...v, sort_order: e.target.value }))}
                min={0}
              />
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">父级 ID</label>
              <input
                type="number"
                className="w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                value={createValues.parent_id}
                onChange={(e) => setCreateValues((v) => ({ ...v, parent_id: e.target.value }))}
                placeholder="留空为顶级"
              />
            </div>
          </div>
          <div className="mt-4 flex gap-2">
            <Button size="sm" disabled={busy} onClick={() => void createCategory()}>
              {busy ? "创建中..." : "创建"}
            </Button>
            <Button size="sm" variant="outline" onClick={() => setShowCreate(false)}>
              取消
            </Button>
          </div>
        </div>
      )}

      {categories.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center shadow-none">
          <p className="text-sm text-muted-foreground">暂无分类，请创建第一个分类</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border bg-card shadow-none">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border bg-muted/30 text-xs text-muted-foreground">
              <tr>
                <th className="px-4 py-3 font-medium w-8">排序</th>
                <th className="px-4 py-3 font-medium">名称</th>
                <th className="px-4 py-3 font-medium">Slug</th>
                <th className="px-4 py-3 font-medium">分区</th>
                <th className="px-4 py-3 font-medium">层级</th>
                <th className="px-4 py-3 font-medium">父级ID</th>
                <th className="px-4 py-3 font-medium">状态</th>
                <th className="px-4 py-3 font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              {categories.map((cat, idx) => (
                <tr key={cat.id} className="border-b border-border hover:bg-muted/20">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1">
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
                          {cat.is_active ? "启用" : "禁用"}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex gap-1">
                          <Button size="sm" disabled={busy} onClick={() => void updateCategory(cat.id)}>
                            保存
                          </Button>
                          <Button size="sm" variant="outline" onClick={cancelEdit}>
                            取消
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
                          {cat.is_active ? "启用" : "禁用"}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex gap-1">
                          <Button size="sm" variant="outline" onClick={() => startEdit(cat)}>
                            编辑
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
                            删除
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
        title="删除分类"
        description={confirmAction ? `确认删除分类「${confirmAction.name}」吗？如果该分类下有关联内容，删除将被拒绝。` : ""}
        confirmLabel="确认删除"
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
