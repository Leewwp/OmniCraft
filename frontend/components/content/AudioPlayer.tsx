"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { AlertTriangle, Loader2, Pause, Play } from "lucide-react";

import { DownloadButton } from "@/components/content/DownloadButton";

/**
 * AudioPlayer（#722）：音频附件的内置播放器——SoundCloud 式紧凑条
 * （标题 + 播放/暂停 + 可点击/拖动 seek 的进度条 + 时长显示），底层走
 * 原生 <audio> 能力、不引新依赖。经 AttachmentPreview 的预览入口点击后
 * 挂载；卸载/切换附件时停止播放并释放资源（合同见 #722）。
 *
 * 键盘可操作：进度条为 role="slider"（聚焦后 ←/→ ±5s、Home/End 首尾），
 * 播放按钮原生可达；aria 标签齐全。
 */

export interface AudioPlayerProps {
  url: string;
  fileName: string;
  fileSize?: number;
  contentId: number;
  attachmentId: number;
  allowCopy: boolean;
}

/** 秒 → m:ss（导出供单测）；不可有限时长（流式/未加载）显示占位。 */
export function formatAudioTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "--:--";
  const total = Math.floor(seconds);
  return `${Math.floor(total / 60)}:${(total % 60).toString().padStart(2, "0")}`;
}

const KEYBOARD_STEP_SECONDS = 5;

export function AudioPlayer({
  url,
  fileName,
  fileSize,
  contentId,
  attachmentId,
  allowCopy,
}: AudioPlayerProps) {
  const t = useTranslations();
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const barRef = useRef<HTMLDivElement | null>(null);
  const [playing, setPlaying] = useState(false);
  const [current, setCurrent] = useState(0);
  const [duration, setDuration] = useState<number | null>(null);
  const [error, setError] = useState(false);

  // 卸载/切换附件：停止播放并释放资源（src 移除 + load() 中断请求）。
  // StrictMode 双挂载防护：清理移除 src 后 React 不会重挂回填（属性未变），
  // 重挂载时显式恢复并重新加载。
  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;
    if (audio.getAttribute("src") !== url) {
      audio.src = url;
      audio.load();
    }
    return () => {
      audio.pause();
      audio.removeAttribute("src");
      audio.load();
    };
  }, [url]);

  const seekable = duration != null && Number.isFinite(duration) && duration > 0;
  const progress = seekable ? Math.min(current / (duration as number), 1) : 0;

  const commitSeek = (fraction: number) => {
    const audio = audioRef.current;
    if (!audio || !seekable) return;
    const target = Math.min(Math.max(fraction, 0), 0.999) * (duration as number);
    audio.currentTime = target;
    setCurrent(target);
  };

  const fractionFromPointer = (clientX: number): number => {
    const bar = barRef.current;
    if (!bar) return 0;
    const rect = bar.getBoundingClientRect();
    if (rect.width <= 0) return 0;
    return (clientX - rect.left) / rect.width;
  };

  // 点击 + 拖动 seek：pointerdown 捕获后拖动实时预览、抬起提交。
  const onPointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    if (!seekable) return;
    const bar = barRef.current;
    if (!bar) return;
    bar.setPointerCapture(event.pointerId);
    commitSeek(fractionFromPointer(event.clientX));
  };
  const onPointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    if (!seekable || !event.currentTarget.hasPointerCapture(event.pointerId)) return;
    commitSeek(fractionFromPointer(event.clientX));
  };

  const onKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (!seekable) return;
    const audio = audioRef.current;
    if (!audio) return;
    if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
      event.preventDefault();
      const step = event.key === "ArrowRight" ? KEYBOARD_STEP_SECONDS : -KEYBOARD_STEP_SECONDS;
      const target = Math.min(Math.max(audio.currentTime + step, 0), (duration as number) - 0.01);
      audio.currentTime = target;
      setCurrent(target);
    } else if (event.key === "Home") {
      event.preventDefault();
      audio.currentTime = 0;
      setCurrent(0);
    } else if (event.key === "End") {
      event.preventDefault();
      const target = (duration as number) - 0.01;
      audio.currentTime = target;
      setCurrent(target);
    }
  };

  const togglePlay = () => {
    const audio = audioRef.current;
    if (!audio || error) return;
    if (audio.paused) void audio.play().catch(() => setError(true));
    else audio.pause();
  };

  if (error) {
    return (
      <div
        className="flex items-center justify-between gap-3 rounded-lg border border-border bg-card p-4"
        data-testid="audio-player-error"
      >
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <AlertTriangle className="h-4 w-4 shrink-0 text-destructive" />
          {t("content.attachmentPreview.failed")}
        </div>
        {allowCopy && (
          <DownloadButton contentId={contentId} attachmentId={attachmentId} contentType="audio" size="sm" />
        )}
      </div>
    );
  }

  return (
    <div
      className="w-full rounded-lg border border-border bg-card p-4"
      data-testid="audio-player"
      aria-label={t("content.attachmentPreview.audio.playerLabel", { name: fileName })}
    >
      {/* eslint-disable-next-line jsx-a11y/media-has-caption -- 附件音频无字幕轨道 */}
      <audio
        ref={audioRef}
        src={url}
        preload="metadata"
        onLoadedMetadata={(event) => {
          const value = event.currentTarget.duration;
          setDuration(Number.isFinite(value) ? value : null);
        }}
        onTimeUpdate={(event) => setCurrent(event.currentTarget.currentTime)}
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onEnded={() => {
          setPlaying(false);
          if (duration != null) setCurrent(duration);
        }}
        onError={() => setError(true)}
      />
      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={togglePlay}
          aria-label={playing ? t("content.attachmentPreview.audio.pause") : t("content.attachmentPreview.audio.play")}
          data-testid="audio-play-button"
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground transition-colors hover:bg-primary/90"
        >
          {playing ? <Pause className="h-4 w-4" /> : <Play className="ml-0.5 h-4 w-4" />}
        </button>
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm text-foreground" title={fileName}>
            {fileName}
          </div>
          <div className="text-xs text-muted-foreground" data-testid="audio-time">
            {duration == null ? (
              <span className="inline-flex items-center gap-1">
                <Loader2 className="h-3 w-3 animate-spin" />
                {t("content.attachmentPreview.audio.loading")}
              </span>
            ) : (
              `${formatAudioTime(current)} / ${formatAudioTime(duration)}`
            )}
          </div>
        </div>
      </div>
      <div
        ref={barRef}
        role="slider"
        tabIndex={0}
        aria-label={t("content.attachmentPreview.audio.seekLabel")}
        aria-valuemin={0}
        aria-valuemax={Math.round(duration ?? 0)}
        aria-valuenow={Math.round(current)}
        aria-valuetext={seekable ? `${formatAudioTime(current)} / ${formatAudioTime(duration as number)}` : undefined}
        data-testid="audio-progress"
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onKeyDown={onKeyDown}
        className={`mt-3 h-1.5 w-full rounded-full bg-muted ${seekable ? "cursor-pointer" : "cursor-default"}`}
      >
        <div
          className="pointer-events-none h-full rounded-full bg-primary transition-[width] duration-100"
          style={{ width: `${progress * 100}%` }}
        />
      </div>
      {fileSize != null && fileSize > 0 && (
        <div className="mt-2 text-right text-xs text-muted-foreground">
          {(fileSize / 1024 / 1024).toFixed(2)} MB
        </div>
      )}
    </div>
  );
}
