"use client";

import { useEffect, useState, useCallback, useMemo } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { ScrollText, ChevronDown, ChevronRight, History, GitBranch, Plus, RotateCcw } from "lucide-react";
import { cn } from "@/lib/utils";

interface PromptSlotView {
  name: string;
  description: string;
  required_placeholders: string[];
  production_version: number;
  staging_version: number;
  latest_version: number;
}

interface PromptVersion {
  id: number;
  name: string;
  version: number;
  content: string;
  required_placeholders: string[] | string;
  created_by: number | null;
  created_at: string;
}

interface PromptLabel {
  id: number;
  name: string;
  label: string;
  version: number;
  updated_at: string;
}

const diffLines = (a: string, b: string) => {
  const aLines = a.split("\n");
  const bLines = b.split("\n");
  const rows: { kind: "same" | "add" | "del"; text: string }[] = [];
  let i = 0;
  let j = 0;
  while (i < aLines.length || j < bLines.length) {
    if (i < aLines.length && j < bLines.length && aLines[i] === bLines[j]) {
      rows.push({ kind: "same", text: aLines[i] });
      i++;
      j++;
    } else if (j < bLines.length && (i >= aLines.length || aLines[i] !== bLines[j])) {
      const nextA = aLines.indexOf(bLines[j], i);
      if (nextA === -1) {
        rows.push({ kind: "add", text: bLines[j] });
        j++;
      } else {
        while (i < nextA) {
          rows.push({ kind: "del", text: aLines[i] });
          i++;
        }
      }
    } else if (i < aLines.length) {
      rows.push({ kind: "del", text: aLines[i] });
      i++;
    }
  }
  return rows;
};

