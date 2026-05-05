"use client";

import { FileMusic, Download, Music, FileText } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface SheetMusicAttachment {
  id: number;
  file_type?: string;
  mime_type?: string;
  oss_url?: string;
  file_size?: number;
}

interface SheetMusicViewerProps {
  attachments: SheetMusicAttachment[];
  allowCopy?: boolean;
  className?: string;
}

function formatType(mimeType?: string, fileType?: string): string {
  if (fileType === "sheet_music_midi") return "MIDI";
  if (fileType === "sheet_music_xml") return "MusicXML";
  if (fileType === "sheet_music_mscz") return "MuseScore (MSCZ)";
  if (fileType === "sheet_music_mscx") return "MuseScore (MSCX)";
  if (fileType === "sheet_music_pdf" || mimeType === "application/pdf") return "PDF 乐谱";
  if (mimeType === "audio/midi") return "MIDI";
  if (mimeType?.includes("musicxml")) return "MusicXML";
  return "乐谱文件";
}

function getTypeIcon(fileType?: string) {
  if (fileType === "sheet_music_midi" || fileType === "sheet_music_xml") return Music;
  if (fileType === "sheet_music_mscz" || fileType === "sheet_music_mscx") return FileMusic;
  return FileText;
}

export function SheetMusicViewer({ attachments, allowCopy, className }: SheetMusicViewerProps) {
  if (!attachments || attachments.length === 0) {
    return (
      <div className={cn("flex items-center justify-center rounded-md border border-border bg-muted/20 p-8", className)}>
        <p className="text-sm text-muted-foreground">暂无乐谱文件</p>
      </div>
    );
  }

  return (
    <div className={cn("space-y-3", className)}>
      <div className="rounded-md border border-border bg-card p-4 shadow-none">
        <h3 className="mb-3 text-sm font-semibold">乐谱附件</h3>
        <ul className="divide-y divide-border">
          {attachments.map((att) => {
            const Icon = getTypeIcon(att.file_type);
            return (
              <li key={att.id} className="flex items-center justify-between py-3 first:pt-0 last:pb-0">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-md border border-border bg-muted/30">
                    <Icon className="h-5 w-5 text-muted-foreground" />
                  </div>
                  <div>
                    <p className="text-sm font-medium">
                      {formatType(att.mime_type, att.file_type)}
                    </p>
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
                      下载
                    </Button>
                  </a>
                )}
              </li>
            );
          })}
        </ul>
      </div>

      <div className="rounded-md border border-border bg-canvas-subtle p-4 shadow-none">
        <div className="flex items-center gap-2">
          <FileMusic className="h-5 w-5 text-muted-foreground" />
          <div>
            <p className="text-sm font-medium">乐谱预览</p>
            <p className="text-xs text-muted-foreground">
              下载文件后用 MuseScore 或其他乐谱软件打开查看完整乐谱
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
