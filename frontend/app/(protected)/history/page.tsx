"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import Link from "next/link";
import { Clock, History, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { EmptyState } from "@/components/ui/empty-state";
import { SkeletonCard } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/Toast";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";

type ContentSummary = {
  id: number;
  title: string;
  zone?: string;
  content_type?: string;
  cover_image_url?: string;
  author?: { id: number; username: string; avatar_url?: string };
};

type HistoryRecord = {
  id: number;
  content?: ContentSummary | null;
  content_item?: ContentSummary | null;
  viewed_at: string;
};

type HistoryResponse = {
  items?: HistoryRecord[];
  history?: HistoryRecord[];
  total?: number;
  page?: number;
  page_size?: number;
  retention_days?: number;
};

type HistoryGroup = {
  label: string;
  records: HistoryRecord[];
};

const contentTypes = [
  "article",
  "image",
  "video",
  "audio",
  "template",
  "sheet_music",
  "mod",
  "prompt",
  "other",
] as const;

type ContentTypeFilter = (typeof contentTypes)[number];

export default function HistoryPage() {
  const t = useTranslations();
  const { toast } = useToast();
  const initialFilters = useMemo(readInitialFilters, []);
  const [records, setRecords] = useState<HistoryRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [retentionDays, setRetentionDays] = useState<number | null>(null);
  const [contentType, setContentType] = useState(initialFilters.contentType);
  const [startDate, setStartDate] = useState(initialFilters.startDate);
  const [endDate, setEndDate] = useState(initialFilters.endDate);
  const [loading, setLoading] = useState(true);
  const [inlineError, setInlineError] = useState(false);
  const [dateError, setDateError] = useState(false);
  const [batchMode, setBatchMode] = useState(false);
  const [selectedIDs, setSelectedIDs] = useState<number[]>([]);
  const [confirmMode, setConfirmMode] = useState<"selected" | "all" | null>(null);
  const recordsRef = useRef<HistoryRecord[]>([]);

  const load = useCallback(async () => {
    if (startDate && endDate && startDate > endDate) {
      setDateError(true);
      setInlineError(false);
      setLoading(false);
      return;
    }
    setDateError(false);

    const params = new URLSearchParams({ page: "1", page_size: "20" });
    if (contentType) params.set("content_type", contentType);
    if (startDate) params.set("start_date", startDate);
    if (endDate) params.set("end_date", endDate);
    const path = `/api/v1/users/me/history?${params.toString()}`;

    try {
      setLoading(recordsRef.current.length === 0);
      const data = await api.get<HistoryResponse>(path);
      const nextRecords = normalizeHistory(data);
      recordsRef.current = nextRecords;
      setRecords(nextRecords);
      setTotal(data.total ?? nextRecords.length);
      setRetentionDays(typeof data.retention_days === "number" ? data.retention_days : null);
      setInlineError(false);
      if (typeof window !== "undefined") {
        window.history.replaceState({}, "", `/history?${params.toString()}`);
      }
    } catch (error) {
      silentError(error, { component: "HistoryPage", action: "load" });
      toast("error", t("history.error.load"));
      setInlineError(recordsRef.current.length > 0);
    } finally {
      setLoading(false);
    }
  }, [contentType, endDate, startDate, t, toast]);

  useEffect(() => {
    void load();
  }, [load]);

  const groups = useMemo(() => groupByDate(records, t), [records, t]);

  async function deleteHistory(mode: "selected" | "all") {
    try {
      const body = mode === "selected" ? { ids: selectedIDs } : { clear_all: true };
      await api.deleteWithBody("/api/v1/users/me/history", body);
      toast("success", t("history.toast.deleted"));
      setSelectedIDs([]);
      setBatchMode(false);
      setConfirmMode(null);
      await load();
    } catch (error) {
      silentError(error, { component: "HistoryPage", action: "deleteHistory" });
      toast("error", t("history.toast.deleteFailed"));
    }
  }

  function toggleSelected(id: number) {
    setSelectedIDs((current) =>
      current.includes(id) ? current.filter((candidate) => candidate !== id) : [...current, id],
    );
  }

  return (
    <main className="mx-auto max-w-[960px] px-4 py-6">
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <History className="h-5 w-5 text-muted-foreground" />
          <h1 className="text-2xl font-semibold">{t("history.title")}</h1>
          {total > 0 && <span className="text-sm text-muted-foreground">({total})</span>}
        </div>
        <div className="flex gap-2">
          {records.length > 0 && (
            <Button variant="outline" size="sm" onClick={() => setBatchMode((value) => !value)}>
              {batchMode ? t("history.bulk.exit") : t("history.bulk.enter")}
            </Button>
          )}
          {records.length > 0 && (
            <Button variant="outline" size="sm" className="text-destructive" onClick={() => setConfirmMode("all")}>
              <Trash2 className="mr-1 h-4 w-4" />
              {t("history.clearAll")}
            </Button>
          )}
        </div>
      </div>

      <section aria-label={t("history.a11y.toolbar")} className="mb-5 rounded-md border border-border p-3">
        <div className="flex gap-2 overflow-x-auto pb-2">
          <FilterButton active={!contentType} onClick={() => setContentType("")}>{t("history.filters.all")}</FilterButton>
          {contentTypes.map((type) => (
            <FilterButton key={type} active={contentType === type} onClick={() => setContentType(contentType === type ? "" : type)}>
              {t(`history.filters.${type}`)}
            </FilterButton>
          ))}
        </div>
        <div className="grid gap-3 pt-2 sm:grid-cols-2">
          <label className="text-sm">
            <span className="mb-1 block text-muted-foreground">{t("history.dateRange.start")}</span>
            <input
              type="date"
              value={startDate}
              onChange={(event) => setStartDate(event.currentTarget.value)}
              className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm"
            />
          </label>
          <label className="text-sm">
            <span className="mb-1 block text-muted-foreground">{t("history.dateRange.end")}</span>
            <input
              type="date"
              value={endDate}
              onChange={(event) => setEndDate(event.currentTarget.value)}
              className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm"
            />
          </label>
        </div>
        {dateError && <p className="mt-3 text-sm text-destructive">{t("history.error.invalidDateRange")}</p>}
      </section>

      {batchMode && (
        <div className="mb-4 flex items-center justify-between rounded-md border border-border p-3">
          <span className="text-sm text-muted-foreground">
            {retentionDays ? t("history.dateRange.retention", { days: retentionDays }) : t("history.dateRange.retentionUnknown")}
          </span>
          <Button
            variant="destructive"
            size="sm"
            disabled={selectedIDs.length === 0}
            onClick={() => setConfirmMode("selected")}
          >
            {t("history.bulk.deleteSelected", { count: selectedIDs.length })}
          </Button>
        </div>
      )}

      {!batchMode && (
        <p className="mb-4 text-sm text-muted-foreground">
          {retentionDays ? t("history.dateRange.retention", { days: retentionDays }) : t("history.dateRange.retentionUnknown")}
        </p>
      )}

      {inlineError && (
        <p className="mb-4 rounded-md border border-destructive/30 p-3 text-sm text-destructive">
          {t("history.error.inline")}
        </p>
      )}

      {loading ? (
        <div className="space-y-3">{Array.from({ length: 5 }).map((_, index) => <SkeletonCard key={index} />)}</div>
      ) : records.length === 0 ? (
        <EmptyState
          icon={Clock}
          title={t("history.empty.title")}
          description={t("history.empty.description")}
          action={
            <Link
              href="/home"
              className="inline-flex h-8 items-center rounded-md bg-primary px-3 text-sm font-medium text-primary-foreground"
            >
              {t("history.empty.action")}
            </Link>
          }
        />
      ) : (
        <div className="space-y-6">
          {groups.map((group) => (
            <section key={group.label}>
              <h2 className="mb-3 text-xs font-medium text-muted-foreground">{group.label}</h2>
              <div className="space-y-3">
                {group.records.map((record) => (
                  <HistoryRow
                    key={record.id}
                    record={record}
                    batchMode={batchMode}
                    selected={selectedIDs.includes(record.id)}
                    onToggle={() => toggleSelected(record.id)}
                    t={t}
                  />
                ))}
              </div>
            </section>
          ))}
        </div>
      )}

      <ConfirmModal
        open={confirmMode !== null}
        onOpenChange={(open) => !open && setConfirmMode(null)}
        title={confirmMode === "selected" ? t("history.bulk.confirmSelectedTitle") : t("history.bulk.confirmAllTitle")}
        description={confirmMode === "selected" ? t("history.bulk.confirmSelectedDescription") : t("history.bulk.confirmAllDescription")}
        confirmLabel={t("history.bulk.confirmDelete")}
        onConfirm={() => {
          if (confirmMode) return deleteHistory(confirmMode);
        }}
      />
    </main>
  );
}

function FilterButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`min-h-11 shrink-0 whitespace-nowrap rounded-md border px-3 text-sm ${active ? "border-primary bg-primary text-primary-foreground" : "border-border bg-background"}`}
    >
      {children}
    </button>
  );
}

