"use client";

import { useState, useRef } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { ArrowLeft, Send, ChevronDown, Image, Eye, EyeOff, MessageCircle, Undo2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { FileUploader } from "@/components/content/FileUploader";
import { MarkdownEditor } from "@/components/content/MarkdownEditor";
import { TagBadge } from "@/components/ui/TagBadge";
import { cn } from "@/lib/utils";
import { AgentFeatureGate } from "@/components/agent/AgentFeatureGate";
import { AgentUploadAssistPanel } from "@/components/agent/UploadAssistPanel";
import { AgentComplianceCheckBadge } from "@/components/agent/ComplianceCheckBadge";
import type { UploadedAsset } from "@/components/content/FileUploader";

const ORIGINAL_CATEGORIES = [
  "film_tv", "gaming", "literature", "pet", "food",
  "beauty_fashion", "home", "tech_digital", "travel", "sports", "productivity",
];

const CATEGORY_I18N: Record<string, string> = {
  film_tv: "home.categoryFilmTv", gaming: "home.categoryGaming", literature: "home.categoryLiterature", pet: "home.categoryPet",
  food: "home.categoryFood", beauty_fashion: "home.categoryBeautyFashion", home: "home.categoryHome",
  tech_digital: "home.categoryTechDigital", travel: "home.categoryTravel", sports: "home.categorySports", productivity: "home.categoryProductivity",
};

// Types that use file upload as primary content
const FILE_PRIMARY_TYPES = ["image", "video", "audio", "sheet_music", "mod", "template"];
// Types that use text editor as primary
const TEXT_PRIMARY_TYPES = ["article", "prompt", "other"];

interface PublishFormProps {
  zone: "original" | "fanwork";
  contentType: string;
  onBack: () => void;
}

export function PublishForm({ zone, contentType, onBack }: PublishFormProps) {
  const t = useTranslations();
  const router = useRouter();
  const { toast } = useToast();
  const isFilePrimary = FILE_PRIMARY_TYPES.includes(contentType);

  // Core fields
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [briefDesc, setBriefDesc] = useState("");

  // Zone-specific
  const [category, setCategory] = useState("");
  const [ipSearch, setIpSearch] = useState("");
  const [sourceSearch, setSourceSearch] = useState("");

  // Settings
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");
  const [hasCustomCover, setHasCustomCover] = useState(false);
  const [allowComments, setAllowComments] = useState(true);
  const [allowCopy, setAllowCopy] = useState(true);
  const [isPublic, setIsPublic] = useState(true);
  const [agentEnabled, setAgentEnabled] = useState(false);

  const [submitting, setSubmitting] = useState(false);
  const [uploadedFiles, setUploadedFiles] = useState<UploadedAsset[]>([]);
  const [coverFile, setCoverFile] = useState<UploadedAsset | null>(null);
  const [uploadError, setUploadError] = useState("");
  const [complianceViolation, setComplianceViolation] = useState(false);

  const undoSnapshot = useRef<{ title: string; briefDesc: string; body: string; tags: string[]; category: string } | null>(null);
  const [canUndo, setCanUndo] = useState(false);

  function addTag() {
    const val = tagInput.trim();
    if (val && !tags.includes(val) && tags.length < 10) {
      setTags([...tags, val]);
    }
    setTagInput("");
  }

  const description = isFilePrimary ? briefDesc : body;

  function handleAssistFill(data: { suggested_title?: string; suggested_description?: string; suggested_tags?: string[]; suggested_category?: string }) {
    undoSnapshot.current = { title, briefDesc, body, tags, category };
    setCanUndo(true);
    if (data.suggested_title) setTitle(data.suggested_title);
    if (data.suggested_description) {
      if (isFilePrimary) setBriefDesc(data.suggested_description);
      else setBody(data.suggested_description);
    }
    if (data.suggested_tags && data.suggested_tags.length > 0) {
      setTags((prev) => {
        const merged = [...prev];
        for (const tag of data.suggested_tags!) {
          if (!merged.includes(tag) && merged.length < 10) merged.push(tag);
        }
        return merged;
      });
    }
    if (data.suggested_category && zone === "original" && ORIGINAL_CATEGORIES.includes(data.suggested_category)) {
      setCategory(data.suggested_category);
    }
  }

  function handleUndo() {
    if (!undoSnapshot.current) return;
    const snap = undoSnapshot.current;
    setTitle(snap.title);
    setBriefDesc(snap.briefDesc);
    setBody(snap.body);
    setTags(snap.tags);
    setCategory(snap.category);
    undoSnapshot.current = null;
    setCanUndo(false);
  }

  function handleComplianceResult(result: { risk_level: "safe" | "warning" | "violation" }) {
    setComplianceViolation(result.risk_level === "violation");
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) { toast("error", t('studio.publish.titleRequired')); return; }
    if (zone === "original" && !category) { toast("error", t('studio.publish.categoryRequired')); return; }
    if (zone === "fanwork" && !ipSearch.trim()) { toast("error", t('studio.publish.ipRequired')); return; }

    setSubmitting(true);
    try {
      const payload: Record<string, unknown> = {
        title: title.trim(), zone, content_type: contentType,
        body: description, tags, is_public: isPublic,
        allow_copy: allowCopy, agent_enabled: agentEnabled,
        allow_comments: allowComments,
      };
      if (zone === "original") payload.category = category;
      if (uploadedFiles.length > 0) {
        payload.attachments = uploadedFiles.map((f) => ({
          oss_key: f.ossKey,
          file_name: f.fileName,
          file_type: f.fileType,
          mime_type: f.mimeType,
          file_size: f.fileSize,
        }));
      }
      if (coverFile) {
        payload.cover_oss_key = coverFile.ossKey;
      }
      if (zone === "fanwork" && ipSearch.trim()) {
        payload.ip_name = ipSearch.trim();
      }
      await api.post("/api/v1/contents", payload);
      toast("success", t('studio.publish.success'));
      router.push("/studio/contents");
    } catch {
      toast("error", t('studio.publish.failed'));
    } finally {
      setSubmitting(false);
    }
  }

  const fileType = (
    ["image", "video", "sheet_music", "mod"].includes(contentType)
      ? contentType
      : "text"
  ) as "image" | "video" | "text" | "mod" | "sheet_music";

  const maxMB = contentType === "mod" ? 500 : contentType === "sheet_music" ? 50 : contentType === "video" ? 300 : 20;

  return (
    <form onSubmit={handleSubmit} className="max-w-2xl space-y-6">
      {/* Back */}
      <button type="button" onClick={onBack}
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors">
        <ArrowLeft className="h-4 w-4" /> {t('studio.publish.backToTypes')}
      </button>

      {/* Title — always first */}
      <div>
        <label className="mb-1.5 block text-sm font-medium text-foreground">
          {t('publish.contentTitle')} <span className="text-destructive">*</span>
        </label>
        <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder={t('publish.titlePlaceholder')} maxLength={200} />
      </div>

      {/* Zone-specific: Original category */}
      {zone === "original" && (
        <div>
          <label className="mb-1.5 block text-sm font-medium text-foreground">
            {t('studio.publish.categoryLabel')} <span className="text-destructive">*</span>
          </label>
          <div className="flex flex-wrap gap-2">
            {ORIGINAL_CATEGORIES.map((cat) => (
              <button key={cat} type="button" onClick={() => setCategory(cat)}
                className={cn(
                  "rounded-full border px-3.5 py-1.5 text-xs font-medium transition-all",
                  category === cat
                    ? "border-[var(--accent-emphasis)] bg-[var(--accent-subtle)] text-[var(--accent-emphasis)]"
                    : "border-border text-muted-foreground hover:border-border/80 hover:text-foreground"
                )}>
                {t(CATEGORY_I18N[cat])}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Zone-specific: Fanwork IP + Source */}
      {zone === "fanwork" && (
        <div className="space-y-3">
          <div>
            <label className="mb-1.5 block text-sm font-medium text-foreground">
              {t('studio.publish.ipLabel')} <span className="text-destructive">*</span>
            </label>
            <Input value={ipSearch} onChange={(e) => setIpSearch(e.target.value)} placeholder={t('studio.publish.ipSearchPlaceholder')} />
          </div>
          <div>
            <label className="mb-1.5 block text-sm font-medium text-foreground">{t('studio.publish.sourceOriginalOptional')}</label>
            <Input value={sourceSearch} onChange={(e) => setSourceSearch(e.target.value)} placeholder={t('studio.publish.searchOriginalPlaceholder')} />
          </div>
        </div>
      )}

      {/* Primary content area: Text types → Markdown */}
      {TEXT_PRIMARY_TYPES.includes(contentType) && (
        <div>
          <label className="mb-1.5 block text-sm font-medium text-foreground">{t('studio.publish.bodyLabel')}</label>
          <MarkdownEditor value={body} onChange={(val) => setBody(val)} />
        </div>
      )}

      {/* Primary content area: File types → upload + brief desc */}
      {isFilePrimary && (
        <div className="space-y-4">
          <div>
            <label className="mb-1.5 block text-sm font-medium text-foreground">
              {t(contentType === "image" ? 'studio.publish.uploadLabel.image' : contentType === "video" ? 'studio.publish.uploadLabel.video' : contentType === "audio" ? 'studio.publish.uploadLabel.audio' : contentType === "sheet_music" ? 'studio.publish.uploadLabel.sheet_music' : contentType === "mod" ? 'studio.publish.uploadLabel.mod' : 'studio.publish.uploadLabel.default')}
              <span className="text-destructive"> *</span>
            </label>
            <FileUploader fileType={fileType} maxMB={maxMB} accept="*" onUploaded={(files) => { setUploadedFiles(files); setUploadError(""); }} />
            <p className="mt-1 text-xs text-muted-foreground">
              {contentType === "sheet_music" ? t('studio.publish.uploadHint.sheet_music') :
               contentType === "mod" ? t('studio.publish.uploadHint.mod') :
               contentType === "video" ? t('studio.publish.uploadHint.video') :
               contentType === "image" ? t('studio.publish.uploadHint.image') : ""}
            </p>
          </div>
          <div>
            <label className="mb-1.5 block text-sm font-medium text-foreground">{t('studio.publish.descriptionOptional')}</label>
            <textarea
              value={briefDesc}
              onChange={(e) => setBriefDesc(e.target.value)}
              rows={3}
              placeholder={t('studio.publish.bodyPlaceholder')}
              className="w-full rounded-lg border border-border bg-card px-3 py-2 text-sm placeholder:text-muted-foreground/60 focus:outline-none focus:ring-2 focus:ring-ring/20 resize-none"
            />
          </div>
        </div>
      )}

      {/* AI Upload Assist */}
      {isFilePrimary && (
        <AgentFeatureGate capability="webAgent">
          <AgentUploadAssistPanel
            uploadedFiles={uploadedFiles.map((f) => f.fileName)}
            title={title}
            description={description}
            contentType={contentType}
            onFill={handleAssistFill}
          />
          {canUndo && (
            <Button type="button" variant="ghost" size="sm" className="mt-2 text-xs" onClick={handleUndo}>
              <Undo2 className="mr-1 h-3 w-3" />
              {t("agent.undoSuggestion")}
            </Button>
          )}
        </AgentFeatureGate>
      )}

      {/* ──── Publishing Settings Panel ──── */}
      <div className="rounded-xl border border-border/60 bg-card">
        <div className="flex items-center gap-2 px-5 py-3 border-b border-border/40">
          <ChevronDown className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm font-semibold text-foreground">{t('studio.publish.publishSettings')}</span>
        </div>
        <div className="p-5 space-y-4">

          {/* Cover toggle */}
          <div className="flex items-center justify-between">
            <div>
              <span className="text-sm font-medium text-foreground">{t('studio.publish.customCover')}</span>
              <p className="text-xs text-muted-foreground">{t('studio.publish.coverDescription')}</p>
            </div>
            <button type="button" role="switch" aria-checked={hasCustomCover}
              onClick={() => setHasCustomCover(!hasCustomCover)}
              className={cn(
                "relative inline-flex h-6 w-11 items-center rounded-full transition-colors",
                hasCustomCover ? "bg-[var(--accent-emphasis)]" : "bg-muted-foreground/25"
              )}>
              <span className={cn("inline-block h-4 w-4 rounded-full bg-white transition-transform", hasCustomCover ? "translate-x-6" : "translate-x-1")} />
            </button>
          </div>

          {/* Custom cover upload (conditional) */}
          {hasCustomCover && (
            <div className="pl-2 border-l-2 border-[var(--accent-subtle)]">
              <FileUploader fileType="image" maxMB={20} accept="image/*" onUploaded={(files) => { if (files.length > 0) setCoverFile(files[0]); }} />
            </div>
          )}

          {/* Comment toggle */}
          <div className="flex items-center justify-between border-t border-border/40 pt-4">
            <div className="flex items-center gap-2">
              <MessageCircle className="h-4 w-4 text-muted-foreground" />
              <span className="text-sm font-medium text-foreground">{t('studio.publish.allowComment')}</span>
            </div>
            <button type="button" role="switch" aria-checked={allowComments}
              onClick={() => setAllowComments(!allowComments)}
              className={cn(
                "relative inline-flex h-6 w-11 items-center rounded-full transition-colors",
                allowComments ? "bg-[var(--accent-emphasis)]" : "bg-muted-foreground/25"
              )}>
              <span className={cn("inline-block h-4 w-4 rounded-full bg-white transition-transform", allowComments ? "translate-x-6" : "translate-x-1")} />
            </button>
          </div>

          {/* Tags */}
          <div className="border-t border-border/40 pt-4">
            <label className="mb-1.5 block text-sm font-medium text-foreground">{t('studio.publish.tagsLabel')}</label>
            <div className="flex gap-2">
              <Input value={tagInput} onChange={(e) => setTagInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addTag(); } }}
                placeholder={t('studio.publish.tagPlaceholder')} className="flex-1" />
              <Button type="button" variant="outline" size="sm" onClick={addTag}>{t('studio.publish.addTag')}</Button>
            </div>
            {tags.length > 0 && (
              <div className="mt-2 flex flex-wrap gap-1.5">
                {tags.map((tag) => (
                  <TagBadge key={tag} color="blue" onRemove={() => setTags(tags.filter((t) => t !== tag))}>{tag}</TagBadge>
                ))}
              </div>
            )}
          </div>

          {/* Allow copy (fanwork only) */}
          {zone === "fanwork" && (
            <div className="flex items-center justify-between border-t border-border/40 pt-4">
              <div>
                <span className="text-sm font-medium text-foreground">{t('studio.publish.allowCopyPR')}</span>
                <p className="text-xs text-muted-foreground">{t('studio.publish.allowCopyPRDesc')}</p>
              </div>
              <button type="button" role="switch" aria-checked={allowCopy}
                onClick={() => setAllowCopy(!allowCopy)}
                className={cn(
                  "relative inline-flex h-6 w-11 items-center rounded-full transition-colors",
                  allowCopy ? "bg-[var(--accent-emphasis)]" : "bg-muted-foreground/25"
                )}>
                <span className={cn("inline-block h-4 w-4 rounded-full bg-white transition-transform", allowCopy ? "translate-x-6" : "translate-x-1")} />
              </button>
            </div>
          )}

          {/* Public toggle */}
          <div className="flex items-center justify-between border-t border-border/40 pt-4">
            <div className="flex items-center gap-2">
              {isPublic ? <Eye className="h-4 w-4 text-muted-foreground" /> : <EyeOff className="h-4 w-4 text-muted-foreground" />}
              <span className="text-sm font-medium text-foreground">{t('studio.publish.publicVisible')}</span>
            </div>
            <button type="button" role="switch" aria-checked={isPublic}
              onClick={() => setIsPublic(!isPublic)}
              className={cn(
                "relative inline-flex h-6 w-11 items-center rounded-full transition-colors",
                isPublic ? "bg-[var(--accent-emphasis)]" : "bg-muted-foreground/25"
              )}>
              <span className={cn("inline-block h-4 w-4 rounded-full bg-white transition-transform", isPublic ? "translate-x-6" : "translate-x-1")} />
            </button>
          </div>

          {/* Agent deploy (mod/prompt only) */}
          <AgentFeatureGate capability="desktopDeploy">
          {(contentType === "mod" || contentType === "prompt") && (
            <div className="flex items-center justify-between border-t border-border/40 pt-4">
              <div>
                <span className="text-sm font-medium text-foreground">{t('studio.publish.agentDeploy')}</span>
                <p className="text-xs text-muted-foreground">{t('studio.publish.agentDeployDesc')}</p>
              </div>
              <button type="button" role="switch" aria-checked={agentEnabled}
                onClick={() => setAgentEnabled(!agentEnabled)}
                className={cn(
                  "relative inline-flex h-6 w-11 items-center rounded-full transition-colors",
                  agentEnabled ? "bg-[var(--accent-emphasis)]" : "bg-muted-foreground/25"
                )}>
                <span className={cn("inline-block h-4 w-4 rounded-full bg-white transition-transform", agentEnabled ? "translate-x-6" : "translate-x-1")} />
              </button>
            </div>
          )}
          </AgentFeatureGate>
        </div>
      </div>

      {/* AI Compliance Check */}
      <AgentFeatureGate capability="webAgent">
        <div className="flex items-center gap-3">
          <AgentComplianceCheckBadge
            title={title}
            description={description}
            contentType={contentType}
          />
          {complianceViolation && (
            <p className="text-xs text-destructive">{t("agent.complianceBlockSubmit")}</p>
          )}
        </div>
      </AgentFeatureGate>

      {/* Bottom actions */}
      <div className="flex items-center gap-3 pt-2">
        <Button type="submit" size="lg" disabled={submitting || complianceViolation} className="gap-2 rounded-full px-8">
          <Send className="h-4 w-4" />
          {submitting ? t('studio.publish.submitting') : t('studio.publish.submit')}
        </Button>
        <Button type="button" variant="ghost" onClick={onBack}>{t('studio.publish.cancel')}</Button>
      </div>
    </form>
  );
}
