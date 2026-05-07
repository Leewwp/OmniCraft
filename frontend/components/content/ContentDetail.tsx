"use client";

import { useTranslations } from "next-intl";
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
  Download,
  Rocket,
  Bookmark,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { MarkdownRenderer } from "@/components/content/MarkdownRenderer";
import { SheetMusicViewer } from "@/components/content/SheetMusicViewer";
import { ReactionBar } from "@/components/social/ReactionBar";
import { CommentSection } from "@/components/social/CommentSection";
import { useAuth } from "@/contexts/AuthContext";
import { useToast } from "@/components/ui/Toast";
import { api, ApiRequestError } from "@/lib/api";
import { cn } from "@/lib/utils";

interface Attachment {
  id: number;
  file_type?: string;
  mime_type?: string;
  oss_key?: string;
  oss_url?: string;
  file_size?: number;
  is_primary?: boolean;
}

interface ContentDetailData {
  id: number;
  title: string;
  author?: { id?: number; username?: string; avatar_url?: string };
  author_id?: number;
  zone?: string;
  ip?: { id?: number; name?: string; slug?: string };
  category?: string;
  content_type?: string;
  cover_image_url?: string;
  status?: string;
  view_count?: number;
  like_count?: number;
  dislike_count?: number;
  description?: string;
  body?: string;
  is_public?: boolean;
  allow_copy?: boolean;
  agent_enabled?: boolean;
  attachments?: Attachment[];
  tags?: string[];
  created_at?: string;
  updated_at?: string;
}

