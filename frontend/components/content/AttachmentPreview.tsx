"use client";

import dynamic from "next/dynamic";
import { useState } from "react";
import { useTranslations } from "next-intl";
import { Box, FileText, FileWarning, Music, ShieldAlert } from "lucide-react";

import type { PublicConfig } from "@/lib/public-config";
import { usePublicConfig } from "@/lib/use-public-config";
import { DownloadButton } from "@/components/content/DownloadButton";
import { AudioPlayer } from "@/components/content/AudioPlayer";

/**
 * 全站首个 next/dynamic 用例（#688）：查看器重依赖（mammoth/exceljs）
 * 全部懒加载分块，不进主包。
 */
const DocumentViewer = dynamic(
  () => import("@/components/content/DocumentViewer").then((m) => m.DocumentViewer),
  {
    ssr: false,
    loading: () => null,
  },
);

/** #689：3D 查看器独立懒分块（three.js 全家桶不进主包）。 */
const ModelViewer = dynamic(
  () => import("@/components/content/ModelViewer").then((m) => m.ModelViewer),
  {
    ssr: false,
    loading: () => null,
  },
);

export interface PreviewableAttachment {
  id: number;
  file_type?: string;
  mime_type?: string;
  oss_key?: string;
  oss_url?: string;
  file_size?: number | null;
  scan_status?: string;
  original_file_name?: string | null;
}

/** 扫描门放行的状态（后端非 clean 不签发 oss_url；这里双保险渲染状态卡）。 */
function isPreviewBlockedByScan(scanStatus?: string): boolean {
  if (!scanStatus) return false;
  return scanStatus !== "clean" && scanStatus !== "not_required";
}

export type PreviewVariant = "scan-card" | "entry" | "download-card";

/**
 * 预览分发判定（#722 导出供单测）：先扫描门，再看族群与查看器资格。
 * document/model3d/audio 三族群有查看器且有签名 URL → entry（预览入口卡，
 * 用户点击后才挂载查看器并加载文件字节）；其余（.mtl、缺 URL、无查看器
 * 族群）回落下载卡。
 */
export function resolvePreviewVariant(attachment: PreviewableAttachment): PreviewVariant {
  if (isPreviewBlockedByScan(attachment.scan_status)) return "scan-card";
  const name = attachment.original_file_name || attachment.oss_key || "";
  if (attachment.file_type === "document" && attachment.oss_url) return "entry";
  if (attachment.file_type === "model3d" && attachment.oss_url && !name.toLowerCase().endsWith(".mtl")) return "entry";
  if (attachment.file_type === "audio" && attachment.oss_url) return "entry";
  return "download-card";
}

function familyIcon(fileType?: string) {
  if (fileType === "model3d") return Box;
  if (fileType === "audio") return Music;
  return FileText;
}

/**
 * 预览入口卡（#722）：默认不 fetch 预览字节——紧凑呈现文件名/类型/大小 +
 * 预览按钮，点击后才挂载查看器。图片走既有媒体画廊不变；超预算降级卡在
 * 查看器内部维持现状（点击后呈现）。
 */
