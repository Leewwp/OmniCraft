"use client";

import { useTranslations } from "next-intl";
import { useState, useEffect, useRef, useCallback } from "react";
import {
  FileMusic,
  Download,
  Music,
  FileText,
  Play,
  Pause,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface SheetMusicAttachment {
  id: number;
  file_type?: string;
  mime_type?: string;
  oss_url?: string;
  oss_key?: string;
  file_size?: number;
}

interface SheetMusicViewerProps {
  attachments: SheetMusicAttachment[];
  allowCopy?: boolean;
  className?: string;
}

/* ── Format type label ─────────────────────── */
function formatType(t: (key: string) => string, fileType?: string, mimeType?: string): string {
  if (fileType === "sheet_music_midi" || mimeType === "audio/midi") return "MIDI";
  if (fileType === "sheet_music_xml" || mimeType?.includes("musicxml")) return "MusicXML";
  if (fileType === "sheet_music_mscz") return "MuseScore (MSCZ)";
  if (fileType === "sheet_music_mscx") return "MuseScore (MSCX)";
  if (fileType === "sheet_music_pdf" || mimeType === "application/pdf") return t("content.pdfSheetMusic");
  return t("content.sheetMusicFile");
}

function fileTypeFor(att: SheetMusicAttachment): string {
  const ft = att.file_type || "";
  const mt = att.mime_type || "";
  if (ft === "sheet_music_midi" || mt === "audio/midi") return "midi";
  if (ft === "sheet_music_xml" || mt.includes("musicxml")) return "musicxml";
  if (ft === "sheet_music_pdf" || mt === "application/pdf") return "pdf";
  if (ft === "sheet_music_mscz") return "mscz";
  if (ft === "sheet_music_mscx") return "mscx";
  return "other";
}

/* ── OSMDRenderer (MusicXML → staff notation) ─ */
function OSMDRenderer({ ossUrl }: { ossUrl: string }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    const el = containerRef.current;
    if (!el) return;

    async function init() {
      try {
        const target = el;
        if (!target) return;
        const { OpenSheetMusicDisplay } = await import("opensheetmusicdisplay");
        target.innerHTML = "";
        const osmd = new OpenSheetMusicDisplay(target, {
          autoResize: true,
          backend: "svg",
          drawTitle: false,
        });
        await osmd.load(ossUrl);
        if (!cancelled) osmd.render();
      } catch (e: unknown) {
        if (!cancelled) setError((e as Error).message || "Failed to render sheet music");
      }
    }
    init();
    return () => { cancelled = true; };
  }, [ossUrl]);

  return (
    <div className="relative min-h-[300px] overflow-x-auto rounded-md border border-border bg-white p-4">
      {error && (
        <div className="absolute inset-0 flex items-center justify-center bg-white/80">
          <p className="text-sm text-destructive">{error}</p>
        </div>
      )}
      <div ref={containerRef} className="osmd-container" />
    </div>
  );
}

