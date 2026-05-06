"use client";

import { useState, useEffect } from "react";
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
import { api } from "@/lib/api";
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

function getTypeLabel(contentType: string): string {
  switch (contentType) {
    case "article": return "文字";
    case "image": return "图片";
    case "video": return "视频";
    case "audio": return "音频";
    case "mod": return "Mod";
    case "prompt": return "AI 提示词";
    case "sheet_music": return "乐谱";
    case "template": return "模板";
    default: return "其他";
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

function CoverImage({ url, contentType, title }: { url?: string; contentType?: string; title: string }) {
  const Icon = getTypeIcon(contentType || "other");

  if (url) {
    return (
      <div className="overflow-hidden rounded-md border border-border">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={url}
          alt={title}
          className="max-h-96 w-full object-cover"
        />
      </div>
    );
  }

  return (
    <div className="flex h-48 w-full items-center justify-center rounded-md border border-border bg-muted/20">
      <div className="flex flex-col items-center gap-2 text-muted-foreground">
        <Icon className="h-12 w-12" />
        <span className="text-xs">{getTypeLabel(contentType || "other")}</span>
      </div>
    </div>
  );
}

export function ContentDetail({ data, className }: ContentDetailProps) {
  const contentType = data.content_type || "other";
  const description = data.description || data.body || "";
  const { user } = useAuth();
  const [favorited, setFavorited] = useState(false);
  const [favBusy, setFavBusy] = useState(false);

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
            作者：{data.author?.username ?? `用户 #${data.author_id ?? "-"}`}
          </span>
          {data.zone === "fanwork" && data.ip && (
            <span>
              IP：{data.ip.name}
            </span>
          )}
          <span>类型：{getTypeLabel(contentType)}</span>
          {data.view_count != null && (
            <span>{data.view_count} 次浏览</span>
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
          <h2 className="text-sm font-semibold">附件</h2>
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
            <Badge key={tag} variant="secondary">
              {tag}
            </Badge>
          ))}
        </div>
      )}

      {/* Download All Button */}
      {data.allow_copy && data.attachments && data.attachments.length > 1 && (
        <div className="flex gap-2">
          {data.attachments
            .filter((a) => a.oss_url)
            .map((att) => (
              <a key={att.id} href={att.oss_url} download target="_blank" rel="noopener noreferrer">
                <Button variant="outline" size="sm">
                  <Download className="mr-1 h-3.5 w-3.5" />
                  下载 {att.file_type || "文件"}
                </Button>
              </a>
            ))}
        </div>
      )}

      {/* Agent Deploy Button */}
      {data.agent_enabled && (contentType === "mod" || contentType === "prompt") && (
        <div className="rounded-md border border-border bg-card p-4 shadow-none">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-semibold">一键部署</p>
              <p className="text-xs text-muted-foreground">
                通过 OmniCraft 客户端自动安装此{contentType === "mod" ? "Mod" : "提示词"}
              </p>
            </div>
            <a
              href={`omnicraft://deploy?content_id=${data.id}`}
              className="inline-flex shrink-0 items-center rounded-md border border-border bg-accent px-3 py-2 text-xs font-medium text-accent-foreground hover:bg-accent/80"
            >
              <Rocket className="mr-1 h-3.5 w-3.5" />
              一键部署
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
          {favorited ? "已收藏" : "收藏"}
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
