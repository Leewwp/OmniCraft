"use client";

import { useTranslations } from "next-intl";
import { useEffect, useRef, useState } from "react";
import type { DragEvent } from "react";
import {
  ChevronDown,
  ChevronUp,
  GripVertical,
  Image as ImageIcon,
  LoaderCircle,
  Trash2,
  UploadCloud,
  Video,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";

export interface UploadedAsset {
  fileName: string;
  fileType: string;
  mimeType: string;
  ossKey: string;
  grantId?: string;
  fileSize: number;
  width?: number;
  height?: number;
  sortOrder?: number;
  posterGrantId?: string;
  posterUrl?: string;
  isPrimary?: boolean;
}

export interface UploadItem {
  id: string;
  file: File;
  type: "image" | "video";
  width?: number;
  height?: number;
  sortOrder: number;
  posterGrantId?: string;
  posterUrl?: string;
  previewUrl?: string;
  status: "pending" | "uploading" | "done" | "error";
  progress?: number;
  error?: string;
  fileName?: string;
  fileType?: string;
  mimeType?: string;
  ossKey?: string;
  grantId?: string;
  fileSize?: number;
}

interface OSSUploadToken {
  upload_url: string;
  oss_key: string;
  grant_id: string;
  expires_in: number;
}

interface FileUploaderProps {
  mode?: "media-gallery" | "attachment";
  className?: string;
  fileType?: "image" | "video" | "text" | "mod" | "sheet_music";
  contentType?: "image" | "video" | string;
  maxMB?: number;
  accept?: string;
  multiple?: boolean;
  minCount?: number;
  maxCount?: number;
  value?: UploadItem[];
  onChange?: (items: UploadItem[]) => void;
  isBusy?: boolean;
  disabled?: boolean;
  error?: string;
  onUploaded?: (files: UploadedAsset[]) => void;
}

type FileValidationCode =
  | "imageReadFailed"
  | "videoReadFailed"
  | "posterUnavailable"
  | "invalidType"
  | "fileSizeExceeded"
  | "wrongType";

class FileValidationError extends Error {
  constructor(readonly code: FileValidationCode, readonly localizedDetail?: string) {
    super();
    this.name = "FileValidationError";
  }
}

interface Dimensions {
  width: number;
  height: number;
}

interface VideoPoster extends Dimensions {
  blob: Blob;
}

function releaseObjectURL(url?: string) {
  if (url?.startsWith("blob:")) {
    URL.revokeObjectURL(url);
  }
}

function createObjectURL(file: Blob): string | undefined {
  if (typeof URL.createObjectURL !== "function") {
    return undefined;
  }
  return URL.createObjectURL(file);
}

function readImageDimensions(file: Blob): Promise<Dimensions> {
  const objectURL = createObjectURL(file);
  if (!objectURL) {
    return Promise.reject(new FileValidationError("imageReadFailed"));
  }

  return new Promise((resolve, reject) => {
    const image = new window.Image();
    image.onload = () => {
      const dimensions = { width: image.naturalWidth, height: image.naturalHeight };
      releaseObjectURL(objectURL);
      if (dimensions.width <= 0 || dimensions.height <= 0) {
        reject(new FileValidationError("imageReadFailed"));
        return;
      }
      resolve(dimensions);
    };
    image.onerror = () => {
      releaseObjectURL(objectURL);
      reject(new FileValidationError("imageReadFailed"));
    };
    image.src = objectURL;
  });
}

function captureVideoPoster(file: File): Promise<VideoPoster> {
  const sourceURL = createObjectURL(file);
  if (!sourceURL) {
    return Promise.reject(new FileValidationError("posterUnavailable"));
  }

  return new Promise((resolve, reject) => {
    const video = document.createElement("video");
    let settled = false;

    const cleanup = () => {
      video.onloadedmetadata = null;
      video.onerror = null;
      video.onseeked = null;
      video.removeAttribute("src");
      video.load();
      releaseObjectURL(sourceURL);
    };

    const fail = (error: Error) => {
      if (settled) return;
      settled = true;
      cleanup();
      reject(error);
    };

    const renderFrame = () => {
      if (settled) return;
      const width = video.videoWidth;
      const height = video.videoHeight;
      if (width <= 0 || height <= 0) {
        fail(new FileValidationError("posterUnavailable"));
        return;
      }

      const canvas = document.createElement("canvas");
      canvas.width = width;
      canvas.height = height;
      const context = canvas.getContext("2d");
      if (!context) {
        fail(new FileValidationError("posterUnavailable"));
        return;
      }
      context.drawImage(video, 0, 0, width, height);
      canvas.toBlob((blob) => {
        if (!blob) {
          fail(new FileValidationError("posterUnavailable"));
          return;
        }
        settled = true;
        cleanup();
        resolve({ blob, width, height });
      }, "image/jpeg", 0.9);
    };

    video.preload = "metadata";
    video.muted = true;
    video.playsInline = true;
    video.onloadedmetadata = () => {
      const duration = Number.isFinite(video.duration) && video.duration > 0 ? video.duration : 0;
      const targetTime = Math.min(0.1, duration);
      if (targetTime === 0) {
        renderFrame();
        return;
      }
      video.onseeked = renderFrame;
      video.currentTime = targetTime;
    };
    video.onerror = () => fail(new FileValidationError("videoReadFailed"));
    video.src = sourceURL;
    video.load();
  });
}

function uploadWithXHR(
  uploadURL: string,
  file: File,
  onProgress?: (percent: number) => void,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", uploadURL);
    xhr.setRequestHeader("Content-Type", file.type || "application/octet-stream");
    xhr.upload.onprogress = (event) => {
      if (!onProgress || !event.lengthComputable) return;
      onProgress(Math.round((event.loaded / event.total) * 100));
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve();
      } else {
        reject(new Error("upload failed"));
      }
    };
    xhr.onerror = () => reject(new Error("upload failed"));
    xhr.send(file);
  });
}

