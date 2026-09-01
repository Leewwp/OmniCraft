"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ArrowDown, ArrowLeft, ArrowUp, BookOpen, Plus, Save, Trash2, X } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/components/ui/Toast";
import { ApiRequestError } from "@/lib/api";
import {
  addSeriesItem,
  createSeries,
  deleteSeries,
  getSeriesDetail,
  listSeriesCandidates,
  listOwnedSeries,
  removeSeriesItem,
  reorderSeriesItems,
  updateSeries,
  type SeriesContent,
  type SeriesDetailResponse,
  type SeriesSummary,
  type SeriesZone,
} from "@/lib/series";

export default function StudioSeriesPage() {
  const t = useTranslations();
  const { toast } = useToast();
  const [series, setSeries] = useState<SeriesSummary[]>([]);
  const [selectedID, setSelectedID] = useState<number | null>(null);
  const [detail, setDetail] = useState<SeriesDetailResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [createTitle, setCreateTitle] = useState("");
  const [createDescription, setCreateDescription] = useState("");
  const [createZone, setCreateZone] = useState<SeriesZone>("original");
  const [editTitle, setEditTitle] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [editCoverID, setEditCoverID] = useState("");
  const [search, setSearch] = useState("");
  const [contents, setContents] = useState<SeriesContent[]>([]);
  const [contentsLoading, setContentsLoading] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [mobileDetailOpen, setMobileDetailOpen] = useState(false);
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const detailRequestRef = useRef(0);
  const selectedIDRef = useRef<number | null>(null);

  const loadDetail = useCallback(async (id: number) => {
    const requestID = ++detailRequestRef.current;
    setDetailLoading(true);
    setDetail(null);
    try {
      const nextDetail = await getSeriesDetail(id, { manage: true });
      if (requestID !== detailRequestRef.current) return;
      setDetail(nextDetail);
      setEditTitle(nextDetail.series.title);
      setEditDescription(nextDetail.series.description);
      setEditCoverID(nextDetail.series.cover_content_id ? String(nextDetail.series.cover_content_id) : "");
    } catch {
      if (requestID === detailRequestRef.current) toast("error", t("studio.series.error.loadFailed"));
    } finally {
      if (requestID === detailRequestRef.current) setDetailLoading(false);
    }
  }, [t, toast]);

  const loadSeries = useCallback(async (preferredID?: number) => {
    setLoading(true);
    setError(false);
    try {
      const nextSeries = await listOwnedSeries();
      setSeries(nextSeries);
      const nextID = preferredID && nextSeries.some((item) => item.id === preferredID)
        ? preferredID
        : nextSeries[0]?.id ?? null;
      setSelectedID(nextID);
      selectedIDRef.current = nextID;
      if (nextID) await loadDetail(nextID);
      else setDetail(null);
    } catch (err) {
      setError(true);
      if (!(err instanceof ApiRequestError)) toast("error", t("studio.series.error.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [loadDetail, t, toast]);

  useEffect(() => {
    void loadSeries();
  }, [loadSeries]);

  useEffect(() => {
    let cancelled = false;
    const timer = window.setTimeout(() => {
    async function loadContents() {
      if (!detail?.series.zone) {
        setContents([]);
        return;
      }
      setContentsLoading(true);
      try {
        const response = await listSeriesCandidates(detail.series.zone, search);
        if (!cancelled) setContents(response);
      } catch {
        if (!cancelled) setContents([]);
      } finally {
        if (!cancelled) setContentsLoading(false);
      }
    }
    void loadContents();
    }, 250);
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [detail?.series.zone, search]);

  const currentContentIDs = useMemo(() => new Set(detail?.items.map((item) => item.content_item_id) ?? []), [detail]);
  const searchResults = useMemo(() => {
    return contents
      .filter((item) => detail?.series.zone === item.zone && !currentContentIDs.has(item.id))
      .slice(0, 8);
  }, [contents, currentContentIDs, detail?.series.zone]);

  async function handleCreate(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!createTitle.trim() || busyAction) return;
    setBusyAction("create");
    try {
      const response = await createSeries({ title: createTitle.trim(), description: createDescription.trim(), zone: createZone });
      setCreateTitle("");
      setCreateDescription("");
      setShowCreate(false);
      toast("success", t("studio.series.toast.created"));
      await loadSeries(response.series.id);
    } catch {
      toast("error", t("studio.series.error.saveFailed"));
    } finally {
      setBusyAction(null);
    }
  }

  async function handleSave() {
    if (!detail || !editTitle.trim() || busyAction) return;
    setBusyAction("save");
    try {
      await updateSeries(detail.series.id, {
        title: editTitle.trim(),
        description: editDescription.trim(),
        cover_content_id: editCoverID ? Number(editCoverID) : null,
      });
      toast("success", t("studio.series.toast.saved"));
      if (selectedIDRef.current === detail.series.id) await loadDetail(detail.series.id);
      setSeries((items) => items.map((item) => item.id === detail.series.id ? { ...item, title: editTitle.trim(), description: editDescription.trim() } : item));
    } catch {
      toast("error", t("studio.series.error.saveFailed"));
    } finally {
      setBusyAction(null);
    }
  }

  async function handleDelete() {
    if (!detail || busyAction) return;
    setBusyAction("delete");
    try {
      await deleteSeries(detail.series.id);
      setConfirmDelete(false);
      toast("success", t("studio.series.toast.deleted"));
      await loadSeries();
    } catch {
      toast("error", t("studio.series.error.deleteFailed"));
    } finally {
      setBusyAction(null);
    }
  }

  async function handleAdd(contentID: number) {
    if (!detail || busyAction) return;
    setBusyAction("add");
    try {
      await addSeriesItem(detail.series.id, contentID);
      toast("success", t("studio.series.toast.itemAdded"));
      setSearch("");
      if (selectedIDRef.current === detail.series.id) await loadDetail(detail.series.id);
    } catch {
      toast("error", t("studio.series.error.itemFailed"));
    } finally {
      setBusyAction(null);
    }
  }

  async function handleRemove(itemID: number) {
    if (!detail || busyAction) return;
    setBusyAction("remove");
    try {
      await removeSeriesItem(detail.series.id, itemID);
      toast("success", t("studio.series.toast.itemRemoved"));
      if (selectedIDRef.current === detail.series.id) await loadDetail(detail.series.id);
    } catch {
      toast("error", t("studio.series.error.itemFailed"));
    } finally {
      setBusyAction(null);
    }
  }

  async function moveItem(index: number, direction: -1 | 1) {
    if (!detail || busyAction) return;
    const nextIndex = index + direction;
    if (nextIndex < 0 || nextIndex >= detail.items.length) return;
    const nextItems = [...detail.items];
    [nextItems[index], nextItems[nextIndex]] = [nextItems[nextIndex], nextItems[index]];
    try {
      setBusyAction("reorder");
      // reorderSeriesItems sends the complete item_ids set required by the API.
      await reorderSeriesItems(detail.series.id, nextItems.map((item) => item.id));
      setDetail({ ...detail, items: nextItems.map((item, order) => ({ ...item, sort_order: order })) });
      toast("success", t("studio.series.toast.reordered"));
    } catch {
      toast("error", t("studio.series.error.itemFailed"));
    } finally {
      setBusyAction(null);
    }
  }

  if (loading) {
    return <div className="space-y-4" aria-busy="true"><Skeleton className="h-8 w-48" /><div className="grid min-h-[480px] grid-cols-1 gap-4 lg:grid-cols-[320px_minmax(0,1fr)]"><Skeleton className="h-full min-h-80" /><Skeleton className="h-full min-h-80" /></div></div>;
  }

  if (error) {
    return <EmptyState icon={BookOpen} title={t("studio.series.error.loadFailed")} action={<Button type="button" variant="outline" onClick={() => void loadSeries()}>{t("common.retry")}</Button>} />;
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div><h1 className="text-xl font-semibold text-fg-default">{t("studio.series.title")}</h1><p className="mt-1 text-sm text-fg-muted">{t("studio.series.subtitle")}</p></div>
        <Button type="button" onClick={() => setShowCreate((value) => !value)}><Plus className="h-4 w-4" />{t("studio.series.create")}</Button>
      </div>

      {showCreate && (
        <form onSubmit={(event) => void handleCreate(event)} className="space-y-3 rounded-lg border border-border-default bg-card p-4">
          <div className="grid gap-3 md:grid-cols-2">
            <label className="space-y-1 text-sm text-fg-default"><span>{t("studio.series.form.title")}</span><Input value={createTitle} onChange={(event) => setCreateTitle(event.target.value)} required /></label>
            <label className="space-y-1 text-sm text-fg-default"><span>{t("studio.series.form.zone")}</span><select value={createZone} onChange={(event) => setCreateZone(event.target.value as SeriesZone)} className="h-9 w-full rounded-lg border border-border bg-background px-3 text-sm"><option value="original">{t("series.detail.header.zoneOriginal")}</option><option value="fanwork">{t("series.detail.header.zoneFanwork")}</option></select></label>
          </div>
          <label className="block space-y-1 text-sm text-fg-default"><span>{t("studio.series.form.description")}</span><Textarea value={createDescription} onChange={(event) => setCreateDescription(event.target.value)} rows={3} /></label>
          <div className="flex justify-end gap-2"><Button type="button" variant="outline" onClick={() => setShowCreate(false)}>{t("common.cancel")}</Button><Button type="submit">{t("studio.series.form.createSubmit")}</Button></div>
        </form>
      )}

      {series.length === 0 ? (
        <EmptyState icon={BookOpen} title={t("studio.series.empty.title")} description={t("studio.series.empty.description")} action={<Button type="button" onClick={() => setShowCreate(true)}>{t("studio.series.create")}</Button>} />
      ) : (
        <div className="grid min-h-[520px] grid-cols-1 gap-4 lg:grid-cols-[320px_minmax(0,1fr)]">
          <aside className={`${mobileDetailOpen ? "hidden lg:block" : "block"} rounded-lg border border-border-default bg-card p-2`} aria-label={t("studio.series.list.ariaLabel")}>
            <div className="space-y-1">
              {series.map((item) => (
                <button key={item.id} type="button" aria-current={selectedID === item.id ? "true" : undefined} disabled={Boolean(busyAction)} className={`flex min-h-11 w-full items-center rounded-md px-3 text-left text-sm ${selectedID === item.id ? "bg-accent-subtle text-accent-emphasis" : "text-fg-muted hover:bg-canvas-subtle hover:text-fg-default"}`} onClick={() => { selectedIDRef.current = item.id; setSelectedID(item.id); setMobileDetailOpen(true); void loadDetail(item.id); }}>
                  <span className="truncate">{item.title}</span><span className="ml-auto text-xs text-fg-muted">{item.zone === "fanwork" ? t("series.detail.header.zoneFanwork") : t("series.detail.header.zoneOriginal")}</span>
                </button>
              ))}
            </div>
          </aside>

          <section className={`${mobileDetailOpen ? "block" : "hidden lg:block"} min-w-0 rounded-lg border border-border-default bg-card p-4`} aria-label={t("studio.series.detail.ariaLabel")}>
            {detailLoading || !detail ? <Skeleton className="h-72 w-full" /> : (
              <>
                <Button type="button" variant="ghost" className="mb-3 min-h-11 lg:hidden" onClick={() => setMobileDetailOpen(false)}><ArrowLeft className="h-4 w-4" />{t("studio.series.a11y.backToList")}</Button>
                <div className="mb-5 flex items-center justify-between gap-2"><div><h2 className="text-lg font-semibold text-fg-default">{detail.series.title}</h2><p className="text-xs text-fg-muted">{detail.series.zone === "fanwork" ? t("series.detail.header.zoneFanwork") : t("series.detail.header.zoneOriginal")}</p></div><Button type="button" variant="destructive" size="sm" disabled={Boolean(busyAction)} className="min-h-11" onClick={() => setConfirmDelete(true)}><Trash2 className="h-4 w-4" />{t("studio.series.delete")}</Button></div>
                <div className="grid gap-3 md:grid-cols-2"><label className="space-y-1 text-sm text-fg-default"><span>{t("studio.series.form.title")}</span><Input value={editTitle} onChange={(event) => setEditTitle(event.target.value)} /></label><label className="space-y-1 text-sm text-fg-default"><span>{t("studio.series.form.zone")}</span><Input value={detail.series.zone === "fanwork" ? t("series.detail.header.zoneFanwork") : t("series.detail.header.zoneOriginal")} disabled /></label></div>
                <label className="mt-3 block space-y-1 text-sm text-fg-default"><span>{t("studio.series.form.description")}</span><Textarea value={editDescription} onChange={(event) => setEditDescription(event.target.value)} rows={3} /></label>
                <label className="mt-3 block space-y-1 text-sm text-fg-default"><span>{t("studio.series.form.cover")}</span><select value={editCoverID} onChange={(event) => setEditCoverID(event.target.value)} className="h-9 w-full rounded-lg border border-border bg-background px-3 text-sm"><option value="">{t("studio.series.form.coverAutomatic")}</option>{detail.items.map((item) => <option key={item.id} value={item.content_item_id}>{item.content.title}</option>)}</select></label>
                <div className="mt-3 flex justify-end"><Button type="button" disabled={Boolean(busyAction)} className="min-h-11" onClick={() => void handleSave()}><Save className="h-4 w-4" />{t("studio.series.form.save")}</Button></div>

                <div className="mt-6"><h3 className="mb-2 text-sm font-semibold text-fg-default">{t("studio.series.items.title")}</h3>{detail.items.length === 0 ? <EmptyState icon={BookOpen} title={t("studio.series.items.empty")} /> : <ol className="divide-y divide-border-default rounded-md border border-border-default" aria-label={t("studio.series.items.ariaLabel")}>
                  {detail.items.map((item, index) => <li key={item.id} className="flex min-h-14 items-center gap-2 px-3 py-2"><span className="w-7 text-xs text-fg-muted">{index + 1}</span><span className="min-w-0 flex-1 truncate text-sm text-fg-default">{item.content.title}</span><Button type="button" variant="ghost" size="icon" className="min-h-11 min-w-11" aria-label={t("studio.series.a11y.moveUp", { title: item.content.title })} disabled={Boolean(busyAction) || index === 0} onClick={() => void moveItem(index, -1)}><ArrowUp className="h-4 w-4" /></Button><Button type="button" variant="ghost" size="icon" className="min-h-11 min-w-11" aria-label={t("studio.series.a11y.moveDown", { title: item.content.title })} disabled={Boolean(busyAction) || index === detail.items.length - 1} onClick={() => void moveItem(index, 1)}><ArrowDown className="h-4 w-4" /></Button><Button type="button" variant="ghost" size="icon" className="min-h-11 min-w-11" aria-label={t("studio.series.a11y.removeItem", { title: item.content.title })} disabled={Boolean(busyAction)} onClick={() => void handleRemove(item.id)}><X className="h-4 w-4" /></Button></li>)}
                </ol>}</div>

                <div className="mt-6"><h3 className="mb-2 text-sm font-semibold text-fg-default">{t("studio.series.search.title")}</h3><Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t("studio.series.search.placeholder")} aria-label={t("studio.series.search.label")} />{contentsLoading ? <Skeleton className="mt-3 h-12 w-full" /> : <div className="mt-2 space-y-1">{searchResults.map((item) => <div key={item.id} className="flex min-h-11 items-center gap-2 rounded-md border border-border-default px-3"><span className="min-w-0 flex-1 truncate text-sm">{item.title}</span><Button type="button" size="sm" className="min-h-11" disabled={Boolean(busyAction)} onClick={() => void handleAdd(item.id)}>{t("studio.series.items.add")}</Button></div>)}</div>}</div>
              </>
            )}
          </section>
        </div>
      )}

      <ConfirmModal open={confirmDelete} onOpenChange={setConfirmDelete} title={t("studio.series.confirm.deleteTitle")} description={t("studio.series.confirm.deleteDescription", { title: detail?.series.title ?? "" })} confirmLabel={t("studio.series.delete")} onConfirm={handleDelete} />
    </div>
  );
}
