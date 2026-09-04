"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";

interface ConfigData {
  limits: {
    video_max_mb: number;
    video_max_sec: number;
    image_max_mb: number;
    text_max_mb: number;
    mod_max_mb: number;
    sheet_music_max_mb: number;
  };
  features: {
    payment_enabled: boolean;
    creator_support_enabled: boolean;
  };
  reputation: {
    quality_content_threshold: number;
    quality_comment_threshold: number;
    repeat_violation_window_days: number;
    repeat_violation_threshold: number;
    repeat_violation_extra_penalty: number;
  };
  agent: {
    web_agent_enabled: boolean;
    rate_limit_per_day: number;
  };
  social: {
    report_auto_hide_rate: number;
    comment_fold_threshold: number;
  };
  judge: {
    min_votes_required: number;
    pass_threshold: number;
    exam_pass_rate: number;
    error_rate_revoke: number;
    error_rate_window: number;
  };
}

// No client-side default config: fabricating defaults is exactly how the
// config-pollution chain starts (open page → save → persist fake values).
// When the backend config is missing or incomplete the page must stay
// read-only with a retry affordance (T26 FIX-33).
function isCompleteConfig(value: unknown): value is ConfigData {
  if (!value || typeof value !== "object") return false;
  const c = value as Partial<ConfigData>;
  return (
    typeof c.limits?.video_max_mb === "number" &&
    typeof c.features?.payment_enabled === "boolean" &&
    typeof c.reputation?.quality_content_threshold === "number" &&
    typeof c.agent?.web_agent_enabled === "boolean" &&
    typeof c.social?.report_auto_hide_rate === "number" &&
    typeof c.judge?.pass_threshold === "number"
  );
}

