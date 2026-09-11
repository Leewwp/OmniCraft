"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { BookOpen, Sparkles } from "lucide-react";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/components/ui/Toast";

type GuideLocale = "zh" | "en";

interface SavedGuideRow {
  requirements?: string;
  steps?: string;
  notes?: string;
  source?: string;
}

interface UsageGuideDialogProps {
  contentId: number;
  contentType: string;
  onClose: () => void;
}

function parseJsonArray(raw: unknown): string[] {
  if (typeof raw !== "string" || !raw.trim()) return [];
  try {
    const parsed = JSON.parse(raw) as unknown;
    return Array.isArray(parsed) ? parsed.filter((x): x is string => typeof x === "string") : [];
  } catch {
    return [];
  }
}

// SP-16 #447 studio 最小填写入口：locale 切换 + 三字段（requirements/steps
// 按行拆分数组）+ LLM 草稿建议按钮（走站内 agent 的 draft 强制生成路径）。
export function UsageGuideDialog({ contentId, contentType, onClose }: UsageGuideDialogProps) {
  const t = useTranslations();
  const { toast } = useToast();
  const [locale, setLocale] = useState<GuideLocale>("zh");
  const [requirements, setRequirements] = useState("");
  const [steps, setSteps] = useState("");
  const [notes, setNotes] = useState("");
  const [drafted, setDrafted] = useState(false);
  const [loading, setLoading] = useState(true);
  const [drafting, setDrafting] = useState(false);
  const [saving, setSaving] = useState(false);
  // 切换语言不丢未保存编辑：按 locale 暂存草稿（首次打开该语言时从服务端装载）。
  const drafts = useRef<Partial<Record<GuideLocale, { requirements: string; steps: string; notes: string; drafted: boolean }>>>({});
  const loadedLocales = useRef<Set<GuideLocale>>(new Set());

  const load = useCallback(async (target: GuideLocale) => {
    setLoading(true);
    try {
      const res = await api.get(`/api/v1/contents/${contentId}/guide/specifics?locale=${target}`) as Record<string, unknown>;
      const row = (res?.guide ?? null) as SavedGuideRow | null;
      setRequirements((row ? parseJsonArray(row.requirements) : []).join("\n"));
      setSteps((row ? parseJsonArray(row.steps) : []).join("\n"));
      setNotes(row?.notes ?? "");
      setDrafted(row?.source === "llm_assisted");
    } catch {
      toast("error", t(getUserFacingErrorKey(null, "studio.guide.loadFailed")));
    } finally {
      loadedLocales.current.add(target);
      setLoading(false);
    }
  }, [contentId, t, toast]);

  useEffect(() => {
    // 已装载过的语言不再重拉——保留切换语言时还原的本地草稿。
    if (loadedLocales.current.has(locale)) return;
    void load(locale);
  }, [load, locale]);

  function switchLocale(target: GuideLocale) {
    if (target === locale) return;
    // 暂存当前语言的未保存编辑，目标语言已有暂存则还原，否则按需首次装载。
    drafts.current[locale] = { requirements, steps, notes, drafted };
    if (drafts.current[target]) {
      const cached = drafts.current[target]!;
      loadedLocales.current.add(target);
      setLocale(target);
      setRequirements(cached.requirements);
      setSteps(cached.steps);
      setNotes(cached.notes);
      setDrafted(cached.drafted);
    } else {
      setLocale(target);
    }
  }

  async function requestDraft() {
    setDrafting(true);
    try {
      const res = await api.get(`/api/v1/agent/usage-guide/${contentId}?draft=true`) as Record<string, unknown>;
      const guide = typeof res?.guide === "string" ? res.guide : "";
      if (guide) {
        // 草稿是 Markdown 文本，直接进 notes 作为起点；作者确认/改写后保存。
        setNotes((current) => (current.trim() ? current : guide));
        setDrafted(true);
      }
    } catch (e) {
      silentError(e, { component: "UsageGuideDialog", action: "requestDraft" });
      toast("error", t(getUserFacingErrorKey(e, "studio.guide.draftFailed")));
    } finally {
      setDrafting(false);
    }
  }

  async function save() {
    setSaving(true);
    try {
      await api.put(`/api/v1/contents/${contentId}/guide`, {
        locale,
        requirements: requirements.split("\n").map((s) => s.trim()).filter(Boolean),
        steps: steps.split("\n").map((s) => s.trim()).filter(Boolean),
        notes,
        source: drafted ? "llm_assisted" : "author",
      });
      toast("success", t("studio.guide.saved"));
      onClose();
    } catch (e) {
      silentError(e, { component: "UsageGuideDialog", action: "save" });
      toast("error", t(getUserFacingErrorKey(e, "studio.guide.saveFailed")));
    } finally {
      setSaving(false);
    }
  }

  const localeLabel = (l: GuideLocale) => (l === "zh" ? "中文" : "English");

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={t("studio.guide.dialogTitle")}
      className="fixed inset-0 z-50 flex items-center justify-center bg-foreground/40 p-4"
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div className="flex max-h-[85vh] w-full max-w-lg flex-col rounded-lg border border-border bg-card p-5 shadow-lg">
        <div className="flex items-center gap-2">
          <BookOpen className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-base font-semibold text-foreground">{t("studio.guide.dialogTitle")}</h2>
          <BadgeLikeType label={contentType} />
        </div>
        <p className="mt-1 text-xs text-muted-foreground">{t("studio.guide.dialogHint")}</p>

        <div className="mt-3 flex items-center gap-1">
          {(["zh", "en"] as GuideLocale[]).map((l) => (
            <Button
              key={l}
              type="button"
              size="sm"
              variant={locale === l ? "default" : "outline"}
              className="h-7 px-3 text-xs"
              onClick={() => switchLocale(l)}
              disabled={loading || saving}
            >
              {localeLabel(l)}
            </Button>
          ))}
        </div>

        <div className="mt-4 flex-1 space-y-4 overflow-y-auto pr-1">
          <div className="space-y-1.5">
            <Label htmlFor="guide-requirements">{t("studio.guide.fieldRequirements")}</Label>
            <Textarea
              id="guide-requirements"
              value={requirements}
              onChange={(e) => setRequirements(e.target.value)}
              placeholder={t("studio.guide.requirementsPlaceholder")}
              rows={3}
              disabled={loading || saving}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="guide-steps">{t("studio.guide.fieldSteps")}</Label>
            <Textarea
              id="guide-steps"
              value={steps}
              onChange={(e) => setSteps(e.target.value)}
              placeholder={t("studio.guide.stepsPlaceholder")}
              rows={5}
              disabled={loading || saving}
            />
          </div>
          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <Label htmlFor="guide-notes">{t("studio.guide.fieldNotes")}</Label>
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="h-7 gap-1 text-xs"
                onClick={() => void requestDraft()}
                disabled={drafting || saving || loading}
              >
                <Sparkles className="h-3.5 w-3.5" />
                {drafting ? t("studio.guide.drafting") : t("studio.guide.draftButton")}
              </Button>
            </div>
            <Textarea
              id="guide-notes"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder={t("studio.guide.notesPlaceholder")}
              rows={4}
              disabled={loading || saving}
            />
            {drafted && <p className="text-xs text-muted-foreground">{t("studio.guide.draftedHint")}</p>}
          </div>
        </div>

        <div className="mt-5 flex justify-end gap-2">
          <Button variant="outline" size="sm" onClick={onClose} disabled={saving}>
            {t("common.cancel")}
          </Button>
          <Button size="sm" onClick={() => void save()} disabled={saving || loading}>
            {t("studio.guide.save")}
          </Button>
        </div>
      </div>
    </div>
  );
}

function BadgeLikeType({ label }: { label: string }) {
  return (
    <span className="rounded-sm border border-border px-1.5 py-0.5 text-[10px] text-muted-foreground">
      {label}
    </span>
  );
}
