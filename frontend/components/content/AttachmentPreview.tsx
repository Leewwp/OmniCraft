"use client";

import dynamic from "next/dynamic";
import { useTranslations } from "next-intl";
import { FileWarning, ShieldAlert } from "lucide-react";

import type { PublicConfig } from "@/lib/public-config";
import { usePublicConfig } from "@/lib/use-public-config";
import { DownloadButton } from "@/components/content/DownloadButton";

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
 * 统一附件预览分发（#688）。
 *
 * 主路由 = 附件持久化 file_type（发布时服务端校验过的事实）→ 族群内
 * subtype 按扩展名（original_file_name / oss_key 尾缀）→ MIME 辅助。
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
  void effectiveConfig;

  if (isPreviewBlockedByScan(attachment.scan_status)) {
    return (
      <ScanStatusCard
        status={attachment.scan_status as string}
        contentId={contentId}
        attachmentId={attachment.id}
        allowCopy={allowCopy}
      />
    );
  }

  if (attachment.file_type === "document" && attachment.oss_url) {
    return (
      <DocumentViewer
        url={attachment.oss_url}
        fileName={attachment.original_file_name || attachment.oss_key || ""}
        fileSize={attachment.file_size ?? undefined}
        maxPreviewMB={effectiveConfig?.upload?.document_preview_max_mb}
        contentId={contentId}
        attachmentId={attachment.id}
        allowCopy={allowCopy}
      />
    );
  }

  // #689 model3d 族：.mtl 落下载卡（V1 无材质匹配），其余五格式进查看器。
  const model3dName = attachment.original_file_name || attachment.oss_key || "";
  const isMaterialFile = model3dName.toLowerCase().endsWith(".mtl");
  if (attachment.file_type === "model3d" && attachment.oss_url && !isMaterialFile) {
    return (
      <ModelViewer
        url={attachment.oss_url}
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

  // 无查看器的族群（audio 等）与缺 URL 的行：回落下载卡（story 10——
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
