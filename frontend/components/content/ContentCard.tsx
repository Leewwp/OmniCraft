import Link from "next/link";
import {
  Eye,
  Heart,
  MessageCircle,
  FileText,
  Image as ImageIcon,
  Video,
  Music,
  Package,
  Sparkles,
  FileMusic,
  Shapes,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

export interface ContentCardData {
  id: number;
  title: string;
  author_id?: number;
  author?: {
    username?: string;
  };
  zone?: string;
  content_type?: string;
  cover_image_url?: string;
  view_count?: number;
  like_count?: number;
  comment_count?: number;
  tags?: string[];
  category?: string;
}

interface ContentCardProps {
  data: ContentCardData;
  className?: string;
}

function getTypeLabel(contentType: string): string {
  switch (contentType) {
    case "text":
      return "文字";
    case "image":
      return "图片";
    case "video":
      return "视频";
    case "audio":
      return "音频";
    case "mod":
      return "Mod";
    case "prompt":
      return "AI 提示词";
    case "sheet_music":
      return "乐谱";
    default:
      return "其他";
  }
}

function getTypeIcon(contentType: string) {
  switch (contentType) {
    case "text":
      return FileText;
    case "image":
      return ImageIcon;
    case "video":
      return Video;
    case "audio":
      return Music;
    case "mod":
      return Package;
    case "prompt":
      return Sparkles;
    case "sheet_music":
      return FileMusic;
    default:
      return Shapes;
  }
}

function getCardHref(data: ContentCardData): string {
  if (data.zone === "original") {
    return `/original/${data.id}`;
  }
  return `/content/${data.id}`;
}

export function ContentCard({ data, className }: ContentCardProps) {
  const contentType = data.content_type || "other";
  const Icon = getTypeIcon(contentType);
  const tagCandidates =
    data.tags && data.tags.length > 0
      ? data.tags
      : [getTypeLabel(contentType), data.category].filter(
          (item): item is string => Boolean(item)
        );
  const tags = tagCandidates.slice(0, 3);

  return (
    <Link
      href={getCardHref(data)}
      className={cn(
        "group block overflow-hidden rounded-md border border-border bg-card shadow-none transition-colors hover:bg-muted/30",
        className
      )}
    >
      <div className="aspect-[3/4] w-full border-b border-border bg-muted/40">
        {data.cover_image_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={data.cover_image_url}
            alt={data.title}
            className="h-full w-full object-cover"
            loading="lazy"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-muted-foreground">
            <div className="flex flex-col items-center gap-2">
              <Icon className="h-10 w-10" />
              <span className="text-xs">{getTypeLabel(contentType)}</span>
            </div>
          </div>
        )}
      </div>

      <div className="flex flex-col gap-3 p-3">
        <div className="space-y-1">
          <h3 className="line-clamp-2 text-sm font-semibold text-foreground">
            {data.title}
          </h3>
          <p className="text-xs text-muted-foreground">
            {data.author?.username || `用户 #${data.author_id ?? "-"}`}
          </p>
        </div>

        {tags.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {tags.map((tag) => (
              <Badge key={tag} variant="secondary" className="text-[10px]">
                {tag}
              </Badge>
            ))}
          </div>
        )}

        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          <span className="inline-flex items-center gap-1">
            <Heart className="h-3.5 w-3.5" />
            {data.like_count ?? 0}
          </span>
          <span className="inline-flex items-center gap-1">
            <MessageCircle className="h-3.5 w-3.5" />
            {data.comment_count ?? 0}
          </span>
          <span className="inline-flex items-center gap-1">
            <Eye className="h-3.5 w-3.5" />
            {data.view_count ?? 0}
          </span>
        </div>
      </div>
    </Link>
  );
}