export default function AdminConfigPage() {
  const t = useTranslations();
  const [config, setConfig] = useState<ConfigData | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const [invalidFieldId, setInvalidFieldId] = useState<string | null>(null);

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [pendingPatch, setPendingPatch] = useState<Record<string, unknown> | null>(null);

  async function loadConfig() {
    setLoading(true);
    setLoadFailed(false);
    setError("");
    try {
      const data = await api.get<{ config: unknown }>("/api/v1/admin/config");
      // Missing fields (e.g. a stale backend still serializing PascalCase)
      // must be treated as a load failure instead of silently falling back.
      if (isCompleteConfig(data.config)) {
        setConfig(data.config);
      } else {
        setConfig(null);
        setLoadFailed(true);
        setError(t("admin.config.loadFailed"));
      }
    } catch (e) {
      silentError(e, { component: 'AdminConfigPage', action: 'loadConfig' });
      setConfig(null);
      setLoadFailed(true);
      setError(t(getUserFacingErrorKey(e, "admin.config.loadFailed")));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadConfig();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [t]);

  async function saveConfig(patch: Record<string, unknown>) {
    setSaving(true);
    setError("");
    setSaved(false);
    try {
      const data = await api.patch<{ config: unknown }>("/api/v1/admin/config", patch);
      // Replace (not merge) with the server view of the config so the UI can
      // never keep displaying values the backend did not accept.
      if (isCompleteConfig(data.config)) {
        setConfig(data.config);
      }
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    } catch (e) {
      silentError(e, { component: 'AdminConfigPage', action: 'saveConfig' });
      setError(t(getUserFacingErrorKey(e, "admin.config.saveFailed")));
    } finally {
      setSaving(false);
    }
  }

  // Field updaters only run from the main form, which renders exclusively
  // with a loaded config; the guards keep the null state honest for TS.
  function updateLimits(key: string, value: number) {
    setConfig((prev) => (prev ? { ...prev, limits: { ...prev.limits, [key]: value } } : prev));
  }

  function updateReputation(key: string, value: number) {
    setConfig((prev) => (prev ? { ...prev, reputation: { ...prev.reputation, [key]: value } } : prev));
  }

  function updateJudge(key: string, value: number) {
    setConfig((prev) => (prev ? { ...prev, judge: { ...prev.judge, [key]: value } } : prev));
  }

  function updateAgent(key: string, value: number | boolean) {
    setConfig((prev) => (prev ? { ...prev, agent: { ...prev.agent, [key]: value } } : prev));
  }

  function updateSocial(key: string, value: number) {
    setConfig((prev) => (prev ? { ...prev, social: { ...prev.social, [key]: value } } : prev));
  }

  function updateFeatures(key: string, value: boolean) {
    setConfig((prev) => (prev ? { ...prev, features: { ...prev.features, [key]: value } } : prev));
  }

  function buildPatch(): Record<string, unknown> {
    if (!config) return {};
    return {
      limits: { ...config.limits },
      features: { ...config.features },
      reputation: { ...config.reputation },
      agent: { ...config.agent },
      social: { ...config.social },
      judge: { ...config.judge },
    };
  }

  function prepareSave() {
    if (!config) {
      setError(t("admin.config.loadFailed"));
      return;
    }
    const invalidField = document.querySelector<HTMLInputElement>("[data-config-field]:invalid");
    if (invalidField) {
      setInvalidFieldId(invalidField.id);
      setError(t("admin.config.invalidValue"));
      invalidField.focus();
      return;
    }
    setInvalidFieldId(null);
    setError("");
    setPendingPatch(buildPatch());
    setConfirmOpen(true);
  }

  if (loading) {
    return (
      <div className="space-y-4 p-6">
        <div className="space-y-3 rounded-md border border-border bg-card p-6 ">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="h-6 w-full animate-pulse rounded bg-muted" />
          ))}
        </div>
      </div>
    );
  }

  // Load failed (or incomplete payload): no fabricated defaults, no saving —
  // offer a retry instead (T26 FIX-33).
  if (!config) {
    return (
      <div className="space-y-4 p-6">
        <div className="space-y-3 rounded-md border border-destructive/30 bg-destructive/5 p-6">
          <p role="alert" className="text-sm text-destructive">{t("admin.config.loadFailed")}</p>
          <Button size="sm" variant="outline" onClick={loadConfig}>
            {t("common.retry")}
          </Button>
        </div>
      </div>
    );
  }

  function FieldRow({ id, label, value, onChange, min, max, step = 1, unit }: {
    id: string;
    label: string;
    value: number;
    onChange: (v: number) => void;
    min?: number;
    max?: number;
    step?: number;
    unit?: string;
  }) {
    return (
      <div className="flex items-center justify-between gap-4 border-b border-border py-3 last:border-b-0">
        <label htmlFor={id} className="text-sm font-medium">{label}</label>
        <div className="flex items-center gap-2">
          <input
            id={id}
            data-config-field
            type="number"
            className="[@media(pointer:coarse)]:min-h-11 w-24 rounded-md border border-border bg-background px-2 py-1.5 text-right text-sm focus:outline-none focus:ring-2 focus:ring-ring aria-invalid:border-destructive aria-invalid:ring-2 aria-invalid:ring-destructive/20"
            value={value}
            min={min}
            max={max}
            step={step}
            onChange={(e) => {
              onChange(Number(e.target.value));
              if (invalidFieldId === id && e.currentTarget.validity.valid) {
                setInvalidFieldId(null);
                setError("");
              }
            }}
            aria-invalid={invalidFieldId === id}
            aria-describedby={invalidFieldId === id ? "config-error" : undefined}
          />
          {unit && <span className="text-xs text-muted-foreground w-8">{unit}</span>}
        </div>
      </div>
    );
  }

  function ToggleRow({ id, label, value, onChange }: {
    id: string;
    label: string;
    value: boolean;
    onChange: (v: boolean) => void;
  }) {
    return (
      <div className="flex items-center justify-between gap-4 border-b border-border py-3 last:border-b-0">
        <label htmlFor={id} className="text-sm font-medium">{label}</label>
        <button
          id={id}
          type="button"
          role="switch"
          aria-checked={value}
          aria-label={label}
          className="inline-flex [@media(pointer:coarse)]:min-h-11 [@media(pointer:coarse)]:min-w-11 items-center justify-center rounded-full focus:outline-none focus:ring-2 focus:ring-ring"
          onClick={() => onChange(!value)}
        >
          <span className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${value ? "bg-accent" : "bg-muted"}`}>
            <span
              aria-hidden="true"
              className={`inline-block h-4 w-4 rounded-full bg-white transition-transform ${
                value ? "translate-x-6" : "translate-x-1"
              }`}
            />
          </span>
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4 ">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('admin.config.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t('admin.config.subtitle')}
          </p>
        </div>
        <Button
          size="sm"
          disabled={saving}
          onClick={prepareSave}
        >
          {saving ? t('common.saving') : t('admin.config.saveButton')}
        </Button>
      </div>

      {saved && (
        <div role="status" className="rounded-md border border-emerald-500/30 bg-emerald-50 p-3 text-sm text-emerald-700">
          {t('admin.config.saved')}
        </div>
      )}
      {error && <p id="config-error" role="alert" className="text-sm text-destructive">{error}</p>}

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Limits */}
        <div className="rounded-md border border-border bg-card ">
          <div className="border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">{t('admin.config.sectionLimits')}</h3>
          </div>
          <div className="px-4 py-1">
            <FieldRow id="config-video-max-mb" label={t('admin.config.videoMaxMb')} value={config.limits.video_max_mb} onChange={(v) => updateLimits("video_max_mb", v)} min={1} unit="MB" />
            <FieldRow id="config-video-max-sec" label={t('admin.config.videoMaxDuration')} value={config.limits.video_max_sec} onChange={(v) => updateLimits("video_max_sec", v)} min={1} unit="s" />
            <FieldRow id="config-image-max-mb" label={t('admin.config.imageMaxMb')} value={config.limits.image_max_mb} onChange={(v) => updateLimits("image_max_mb", v)} min={1} unit="MB" />
            <FieldRow id="config-text-max-mb" label={t('admin.config.textMaxMb')} value={config.limits.text_max_mb} onChange={(v) => updateLimits("text_max_mb", v)} min={1} unit="MB" />
            <FieldRow id="config-mod-max-mb" label={t('admin.config.modMaxMb')} value={config.limits.mod_max_mb} onChange={(v) => updateLimits("mod_max_mb", v)} min={1} unit="MB" />
            <FieldRow id="config-sheet-music-max-mb" label={t('admin.config.sheetMusicMaxMb')} value={config.limits.sheet_music_max_mb} onChange={(v) => updateLimits("sheet_music_max_mb", v)} min={1} unit="MB" />
          </div>
        </div>

        {/* Features */}
        <div className="rounded-md border border-border bg-card ">
          <div className="border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">{t('admin.config.sectionFeatures')}</h3>
          </div>
          <div className="px-4 py-1">
            <ToggleRow id="config-payment-enabled" label={t('admin.config.paymentEnabled')} value={config.features.payment_enabled} onChange={(v) => updateFeatures("payment_enabled", v)} />
            <ToggleRow id="config-creator-support-enabled" label={t('admin.config.creatorSupport')} value={config.features.creator_support_enabled} onChange={(v) => updateFeatures("creator_support_enabled", v)} />
          </div>
        </div>

        {/* Reputation */}
        <div className="rounded-md border border-border bg-card ">
          <div className="border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">{t('admin.config.sectionReputation')}</h3>
          </div>
          <div className="px-4 py-1">
            <FieldRow id="config-quality-content-threshold" label={t('admin.config.qualityContentThreshold')} value={config.reputation.quality_content_threshold} onChange={(v) => updateReputation("quality_content_threshold", v)} min={1} />
            <FieldRow id="config-quality-comment-threshold" label={t('admin.config.qualityCommentThreshold')} value={config.reputation.quality_comment_threshold} onChange={(v) => updateReputation("quality_comment_threshold", v)} min={1} />
            <FieldRow id="config-repeat-violation-window" label={t('admin.config.repeatViolationWindow')} value={config.reputation.repeat_violation_window_days} onChange={(v) => updateReputation("repeat_violation_window_days", v)} min={1} unit={t('common.units.days')} />
            <FieldRow id="config-repeat-violation-threshold" label={t('admin.config.repeatViolationThreshold')} value={config.reputation.repeat_violation_threshold} onChange={(v) => updateReputation("repeat_violation_threshold", v)} min={1} unit={t('common.units.times')} />
            {/* F-115: penalty semantics allow negative values (extra punishment);
                min=0 would trip native :invalid on the real -1 value and block
                every save once the snake_case contract lands. */}
            <FieldRow id="config-repeat-violation-penalty" label={t('admin.config.repeatViolationPenalty')} value={config.reputation.repeat_violation_extra_penalty} onChange={(v) => updateReputation("repeat_violation_extra_penalty", v)} min={-10} />
          </div>
        </div>

        {/* Judge */}
        <div className="rounded-md border border-border bg-card ">
          <div className="border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">{t('admin.config.sectionJudge')}</h3>
          </div>
          <div className="px-4 py-1">
            <FieldRow id="config-min-votes" label={t('admin.config.minVotes')} value={config.judge.min_votes_required} onChange={(v) => updateJudge("min_votes_required", v)} min={1} unit={t('common.units.votes')} />
            <FieldRow id="config-exam-pass-rate" label={t('admin.config.examPassRate')} value={config.judge.exam_pass_rate} onChange={(v) => updateJudge("exam_pass_rate", v)} min={0} max={1} step={0.05} />
            <FieldRow id="config-verdict-pass-threshold" label={t('admin.config.verdictPassThreshold')} value={config.judge.pass_threshold} onChange={(v) => updateJudge("pass_threshold", v)} min={0} max={1} step={0.05} />
            <FieldRow id="config-revoke-error-rate" label={t('admin.config.revokeErrorRate')} value={config.judge.error_rate_revoke} onChange={(v) => updateJudge("error_rate_revoke", v)} min={0} max={1} step={0.05} />
            <FieldRow id="config-error-rate-window" label={t('admin.config.errorRateWindow')} value={config.judge.error_rate_window} onChange={(v) => updateJudge("error_rate_window", v)} min={1} unit={t('common.units.times')} />
          </div>
        </div>

        {/* Agent */}
        <div className="rounded-md border border-border bg-card ">
          <div className="border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">{t('admin.config.sectionAgent')}</h3>
          </div>
          <div className="px-4 py-1">
            <ToggleRow id="config-web-agent-enabled" label={t('admin.config.webAgent')} value={config.agent.web_agent_enabled} onChange={(v) => updateAgent("web_agent_enabled", v)} />
            <FieldRow id="config-agent-daily-limit" label={t('admin.config.dailyLimit')} value={config.agent.rate_limit_per_day} onChange={(v) => updateAgent("rate_limit_per_day", v)} min={1} unit={t('common.units.times')} />
          </div>
        </div>

        {/* Social */}
        <div className="rounded-md border border-border bg-card ">
          <div className="border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">{t('admin.config.sectionSocial')}</h3>
          </div>
          <div className="px-4 py-1">
            <FieldRow id="config-report-auto-hide-rate" label={t('admin.config.reportAutoHideRate')} value={config.social.report_auto_hide_rate} onChange={(v) => updateSocial("report_auto_hide_rate", v)} min={0} max={1} step={0.01} />
            <FieldRow id="config-comment-fold-threshold" label={t('admin.config.commentFoldThreshold')} value={config.social.comment_fold_threshold} onChange={(v) => updateSocial("comment_fold_threshold", v)} min={0} max={1} step={0.01} />
          </div>
        </div>
      </div>

      <ConfirmModal
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('admin.config.saveConfirmTitle')}
        description={t('admin.config.saveConfirmMsg')}
        confirmLabel={t('admin.config.confirmSave')}
        confirmVariant="default"
        onConfirm={async () => {
          if (pendingPatch) {
            await saveConfig(pendingPatch);
          }
        }}
      />
    </div>
  );
}
