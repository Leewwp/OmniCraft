"use client";
/**
 * 【原型专用，随时可删】发起共治提案表单（浮层）。
 * 规则镜像交接文档 §1.3：简介/封面/标签可提案（一次可含多个字段；标签一次 ± 一个）、
 * 名称/slug/类目锁死、已存在标签不能再提案加入、移除不存在的标签被拒绝、
 * 提交走 AI 文本审核（本地 fail-open → 原型里用延时模拟）。
 */
import { useState } from "react";
import { X } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { t } from "./copy";
import { CoverDiff } from "./bits";
import { PROPOSAL_CONFIG, type DiffSeg, type Profile, type Proposal } from "./mock-data";

const COVER_CHOICES: React.CSSProperties[] = [
  { backgroundImage: "linear-gradient(135deg, #0f766e, #2563eb 60%, #4f46e5)" },
  { backgroundImage: "linear-gradient(135deg, #b45309, #dc2626 55%, #7c3aed)" },
  { backgroundImage: "linear-gradient(135deg, #0369a1, #0891b2 60%, #059669)" },
];

type FieldKey = "intro" | "cover" | "tags";

export function ProposalForm({
  profile,
  onSubmit,
  onClose,
}: {
  profile: Profile;
  onSubmit: (proposal: Proposal) => void;
  onClose: () => void;
}) {
  const [fields, setFields] = useState<Record<FieldKey, boolean>>({ intro: true, cover: false, tags: false });
  const [intro, setIntro] = useState("");
  const [coverIdx, setCoverIdx] = useState(0);
  const [tagMode, setTagMode] = useState<"add" | "remove">("add");
  const [tagValue, setTagValue] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [touched, setTouched] = useState(false);

  const anyField = fields.intro || fields.cover || fields.tags;
  const introEmpty = fields.intro && intro.trim() === "";
  const introSame = fields.intro && intro.trim() !== "" && intro.trim() === profile.intro.trim();
  const tagExists = fields.tags && tagMode === "add" && profile.tags.includes(tagValue.trim());
  const tagMissing = fields.tags && tagMode === "remove" && tagValue.trim() !== "" && !profile.tags.includes(tagValue.trim());
  const valid = anyField && !introEmpty && !introSame && !tagExists && !tagMissing;

  function toggleField(key: FieldKey) {
    setFields((f) => ({ ...f, [key]: !f[key] }));
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    setTouched(true);
    if (!valid || submitting) return;
    setSubmitting(true);
    // 模拟 AI 文本审核（本地 fail-open：延时后必过）
    window.setTimeout(() => {
      const diffSegments = buildIntroDiff(profile.intro, intro);
      const proposal: Proposal = {
        id: `new-${Date.now()}`,
        proposer: "我（当前用户）",
        startedDisplay: "刚刚",
        status: "open",
        ...(fields.intro ? { introDiff: diffSegments } : {}),
        ...(fields.cover
          ? {
              coverDiff: {
                oldStyle: profile.coverStyle,
                newStyle: COVER_CHOICES[coverIdx % COVER_CHOICES.length],
              },
            }
          : {}),
        ...(fields.tags && tagValue.trim() ? { tagChange: { mode: tagMode, tag: tagValue.trim() } } : {}),
        votesFor: 0,
        votesAgainst: 0,
        deadlineDays: PROPOSAL_CONFIG.deadlineDays,
        effectiveIdx: 0,
      };
      onSubmit(proposal);
    }, 900);
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" role="dialog" aria-modal="true" aria-label={t("proposal.form.title")}>
      <div className="absolute inset-0 bg-black/50" onClick={onClose} aria-hidden="true" />
      <form
        onSubmit={submit}
        className="relative max-h-[85vh] w-full max-w-xl overflow-y-auto rounded-lg border border-border bg-canvas-default p-5 shadow-[var(--elevation-3)]"
      >
        <button
          type="button"
          onClick={onClose}
          aria-label={t("common.close")}
          className="absolute right-3 top-3 inline-flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors duration-150 hover:bg-muted hover:text-foreground"
        >
          <X className="size-4" aria-hidden="true" />
        </button>

        <h2 className="pr-8 text-xl font-semibold">{t("proposal.form.title")}</h2>
        <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
          {t("proposal.form.configNote", {
            config: t("proposal.deadlineConfig", {
              minVotes: PROPOSAL_CONFIG.minVotes,
              threshold: Math.round(PROPOSAL_CONFIG.passThreshold * 100),
              days: PROPOSAL_CONFIG.deadlineDays,
            }),
          })}
        </p>

        {/* 字段多选 */}
        <fieldset className="mt-4">
          <legend className="text-sm font-medium">{t("proposal.form.pickFields")}</legend>
          <div className="mt-2 flex gap-2">
            {(["intro", "cover", "tags"] as FieldKey[]).map((key) => (
              <button
                key={key}
                type="button"
                aria-pressed={fields[key]}
                onClick={() => toggleField(key)}
                className={cn(
                  "inline-flex min-h-9 items-center gap-1 rounded-full border px-3 text-xs font-medium transition-colors duration-150",
                  fields[key]
                    ? "border-accent-emphasis bg-accent-subtle font-semibold text-accent-emphasis"
                    : "border-border text-muted-foreground hover:bg-muted hover:text-foreground",
                )}
              >
                {t(`proposal.field.${key}`)}
              </button>
            ))}
          </div>
          {touched && !anyField && <p className="mt-1.5 text-xs text-destructive">{t("proposal.form.needField")}</p>}
        </fieldset>

        {/* 简介 */}
        {fields.intro && (
          <div className="mt-4">
            <label htmlFor="proto-intro" className="text-sm font-medium">
              {t("proposal.form.introLabel")}
            </label>
            <Textarea
              id="proto-intro"
              value={intro}
              onChange={(e) => setIntro(e.target.value)}
              placeholder={t("proposal.form.introPlaceholder")}
              rows={4}
              className="mt-1.5"
              aria-invalid={touched && (introEmpty || introSame)}
            />
            {(introEmpty || introSame) && (
              <p className="mt-1 text-xs text-destructive">
                {introSame ? t("proposal.form.introSame") : t("proposal.form.introRequired")}
              </p>
            )}
          </div>
        )}

        {/* 封面 */}
        {fields.cover && (
          <div className="mt-4">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">{t("proposal.form.coverLabel")}</span>
              <Button type="button" size="xs" variant="outline" onClick={() => setCoverIdx((i) => i + 1)}>
                {t("proposal.form.coverPick")}
              </Button>
            </div>
            <div className="mt-2">
              <CoverDiff oldStyle={profile.coverStyle} newStyle={COVER_CHOICES[coverIdx % COVER_CHOICES.length]} />
            </div>
          </div>
        )}

        {/* 标签 */}
        {fields.tags && (
          <div className="mt-4">
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium">{t("proposal.field.tags")}</span>
              <div className="flex gap-1">
                {(["add", "remove"] as const).map((mode) => (
                  <button
                    key={mode}
                    type="button"
                    aria-pressed={tagMode === mode}
                    onClick={() => {
                      setTagMode(mode);
                      setTagValue("");
                    }}
                    className={cn(
                      "inline-flex min-h-7 items-center rounded-full border px-2.5 text-xs transition-colors duration-150",
                      tagMode === mode
                        ? "border-accent-emphasis bg-accent-subtle font-semibold text-accent-emphasis"
                        : "border-border text-muted-foreground hover:bg-muted hover:text-foreground",
                    )}
                  >
                    {mode === "add" ? t("proposal.form.tagModeAdd") : t("proposal.form.tagModeRemove")}
                  </button>
                ))}
              </div>
            </div>
            <Input
              value={tagValue}
              onChange={(e) => setTagValue(e.target.value)}
              placeholder={t("proposal.form.tagPlaceholder")}
              aria-label={t("proposal.form.tagLabel")}
              className="mt-2 max-w-56"
              aria-invalid={touched && (tagExists || tagMissing)}
            />
            {tagExists && <p className="mt-1 text-xs text-destructive">{t("proposal.form.tagExists")}</p>}
            {tagMissing && <p className="mt-1 text-xs text-destructive">{t("proposal.form.tagMissing")}</p>}
            {profile.tags.length > 0 && (
              <div className="mt-2">
                <p className="text-xs text-muted-foreground">{t("proposal.form.currentTags")}</p>
                <div className="mt-1.5 flex flex-wrap gap-1.5">
                  {profile.tags.map((tag) => (
                    <button
                      key={tag}
                      type="button"
                      onClick={() => setTagValue(tag)}
                      className="inline-flex h-5 items-center rounded-full bg-canvas-subtle px-2 text-xs text-muted-foreground transition-colors duration-150 hover:bg-muted hover:text-foreground"
                    >
                      {tag}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {/* 提交行 */}
        <div className="mt-5 flex items-center justify-end gap-2 border-t border-border pt-4">
          <Button type="button" variant="outline" onClick={onClose} disabled={submitting}>
            {t("proposal.form.cancel")}
          </Button>
          <Button type="submit" disabled={submitting} className="gap-1.5">
            {submitting && (
              <span
                className="size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent motion-reduce:animate-none"
                aria-hidden="true"
              />
            )}
            {submitting ? t("proposal.form.submitting") : t("proposal.form.submit")}
          </Button>
        </div>
      </form>
    </div>
  );
}

/** 提交的整段新简介 → del/ins 全量 diff 片段（原型级粗粒度 diff）。 */
function buildIntroDiff(current: string, next: string): DiffSeg[] {
  return [
    { kind: "del", text: current },
    { kind: "ins", text: next },
  ];
}
