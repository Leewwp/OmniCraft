"use client";

import { useTranslations } from "next-intl";
import { useRef, useState } from "react";
import { UploadCloud } from "lucide-react";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";

export interface UploadedAsset {
  fileName: string;
  fileType: string;
  mimeType: string;
  ossKey: string;
  fileSize: number;
  isPrimary?: boolean;
}

interface OSSUploadToken {
  upload_url: string;
  oss_key: string;
  expires_in: number;
}

interface FileUploaderProps {
  fileType: "image" | "video" | "text" | "mod" | "sheet_music";
  maxMB: number;
  accept: string;
  multiple?: boolean;
  disabled?: boolean;
  onUploaded: (files: UploadedAsset[]) => void;
}

function uploadWithXHR(
  uploadURL: string,
  file: File,
  onProgress?: (percent: number) => void
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", uploadURL);
    xhr.setRequestHeader("Content-Type", file.type || "application/octet-stream");
    xhr.upload.onprogress = (event) => {
      if (!onProgress || !event.lengthComputable) {
        return;
      }
      const percent = Math.round((event.loaded / event.total) * 100);
      onProgress(percent);
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve();
      } else {
        reject(new Error(`upload failed with status ${xhr.status}`));
      }
    };
    xhr.onerror = () => reject(new Error("upload failed"));
    xhr.send(file);
  });
}

export function FileUploader({
  fileType,
  maxMB,
  accept,
  multiple,
  disabled,
  onUploaded,
}: FileUploaderProps) {
  const t = useTranslations();
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState("");

  async function handleFiles(fileList: FileList | null) {
    if (!fileList || fileList.length === 0) {
      return;
    }

    const files = Array.from(fileList);
    const maxBytes = maxMB * 1024 * 1024;

    setError("");
    setIsUploading(true);
    setProgress(0);

    try {
      const uploaded: UploadedAsset[] = [];

      for (const file of files) {
        if (file.size > maxBytes) {
          throw new Error(t('content.fileSizeExceeds', { name: file.name, maxMB }));
        }

        const token = await api.post<OSSUploadToken>("/api/v1/contents/oss-token", {
          file_name: file.name,
          file_type: fileType,
          mime_type: file.type || "application/octet-stream",
          file_size: file.size,
        });

        await uploadWithXHR(token.upload_url, file, setProgress);

        uploaded.push({
          fileName: file.name,
          fileType,
          mimeType: file.type || "application/octet-stream",
          ossKey: token.oss_key,
          fileSize: file.size,
        });
      }

      onUploaded(uploaded);
    } catch (e) {
      setError(e instanceof Error ? e.message : t('content.uploadFailed'));
    } finally {
      setIsUploading(false);
      setProgress(0);
      if (inputRef.current) {
        inputRef.current.value = "";
      }
    }
  }

  return (
    <div className="space-y-2 rounded-md border border-border bg-card p-3 shadow-none">
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground">{t('content.limitMb', { maxMB })}</p>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={disabled || isUploading}
          onClick={() => inputRef.current?.click()}
        >
          <UploadCloud className="mr-2 h-4 w-4" />
          {isUploading ? t('content.uploading') : t('content.selectFile')}
        </Button>
      </div>

      <input
        ref={inputRef}
        type="file"
        hidden
        multiple={multiple}
        accept={accept}
        disabled={disabled || isUploading}
        onChange={(e) => {
          void handleFiles(e.target.files);
        }}
      />

      {error ? <p className="text-xs text-destructive">{error}</p> : null}
      {isUploading ? (
        <div className="space-y-1">
          <div className="h-1.5 w-full rounded bg-muted">
            <div className="h-1.5 rounded bg-primary" style={{ width: `${progress}%` }} />
          </div>
          <p className="text-xs text-muted-foreground">{t('content.uploadProgress', { progress })}</p>
        </div>
      ) : null}
    </div>
  );
}