/* ── MIDIPlayer (MIDI playback with soundfont) ─ */
function MIDIPlayer({ ossUrl }: { ossUrl: string }) {
  const t = useTranslations();
  const [playing, setPlaying] = useState(false);
  const [loading, setLoading] = useState(true);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [error, setError] = useState("");
  const ctxRef = useRef<AudioContext | null>(null);
  const playerRef = useRef<import("midi-player-js").Player | null>(null);
  const pianoRef = useRef<import("soundfont-player").Player | null>(null);
  const startRef = useRef(0);
  const animRef = useRef<number>(0);

  useEffect(() => {
    let cancelled = false;
    const ctx = new AudioContext();
    ctxRef.current = ctx;

    async function init() {
      try {
        const [{ Player }, { instrument }] = await Promise.all([
          import("midi-player-js"),
          import("soundfont-player"),
        ]);

        if (cancelled) return;

        const piano = await instrument(ctx, "acoustic_grand_piano");
        pianoRef.current = piano;

        const player = new Player(() => {});

        // Fetch MIDI data
        const res = await fetch(ossUrl);
        if (!res.ok) throw new Error("Failed to load MIDI file");
        const arrayBuffer = await res.arrayBuffer();
        player.loadArrayBuffer(arrayBuffer);

        player.on("midiEvent", (event: import("midi-player-js").Event) => {
          if (event.name === "Note on" && event.noteNumber != null) {
            piano.play(event.noteNumber, 0, { gain: (event.velocity ?? 100) / 127 });
          }
        });

        playerRef.current = player;

        const totalTicks = player.getTotalTicks();
        const secs = totalTicks > 0 ? player.getSongTime() : 30;
        if (!cancelled) {
          setDuration(Math.max(secs, 10));
          setLoading(false);
        }
      } catch (e: unknown) {
        if (!cancelled) {
          setError((e as Error).message || "Failed to load MIDI");
          setLoading(false);
        }
      }
    }

    init();
    return () => {
      cancelled = true;
      try { ctx.close(); } catch { /* ignore */ }
    };
  }, [ossUrl]);

  function togglePlay() {
    const player = playerRef.current;
    const ctx = ctxRef.current;
    if (!player || !ctx) return;

    if (playing) {
      player.pause();
      setPlaying(false);
      cancelAnimationFrame(animRef.current);
    } else {
      if (ctx.state === "suspended") ctx.resume();
      player.play();
      setPlaying(true);
      startRef.current = performance.now();
      function tick() {
        const elapsed = (performance.now() - startRef.current) / 1000;
        setCurrentTime(Math.min(elapsed, duration));
        if (elapsed < duration) {
          animRef.current = requestAnimationFrame(tick);
        } else {
          setPlaying(false);
        }
      }
      animRef.current = requestAnimationFrame(tick);
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center rounded-md border border-border bg-muted/20 p-8">
        <p className="text-sm text-muted-foreground">{t("common.loading")}</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-md border border-border bg-canvas-subtle p-4 text-center">
        <p className="text-sm text-destructive">{error}</p>
      </div>
    );
  }

  const formatTime = (s: number) =>
    `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, "0")}`;

  return (
    <div className="rounded-md border border-border bg-card p-4 space-y-3">
      <div className="flex items-center gap-3">
        <Button
          variant="outline"
          size="sm"
          onClick={togglePlay}
          className="h-9 w-9 p-0"
          aria-label={playing ? "Pause" : "Play"}
        >
          {playing ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
        </Button>
        <div className="flex-1">
          <div className="relative h-2 w-full rounded-full bg-muted/40">
            <div
              className="absolute inset-y-0 left-0 rounded-full bg-accent transition-[width] duration-200"
              style={{ width: duration > 0 ? `${(currentTime / duration) * 100}%` : "0%" }}
            />
          </div>
        </div>
      </div>
      <div className="flex justify-between text-xs text-muted-foreground">
        <span>{formatTime(currentTime)}</span>
        <span>{formatTime(duration)}</span>
      </div>
    </div>
  );
}

/* ── PDFViewer ─────────────────────────────── */
function PDFViewer({ ossUrl }: { ossUrl: string }) {
  const t = useTranslations();
  return (
    <div className="space-y-2">
      <embed
        src={ossUrl}
        type="application/pdf"
        className="h-[500px] w-full rounded-md border border-border"
      />
      <a href={ossUrl} target="_blank" rel="noopener noreferrer" className="inline-block">
        <Button variant="outline" size="sm">
          {t("content.download")}
        </Button>
      </a>
    </div>
  );
}

/* ── DownloadPrompt (MSCZ / MSCX) ──────────── */
function DownloadPrompt({ att }: { att: SheetMusicAttachment }) {
  const t = useTranslations();
  return (
    <div className="rounded-md border border-border bg-card p-6 text-center ">
      <FileMusic className="mx-auto h-10 w-10 text-muted-foreground" />
      <p className="mt-3 text-sm font-medium">
        {formatType(t, att.file_type, att.mime_type)}
      </p>
      <p className="mt-1 text-xs text-muted-foreground">
        {t("content.sheetMusicPreviewHint")}
      </p>
      {att.oss_url && (
        <a
          href={att.oss_url}
          download
          target="_blank"
          rel="noopener noreferrer"
          className="mt-3 inline-block"
        >
          <Button size="sm">
            <Download className="mr-1 h-3.5 w-3.5" />
            {t("content.download")}
          </Button>
        </a>
      )}
    </div>
  );
}

/* ── Attachment list row (non-renderable types) ─ */
function AttachmentRow({
  att,
  allowCopy,
  t,
}: {
  att: SheetMusicAttachment;
  allowCopy: boolean;
  t: (key: string) => string;
}) {
  const Icon = att.file_type === "sheet_music_xml" || att.file_type === "sheet_music_midi"
    ? Music
    : att.file_type === "sheet_music_mscz" || att.file_type === "sheet_music_mscx"
      ? FileMusic
      : FileText;
  return (
    <li className="flex items-center justify-between py-3 first:pt-0 last:pb-0">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-md border border-border bg-muted/30">
          <Icon className="h-5 w-5 text-muted-foreground" />
        </div>
        <div>
          <p className="text-sm font-medium">{formatType(t, att.file_type, att.mime_type)}</p>
          {att.file_size != null && (
            <p className="text-xs text-muted-foreground">
              {(att.file_size / 1024).toFixed(1)} KB
            </p>
          )}
        </div>
      </div>
      {allowCopy && att.oss_url && (
        <a href={att.oss_url} download target="_blank" rel="noopener noreferrer">
          <Button variant="outline" size="sm">
            <Download className="mr-1 h-3.5 w-3.5" />
            {t("content.download")}
          </Button>
        </a>
      )}
    </li>
  );
}

/* ── Main SheetMusicViewer ─────────────────── */
export function SheetMusicViewer({ attachments, allowCopy, className }: SheetMusicViewerProps) {
  const t = useTranslations();

  if (!attachments || attachments.length === 0) {
    return (
      <div
        className={cn(
          "flex items-center justify-center rounded-md border border-border bg-muted/20 p-8",
          className,
        )}
      >
        <p className="text-sm text-muted-foreground">{t("content.noSheetMusic")}</p>
      </div>
    );
  }

  // Find the primary renderable attachment
  const renderable = attachments.find(
    (a) => fileTypeFor(a) === "musicxml" || fileTypeFor(a) === "midi" || fileTypeFor(a) === "pdf",
  );
  const other = attachments.filter((a) => a !== renderable);

  return (
    <div className={cn("space-y-4", className)}>
      {/* Primary interactive viewer */}
      {renderable && renderable.oss_url && fileTypeFor(renderable) === "musicxml" && (
        <OSMDRenderer ossUrl={renderable.oss_url} />
      )}
      {renderable && renderable.oss_url && fileTypeFor(renderable) === "midi" && (
        <MIDIPlayer ossUrl={renderable.oss_url} />
      )}
      {renderable && renderable.oss_url && fileTypeFor(renderable) === "pdf" && (
        <PDFViewer ossUrl={renderable.oss_url} />
      )}
      {renderable && (fileTypeFor(renderable) === "mscz" || fileTypeFor(renderable) === "mscx") && (
        <DownloadPrompt att={renderable} />
      )}

      {/* Other attachments list */}
      {other.length > 0 && (
        <div className="rounded-md border border-border bg-card p-4 ">
          <h3 className="mb-3 text-sm font-semibold">{t("content.sheetMusicAttachments")}</h3>
          <ul className="divide-y divide-border">
            {other.map((att) => (
              <AttachmentRow key={att.id} att={att} allowCopy={!!allowCopy} t={t} />
            ))}
          </ul>
        </div>
      )}

      {/* Generic preview hint for types without interactive viewer */}
      {!renderable && (
        <div className="rounded-md border border-border bg-canvas-subtle p-4 ">
          <div className="flex items-center gap-2">
            <FileMusic className="h-5 w-5 text-muted-foreground" />
            <div>
              <p className="text-sm font-medium">{t("content.sheetMusicPreview")}</p>
              <p className="text-xs text-muted-foreground">{t("content.sheetMusicPreviewHint")}</p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
