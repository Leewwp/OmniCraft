"use client";

import { useEffect, useState } from "react";
import { api, ApiRequestError } from "@/lib/api";
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

const defaultConfig: ConfigData = {
  limits: { video_max_mb: 300, video_max_sec: 180, image_max_mb: 20, text_max_mb: 10, mod_max_mb: 500, sheet_music_max_mb: 50 },
  features: { payment_enabled: false, creator_support_enabled: false },
  reputation: { quality_content_threshold: 10, quality_comment_threshold: 5, repeat_violation_window_days: 7, repeat_violation_threshold: 2, repeat_violation_extra_penalty: 1 },
  agent: { web_agent_enabled: false, rate_limit_per_day: 50 },
  social: { report_auto_hide_rate: 0.1, comment_fold_threshold: 0.3 },
  judge: { min_votes_required: 20, pass_threshold: 0.8, exam_pass_rate: 0.8, error_rate_revoke: 0.5, error_rate_window: 10 },
};

export default function AdminConfigPage() {
  const [config, setConfig] = useState<ConfigData>(defaultConfig);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [pendingPatch, setPendingPatch] = useState<Record<string, unknown> | null>(null);

  useEffect(() => {
    (async () => {
      setLoading(true);
      try {
        const data = await api.get<{ config: ConfigData }>("/api/v1/admin/config");
        if (data.config) {
          setConfig({
            limits: { ...defaultConfig.limits, ...data.config.limits },
            features: { ...defaultConfig.features, ...data.config.features },
            reputation: { ...defaultConfig.reputation, ...data.config.reputation },
            agent: { ...defaultConfig.agent, ...data.config.agent },
            social: { ...defaultConfig.social, ...data.config.social },
            judge: { ...defaultConfig.judge, ...data.config.judge },
          });
        }
      } catch (e) {
        setError(e instanceof ApiRequestError ? e.message : "加载配置失败");
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  async function saveConfig(patch: Record<string, unknown>) {
    setSaving(true);
    setError("");
    setSaved(false);
    try {
      const data = await api.patch<{ config: ConfigData }>("/api/v1/admin/config", patch);
      if (data.config) {
        setConfig((prev) => ({
          limits: { ...prev.limits, ...data.config!.limits },
          features: { ...prev.features, ...data.config!.features },
          reputation: { ...prev.reputation, ...data.config!.reputation },
          agent: { ...prev.agent, ...data.config!.agent },
          social: { ...prev.social, ...data.config!.social },
          judge: { ...prev.judge, ...data.config!.judge },
        }));
      }
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "保存配置失败");
    } finally {
      setSaving(false);
    }
  }

  function updateLimits(key: string, value: number) {
    setConfig((prev) => ({
      ...prev,
      limits: { ...prev.limits, [key]: value },
    }));
  }

  function updateReputation(key: string, value: number) {
    setConfig((prev) => ({
      ...prev,
      reputation: { ...prev.reputation, [key]: value },
    }));
  }

  function updateJudge(key: string, value: number) {
    setConfig((prev) => ({
      ...prev,
      judge: { ...prev.judge, [key]: value },
    }));
  }

  function updateAgent(key: string, value: number | boolean) {
    setConfig((prev) => ({
      ...prev,
      agent: { ...prev.agent, [key]: value },
    }));
  }

  function updateSocial(key: string, value: number) {
    setConfig((prev) => ({
      ...prev,
      social: { ...prev.social, [key]: value },
    }));
  }

  function updateFeatures(key: string, value: boolean) {
    setConfig((prev) => ({
      ...prev,
      features: { ...prev.features, [key]: value },
    }));
  }

  function buildPatch(): Record<string, unknown> {
    return {
      limits: { ...config.limits },
      features: { ...config.features },
      reputation: { ...config.reputation },
      agent: { ...config.agent },
      social: { ...config.social },
      judge: { ...config.judge },
    };
  }

  if (loading) {
    return (
      <div className="space-y-4 p-6">
        <div className="space-y-3 rounded-md border border-border bg-card p-6 shadow-none">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="h-6 w-full animate-pulse rounded bg-muted" />
          ))}
        </div>
      </div>
    );
  }

  function FieldRow({ label, value, onChange, min, max, step = 1, unit }: {
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
        <label className="text-sm font-medium">{label}</label>
        <div className="flex items-center gap-2">
          <input
            type="number"
            className="w-24 rounded-md border border-border bg-background px-2 py-1.5 text-sm text-right focus:outline-none focus:ring-2 focus:ring-ring"
            value={value}
            min={min}
            max={max}
            step={step}
            onChange={(e) => onChange(Number(e.target.value))}
          />
          {unit && <span className="text-xs text-muted-foreground w-8">{unit}</span>}
        </div>
      </div>
    );
  }

  function ToggleRow({ label, value, onChange }: {
    label: string;
    value: boolean;
    onChange: (v: boolean) => void;
  }) {
    return (
      <div className="flex items-center justify-between gap-4 border-b border-border py-3 last:border-b-0">
        <label className="text-sm font-medium">{label}</label>
        <button
          type="button"
          className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
            value ? "bg-accent" : "bg-muted"
          }`}
          onClick={() => onChange(!value)}
        >
          <span
            className={`inline-block h-4 w-4 rounded-full bg-white transition-transform ${
              value ? "translate-x-6" : "translate-x-1"
            }`}
          />
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4 shadow-none">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">系统配置</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            热更新系统配置项（重启后恢复 config.yaml 默认值）
          </p>
        </div>
        <Button
          size="sm"
          disabled={saving}
          onClick={() => {
            setPendingPatch(buildPatch());
            setConfirmOpen(true);
          }}
        >
          {saving ? "保存中..." : "保存配置"}
        </Button>
      </div>

      {saved && (
        <div className="rounded-md border border-emerald-500/30 bg-emerald-50 p-3 text-sm text-emerald-700 shadow-none">
          配置已保存（热更新生效）
        </div>
      )}
      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Limits */}
        <div className="rounded-md border border-border bg-card shadow-none">
          <div className="border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">上传限制 (Limits)</h3>
          </div>
          <div className="px-4 py-1">
            <FieldRow label="视频最大 (MB)" value={config.limits.video_max_mb} onChange={(v) => updateLimits("video_max_mb", v)} min={1} unit="MB" />
            <FieldRow label="视频最大时长 (秒)" value={config.limits.video_max_sec} onChange={(v) => updateLimits("video_max_sec", v)} min={1} unit="s" />
            <FieldRow label="图片最大 (MB)" value={config.limits.image_max_mb} onChange={(v) => updateLimits("image_max_mb", v)} min={1} unit="MB" />
            <FieldRow label="文本最大 (MB)" value={config.limits.text_max_mb} onChange={(v) => updateLimits("text_max_mb", v)} min={1} unit="MB" />
            <FieldRow label="Mod 最大 (MB)" value={config.limits.mod_max_mb} onChange={(v) => updateLimits("mod_max_mb", v)} min={1} unit="MB" />
            <FieldRow label="乐谱最大 (MB)" value={config.limits.sheet_music_max_mb} onChange={(v) => updateLimits("sheet_music_max_mb", v)} min={1} unit="MB" />
          </div>
        </div>

        {/* Features */}
        <div className="rounded-md border border-border bg-card shadow-none">
          <div className="border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">功能开关 (Features)</h3>
          </div>
          <div className="px-4 py-1">
            <ToggleRow label="支付模块" value={config.features.payment_enabled} onChange={(v) => updateFeatures("payment_enabled", v)} />
            <ToggleRow label="创作者支持" value={config.features.creator_support_enabled} onChange={(v) => updateFeatures("creator_support_enabled", v)} />
          </div>
        </div>

        {/* Reputation */}
        <div className="rounded-md border border-border bg-card shadow-none">
          <div className="border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">信誉分 (Reputation)</h3>
          </div>
          <div className="px-4 py-1">
            <FieldRow label="优质内容获赞阈值" value={config.reputation.quality_content_threshold} onChange={(v) => updateReputation("quality_content_threshold", v)} min={1} />
            <FieldRow label="优质评论获赞阈值" value={config.reputation.quality_comment_threshold} onChange={(v) => updateReputation("quality_comment_threshold", v)} min={1} />
            <FieldRow label="重复违规窗口 (天)" value={config.reputation.repeat_violation_window_days} onChange={(v) => updateReputation("repeat_violation_window_days", v)} min={1} unit="天" />
            <FieldRow label="重复违规阈值" value={config.reputation.repeat_violation_threshold} onChange={(v) => updateReputation("repeat_violation_threshold", v)} min={1} unit="次" />
            <FieldRow label="二次违规额外扣分" value={config.reputation.repeat_violation_extra_penalty} onChange={(v) => updateReputation("repeat_violation_extra_penalty", v)} min={0} />
          </div>
        </div>

        {/* Judge */}
        <div className="rounded-md border border-border bg-card shadow-none">
          <div className="border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">赛博判官 (Judge)</h3>
          </div>
          <div className="px-4 py-1">
            <FieldRow label="最少投票数" value={config.judge.min_votes_required} onChange={(v) => updateJudge("min_votes_required", v)} min={1} unit="票" />
            <FieldRow label="考核通过率" value={config.judge.exam_pass_rate} onChange={(v) => updateJudge("exam_pass_rate", v)} min={0} max={1} step={0.05} />
            <FieldRow label="判决通过阈值" value={config.judge.pass_threshold} onChange={(v) => updateJudge("pass_threshold", v)} min={0} max={1} step={0.05} />
            <FieldRow label="撤权错误率" value={config.judge.error_rate_revoke} onChange={(v) => updateJudge("error_rate_revoke", v)} min={0} max={1} step={0.05} />
            <FieldRow label="错误率窗口" value={config.judge.error_rate_window} onChange={(v) => updateJudge("error_rate_window", v)} min={1} unit="次" />
          </div>
        </div>

        {/* Agent */}
        <div className="rounded-md border border-border bg-card shadow-none">
          <div className="border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">智能体 (Agent)</h3>
          </div>
          <div className="px-4 py-1">
            <ToggleRow label="Web Agent" value={config.agent.web_agent_enabled} onChange={(v) => updateAgent("web_agent_enabled", v)} />
            <FieldRow label="每日限流" value={config.agent.rate_limit_per_day} onChange={(v) => updateAgent("rate_limit_per_day", v)} min={1} unit="次" />
          </div>
        </div>

        {/* Social */}
        <div className="rounded-md border border-border bg-card shadow-none">
          <div className="border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">社交风控 (Social)</h3>
          </div>
          <div className="px-4 py-1">
            <FieldRow label="举报自动隐藏率" value={config.social.report_auto_hide_rate} onChange={(v) => updateSocial("report_auto_hide_rate", v)} min={0} max={1} step={0.01} />
            <FieldRow label="评论折叠阈值" value={config.social.comment_fold_threshold} onChange={(v) => updateSocial("comment_fold_threshold", v)} min={0} max={1} step={0.01} />
          </div>
        </div>
      </div>

      <ConfirmModal
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title="保存系统配置"
        description="确认保存当前配置吗？修改将立即热更新生效（重启后恢复 config.yaml 默认值）。"
        confirmLabel="确认保存"
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
