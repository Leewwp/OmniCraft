"use client";

import { useTranslations, useLocale } from "next-intl";
import { useState, useEffect } from "react";
import Image from "next/image";
import {
  FileText,
  Image as ImageIcon,
  Video,
  Music,
  Package,
  Sparkles,
  FileMusic,
  Shapes,
  Rocket,
  Bookmark,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { MarkdownRenderer } from "@/components/content/MarkdownRenderer";
import { SheetMusicViewer } from "@/components/content/SheetMusicViewer";
import { DownloadButton } from "@/components/content/DownloadButton";
import { CollectionPicker } from "@/components/content/CollectionPicker";
import { UsageGuidePanel } from "@/components/agent/UsageGuidePanel";
import { ReactionBar } from "@/components/social/ReactionBar";
import { CommentSection } from "@/components/social/CommentSection";
import { useAuth } from "@/contexts/AuthContext";
import { useToast } from "@/components/ui/Toast";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { cn } from "@/lib/utils";
import { AgentFeatureGate } from "@/components/agent/AgentFeatureGate";
import { SeriesNav } from "@/components/content/SeriesNav";
import { MediaGallery, selectMediaItems } from "@/components/content/MediaGallery";
import type { AttachmentData, ContentDetailData } from "@/lib/content";

interface ContentDetailProps {
  data: ContentDetailData;
  className?: string;
  /** 浮层封面同步（#64 决策 11）：开启后正文在封面加载落定前保持布局不可见。 */
  coverSync?: boolean;
  /** #88 桌面双栏：媒体区由浮层层（Overlay Layer）在左栏另行渲染（≥1100px），
      行内媒体区仅保留给 <1100px 单列视图（min-[1100px]:hidden）。 */
  mediaSlot?: "inline" | "split";
  /** #88 双栏模式下行外媒体区（左栏 MediaGallery）的首项加载落定信号。 */
  coverReady?: boolean;
  /** #89 连续浏览：移动端行内媒体集最后一项继续上滑时触发（上层切篇）。 */
  onGalleryReachEnd?: () => void;
  /** #89 连续浏览：上下文列表到底时在媒体区下显示「已经到底」提示。 */
  galleryEndHint?: boolean;
}

function getTypeLabel(t: (key: string) => string, contentType: string): string {
  switch (contentType) {
    case "article": return t('home.text');
    case "image": return t('home.image');
    case "video": return t('home.video');
    case "audio": return t('home.audio');
    case "mod": return t('home.mod');
    case "prompt": return t('home.aiPrompt');
    case "sheet_music": return t('home.sheetMusic');
    case "template": return t('home.template');
    default: return t('home.other');
  }
}

function getTypeIcon(contentType: string) {
  switch (contentType) {
    case "article": return FileText;
    case "image": return ImageIcon;
    case "video": return Video;
    case "audio": return Music;
    case "mod": return Package;
    case "prompt": return Sparkles;
    case "sheet_music": return FileMusic;
    case "template": return Shapes;
    default: return Shapes;
  }
}

interface CoverImageProps {
  url?: string;
  contentType?: string;
  title: string;
  typeLabel: string;
  /** 浮层封面同步（决策 11）：加载态在最终封面几何内展示，成功后正文才 reveal。 */
  coverSync?: boolean;
  coverState: "loading" | "ready" | "error";
  onCoverSettled: (state: "ready" | "error") => void;
}

function CoverImage({ url, contentType, title, typeLabel, coverSync, coverState, onCoverSettled }: CoverImageProps) {
  const Icon = getTypeIcon(contentType || "other");
  const showImage = Boolean(url && coverState !== "error");
  const showSkeleton = Boolean(coverSync && url && coverState === "loading");

  return (
    <div
      data-slot="detail-cover"
      className="relative w-full overflow-hidden rounded-md border border-border bg-muted"
    >
      {/* 封面与正文共享同一水平框架：外层 w-full 恒定，高度上限只裁内框不缩宽度（#64 决策 12）。 */}
      <div className="relative aspect-[16/9] max-h-96 w-full">
        {showImage && url ? (
          <Image
            src={url}
            alt={title}
            fill
            className={cn("object-cover", showSkeleton && "opacity-0")}
            onLoad={() => onCoverSettled("ready")}
            onError={() => onCoverSettled("error")}
            sizes="(max-width: 768px) 100vw, 800px"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center">
            <div className="flex flex-col items-center gap-2 text-muted-foreground">
              <Icon className="h-12 w-12" />
              <span className="text-xs">{typeLabel}</span>
            </div>
          </div>
        )}
        {showSkeleton && (
          <div aria-hidden="true" className="absolute inset-0 flex animate-pulse items-center justify-center bg-muted">
            <Icon className="h-12 w-12 text-muted-foreground/40" />
          </div>
        )}
      </div>
    </div>
  );
}

export function ContentDetail({
  data,
  className,
  coverSync = false,
  mediaSlot = "inline",
  coverReady,
  onGalleryReachEnd,
  galleryEndHint = false,
}: ContentDetailProps) {
  const t = useTranslations();
  const locale = useLocale();
  const contentType = data.content_type || "other";
  const typeLabel = getTypeLabel(t, contentType);
  const description = data.description || data.body || "";
  const { user } = useAuth();
  const { toast } = useToast();
  const [collectionPickerOpen, setCollectionPickerOpen] = useState(false);
  const [tagSuggestionBusy, setTagSuggestionBusy] = useState<string | null>(null);

  const { media: mediaItems, downloads: downloadItems } = selectMediaItems(
    data.attachments ?? [],
    contentType,
    contentType === "video" ? data.cover_image_url : undefined,
  );
  const isMediaContent = contentType === "image" || contentType === "video";
  const usesGallery = isMediaContent && mediaItems.length > 0;
  const [coverState, setCoverState] = useState<"loading" | "ready" | "error">(() =>
    coverSync && usesGallery ? "loading" : coverSync && data.cover_image_url ? "loading" : "ready",
  );
  /* #88 双栏：≥1100px 时行内媒体区被隐藏（min-[1100px]:hidden），其首项不会触发
     落定事件；正文 reveal 改由左栏行外媒体区的 coverReady 信号驱动（任一路径落定即显示，
     错误态同 reveal，与 ui-spec:2411 稳定占位符语义一致）。 */
  const settled =
    coverState !== "loading" || (mediaSlot === "split" && coverReady !== undefined);
  const bodyVisible = !coverSync || settled;

  async function handleTagSuggestion(tag: string, action: "add" | "remove") {
    if (!user) return;
    setTagSuggestionBusy(`${tag}:${action}`);
    try {
      await api.post(`/api/v1/contents/${data.id}/tags/suggest`, { tag, action });
      toast("success", t("content.tagSuggestionSubmitted"));
    } catch (e) {
      toast("error", t(getUserFacingErrorKey(e)));
      silentError(e, { component: 'ContentDetail', action: 'handleTagSuggestion' });
    } finally {
      setTagSuggestionBusy(null);
    }
  }

  useEffect(() => {
    if (!user) return;
    api.post("/api/v1/users/me/history", { content_item_id: data.id }).catch((e) => { silentError(e, { component: 'ContentDetail', action: 'recordHistory' }); });
  }, [user, data.id]);

  const collectionZone = data.zone === "fanwork" ? "fanwork" : "original";

  return (
    <div className={cn("space-y-6", className)}>
      {/* Header（浮层封面同步：随正文一起在封面落定后 reveal） */}
      <div className={cn("space-y-4", bodyVisible ? undefined : "invisible")}>
        <h1 className="text-2xl font-bold tracking-tight text-foreground">
          {data.title}
        </h1>

        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
          <span>
            {t('content.author', { name: data.author?.username ?? t('common.userLabel', { id: data.author_id ?? "-" }) })}
          </span>
          {data.zone === "fanwork" && data.ip?.name && (
            <span>
              {t('content.ipLabel', { name: data.ip.name })}
            </span>
          )}
          <span>{t('content.type', { type: typeLabel })}</span>
          {data.view_count != null && (
            <span>{t('content.views', { count: data.view_count })}</span>
          )}
          {data.created_at && (
            <span>
              {new Date(data.created_at).toLocaleDateString(locale === "en" ? "en-US" : "zh-CN", {
                year: "numeric",
                month: "2-digit",
                day: "2-digit",
              })}
            </span>
          )}
        </div>
      </div>

      {/* Media area: image/video content renders the stable MediaGallery
          (contain, no crop); other types keep the cover image. The gallery
          carries data-slot="detail-cover" so the overlay FLIP/cover-sync
          contract keeps working on the shared surface. #88 双栏模式下该行内
          媒体区在 ≥1100px 隐藏（由 Overlay 层的左栏媒体列承担）。 */}
      <div className={cn(mediaSlot === "split" && "min-[1100px]:hidden")}>
        {usesGallery ? (
          <MediaGallery
            items={mediaItems}
            onFirstMediaSettled={setCoverState}
            onReachEnd={onGalleryReachEnd}
          />
        ) : (
          <CoverImage
            url={data.cover_image_url}
            contentType={contentType}
            title={data.title}
            typeLabel={typeLabel}
            coverSync={coverSync}
            coverState={coverState}
            onCoverSettled={setCoverState}
          />
        )}
        {/* #89 连续浏览：上下文列表到底提示（随媒体区一起在双栏桌面端隐藏，
            与瀑布流统一「已经到底了」语义一致）。 */}
        {usesGallery && galleryEndHint && (
          <div
            data-slot="media-continue-end"
            className="flex items-center justify-center py-3 text-xs text-muted-foreground"
          >
            {t("media.continue.endReached")}
          </div>
        )}
      </div>

      {/* Content Body（浮层封面同步：正文在封面落定前保持布局、不可见，避免跳版） */}
      <div
        data-slot="detail-body"
        className={cn("space-y-6", bodyVisible ? undefined : "invisible")}
      >
        {description && contentType === "article" && (
          <section className="rounded-md border border-border bg-card p-4 ">
            <MarkdownRenderer content={description} />
          </section>
        )}

      {description && contentType !== "article" && contentType !== "sheet_music" && (
        <section className="rounded-md border border-border bg-card p-4 ">
          <p className="text-sm leading-relaxed text-foreground/90 whitespace-pre-wrap">
            {description}
          </p>
        </section>
      )}

      {/* Attachments download list: media-set entries (image/video items of
          image/video content) are excluded — they are browsed in the gallery.
          Other content types keep the full attachment list semantics (AC3). */}
      {downloadItems.length > 0 && contentType !== "sheet_music" && (
        <section className="space-y-2 rounded-md border border-border bg-card p-4 ">
          <h2 className="text-sm font-semibold">{t('content.attachments')}</h2>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            {downloadItems.map((att) => (
              <div
                key={att.id}
                className="flex items-center justify-between rounded border border-border bg-muted/10 p-2"
              >
                <span className="text-xs text-muted-foreground">
                  {att.file_type || t("content.attachmentUnknownType")}
                  {att.file_size != null && ` (${(att.file_size / 1024).toFixed(1)} KB)`}
                </span>
                {data.allow_copy && (
                  <DownloadButton
                    contentId={data.id}
                    attachmentId={att.id}
                    contentType={att.file_type}
                    size="sm"
                  />
                )}
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Sheet Music Viewer */}
      {contentType === "sheet_music" && (
        <SheetMusicViewer
          contentId={data.id}
          attachments={data.attachments || []}
          allowCopy={data.allow_copy}
        />
      )}

      {/* Tags */}
      {data.tags && data.tags.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {data.tags.map((tag) => (
            <span key={tag} className="group relative inline-flex items-center">
              <Badge variant="secondary">{tag}</Badge>
              <span className="ml-0.5 hidden items-center gap-0 group-hover:flex">
                <button
                  type="button"
                  className="inline-flex h-4 w-4 items-center justify-center rounded text-[10px] font-bold text-emerald-600 hover:bg-emerald-100 transition-colors"
                  onClick={() => handleTagSuggestion(tag, "add")}
                  disabled={tagSuggestionBusy === `${tag}:add`}
                  aria-label={t("content.suggestAddTag", { tag })}
                  title={t("content.suggestAddTagHint")}
                >
                  +
                </button>
                <button
                  type="button"
                  className="inline-flex h-4 w-4 items-center justify-center rounded text-[10px] font-bold text-red-500 hover:bg-red-100 transition-colors"
                  onClick={() => handleTagSuggestion(tag, "remove")}
                  disabled={tagSuggestionBusy === `${tag}:remove`}
                  aria-label={t("content.suggestRemoveTag", { tag })}
                  title={t("content.suggestRemoveTagHint")}
                >
                  −
                </button>
              </span>
            </span>
          ))}
        </div>
      )}

      {/* Download All Button（仅非媒体集素材） */}
      {data.allow_copy && downloadItems.length > 1 && (
        <div className="flex flex-wrap gap-2">
          {downloadItems.map((att) => (
            <DownloadButton
              key={att.id}
              contentId={data.id}
              attachmentId={att.id}
              contentType={att.file_type}
              size="sm"
            />
          ))}
        </div>
      )}

      {/* Agent Deploy Button */}
      <AgentFeatureGate capability="desktopDeploy">
      {data.agent_enabled && (contentType === "mod" || contentType === "prompt") && (
        <div className="rounded-md border border-border bg-card p-4 ">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0">
              <p className="text-sm font-semibold">{t('content.oneClickDeploy')}</p>
              <p className="text-xs text-muted-foreground">
                {t('content.deployViaClient', { type: contentType === "mod" ? t('home.mod') : t('home.aiPrompt') })}
              </p>
            </div>
            <a
              href={`omnicraft://deploy?content_id=${data.id}`}
              className="inline-flex shrink-0 items-center justify-center rounded-md border border-border bg-accent px-3 py-2 text-xs font-medium text-accent-foreground transition-all duration-150 hover:bg-accent/80 active:scale-95"
            >
              <Rocket className="mr-1 h-3.5 w-3.5" />
              {t('content.oneClickDeploy')}
            </a>
          </div>
        </div>
      )}
      </AgentFeatureGate>

      {/* AI Usage Guide */}
      <AgentFeatureGate capability="webAgent">
        {(contentType === "mod" || contentType === "sheet_music") && data.status === "published" && (
          <UsageGuidePanel contentId={data.id} />
        )}
      </AgentFeatureGate>

      {/* Collection Picker Button */}
      <div className="flex items-center gap-2 rounded-md border border-border bg-card px-4 py-3 ">
        <Button
          variant="outline"
          size="sm"
          disabled={!user}
          onClick={() => setCollectionPickerOpen(true)}
        >
          <Bookmark className="mr-1 h-3.5 w-3.5" />
          {t("collections.picker.actions.open")}
        </Button>
        <CollectionPicker
          contentId={data.id}
          contentTitle={data.title}
          zone={collectionZone}
          open={collectionPickerOpen}
          onOpenChange={setCollectionPickerOpen}
        />
      </div>

      {/* Reaction Bar */}
      <ReactionBar
        contentId={data.id}
        initialLikes={data.like_count ?? 0}
        initialDislikes={data.dislike_count ?? 0}
      />

      {data.series_memberships && data.series_memberships.length > 0 && (
        <SeriesNav memberships={data.series_memberships} />
      )}

      {/* Comments */}
      <section className="rounded-md border border-border bg-card p-4 ">
        <CommentSection contentId={data.id} />
      </section>
      </div>
    </div>
  );
}
