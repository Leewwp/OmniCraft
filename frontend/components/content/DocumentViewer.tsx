"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { AlertTriangle, Download, FileText, Loader2 } from "lucide-react";
import DOMPurify from "dompurify";

import { DownloadButton } from "@/components/content/DownloadButton";

/**
 * DocumentViewer（#688）：docx = mammoth → 严格 sanitizer → 渲染；
 * xlsx = exceljs 只读网格 + 多 sheet 页签；csv = 文本解析同网格。
 * 预算合同（v2.2）：document_preview_max_mb（压缩体积）+ 解析规模上限
 * （xlsx 行×列、docx HTML 规模）——虚拟滚动不解决解析期内存，超限优雅
 * 降级为「文件较大，暂不提供在线预览」+ 下载入口。
 */

const DEFAULT_MAX_PREVIEW_MB = 10;
/** xlsx 解析规模上限（行列积）：超出降级，不做全量解析。 */
const MAX_SHEET_CELLS = 200_000;
/** docx 转出 HTML 规模上限。 */
const MAX_DOCX_HTML_LENGTH = 2_000_000;
/** 网格首屏行数（大表按页加载，不整表渲染）。 */
const GRID_PAGE_ROWS = 100;

type ViewerState =
  | { kind: "loading" }
  | { kind: "degraded"; reason: "budget" | "parseScale" }
  | { kind: "error" }
  | { kind: "docx"; html: string }
  | { kind: "grid"; sheets: { name: string; rows: string[][] }[] };

function fileExtension(name: string): string {
  const dot = name.lastIndexOf(".");
  return dot >= 0 ? name.slice(dot).toLowerCase() : "";
}

/**
 * 严格 sanitizer（v2 #6）：mammoth 官方明示不 sanitize 输出。这里剥
 * script 与事件 handler、URL 协议白名单（http/https/mailto/#/相对）、
 * 外链资源不加载（img 仅接受内嵌 data:image/*，其余 src 移除）。
 * 导出供注入用例单测。
 */
// 惰性绑定：浏览器构建里默认实例已绑 global window；测试/jsdom 环境经
// createDOMPurify 显式绑定一次。
let boundPurifier: typeof DOMPurify | null = null;
function purifier(): typeof DOMPurify | null {
  if (typeof window === "undefined") return null;
  if (boundPurifier?.isSupported) return boundPurifier;
  if (DOMPurify.isSupported) {
    boundPurifier = DOMPurify;
    return boundPurifier;
  }
  // Node/jsdom：默认导出兼作工厂，显式绑 window。
  const factory = DOMPurify as unknown as ((w: Window) => typeof DOMPurify) | null;
  if (typeof factory === "function") {
    const created = factory(window);
    if (created?.isSupported) {
      boundPurifier = created;
      return created;
    }
  }
  return null;
}