interface ContentDetailProps {
  data: ContentDetailData;
  className?: string;
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

function CoverImage({ url, contentType, title, typeLabel }: { url?: string; contentType?: string; title: string; typeLabel: string }) {
  const Icon = getTypeIcon(contentType || "other");

  if (url) {
    return (
      <div className="overflow-hidden rounded-md border border-border relative aspect-[16/9] max-h-96">
        <Image
          src={url}
          alt={title}
          fill
          className="object-cover"
          sizes="(max-width: 768px) 100vw, 800px"
        />
      </div>
    );
  }

  return (
    <div className="flex h-48 w-full items-center justify-center rounded-md border border-border bg-muted/20">
      <div className="flex flex-col items-center gap-2 text-muted-foreground">
        <Icon className="h-12 w-12" />
        <span className="text-xs">{typeLabel}</span>
      </div>
    </div>
  );
}

export function ContentDetail({ data, className }: ContentDetailProps) {
  const t = useTranslations();
  const contentType = data.content_type || "other";
  const typeLabel = getTypeLabel(t, contentType);
  const description = data.description || data.body || "";
  const { user } = useAuth();
  const { toast } = useToast();
  const [favorited, setFavorited] = useState(false);
  const [favBusy, setFavBusy] = useState(false);
  const [tagSuggestionBusy, setTagSuggestionBusy] = useState<string | null>(null);

  async function handleTagSuggestion(tag: string, action: "add" | "remove") {
    if (!user) return;
    setTagSuggestionBusy(`${tag}:${action}`);
    try {
      await api.post(`/api/v1/contents/${data.id}/tags/suggest`, { tag, action });
      toast("success", t("content.tagSuggestionSubmitted"));
    } catch (e) {
      toast("error", e instanceof ApiRequestError ? e.message : t("common.operationFailed"));
    } finally {
      setTagSuggestionBusy(null);
    }
  }

  useEffect(() => {
    if (!user) return;
    void checkFavorite();
  }, [user, data.id]);

  async function checkFavorite() {
    try {
      const favData = await api.get<{ favorites?: { content_item_id: number }[] }>(
        `/api/v1/users/${user!.id}/favorites`
      );
      const favs = favData.favorites || [];
      setFavorited(favs.some((f) => f.content_item_id === data.id));
    } catch { /* ignore */ }
  }

  async function toggleFavorite() {
    if (!user) return;
    setFavBusy(true);
    try {
      if (favorited) {
        await api.delete(`/api/v1/favorites/${data.id}`);
        setFavorited(false);
      } else {
        await api.post("/api/v1/favorites", { content_item_id: data.id });
        setFavorited(true);
      }
    } catch { /* ignore */ }
    finally { setFavBusy(false); }
  }

  return (
    <div className={cn("space-y-6", className)}>
      {/* Header */}
      <div className="space-y-4">
        <h1 className="text-2xl font-bold tracking-tight text-foreground">
          {data.title}
        </h1>

        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
          <span>
            {t('content.author', { name: data.author?.username ?? t('common.userLabel', { id: data.author_id ?? "-" }) })}
          </span>
          {data.zone === "fanwork" && data.ip && (
            <span>
              IP：{data.ip.name}
            </span>
          )}
          <span>{t('content.type', { type: typeLabel })}</span>
          {data.view_count != null && (
            <span>{t('content.views', { count: data.view_count })}</span>
          )}
          {data.created_at && (
            <span>
              {new Date(data.created_at).toLocaleDateString("zh-CN", {
                year: "numeric",
                month: "2-digit",
                day: "2-digit",
              })}
            </span>
          )}
        </div>
      </div>

      {/* Cover Image */}
      <CoverImage
        url={data.cover_image_url}
        contentType={contentType}
        title={data.title}
        typeLabel={typeLabel}
      />

      {/* Content Body */}
      {description && contentType === "article" && (
        <section className="rounded-md border border-border bg-card p-4 shadow-none">
          <MarkdownRenderer content={description} />
        </section>
      )}

      {description && contentType !== "article" && contentType !== "sheet_music" && (
        <section className="rounded-md border border-border bg-card p-4 shadow-none">
          <p className="text-sm leading-relaxed text-foreground/90 whitespace-pre-wrap">
            {description}
          </p>
        </section>
      )}

      {/* Attachments / Gallery for non-sheet-music types */}
      {data.attachments && data.attachments.length > 0 && contentType !== "sheet_music" && (
        <section className="space-y-2 rounded-md border border-border bg-card p-4 shadow-none">
          <h2 className="text-sm font-semibold">{t('content.attachments')}</h2>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            {data.attachments.map((att) => (
              <div
                key={att.id}
                className="flex items-center justify-between rounded border border-border bg-muted/10 p-2"
              >
                <span className="text-xs text-muted-foreground">
                  {att.file_type || "file"}
                  {att.file_size != null && ` (${(att.file_size / 1024).toFixed(1)} KB)`}
                </span>
                {data.allow_copy && att.oss_url && (
                  <a href={att.oss_url} download target="_blank" rel="noopener noreferrer">
                    <Button variant="outline" size="sm">
                      <Download className="h-3 w-3" />
                    </Button>
                  </a>
                )}
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Sheet Music Viewer */}
      {contentType === "sheet_music" && (
        <SheetMusicViewer
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

      {/* Download All Button */}
      {data.allow_copy && data.attachments && data.attachments.length > 1 && (
        <div className="flex flex-wrap gap-2">
          {data.attachments
            .filter((a) => a.oss_url)
            .map((att) => (
              <a key={att.id} href={att.oss_url} download target="_blank" rel="noopener noreferrer">
                <Button variant="outline" size="sm">
                  <Download className="mr-1 h-3.5 w-3.5" />
                  {t('content.downloadFile', { type: att.file_type || "file" })}
                </Button>
              </a>
            ))}
        </div>
      )}

      {/* Agent Deploy Button */}
      {data.agent_enabled && (contentType === "mod" || contentType === "prompt") && (
        <div className="rounded-md border border-border bg-card p-4 shadow-none">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0">
              <p className="text-sm font-semibold">{t('content.oneClickDeploy')}</p>
              <p className="text-xs text-muted-foreground">
                {t('content.deployViaClient', { type: contentType === "mod" ? t('home.mod') : t('home.aiPrompt') })}
              </p>
            </div>
            <a
              href={`omnicraft://deploy?content_id=${data.id}`}
              className="inline-flex shrink-0 items-center justify-center rounded-md border border-border bg-accent px-3 py-2 text-xs font-medium text-accent-foreground hover:bg-accent/80"
            >
              <Rocket className="mr-1 h-3.5 w-3.5" />
              {t('content.oneClickDeploy')}
            </a>
          </div>
        </div>
      )}

      {/* Favorite Button */}
      <div className="flex items-center gap-2 rounded-md border border-border bg-card px-4 py-3 shadow-none">
        <Button
          variant={favorited ? "default" : "outline"}
          size="sm"
          disabled={!user || favBusy}
          onClick={() => void toggleFavorite()}
        >
          <Bookmark className="mr-1 h-3.5 w-3.5" />
          {favorited ? t('content.favorited') : t('content.favorite')}
        </Button>
      </div>

      {/* Reaction Bar */}
      <ReactionBar
        contentId={data.id}
        initialLikes={data.like_count ?? 0}
        initialDislikes={data.dislike_count ?? 0}
      />

      {/* Comments */}
      <section className="rounded-md border border-border bg-card p-4 shadow-none">
        <CommentSection contentId={data.id} />
      </section>
    </div>
  );
}
