"use client";

import { useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { Download, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { api, ApiRequestError } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { cn } from "@/lib/utils";

interface DownloadButtonProps {
  className?: string;
  contentId: number;
  attachmentId?: number;
  contentTitle?: string;
  contentType?: string;
  isDisabled?: boolean;
  disableReason?: string;
  onDownloadComplete?: () => void;
  variant?: "outline" | "default";
  size?: "sm" | "default";
}

interface DownloadResponse {
  download_url: string;
  expires_in: number;
}

export function DownloadButton({
  className,
  contentId,
  attachmentId,
  contentTitle,
  contentType,
  isDisabled,
  disableReason,
  onDownloadComplete,
  variant = "outline",
  size = "sm",
}: DownloadButtonProps) {
  const t = useTranslations();
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);

  const handleDownload = useCallback(async () => {
    if (isDisabled || loading) return;
    setLoading(true);
    try {
      let path = `/api/v1/contents/${contentId}/download`;
      if (attachmentId != null) {
        path += `?attachment_id=${attachmentId}`;
      }
      const data = await api.get<DownloadResponse>(path);
      if (data.download_url) {
        window.open(data.download_url, "_blank", "noopener,noreferrer");
        onDownloadComplete?.();
      }
    } catch (err) {
      if (err instanceof ApiRequestError) {
        if (err.code === "UNAUTHORIZED") {
          window.location.href = "/login";
          return;
        }
        toast("error", err.message);
      } else {
        toast("error", t("content.downloadFailed"));
      }
    } finally {
      setLoading(false);
    }
  }, [contentId, attachmentId, isDisabled, loading, onDownloadComplete, toast, t]);

  const label = contentType === "sheet_music"
    ? t("content.downloadSheetMusic")
    : t("content.download");

  return (
    <Button
      variant={variant}
      size={size}
      className={cn(className)}
      onClick={handleDownload}
      disabled={isDisabled || loading}
      title={isDisabled ? disableReason : undefined}
    >
      {loading ? (
        <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" />
      ) : (
        <Download className="mr-1 h-3.5 w-3.5" />
      )}
      {loading ? t("content.downloading") : label}
    </Button>
  );
}