export function sanitizeDocumentHtml(html: string): string {
  const purify = purifier();
  if (!html || !purify) return "";
  return purify.sanitize(html, {
    ALLOWED_TAGS: [
      "p", "br", "strong", "em", "b", "i", "u", "s", "h1", "h2", "h3", "h4", "h5", "h6",
      "ul", "ol", "li", "table", "thead", "tbody", "tr", "td", "th", "a", "img", "blockquote", "span",
    ],
    ALLOWED_ATTR: ["href", "src", "alt", "colspan", "rowspan"],
    ALLOW_DATA_ATTR: false,
    ALLOWED_URI_REGEXP: /^(?:(?:https?:|mailto:)|#|\/)/i,
    FORBID_TAGS: ["script", "style", "iframe", "object", "embed", "link", "meta"],
  });
}

/**
 * sanitize 后的二次收口：img src 仅接受内嵌 data:image/*（外部资源不
 * 加载）；链接加 rel 硬化。导出供注入用例单测。
 */
export function restrictImageSources(html: string): string {
  if (typeof window === "undefined" || !html) return "";
  const doc = new DOMParser().parseFromString(`<div id="root">${html}</div>`, "text/html");
  const root = doc.getElementById("root");
  if (!root) return html;
  for (const img of Array.from(root.querySelectorAll("img"))) {
    const src = img.getAttribute("src") ?? "";
    if (!/^data:image\/(png|jpeg|jpg|webp|gif);base64,/i.test(src)) {
      img.removeAttribute("src");
    }
  }
  for (const anchor of Array.from(root.querySelectorAll("a"))) {
    anchor.setAttribute("rel", "noopener noreferrer nofollow");
    const href = anchor.getAttribute("href") ?? "";
    if (!/^(?:(?:https?:|mailto:)|#|\/)/i.test(href)) {
      anchor.setAttribute("href", "#");
    }
  }
  return root.innerHTML;
}

/** CSV 解析：带引号转义的极简 RFC4180 解析器（导出供单测）。 */
export function parseCsv(text: string): string[][] {
  const rows: string[][] = [];
  let row: string[] = [];
  let field = "";
  let quoted = false;
  for (let i = 0; i < text.length; i += 1) {
    const ch = text[i];
    if (quoted) {
      if (ch === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i += 1;
        } else {
          quoted = false;
        }
      } else {
        field += ch;
      }
      continue;
    }
    if (ch === '"') {
      quoted = true;
    } else if (ch === ",") {
      row.push(field);
      field = "";
    } else if (ch === "\n") {
      row.push(field);
      rows.push(row);
      row = [];
      field = "";
    } else if (ch !== "\r") {
      field += ch;
    }
  }
  if (field.length > 0 || row.length > 0) {
    row.push(field);
    rows.push(row);
  }
  return rows;
}

export interface DocumentViewerProps {
  url: string;
  fileName: string;
  fileSize?: number;
  maxPreviewMB?: number;
  contentId: number;
  attachmentId: number;
  allowCopy: boolean;
}

export function DocumentViewer({
  url,
  fileName,
  fileSize,
  maxPreviewMB,
  contentId,
  attachmentId,
  allowCopy,
}: DocumentViewerProps) {
  const t = useTranslations();
  const [state, setState] = useState<ViewerState>({ kind: "loading" });
  const [activeSheet, setActiveSheet] = useState(0);
  const [visibleRows, setVisibleRows] = useState(GRID_PAGE_ROWS);
  const ext = useMemo(() => fileExtension(fileName), [fileName]);
  const budgetMB = maxPreviewMB && maxPreviewMB > 0 ? maxPreviewMB : DEFAULT_MAX_PREVIEW_MB;

  useEffect(() => {
    let cancelled = false;
    setState({ kind: "loading" });
    setVisibleRows(GRID_PAGE_ROWS);
    setActiveSheet(0);

    if (fileSize != null && fileSize > budgetMB * 1024 * 1024) {
      setState({ kind: "degraded", reason: "budget" });
      return () => {
        cancelled = true;
      };
    }

    (async () => {
      try {
        const res = await fetch(url);
        if (!res.ok) throw new Error(`fetch failed: ${res.status}`);
        const buffer = await res.arrayBuffer();
        if (cancelled) return;

        if (ext === ".docx") {
          const mammoth = await import("mammoth");
          const result = await mammoth.convertToHtml({ arrayBuffer: buffer });
          if (cancelled) return;
          if (result.value.length > MAX_DOCX_HTML_LENGTH) {
            setState({ kind: "degraded", reason: "parseScale" });
            return;
          }
          const sanitized = restrictImageSources(sanitizeDocumentHtml(result.value));
          setState({ kind: "docx", html: sanitized });
          return;
        }

        if (ext === ".xlsx") {
          const ExcelJS = await import("exceljs");
          const workbook = new ExcelJS.Workbook();
          await workbook.xlsx.load(buffer);
          if (cancelled) return;
          const sheets: { name: string; rows: string[][] }[] = [];
          workbook.eachSheet((worksheet) => {
            if (worksheet.rowCount * worksheet.columnCount > MAX_SHEET_CELLS) {
              throw new Error("sheet too large");
            }
            const rows: string[][] = [];
            worksheet.eachRow({ includeEmpty: false }, (row) => {
              const cells: string[] = [];
              row.eachCell({ includeEmpty: true }, (cell) => {
                const value = cell.value;
                if (value == null) cells.push("");
                else if (typeof value === "object" && "result" in (value as object)) cells.push(String((value as { result?: unknown }).result ?? ""));
                else if (typeof value === "object" && "richText" in (value as object)) {
                  cells.push(((value as { richText: { text: string }[] }).richText ?? []).map((part) => part.text).join(""));
                } else cells.push(String(value));
              });
              rows.push(cells);
            });
            sheets.push({ name: worksheet.name, rows });
          });
          setState({ kind: "grid", sheets });
          return;
        }

        if (ext === ".csv") {
          const text = new TextDecoder("utf-8").decode(buffer);
          const rows = parseCsv(text);
          if (rows.length * 40 > MAX_SHEET_CELLS) {
            setState({ kind: "degraded", reason: "parseScale" });
            return;
          }
          setState({ kind: "grid", sheets: [{ name: fileName, rows }] });
          return;
        }

        setState({ kind: "error" });
      } catch {
        if (!cancelled) setState({ kind: "error" });
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [url, ext, fileSize, budgetMB, fileName]);

  const downloadEntry = allowCopy ? (
    <DownloadButton contentId={contentId} attachmentId={attachmentId} contentType="document" size="sm" />
  ) : null;

  if (state.kind === "loading") {
    return (
      <div className="flex items-center gap-2 rounded-lg border border-border bg-card p-4 text-sm text-muted-foreground" data-testid="document-viewer-loading" role="status">
        <Loader2 className="h-4 w-4 animate-spin" />
        {t("content.attachmentPreview.loading")}
      </div>
    );
  }

  if (state.kind === "degraded") {
    return (
      <div className="flex items-center justify-between gap-3 rounded-lg border border-border bg-card p-4" data-testid="document-viewer-degraded">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <FileText className="h-4 w-4 shrink-0" />
          {t("content.attachmentPreview.tooLarge")}
        </div>
        {downloadEntry}
      </div>
    );
  }

  if (state.kind === "error") {
    return (
      <div className="flex items-center justify-between gap-3 rounded-lg border border-border bg-card p-4" data-testid="document-viewer-error">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <AlertTriangle className="h-4 w-4 shrink-0 text-destructive" />
          {t("content.attachmentPreview.failed")}
        </div>
        {downloadEntry}
      </div>
    );
  }

  if (state.kind === "docx") {
    return (
      <div className="space-y-2" data-testid="document-viewer-docx">
        <div
          className="prose prose-sm max-w-none rounded-lg border border-border bg-card p-4 text-sm leading-relaxed [&_img]:max-w-full"
          // 内容已经过严格 sanitizer（标签/属性白名单 + 协议白名单 +
          // data:image 内嵌图限定）才进入此处。
          dangerouslySetInnerHTML={{ __html: state.html }}
        />
        {downloadEntry && <div className="flex justify-end">{downloadEntry}</div>}
      </div>
    );
  }

  const sheet = state.sheets[activeSheet] ?? state.sheets[0];
  if (!sheet) {
    return (
      <div className="flex items-center justify-between gap-3 rounded-lg border border-border bg-card p-4" data-testid="document-viewer-error">
        <span className="text-sm text-muted-foreground">{t("content.attachmentPreview.failed")}</span>
        {downloadEntry}
      </div>
    );
  }
  const rows = sheet.rows.slice(0, visibleRows);
  const hasMore = sheet.rows.length > visibleRows;

  return (
    <div className="space-y-2" data-testid="document-viewer-grid">
      {state.sheets.length > 1 && (
        <div className="flex flex-wrap gap-1" role="tablist" aria-label={t("content.attachmentPreview.sheets")}>
          {state.sheets.map((item, index) => (
            <button
              key={`${item.name}-${index}`}
              type="button"
              role="tab"
              aria-selected={index === activeSheet}
              className={`rounded-md px-2.5 py-1 text-xs ${index === activeSheet ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground"}`}
              onClick={() => {
                setActiveSheet(index);
                setVisibleRows(GRID_PAGE_ROWS);
              }}
            >
              {item.name}
            </button>
          ))}
        </div>
      )}
      <div className="overflow-x-auto rounded-lg border border-border bg-card">
        <table className="w-full text-left text-xs">
          <tbody>
            {rows.map((row, rowIndex) => (
              <tr key={rowIndex} className="border-b border-border/60 last:border-b-0">
                {row.map((cell, cellIndex) => (
                  <td key={cellIndex} className="whitespace-nowrap px-2.5 py-1.5 text-foreground/90">
                    {cell}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {hasMore && (
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-2 py-1 text-xs text-muted-foreground hover:text-foreground"
          onClick={() => setVisibleRows((n) => n + GRID_PAGE_ROWS)}
        >
          <Download className="h-3 w-3 rotate-180" />
          {t("content.attachmentPreview.moreRows", { shown: visibleRows, total: sheet.rows.length })}
        </button>
      )}
      {downloadEntry && <div className="flex justify-end">{downloadEntry}</div>}
    </div>
  );
}