export default function AdminPromptsPage() {
  const t = useTranslations();
  const [slots, setSlots] = useState<PromptSlotView[]>([]);
  const [selected, setSelected] = useState<PromptSlotView | null>(null);
  const [versions, setVersions] = useState<PromptVersion[]>([]);
  const [labels, setLabels] = useState<PromptLabel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [editorOpen, setEditorOpen] = useState(false);
  const [draftContent, setDraftContent] = useState("");
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState("");

  const [diffFrom, setDiffFrom] = useState<number | null>(null);
  const [diffTo, setDiffTo] = useState<number | null>(null);

  const [moving, setMoving] = useState(false);

  const loadSlots = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.get<{ slots: PromptSlotView[] }>("/api/v1/admin/prompts");
      setSlots(data.slots || []);
    } catch (e) {
      silentError(e, { component: "AdminPromptsPage", action: "loadSlots" });
      setError(t(getUserFacingErrorKey(e, "admin.prompts.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [t]);

  const loadSlotDetail = useCallback(async (name: string) => {
    try {
      const data = await api.get<{ versions: PromptVersion[]; labels: PromptLabel[] }>(
        `/api/v1/admin/prompts/${name}/versions`
      );
      setVersions(data.versions || []);
      setLabels(data.labels || []);
      setDiffFrom(null);
      setDiffTo(null);
    } catch (e) {
      silentError(e, { component: "AdminPromptsPage", action: "loadSlotDetail" });
    }
  }, []);

  useEffect(() => {
    void loadSlots();
  }, [loadSlots]);

  const openSlot = async (slot: PromptSlotView) => {
    setSelected(slot);
    setEditorOpen(false);
    setFormError("");
    await loadSlotDetail(slot.name);
  };

  const refreshSelected = useCallback(async () => {
    if (!selected) return;
    await Promise.all([loadSlots(), loadSlotDetail(selected.name)]);
  }, [selected, loadSlots, loadSlotDetail]);

  const createVersion = async () => {
    if (!selected) return;
    setSaving(true);
    setFormError("");
    try {
      await api.post(`/api/v1/admin/prompts/${selected.name}/versions`, { content: draftContent });
      setEditorOpen(false);
      setDraftContent("");
      await refreshSelected();
    } catch (e) {
      silentError(e, { component: "AdminPromptsPage", action: "createVersion" });
      const message = (e as { message?: string }).message || "";
      setFormError(
        message.includes("placeholder") || message.includes("占位符")
          ? message
          : t(getUserFacingErrorKey(e, "admin.prompts.saveFailed"))
      );
    } finally {
      setSaving(false);
    }
  };

  const moveLabel = async (label: string, version: number) => {
    if (!selected) return;
    setMoving(true);
    try {
      await api.post(`/api/v1/admin/prompts/${selected.name}/labels`, { label, version });
      await refreshSelected();
    } catch (e) {
      silentError(e, { component: "AdminPromptsPage", action: "moveLabel" });
      setError(t(getUserFacingErrorKey(e, "admin.prompts.moveFailed")));
    } finally {
      setMoving(false);
    }
  };

  const versionByNumber = useCallback(
    (v: number) => versions.find((row) => row.version === v),
    [versions]
  );

  const diffRows = useMemo(() => {
    if (diffFrom == null || diffTo == null) return null;
    const from = versionByNumber(diffFrom);
    const to = versionByNumber(diffTo);
    if (!from || !to) return null;
    return diffLines(from.content, to.content);
  }, [diffFrom, diffTo, versionByNumber]);

  const productionVersion = labels.find((l) => l.label === "production")?.version ?? 0;
  const stagingVersion = labels.find((l) => l.label === "staging")?.version ?? 0;

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-xl font-bold">{t("admin.prompts.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("admin.prompts.subtitle")}</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {loading ? (
        <div className="rounded-md border border-border bg-card p-8 text-center">
          <ScrollText className="mx-auto h-8 w-8 animate-pulse text-muted-foreground" />
          <p className="mt-2 text-sm text-muted-foreground">{t("admin.prompts.loading")}</p>
        </div>
      ) : (
        <div className="grid gap-4 lg:grid-cols-[340px_1fr]">
          <div className="space-y-2">
            {slots.map((slot) => (
              <button
                key={slot.name}
                type="button"
                onClick={() => void openSlot(slot)}
                className={cn(
                  "w-full rounded-md border border-border bg-card p-3 text-left transition-colors hover:bg-muted/50",
                  selected?.name === slot.name && "border-indigo-400 bg-muted"
                )}
              >
                <p className="font-mono text-sm font-medium">{slot.name}</p>
                <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">{slot.description}</p>
                <p className="mt-1.5 flex flex-wrap gap-1 text-[10px]">
                  <span className="rounded-full bg-emerald-100 px-1.5 py-0.5 font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400">
                    production v{slot.production_version}
                  </span>
                  {slot.staging_version > 0 && (
                    <span className="rounded-full bg-blue-100 px-1.5 py-0.5 font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                      staging v{slot.staging_version}
                    </span>
                  )}
                  <span className="rounded-full bg-muted px-1.5 py-0.5 font-medium text-muted-foreground">
                    latest v{slot.latest_version}
                  </span>
                </p>
              </button>
            ))}
          </div>

          <div className="space-y-4">
            {selected ? (
              <>
                <div className="rounded-md border border-border bg-card p-4">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <h2 className="font-mono text-sm font-semibold">{selected.name}</h2>
                      <p className="mt-0.5 text-xs text-muted-foreground">
                        {t("admin.prompts.placeholders")}:{" "}
                        <span className="font-mono">{selected.required_placeholders.join(", ") || "—"}</span>
                      </p>
                    </div>
                    <Button
                      size="sm"
                      onClick={() => {
                        setEditorOpen((v) => !v);
                        setDraftContent(versionByNumber(selected.latest_version)?.content ?? "");
                        setFormError("");
                      }}
                    >
                      <Plus className="mr-1 h-3.5 w-3.5" />
                      {t("admin.prompts.newVersion")}
                    </Button>
                  </div>

                  {editorOpen && (
                    <div className="mt-3 space-y-2">
                      <textarea
                        className="h-64 w-full rounded-md border border-border bg-background p-2 font-mono text-xs"
                        value={draftContent}
                        onChange={(e) => setDraftContent(e.target.value)}
                        aria-label={t("admin.prompts.editorLabel")}
                      />
                      {formError && <p className="text-xs text-destructive">{formError}</p>}
                      <div className="flex gap-2">
                        <Button size="sm" disabled={saving || !draftContent.trim()} onClick={() => void createVersion()}>
                          {t("admin.prompts.save")}
                        </Button>
                        <Button size="sm" variant="outline" onClick={() => setEditorOpen(false)}>
                          {t("common.cancel")}
                        </Button>
                      </div>
                    </div>
                  )}
                </div>

                <div className="rounded-md border border-border bg-card">
                  <div className="flex items-center gap-2 border-b border-border p-3">
                    <History className="h-4 w-4 text-muted-foreground" />
                    <h3 className="text-sm font-semibold">{t("admin.prompts.versions")}</h3>
                    <div className="ml-auto flex items-center gap-1 text-xs text-muted-foreground">
                      diff:
                      <select
                        aria-label={t("admin.prompts.diffFrom")}
                        className="h-7 rounded border border-border bg-background px-1 text-xs"
                        value={diffFrom ?? ""}
                        onChange={(e) => setDiffFrom(e.target.value ? Number(e.target.value) : null)}
                      >
                        <option value="">—</option>
                        {versions.map((v) => (
                          <option key={v.version} value={v.version}>v{v.version}</option>
                        ))}
                      </select>
                      →
                      <select
                        aria-label={t("admin.prompts.diffTo")}
                        className="h-7 rounded border border-border bg-background px-1 text-xs"
                        value={diffTo ?? ""}
                        onChange={(e) => setDiffTo(e.target.value ? Number(e.target.value) : null)}
                      >
                        <option value="">—</option>
                        {versions.map((v) => (
                          <option key={v.version} value={v.version}>v{v.version}</option>
                        ))}
                      </select>
                    </div>
                  </div>

                  {diffRows && (
                    <div className="border-b border-border p-3">
                      <p className="mb-1.5 text-xs font-medium text-muted-foreground">
                        v{diffFrom} → v{diffTo}
                      </p>
                      <pre className="max-h-72 overflow-auto rounded border border-border bg-background p-2 text-xs">
                        {diffRows.map((row, i) => (
                          <span
                            key={i}
                            className={cn(
                              "block",
                              row.kind === "add" && "bg-emerald-50 text-emerald-800 dark:bg-emerald-950/30 dark:text-emerald-400",
                              row.kind === "del" && "bg-red-50 text-red-800 line-through dark:bg-red-950/30 dark:text-red-400"
                            )}
                          >
                            {row.kind === "add" ? "+ " : row.kind === "del" ? "- " : "  "}
                            {row.text}
                          </span>
                        ))}
                      </pre>
                    </div>
                  )}

                  <div className="divide-y divide-border">
                    {versions.map((v) => {
                      const isProd = v.version === productionVersion;
                      const isStaging = v.version === stagingVersion;
                      return (
                        <div key={v.id} className="flex items-start gap-3 p-3">
                          {v.content !== versions[0]?.content ? <ChevronRight className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" /> : <ChevronDown className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />}
                          <div className="min-w-0 flex-1">
                            <p className="flex flex-wrap items-center gap-1.5 text-sm font-medium">
                              v{v.version}
                              {isProd && (
                                <span className="rounded-full bg-emerald-100 px-1.5 py-0.5 text-[10px] font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400">
                                  production
                                </span>
                              )}
                              {isStaging && (
                                <span className="rounded-full bg-blue-100 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                                  staging
                                </span>
                              )}
                            </p>
                            <p className="mt-0.5 text-xs text-muted-foreground">
                              {new Date(v.created_at).toLocaleString()}
                              {v.created_by ? ` · admin #${v.created_by}` : ""}
                            </p>
                            <pre className="mt-1.5 max-h-40 overflow-auto rounded border border-border bg-background p-2 text-[11px] whitespace-pre-wrap">{v.content}</pre>
                          </div>
                          <div className="flex shrink-0 flex-col gap-1">
                            <Button
                              size="sm"
                              variant="outline"
                              className="h-7 text-xs"
                              disabled={moving || isProd}
                              onClick={() => void moveLabel("production", v.version)}
                              title={t("admin.prompts.moveProduction")}
                            >
                              <RotateCcw className="mr-1 h-3 w-3" />
                              {t("admin.prompts.moveProduction")}
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              className="h-7 text-xs"
                              disabled={moving || isStaging}
                              onClick={() => void moveLabel("staging", v.version)}
                            >
                              <GitBranch className="mr-1 h-3 w-3" />
                              {t("admin.prompts.moveStaging")}
                            </Button>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>

                <p className="text-xs text-muted-foreground">{t("admin.prompts.auditHint")}</p>
              </>
            ) : (
              <div className="rounded-md border border-border bg-card p-8 text-center">
                <p className="text-sm text-muted-foreground">{t("admin.prompts.selectSlot")}</p>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