function PreviewEntryCard({
  attachment,
  contentId,
  allowCopy,
  onActivate,
}: {
  attachment: PreviewableAttachment;
  contentId: number;
  allowCopy: boolean;
  onActivate: () => void;
}) {
  const t = useTranslations();
  const Icon = familyIcon(attachment.file_type);
  const fileName = attachment.original_file_name || attachment.oss_key || "";
  return (
    <div
      className="flex items-center justify-between gap-3 rounded-lg border border-border bg-card p-4"
      data-testid="attachment-preview-entry"
    >
      <div className="flex min-w-0 items-center gap-2 text-sm">
        <Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
        <span className="shrink-0 text-xs text-muted-foreground">
          {t(`content.attachmentPreview.familyLabel.${attachment.file_type || "document"}`)}
        </span>
        <span className="truncate text-foreground" title={fileName || undefined}>
          {fileName || t("content.attachmentPreview.noViewer", {
            fileType: attachment.file_type || t("content.attachmentUnknownType"),
          })}
        </span>
        {attachment.file_size != null && (
          <span className="shrink-0 text-xs text-muted-foreground">
            {(attachment.file_size / 1024 / 1024).toFixed(2)} MB
          </span>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <button
          type="button"
          onClick={onActivate}
          data-testid="attachment-preview-button"
          aria-label={t("content.attachmentPreview.previewButtonLabel", { name: fileName })}
          className="rounded-md bg-primary px-3 py-1.5 text-xs text-primary-foreground transition-colors hover:bg-primary/90"
        >
          {t("content.attachmentPreview.previewAction")}
        </button>
        {allowCopy && (
          <DownloadButton
            contentId={contentId}
            attachmentId={attachment.id}
            contentType={attachment.file_type}
            size="sm"
          />
        )}
      </div>
    </div>
  );
}

function ScanStatusCard({ status, contentId, attachmentId, allowCopy }: {
  status: string;
  contentId: number;
  attachmentId: number;
  allowCopy: boolean;
}) {
  const t = useTranslations();
  const known = ["pending", "scanning", "manual_review", "blocked", "failed", "legacy_unscanned"];
  const messageKey = known.includes(status)
    ? `content.attachmentPreview.scanStatus.${status}`
    : "content.attachmentPreview.scanStatus.unavailable";
  return (
    <div
      className="flex items-center justify-between gap-3 rounded-lg border border-border bg-card p-4"
      data-testid="attachment-scan-card"
    >
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <ShieldAlert className="h-4 w-4 shrink-0" />
        <span>{t(messageKey)}</span>
      </div>
      {allowCopy && <DownloadButton contentId={contentId} attachmentId={attachmentId} contentType="file" size="sm" />}
    </div>
  );
}

/**
 * 统一附件预览分发（#688；#722 点击触发化）。
 *
 * 主路由 = 附件持久化 file_type（发布时服务端校验过的事实）→ 族群内
 * subtype 按扩展名（original_file_name / oss_key 尾缀）→ MIME 辅助。
 * document/model3d/audio 三族群经预览入口卡显式点击后挂载查看器；
 * 无查看器的扩展名回落调用方既有下载卡（行为不变）。
 */
export function AttachmentPreview({
  attachment,
  contentId,
  allowCopy,
  config,
}: {
  attachment: PreviewableAttachment;
  contentId: number;
  allowCopy: boolean;
  config?: PublicConfig | null;
}) {
  const t = useTranslations();
  const fetchedConfig = usePublicConfig();
  const effectiveConfig = config ?? fetchedConfig;
  const [active, setActive] = useState(false);
  const variant = resolvePreviewVariant(attachment);

  if (variant === "scan-card") {
    return (
      <ScanStatusCard
        status={attachment.scan_status as string}
        contentId={contentId}
        attachmentId={attachment.id}
        allowCopy={allowCopy}
      />
    );
  }

  if (variant === "entry" && !active) {
    return (
      <PreviewEntryCard
        attachment={attachment}
        contentId={contentId}
        allowCopy={allowCopy}
        onActivate={() => setActive(true)}
      />
    );
  }

  if (variant === "entry" && attachment.file_type === "document") {
    return (
      <DocumentViewer
        url={attachment.oss_url as string}
        fileName={attachment.original_file_name || attachment.oss_key || ""}
        fileSize={attachment.file_size ?? undefined}
        maxPreviewMB={effectiveConfig?.upload?.document_preview_max_mb}
        contentId={contentId}
        attachmentId={attachment.id}
        allowCopy={allowCopy}
      />
    );
  }

  // #689 model3d 族：.mtl 已在 variant 判定中落下载卡，这里五格式进查看器。
  if (variant === "entry" && attachment.file_type === "model3d") {
    const model3dName = attachment.original_file_name || attachment.oss_key || "";
    return (
      <ModelViewer
        url={attachment.oss_url as string}
        fileName={model3dName}
        fileSize={attachment.file_size ?? undefined}
        maxPreviewMB={effectiveConfig?.upload?.model3d_max_preview_mb}
        maxTriangles={effectiveConfig?.upload?.model3d_max_triangles}
        gcodeMaxLines={effectiveConfig?.upload?.gcode_max_lines}
        contentId={contentId}
        attachmentId={attachment.id}
        allowCopy={allowCopy}
      />
    );
  }

  // #722 audio 族：SoundCloud 式紧凑播放器（预览入口点击后挂载）。
  if (variant === "entry" && attachment.file_type === "audio") {
    return (
      <AudioPlayer
        url={attachment.oss_url as string}
        fileName={attachment.original_file_name || attachment.oss_key || ""}
        fileSize={attachment.file_size ?? undefined}
        contentId={contentId}
        attachmentId={attachment.id}
        allowCopy={allowCopy}
      />
    );
  }

  // 无查看器的族群与缺 URL 的行：回落下载卡（story 10——
  // 原始文件名 + 大小 + 下载入口；历史行回退类型标签）。
  return (
    <div
      className="flex items-center justify-between gap-3 rounded-lg border border-border bg-card p-4"
      data-testid="attachment-download-card"
    >
      <div className="flex min-w-0 items-center gap-2 text-sm">
        <FileWarning className="h-4 w-4 shrink-0 text-muted-foreground" />
        <span className="truncate text-foreground" title={attachment.original_file_name || undefined}>
          {attachment.original_file_name || t("content.attachmentPreview.noViewer", {
            fileType: attachment.file_type || t("content.attachmentUnknownType"),
          })}
        </span>
        {attachment.file_size != null && (
          <span className="shrink-0 text-xs text-muted-foreground">
            {(attachment.file_size / 1024 / 1024).toFixed(2)} MB
          </span>
        )}
      </div>
      {allowCopy && (
        <DownloadButton
          contentId={contentId}
          attachmentId={attachment.id}
          contentType={attachment.file_type}
          size="sm"
        />
      )}
    </div>
  );
}
