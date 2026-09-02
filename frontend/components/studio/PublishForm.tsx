"use client";

import { useEffect, useId, useState, useRef } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { ArrowLeft, Send, ChevronDown, Eye, EyeOff, MessageCircle, Undo2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { api } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { FileUploader, toUploadedAsset, type UploadItem } from "@/components/content/FileUploader";
import { MarkdownEditor } from "@/components/content/MarkdownEditor";
import { TagBadge } from "@/components/ui/TagBadge";
import { cn } from "@/lib/utils";
import { AgentFeatureGate } from "@/components/agent/AgentFeatureGate";
import { AgentUploadAssistPanel } from "@/components/agent/UploadAssistPanel";
import { AgentComplianceCheckBadge } from "@/components/agent/ComplianceCheckBadge";
import { IPPicker } from "@/components/studio/IPPicker";
import { SourceContentPicker, type SourceContent } from "@/components/studio/SourceContentPicker";
import { CollabUserPicker, type CollabUser } from "@/components/content/CollabUserPicker";
import { Skeleton } from "@/components/ui/skeleton";
import { normalizeContentDetailResponse } from "@/lib/content";
import type { UploadedAsset } from "@/components/content/FileUploader";
import { fetchPublicConfig } from "@/lib/public-config";
import { silentError } from "@/lib/error-handler";

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
const MAX_SUGGESTED_TITLE_LENGTH = 500;
const MAX_SUGGESTED_DESCRIPTION_LENGTH = 2000;

// Collaboration invites (collab plan Task 7): each invite request gets a
// 5-second client-side timeout and at most three run concurrently. There is
// no auto-retry; failed invites surface as a warning toast after publish.
const INVITE_TIMEOUT_MS = 5000;
const INVITE_CONCURRENCY = 3;

interface GalleryLimit {
  min: number;
  max: number;
}

type GalleryLimits = Partial<Record<"image" | "video", GalleryLimit>>;

interface PublishFormProps {
  zone: "original" | "fanwork";
  contentType: string;
  onBack: () => void;
  /** Query-prefill source ids (fanwork only); already resolved to at most one. */
  prefillSourceOriginalId?: number;
  prefillSourceFanworkId?: number;
  /** Localized prefill warnings resolved by the page from the URL query. */
  prefillWarnings?: PrefillWarning[];
}

export type PrefillWarning = "bothSources" | "invalidId";

function FieldError({ children }: { children: React.ReactNode }) {
  return <p className="mt-1 text-xs text-destructive">{children}</p>;
}

function withTimeout<T>(promise: Promise<T>, ms: number): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error("request timed out")), ms);
    promise.then(
      (value) => {
        clearTimeout(timer);
        resolve(value);
      },
      (error) => {
        clearTimeout(timer);
        reject(error);
      },
    );
  });
}

/** Sends one collab invite per selected user, at most three concurrently.
 *  Never throws: failures are returned so the caller can surface them
 *  without failing the already-completed content publish. */
async function sendCollabInvites(contentId: number, users: CollabUser[]): Promise<CollabUser[]> {
  const failed: CollabUser[] = [];
  for (let offset = 0; offset < users.length; offset += INVITE_CONCURRENCY) {
    const batch = users.slice(offset, offset + INVITE_CONCURRENCY);
    await Promise.all(
      batch.map(async (user) => {
        try {
          await withTimeout(
            api.post(`/api/v1/contents/${contentId}/collab-invites`, { invitee_id: user.id }),
            INVITE_TIMEOUT_MS,
          );
        } catch {
          failed.push(user);
        }
      }),
    );
  }
  return failed;
}