async function requestUploadToken(file: File, fileType: string): Promise<OSSUploadToken> {
  return api.post<OSSUploadToken>("/api/v1/contents/oss-token", {
    file_name: file.name,
    file_type: fileType,
    mime_type: file.type || "application/octet-stream",
    file_size: file.size,
  });
}

export function toUploadedAsset(item: UploadItem): UploadedAsset | null {
  if (!item.ossKey || !item.fileName || !item.mimeType || !item.fileType || item.fileSize === undefined) {
    return null;
  }
  return {
    fileName: item.fileName,
    fileType: item.fileType,
    mimeType: item.mimeType,
    ossKey: item.ossKey,
    grantId: item.grantId,
    fileSize: item.fileSize,
    width: item.width,
    height: item.height,
    sortOrder: item.sortOrder,
    posterGrantId: item.posterGrantId,
    posterUrl: item.posterUrl,
  };
}

export function FileUploader({
  mode = "attachment",
  className,
  fileType,
  contentType,
  maxMB = 20,
  accept = "*",
  multiple,
  minCount = 0,
  maxCount = 9,
  value,
  onChange,
  isBusy = false,
  disabled = false,
  error: externalError,
  onUploaded,
}: FileUploaderProps) {
  const t = useTranslations();
  const inputRef = useRef<HTMLInputElement | null>(null);
  const itemsRef = useRef<UploadItem[]>(value ?? []);
  const [internalItems, setInternalItems] = useState<UploadItem[]>(value ?? []);
  const [isUploading, setIsUploading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState("");
  const [draggedID, setDraggedID] = useState<string | null>(null);
  const [dropTargetID, setDropTargetID] = useState<string | null>(null);

  const isMediaGallery = mode === "media-gallery" && (contentType === "image" || contentType === "video");
  const items = value ?? internalItems;
  itemsRef.current = items;

  useEffect(() => {
    if (value !== undefined) {
      setInternalItems(value);
    }
  }, [value]);

  useEffect(() => {
    return () => {
      for (const item of itemsRef.current) {
        releaseObjectURL(item.previewUrl);
        if (item.posterUrl !== item.previewUrl) releaseObjectURL(item.posterUrl);
      }
    };
  }, []);

  function commitItems(nextItems: UploadItem[]) {
    itemsRef.current = nextItems;
    if (value === undefined) {
      setInternalItems(nextItems);
    }
    onChange?.(nextItems);
  }

  function updateItem(id: string, patch: Partial<UploadItem>) {
    const nextItems = itemsRef.current.map((item) => (item.id === id ? { ...item, ...patch } : item));
    commitItems(nextItems);
  }

  function getValidationMessage(
    error: unknown,
    fallbackKey: "content.uploadFailed" | "studio.publish.media.uploadFailed",
  ) {
    if (!(error instanceof FileValidationError)) {
      return t(getUserFacingErrorKey(error, fallbackKey));
    }

    switch (error.code) {
      case "imageReadFailed":
        return t("studio.publish.media.imageReadFailed");
      case "videoReadFailed":
        return t("studio.publish.media.videoReadFailed");
      case "posterUnavailable":
        return t("studio.publish.media.posterUnavailable");
      case "invalidType":
        return t("studio.publish.media.invalidType");
      case "wrongType":
        return t("studio.publish.media.wrongType");
      case "fileSizeExceeded":
        return error.localizedDetail || t("content.uploadFailed");
    }
  }

  async function handleLegacyFiles(fileList: FileList | null) {
    if (!fileList || fileList.length === 0) return;

    const files = Array.from(fileList);
    const maxBytes = maxMB * 1024 * 1024;
    setError("");
    setIsUploading(true);
    setProgress(0);

    try {
      const uploaded: UploadedAsset[] = [];
      for (const file of files) {
        if (file.size > maxBytes) {
          throw new FileValidationError(
            "fileSizeExceeded",
            t("content.fileSizeExceeds", { name: file.name, maxMB }),
          );
        }

        const resolvedFileType = fileType ?? contentType ?? "text";
        const token = await requestUploadToken(file, resolvedFileType);
        await uploadWithXHR(token.upload_url, file, setProgress);
        let dimensions: Dimensions | undefined;
        if (file.type.startsWith("image/")) {
          try {
            dimensions = await readImageDimensions(file);
          } catch {
            dimensions = undefined;
          }
        }
        uploaded.push({
          fileName: file.name,
          fileType: resolvedFileType,
          mimeType: file.type || "application/octet-stream",
          ossKey: token.oss_key,
          grantId: token.grant_id,
          fileSize: file.size,
          width: dimensions?.width,
          height: dimensions?.height,
        });
      }
      onUploaded?.(uploaded);
    } catch (caughtError) {
      setError(getValidationMessage(caughtError, "content.uploadFailed"));
      silentError(caughtError, { component: "FileUploader", action: "handleLegacyFiles" });
    } finally {
      setIsUploading(false);
      setProgress(0);
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  async function uploadMediaFile(item: UploadItem): Promise<UploadedAsset> {
    if (!contentType || (contentType !== "image" && contentType !== "video")) {
      throw new FileValidationError("invalidType");
    }
    if (item.file.size > maxMB * 1024 * 1024) {
      throw new FileValidationError(
        "fileSizeExceeded",
        t("content.fileSizeExceeds", { name: item.file.name, maxMB }),
      );
    }
    if (!item.file.type.startsWith(`${contentType}/`)) {
      throw new FileValidationError("wrongType");
    }

    let dimensions: Dimensions;
    let posterGrantId: string | undefined;
    let posterUrl: string | undefined;

    if (contentType === "image") {
      dimensions = await readImageDimensions(item.file);
    } else {
      const poster = await captureVideoPoster(item.file);
      dimensions = { width: poster.width, height: poster.height };
      posterUrl = createObjectURL(poster.blob);
      if (!posterUrl) throw new FileValidationError("posterUnavailable");

      const posterFile = new File([poster.blob], `${item.file.name}.poster.jpg`, { type: "image/jpeg" });
      const posterToken = await requestUploadToken(posterFile, "image");
      await uploadWithXHR(posterToken.upload_url, posterFile, (value) => {
        updateItem(item.id, { progress: Math.round(value * 0.4) });
      });
      posterGrantId = posterToken.grant_id;
    }

    const token = await requestUploadToken(item.file, contentType);
    await uploadWithXHR(token.upload_url, item.file, (value) => {
      updateItem(item.id, { progress: contentType === "video" ? 40 + Math.round(value * 0.6) : value });
    });

    return {
      fileName: item.file.name,
      fileType: contentType,
      mimeType: item.file.type || "application/octet-stream",
      ossKey: token.oss_key,
      grantId: token.grant_id,
      fileSize: item.file.size,
      width: dimensions.width,
      height: dimensions.height,
      sortOrder: item.sortOrder,
      posterGrantId,
      posterUrl,
    };
  }

  async function handleMediaFiles(fileList: FileList | null) {
    if (!fileList || fileList.length === 0 || isBusy || disabled) return;

    const files = Array.from(fileList);
    if (items.length + files.length > maxCount) {
      setError(t("studio.publish.media.maxCount", { max: maxCount }));
      return;
    }

    setError("");
    const nextItems: UploadItem[] = files.map((file, index) => ({
      id: `${Date.now()}-${index}-${file.name}`,
      file,
      type: contentType as "image" | "video",
      sortOrder: items.length + index,
      previewUrl: contentType === "image" ? createObjectURL(file) : undefined,
      status: "pending",
      fileName: file.name,
      fileType: contentType,
      mimeType: file.type || "application/octet-stream",
      fileSize: file.size,
    }));
    commitItems([...items, ...nextItems]);
    setIsUploading(true);

    for (const item of nextItems) {
      updateItem(item.id, { status: "uploading", progress: 0, error: undefined });
      try {
        const uploaded = await uploadMediaFile(item);
        updateItem(item.id, { ...uploaded, status: "done", progress: 100 });
      } catch (caughtError) {
        updateItem(item.id, {
          status: "error",
          progress: 0,
          error: getValidationMessage(caughtError, "studio.publish.media.uploadFailed"),
        });
        setError(t("studio.publish.media.uploadFailed"));
        silentError(caughtError, { component: "FileUploader", action: "handleMediaFiles" });
      }
    }

    setIsUploading(false);
    if (inputRef.current) inputRef.current.value = "";
  }

  function moveItem(id: string, direction: -1 | 1) {
    const currentItems = itemsRef.current;
    const sourceIndex = currentItems.findIndex((item) => item.id === id);
    const targetIndex = sourceIndex + direction;
    if (sourceIndex < 0 || targetIndex < 0 || targetIndex >= currentItems.length) return;

    const reordered = [...currentItems];
    const [moved] = reordered.splice(sourceIndex, 1);
    reordered.splice(targetIndex, 0, moved);
    commitItems(reordered.map((item, index) => ({ ...item, sortOrder: index })));
  }

  function removeItem(id: string) {
    const item = itemsRef.current.find((candidate) => candidate.id === id);
    if (!item || item.status === "uploading" || isBusy || disabled) return;
    releaseObjectURL(item.previewUrl);
    if (item.posterUrl !== item.previewUrl) releaseObjectURL(item.posterUrl);
    setError("");
    commitItems(itemsRef.current
      .filter((candidate) => candidate.id !== id)
      .map((candidate, index) => ({ ...candidate, sortOrder: index })));
  }

  function handleDragStart(event: DragEvent<HTMLDivElement>, id: string) {
    if (isBusy || disabled || isUploading) return;
    setDraggedID(id);
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", id);
  }

  function handleDrop(event: DragEvent<HTMLDivElement>, targetID: string) {
    event.preventDefault();
    const sourceID = draggedID ?? event.dataTransfer.getData("text/plain");
    setDraggedID(null);
    setDropTargetID(null);
    if (!sourceID || sourceID === targetID) return;

    const currentItems = itemsRef.current;
    const sourceIndex = currentItems.findIndex((item) => item.id === sourceID);
    const targetIndex = currentItems.findIndex((item) => item.id === targetID);
    if (sourceIndex < 0 || targetIndex < 0) return;
    const reordered = [...currentItems];
    const [moved] = reordered.splice(sourceIndex, 1);
    reordered.splice(targetIndex, 0, moved);
    commitItems(reordered.map((item, index) => ({ ...item, sortOrder: index })));
  }

  if (!isMediaGallery) {
    return (
      <div className={`space-y-2 rounded-md border border-border bg-card p-3 shadow-none ${className ?? ""}`}>
        <div className="flex items-center justify-between gap-3">
          <p className="text-sm text-muted-foreground">{t("content.limitMb", { maxMB })}</p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={disabled || isBusy || isUploading}
            onClick={() => inputRef.current?.click()}
          >
            <UploadCloud className="mr-2 h-4 w-4" />
            {isUploading ? t("content.uploading") : t("content.selectFile")}
          </Button>
        </div>
        <input
          ref={inputRef}
          type="file"
          hidden
          multiple={multiple}
          accept={accept}
          disabled={disabled || isBusy || isUploading}
          onChange={(event) => void handleLegacyFiles(event.target.files)}
        />
        {(externalError || error) ? <p className="text-xs text-destructive">{externalError || error}</p> : null}
        {isUploading ? (
          <div className="space-y-1">
            <div className="h-1.5 w-full rounded bg-muted">
              <div className="h-1.5 rounded bg-primary" style={{ width: `${progress}%` }} />
            </div>
            <p className="text-xs text-muted-foreground">{t("content.uploadProgress", { progress })}</p>
          </div>
        ) : null}
      </div>
    );
  }

  const resolvedContentType = contentType as "image" | "video";
  const canAddMore = !disabled && !isBusy && !isUploading && items.length < maxCount;

  return (
    <div
      className={`space-y-3 rounded-lg border border-border bg-card p-4 shadow-none ${disabled ? "cursor-not-allowed opacity-50" : ""} ${className ?? ""}`}
      aria-busy={isUploading || isBusy}
      data-testid="media-uploader"
      onDragOver={(event) => {
        if (canAddMore) event.preventDefault();
      }}
      onDrop={(event) => {
        if (!canAddMore) return;
        event.preventDefault();
        void handleMediaFiles(event.dataTransfer.files);
      }}
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="text-sm font-medium text-foreground">
            {resolvedContentType === "image" ? t("studio.publish.media.imageTitle") : t("studio.publish.media.videoTitle")}
          </p>
          <p className="text-xs text-muted-foreground">
            {resolvedContentType === "image"
              ? t("studio.publish.media.imageHint", { min: minCount, max: maxCount })
              : t("studio.publish.media.videoHint", { min: minCount, max: maxCount })}
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={!canAddMore}
          onClick={() => inputRef.current?.click()}
        >
          <UploadCloud className="mr-2 h-4 w-4" />
          {isUploading ? t("studio.publish.media.uploading") : t("studio.publish.media.select")}
        </Button>
      </div>

      <input
        ref={inputRef}
        type="file"
        hidden
        multiple
        accept={resolvedContentType === "image" ? "image/*" : "video/*"}
        disabled={!canAddMore}
        onChange={(event) => void handleMediaFiles(event.target.files)}
      />

      {items.length > 0 ? (
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3" role="list" aria-label={t("studio.publish.media.listLabel")}>
          {items.map((item, index) => {
            const isFirst = index === 0;
            const isMoving = item.status === "uploading" || isBusy || disabled;
            return (
              <div
                key={item.id}
                role="listitem"
                draggable={!isMoving && item.status === "done"}
                onDragStart={(event) => handleDragStart(event, item.id)}
                onDragOver={(event) => {
                  if (!isMoving && draggedID && draggedID !== item.id) {
                    event.preventDefault();
                    setDropTargetID(item.id);
                  }
                }}
                onDragLeave={() => setDropTargetID(null)}
                onDrop={(event) => handleDrop(event, item.id)}
                onDragEnd={() => {
                  setDraggedID(null);
                  setDropTargetID(null);
                }}
                className={`min-w-0 rounded-md border border-border bg-background p-1 transition-opacity motion-reduce:transition-none ${draggedID === item.id ? "opacity-40" : ""} ${dropTargetID === item.id ? "ring-2 ring-ring" : ""}`}
              >
                <div className="relative aspect-square overflow-hidden rounded-sm bg-muted">
                  {item.previewUrl || item.posterUrl ? (
                    <img
                      src={item.previewUrl || item.posterUrl}
                      alt={t("studio.publish.media.previewAlt", { name: item.fileName || item.file.name })}
                      className="h-full w-full object-contain"
                      loading="lazy"
                    />
                  ) : (
                    <div className="flex h-full items-center justify-center text-muted-foreground" aria-hidden="true">
                      {resolvedContentType === "image" ? <ImageIcon className="h-8 w-8" /> : <Video className="h-8 w-8" />}
                    </div>
                  )}
                  <div className="absolute inset-x-1 top-1 flex items-start justify-between gap-1">
                    {isFirst ? (
                      <span className="rounded-sm bg-background/90 px-1.5 py-0.5 text-[10px] font-medium text-foreground">
                        {t("studio.publish.media.cover")}
                      </span>
                    ) : <span />}
                    <Button
                      type="button"
                      size="icon"
                      variant="secondary"
                      className="min-h-11 min-w-11 shrink-0 bg-background/90 hover:bg-background"
                      disabled={isMoving}
                      aria-label={t("studio.publish.media.remove", { name: item.fileName || item.file.name })}
                      onClick={() => removeItem(item.id)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                  {item.status === "uploading" ? (
                    <div className="absolute inset-x-1 bottom-1 rounded-sm bg-background/90 px-2 py-1">
                      <div className="flex items-center gap-1.5 text-[11px] text-foreground">
                        <LoaderCircle className="h-3.5 w-3.5 animate-spin motion-reduce:animate-none" />
                        <span>{t("studio.publish.media.uploadingProgress", { progress: item.progress ?? 0 })}</span>
                      </div>
                      <div className="mt-1 h-1 w-full rounded bg-muted">
                        <div className="h-1 rounded bg-primary transition-[width] duration-150 motion-reduce:transition-none" style={{ width: `${item.progress ?? 0}%` }} />
                      </div>
                    </div>
                  ) : null}
                </div>
                <div className="flex items-center gap-1 px-1 pt-1">
                  <GripVertical className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
                  <p className="min-w-0 flex-1 truncate text-[11px] text-muted-foreground" title={item.fileName || item.file.name}>
                    {item.fileName || item.file.name}
                  </p>
                  <div className="flex shrink-0 items-center">
                    <Button
                      type="button"
                      size="icon"
                      variant="ghost"
                      className="size-7 [@media(pointer:coarse)]:size-11"
                      disabled={isMoving || index === 0}
                      aria-label={t("studio.publish.media.moveUp", { name: item.fileName || item.file.name })}
                      onClick={() => moveItem(item.id, -1)}
                    >
                      <ChevronUp className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      type="button"
                      size="icon"
                      variant="ghost"
                      className="size-7 [@media(pointer:coarse)]:size-11"
                      disabled={isMoving || index === items.length - 1}
                      aria-label={t("studio.publish.media.moveDown", { name: item.fileName || item.file.name })}
                      onClick={() => moveItem(item.id, 1)}
                    >
                      <ChevronDown className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>
                {item.status === "error" && item.error ? <p className="px-1 pb-1 text-[11px] text-destructive">{item.error}</p> : null}
              </div>
            );
          })}
        </div>
      ) : (
        <button
          type="button"
          className="flex min-h-28 w-full cursor-pointer flex-col items-center justify-center gap-2 rounded-md border border-dashed border-border bg-background px-4 text-center text-sm text-muted-foreground transition-colors hover:border-ring hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed"
          disabled={!canAddMore}
          onClick={() => inputRef.current?.click()}
        >
          {resolvedContentType === "image" ? <ImageIcon className="h-6 w-6" /> : <Video className="h-6 w-6" />}
          <span>{t("studio.publish.media.empty")}</span>
        </button>
      )}

      {canAddMore && items.length > 0 ? (
        <Button type="button" variant="ghost" className="w-full border border-dashed border-border" onClick={() => inputRef.current?.click()}>
          <UploadCloud className="mr-2 h-4 w-4" />
          {t("studio.publish.media.addMore")}
        </Button>
      ) : null}
      {(externalError || error) ? <p className="text-xs text-destructive" role="alert">{externalError || error}</p> : null}
    </div>
  );
}