function readInitialFilters() {
  if (typeof window === "undefined") {
    return { contentType: "", startDate: "", endDate: "" };
  }
  const params = new URLSearchParams(window.location.search);
  const contentType = params.get("content_type") ?? "";
  return {
    contentType: isContentTypeFilter(contentType) ? contentType : "",
    startDate: params.get("start_date") ?? "",
    endDate: params.get("end_date") ?? "",
  };
}

function isContentTypeFilter(value: string): value is ContentTypeFilter {
  return contentTypes.includes(value as ContentTypeFilter);
}

function HistoryRow({
  record,
  batchMode,
  selected,
  onToggle,
  t,
}: {
  record: HistoryRecord;
  batchMode: boolean;
  selected: boolean;
  onToggle: () => void;
  t: ReturnType<typeof useTranslations>;
}) {
  const content = record.content ?? record.content_item ?? null;
  const viewedAt = new Date(record.viewed_at);
  const labelDate = Number.isNaN(viewedAt.getTime()) ? record.viewed_at : viewedAt.toLocaleString();

  return (
    <div className="grid grid-cols-[auto_1fr_auto] items-center gap-3 rounded-md border border-border bg-card p-3">
      {batchMode && (
        <Checkbox
          checked={selected}
          onChange={onToggle}
          aria-label={
            content
              ? t("history.bulk.select", { title: content.title })
              : t("history.bulk.selectUnavailable", { date: labelDate })
          }
          className="min-h-11 min-w-11"
        />
      )}
      {content ? (
        <Link href={`/content/${content.id}`} className="min-w-0">
          <p className="truncate text-sm font-medium">{content.title}</p>
          <p className="text-xs text-muted-foreground">{content.content_type}</p>
        </Link>
      ) : (
        <div className="min-w-0 rounded-md border border-dashed border-border bg-muted/40 p-3">
          <p className="text-sm font-medium text-muted-foreground">{t("history.unavailable.title")}</p>
          <p className="text-xs text-muted-foreground">{t("history.unavailable.description")}</p>
        </div>
      )}
      <time className="text-xs text-muted-foreground" dateTime={record.viewed_at}>
        {labelDate}
      </time>
    </div>
  );
}