export function PublishForm({ zone, contentType, onBack, prefillSourceOriginalId, prefillSourceFanworkId, prefillWarnings = [] }: PublishFormProps) {
  const t = useTranslations();
  const router = useRouter();
  const { toast } = useToast();
  const isFilePrimary = FILE_PRIMARY_TYPES.includes(contentType);
  const mediaContentType = contentType === "image" || contentType === "video" ? contentType : null;
  const isMediaGallery = mediaContentType !== null;

  useEffect(() => {
    if (!mediaContentType) {
      setGalleryConfigLoading(false);
      return;
    }

    let active = true;
    setGalleryConfigLoading(true);
    void fetchPublicConfig()
      .then((config) => {
        if (!active || !config.upload) return;
        const uploadLimits = config.upload;
        const nextLimits = mediaContentType === "image"
          ? {
              min: uploadLimits.image_gallery_min_items,
              max: uploadLimits.image_gallery_max_items,
            }
          : {
              min: uploadLimits.video_gallery_min_items,
              max: uploadLimits.video_gallery_max_items,
            };
        if (nextLimits.min > 0 && nextLimits.max >= nextLimits.min) {
          setGalleryLimits((current) => ({ ...current, [mediaContentType]: nextLimits }));
        }
      })
      .catch((error) => {
        silentError(error, { component: "PublishForm", action: "fetchGalleryLimits" });
      })
      .finally(() => {
        if (active) setGalleryConfigLoading(false);
      });

    return () => {
      active = false;
    };
  }, [mediaContentType]);

  // Collaborator selection cap (collab plan Task 7): public config value
  // feeds CollabUserPicker as maxSelected. While unavailable the picker
  // stays closed and publishing still works without invitations.
  useEffect(() => {
    let active = true;
    void fetchPublicConfig()
      .then((config) => {
        if (active) {
          setCollabMaxSelected(config.collaboration?.max_invitees_per_publish ?? 0);
        }
      })
      .catch((error) => {
        silentError(error, { component: "PublishForm", action: "fetchCollabLimits" });
      });
    return () => {
      active = false;
    };
  }, []);

  // Query prefill (fanwork only): load the source summary so the picker can
  // show the selected row without a manual search. At most one source id is
  // ever prefilled; the fanwork page already resolved both-ID precedence.
  useEffect(() => {
    if (zone !== "fanwork") return;
    const prefillId = prefillSourceOriginalId ?? prefillSourceFanworkId;
    if (!prefillId) return;
    const kind = prefillSourceOriginalId ? "original" : "fanwork";

    let active = true;
    setPrefillLoading(true);
    setPrefillFailed(false);
    api
      .get<unknown>(`/api/v1/contents/${prefillId}`)
      .then((data) => {
        if (!active) return;
        const content = normalizeContentDetailResponse(data).content;
        if (
          content &&
          (content.zone === "original" || content.zone === "fanwork") &&
          content.zone === kind &&
          content.status === "published"
        ) {
          const summary: SourceContent = { id: content.id, title: content.title, zone: content.zone };
          if (kind === "original") setSourceOriginal(summary);
          else setSourceFanwork(summary);
        } else {
          setPrefillFailed(true);
        }
      })
      .catch(() => {
        if (active) setPrefillFailed(true);
      })
      .finally(() => {
        if (active) setPrefillLoading(false);
      });
    return () => {
      active = false;
    };
  }, [zone, prefillSourceOriginalId, prefillSourceFanworkId]);

  // Core fields
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [briefDesc, setBriefDesc] = useState("");

  // Zone-specific
  const [category, setCategory] = useState("");
  const [selectedIP, setSelectedIP] = useState<{ id: number; name: string } | null>(null);
  const [sourceOriginal, setSourceOriginal] = useState<SourceContent | null>(null);
  const [sourceFanwork, setSourceFanwork] = useState<SourceContent | null>(null);
  const [prefillLoading, setPrefillLoading] = useState(false);
  const [prefillFailed, setPrefillFailed] = useState(false);
  const [dismissedWarnings, setDismissedWarnings] = useState<Set<PrefillWarning>>(new Set());
  const sourceHintId = useId();

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
  const [mediaItems, setMediaItems] = useState<UploadItem[]>([]);
  const [galleryLimits, setGalleryLimits] = useState<GalleryLimits>({});
  const [galleryConfigLoading, setGalleryConfigLoading] = useState(Boolean(mediaContentType));
  const [coverFile, setCoverFile] = useState<UploadedAsset | null>(null);
  const [uploadError, setUploadError] = useState("");
  const [complianceRisk, setComplianceRisk] = useState<"safe" | "warning" | "violation" | null>(null);
  const [warningAcknowledged, setWarningAcknowledged] = useState(false);
  const [collabUsers, setCollabUsers] = useState<CollabUser[]>([]);
  const [collabMaxSelected, setCollabMaxSelected] = useState(0);

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
  const activeGalleryLimits = mediaContentType ? galleryLimits[mediaContentType] : null;
  const galleryUnavailable = isMediaGallery && !activeGalleryLimits;
  const readyMediaAssets = mediaItems
    .filter((item) => item.status === "done")
    .map(toUploadedAsset)
    .filter((asset): asset is UploadedAsset => asset !== null);
  const complianceViolation = complianceRisk === "violation";
  const warningNeedsAcknowledgement = complianceRisk === "warning" && !warningAcknowledged;
  const sourceMissing = zone === "fanwork" && !selectedIP && !sourceOriginal && !sourceFanwork;

  function handleSourceOriginalSelect(content?: SourceContent) {
    setSourceOriginal(content ?? null);
    if (content) setSourceFanwork(null);
  }

  function handleSourceFanworkSelect(content?: SourceContent) {
    setSourceFanwork(content ?? null);
    if (content) setSourceOriginal(null);
  }

  function handleAssistFill(data: { suggested_title?: string; suggested_description?: string; suggested_tags?: string[]; suggested_category?: string }) {
    if (complianceViolation) {
      toast("error", t("agent.uploadAssistBlockedByViolation"));
      return;
    }
    if (warningNeedsAcknowledgement) {
      toast("error", t("agent.warningAckRequired"));
      return;
    }
    undoSnapshot.current = { title, briefDesc, body, tags, category };
    setCanUndo(true);
    if (data.suggested_title) setTitle(data.suggested_title.slice(0, MAX_SUGGESTED_TITLE_LENGTH));
    if (data.suggested_description) {
      const suggestedDescription = data.suggested_description.slice(0, MAX_SUGGESTED_DESCRIPTION_LENGTH);
      if (isFilePrimary) setBriefDesc(suggestedDescription);
      else setBody(suggestedDescription);
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
    setComplianceRisk(result.risk_level);
    setWarningAcknowledged(false);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) { toast("error", t('studio.publish.titleRequired')); return; }
    if (zone === "original" && !category) { toast("error", t('studio.publish.categoryRequired')); return; }
    if (zone === "fanwork" && sourceMissing) {
      toast("error", t('studio.publish.fanwork.validation.sourceRequired'));
      return;
    }
    if (complianceViolation) { toast("error", t("agent.complianceBlockSubmit")); return; }
    if (galleryUnavailable) {
      toast("error", t("studio.publish.media.limitsUnavailable"));
      return;
    }
    if (isMediaGallery && activeGalleryLimits) {
      if (mediaItems.some((item) => item.status === "pending" || item.status === "uploading")) {
        toast("error", t("studio.publish.media.uploadPending"));
        return;
      }
      if (mediaItems.some((item) => item.status === "error")) {
        toast("error", t("studio.publish.media.uploadFailed"));
        return;
      }
      if (readyMediaAssets.length < activeGalleryLimits.min || readyMediaAssets.length > activeGalleryLimits.max) {
        toast("error", t("studio.publish.media.countError", { min: activeGalleryLimits.min, max: activeGalleryLimits.max }));
        return;
      }
      if (mediaContentType === "video" && !coverFile && !readyMediaAssets.some((asset) => asset.posterGrantId)) {
        toast("error", t("studio.publish.media.posterRequired"));
        return;
      }
    }

    setSubmitting(true);
    try {
      const payload: Record<string, unknown> = {
        title: title.trim(), zone, content_type: contentType,
        body: description, tags, is_public: isPublic,
        allow_copy: allowCopy, agent_enabled: agentEnabled,
        allow_comments: allowComments,
      };
      if (zone === "original") payload.category = category;
      const filesForPayload = isMediaGallery ? readyMediaAssets : uploadedFiles;
      if (filesForPayload.length > 0) {
        payload.attachments = filesForPayload.map((f, index) => ({
          grant_id: f.grantId,
          oss_key: f.ossKey,
          file_name: f.fileName,
          file_type: f.fileType,
          mime_type: f.mimeType,
          file_size: f.fileSize,
          ...(isMediaGallery
            ? {
                width: f.width,
                height: f.height,
                sort_order: f.sortOrder ?? index,
              }
            : {}),
        }));
      }
      if (mediaContentType === "video") {
        const posterGrantID = coverFile?.grantId ?? readyMediaAssets.find((asset) => asset.posterGrantId)?.posterGrantId;
        if (posterGrantID) payload.poster_grant_id = posterGrantID;
      }
      if (zone === "fanwork" && selectedIP) {
        payload.ip_id = selectedIP.id;
      }
      if (zone === "fanwork" && sourceOriginal) {
        payload.source_original_id = sourceOriginal.id;
      }
      if (zone === "fanwork" && sourceFanwork) {
        payload.source_fanwork_id = sourceFanwork.id;
      }
      const created = await api.post<{ content?: { id?: number } }>("/api/v1/contents", payload);
      const contentId = created.content?.id;
      if (contentId && collabUsers.length > 0) {
        const failedInvites = await sendCollabInvites(contentId, collabUsers);
        if (failedInvites.length > 0) {
          toast("warning", t("studio.publish.collab.inviteFailed", {
            usernames: failedInvites.map((user) => user.username).join(", "),
            url: `/content/${contentId}`,
          }));
        }
      }
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
                aria-pressed={category === cat}
                className={cn(
                  "rounded-full border px-3.5 py-1.5 text-xs font-medium transition-colors duration-150",
                  category === cat
                    ? "border-accent-emphasis bg-accent-subtle text-accent-emphasis font-semibold"
                    : "border-border text-muted-foreground hover:border-border/80 hover:text-foreground"
                )}>
                {t(CATEGORY_I18N[cat])}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Zone-specific: Fanwork IP + Sources */}
      {zone === "fanwork" && (
        <fieldset className="space-y-3" aria-describedby={sourceMissing ? sourceHintId : undefined}>
          <legend className="text-sm font-medium text-foreground">{t('studio.publish.fanwork.source.legend')}</legend>
          <div>
            <label className="mb-1.5 block text-sm font-medium text-foreground">
              {t('studio.publish.fanwork.source.ipOptional')}
            </label>
            <IPPicker
              value={selectedIP}
              onChange={setSelectedIP}
              placeholder={t('studio.publish.ipSearchPlaceholder')}
              searchLabel={t('studio.publish.ipLabel')}
              loadingLabel={t('studio.publish.ipSearching')}
            />
          </div>
          {prefillLoading ? (
            <div role="status" aria-live="polite" className="space-y-2">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
            </div>
          ) : (
            <>
              <SourceContentPicker
                sourceKind="original"
                selected={sourceOriginal ?? undefined}
                disabled={submitting}
                onSelect={handleSourceOriginalSelect}
              />
              <SourceContentPicker
                sourceKind="fanwork"
                selected={sourceFanwork ?? undefined}
                disabled={submitting}
                onSelect={handleSourceFanworkSelect}
              />
            </>
          )}
          {prefillWarnings
            .filter((warning) => !dismissedWarnings.has(warning))
            .map((warning) => (
              <div key={warning} role="status" className="flex items-center justify-between gap-2 rounded-md border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
                <span>{t(`studio.publish.fanwork.prefill.${warning}`)}</span>
                <button
                  type="button"
                  onClick={() => setDismissedWarnings((current) => new Set(current).add(warning))}
                  aria-label={t('studio.publish.fanwork.a11y.dismissWarning')}
                  className="inline-flex size-8 shrink-0 items-center justify-center rounded-md hover:bg-muted"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
            ))}
          {prefillFailed && (
            <div role="status" className="flex items-center justify-between gap-2 rounded-md border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
              <span>{t('studio.publish.fanwork.prefill.loadFailed')}</span>
              <button
                type="button"
                onClick={() => setPrefillFailed(false)}
                aria-label={t('studio.publish.fanwork.a11y.dismissWarning')}
                className="inline-flex size-8 shrink-0 items-center justify-center rounded-md hover:bg-muted"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          )}
          {sourceMissing && (
            <p id={sourceHintId} className="text-xs text-destructive">
              {t('studio.publish.fanwork.validation.sourceRequired')}
            </p>
          )}
        </fieldset>
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
            {mediaContentType ? (
              activeGalleryLimits ? (
                <FileUploader
                  mode="media-gallery"
                  contentType={mediaContentType}
                  minCount={activeGalleryLimits.min}
                  maxCount={activeGalleryLimits.max}
                  maxMB={maxMB}
                  disabled={submitting}
                  isBusy={galleryConfigLoading}
                  error={uploadError}
                  onChange={(items) => {
                    setMediaItems(items);
                    setUploadedFiles(
                      items
                        .filter((item) => item.status === "done")
                        .map(toUploadedAsset)
                        .filter((asset): asset is UploadedAsset => asset !== null),
                    );
                    setUploadError("");
                  }}
                />
              ) : (
                <div className="rounded-md border border-border bg-card p-4 text-sm text-muted-foreground" role="status" aria-live="polite">
                  {galleryConfigLoading
                    ? t("studio.publish.media.loadingLimits")
                    : t("studio.publish.media.limitsUnavailable")}
                </div>
              )
            ) : (
              <FileUploader
                fileType={fileType}
                maxMB={maxMB}
                accept="*"
                disabled={submitting}
                onUploaded={(files) => {
                  setUploadedFiles(files);
                  setUploadError("");
                }}
              />
            )}
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

      {/* Collaborators: sits after the main content fields (and after the
          fanwork source fields) and before the submit actions; invites are
          sent after content creation succeeds. */}
      <div>
        <CollabUserPicker
          selectedUsers={collabUsers}
          maxSelected={collabMaxSelected}
          disabled={submitting}
          onChange={setCollabUsers}
        />
      </div>

      {/* AI Upload Assist */}
      {isFilePrimary && (
        <AgentFeatureGate capability="webAgent">
          <AgentUploadAssistPanel
            uploadedFiles={uploadedFiles.map((f) => f.fileName)}
            title={title}
            description={description}
            contentType={contentType}
            onFill={handleAssistFill}
            applyDisabled={complianceViolation || warningNeedsAcknowledgement}
            applyDisabledReason={
              complianceViolation
                ? t("agent.uploadAssistBlockedByViolation")
                : warningNeedsAcknowledgement
                  ? t("agent.warningAckRequired")
                  : undefined
            }
          />
          {complianceRisk === "warning" && (
            <label className="mt-2 flex items-start gap-2 text-xs text-muted-foreground">
              <input
                type="checkbox"
                className="mt-0.5 h-3.5 w-3.5 rounded border-border"
                checked={warningAcknowledged}
                onChange={(event) => setWarningAcknowledged(event.target.checked)}
              />
              <span>{t("agent.warningAckLabel")}</span>
            </label>
          )}
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

          {/* Video poster override. Image media uses the first sorted item as its cover. */}
          {mediaContentType === "video" && (
            <>
              <div className="flex items-center justify-between">
                <div>
                  <span className="text-sm font-medium text-foreground">{t('studio.publish.customCover')}</span>
                  <p className="text-xs text-muted-foreground">{t('studio.publish.coverDescription')}</p>
                </div>
                <Switch
                  checked={hasCustomCover}
                  onCheckedChange={(next) => {
                    setHasCustomCover(next);
                    if (!next) setCoverFile(null);
                  }}
                  aria-label={t('studio.publish.customCover')}
                  className={hasCustomCover ? "bg-[var(--accent-emphasis)]" : undefined}
                />
              </div>

              {hasCustomCover && (
                <div className="border-l-2 border-[var(--accent-subtle)] pl-2">
                  <FileUploader
                    fileType="image"
                    maxMB={20}
                    accept="image/*"
                    disabled={submitting}
                    onUploaded={(files) => {
                      setCoverFile(files[0] ?? null);
                    }}
                  />
                </div>
              )}
            </>
          )}

          {/* Comment toggle */}
          <div className="flex items-center justify-between border-t border-border/40 pt-4">
            <div className="flex items-center gap-2">
              <MessageCircle className="h-4 w-4 text-muted-foreground" />
              <span className="text-sm font-medium text-foreground">{t('studio.publish.allowComment')}</span>
            </div>
            <Switch
              checked={allowComments}
              onCheckedChange={setAllowComments}
              aria-label={t('studio.publish.allowComment')}
              className={allowComments ? "bg-[var(--accent-emphasis)]" : undefined}
            />
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
              <Switch
                checked={allowCopy}
                onCheckedChange={setAllowCopy}
                aria-label={t('studio.publish.allowCopyPR')}
                className={allowCopy ? "bg-[var(--accent-emphasis)]" : undefined}
              />
            </div>
          )}

          {/* Public toggle */}
          <div className="flex items-center justify-between border-t border-border/40 pt-4">
            <div className="flex items-center gap-2">
              {isPublic ? <Eye className="h-4 w-4 text-muted-foreground" /> : <EyeOff className="h-4 w-4 text-muted-foreground" />}
              <span className="text-sm font-medium text-foreground">{t('studio.publish.publicVisible')}</span>
            </div>
            <Switch
              checked={isPublic}
              onCheckedChange={setIsPublic}
              aria-label={t('studio.publish.publicVisible')}
              className={isPublic ? "bg-[var(--accent-emphasis)]" : undefined}
            />
          </div>

          {/* Agent deploy (mod/prompt only) */}
          <AgentFeatureGate capability="desktopDeploy">
          {(contentType === "mod" || contentType === "prompt") && (
            <div className="flex items-center justify-between border-t border-border/40 pt-4">
              <div>
                <span className="text-sm font-medium text-foreground">{t('studio.publish.agentDeploy')}</span>
                <p className="text-xs text-muted-foreground">{t('studio.publish.agentDeployDesc')}</p>
              </div>
              <Switch
                checked={agentEnabled}
                onCheckedChange={setAgentEnabled}
                aria-label={t('studio.publish.agentDeploy')}
                className={agentEnabled ? "bg-[var(--accent-emphasis)]" : undefined}
              />
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
            onResult={handleComplianceResult}
          />
          {complianceViolation && (
            <FieldError>{t("agent.complianceBlockSubmit")}</FieldError>
          )}
        </div>
      </AgentFeatureGate>

      {/* Bottom actions */}
      <div className="flex items-center gap-3 pt-2">
        <Button
          type="submit"
          size="lg"
          disabled={submitting || complianceViolation || galleryConfigLoading || galleryUnavailable || mediaItems.some((item) => item.status === "pending" || item.status === "uploading") || sourceMissing}
          className="gap-2 px-8"
        >
          <Send className="h-4 w-4" />
          {submitting ? t('studio.publish.submitting') : t('studio.publish.submit')}
        </Button>
        <Button type="button" variant="ghost" onClick={onBack}>{t('studio.publish.cancel')}</Button>
      </div>
    </form>
  );
}