function normalizeHistory(data: HistoryResponse): HistoryRecord[] {
  if (Array.isArray(data.items)) return data.items.map(normalizeRecord);
  if (Array.isArray(data.history)) return data.history.map(normalizeRecord);
  return [];
}

function normalizeRecord(record: HistoryRecord): HistoryRecord {
  const content = record.content ?? record.content_item ?? null;
  return { ...record, content, content_item: content };
}

function groupByDate(records: HistoryRecord[], t: ReturnType<typeof useTranslations>): HistoryGroup[] {
  const today = new Date().toDateString();
  const yesterday = new Date(Date.now() - 86400000).toDateString();
  const groups = new Map<string, HistoryRecord[]>();
  for (const record of records) {
    const date = new Date(record.viewed_at);
    let label = record.viewed_at;
    if (!Number.isNaN(date.getTime())) {
      const dateString = date.toDateString();
      if (dateString === today) label = t("history.today");
      else if (dateString === yesterday) label = t("history.yesterday");
      else label = t("history.date", { year: date.getFullYear(), month: date.getMonth() + 1, day: date.getDate() });
    }
    groups.set(label, [...(groups.get(label) ?? []), record]);
  }
  return Array.from(groups, ([label, groupedRecords]) => ({ label, records: groupedRecords }));
}
