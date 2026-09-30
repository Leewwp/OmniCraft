"use client";

import { useState, useRef, useEffect } from "react";
import { useTranslations, useLocale } from "next-intl";
import { BookOpen, ChevronDown, ChevronUp, Loader2, RotateCcw, HelpCircle, Flag } from "lucide-react";
import { streamUsageGuide } from "@/lib/usage-guide-stream";
import { MarkdownRenderer } from "@/components/content/MarkdownRenderer";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import Link from "next/link";

interface UsageGuidePanelProps {
  contentId: number;
  className?: string;
}

/**
 * #723 站内面板适用范围（导出供单测）：mod / template / 3d_print /
 * sheet_music 直接展示；其余品类仅当带注册表 document 族（.docx/.xlsx/
 * .csv）附件时展示——text 族说明文件不触发；article 始终隐藏（含历史
 * 异常数据带 document 附件）。feature gate 与 published 条件由调用方
 * 叠加（优先级：feature → published → 本判定）。
 */
export function usageGuidePanelTarget(contentType: string | undefined, attachments: { file_type?: string }[] | undefined): boolean {
  if (contentType === "article") return false;
  if (contentType === "mod" || contentType === "template" || contentType === "3d_print" || contentType === "sheet_music") return true;
  return (attachments ?? []).some((att) => att.file_type === "document");
}

export function UsageGuidePanel({ contentId, className }: UsageGuidePanelProps) {
  const t = useTranslations();
  const locale = useLocale();
  const [expanded, setExpanded] = useState(false);
  const [content, setContent] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState("");
  const contentRef = useRef("");
  const abortRef = useRef<AbortController | null>(null);
  const localeRef = useRef(locale);

  // #723：站点语言切换后旧语言的已载内容不得残留——重置为未载态，
  // 下次展开按新 locale 重新请求；进行中的流中止。
  useEffect(() => {
    if (localeRef.current === locale) return;
    localeRef.current = locale;
    abortRef.current?.abort();
    contentRef.current = "";
    setContent("");
    setLoaded(false);
    setError("");
  }, [locale]);

  // SP-25 中-9：改 GET 流式（后端路由为 GET-only，旧 useSSE 硬编码 POST →
  // 自上线即 404 死链）；行缓冲与双形态（JSON/SSE）收口在 usage-guide-stream。
  function toggle() {
    if (!expanded && !loaded && !streaming) {
      fetchGuide();
    }
    setExpanded(!expanded);
  }

  function fetchGuide() {
    setError("");
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setStreaming(true);
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
    void streamUsageGuide(
      fetch,
      `${apiUrl}/api/v1/agent/usage-guide/${contentId}?stream=true&locale=${locale}`,
      {
        onDelta: (delta) => {
          contentRef.current += delta;
          setContent(contentRef.current);
        },
        onDone: () => {
          setLoaded(true);
          setError("");
        },
        onError: () => {
          setError(t("agent.guideError"));
        },
        onClose: () => {
          setStreaming(false);
        },
      },
      controller.signal,
    );
  }

  function handleRetry() {
    contentRef.current = "";
    setContent("");
    setLoaded(false);
    fetchGuide();
  }

  return (
    <div className={cn("min-w-0 overflow-hidden rounded-md border border-border bg-card", className)}>
      <button
        type="button"
        className="flex w-full items-center justify-between px-4 py-3 text-left"
        onClick={toggle}
      >
        <div className="flex items-center gap-2">
          <BookOpen className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm font-medium">{t("agent.usageGuideTitle")}</span>
        </div>
        {expanded ? (
          <ChevronUp className="h-4 w-4 text-muted-foreground" />
        ) : (
          <ChevronDown className="h-4 w-4 text-muted-foreground" />
        )}
      </button>

      {expanded && (
        <div className="border-t border-border px-4 py-3">
          {streaming && !content && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              {t("agent.loadingGuide")}
            </div>
          )}
          {content && (
            <div className="prose prose-sm min-w-0 max-w-none break-words dark:prose-invert">
              <MarkdownRenderer content={content} />
            </div>
          )}
          {loaded && !content && !error && (
            <p className="text-sm text-muted-foreground">{t("agent.noGuideAvailable")}</p>
          )}
          {error && (
            <div className="space-y-2">
              <p className="text-xs text-destructive">{error}</p>
              <div className="flex items-center gap-2">
                <Button variant="outline" size="sm" className="h-6 text-xs" onClick={handleRetry}>
                  <RotateCcw className="mr-1 h-3 w-3" />
                  {t("agent.retry")}
                </Button>
                <Link href="/help" className="inline-flex items-center text-xs text-muted-foreground hover:text-foreground">
                  <HelpCircle className="mr-1 h-3 w-3" />
                  {t("agent.helpLink")}
                </Link>
                <Link href="/feedback" className="inline-flex items-center text-xs text-muted-foreground hover:text-foreground">
                  <Flag className="mr-1 h-3 w-3" />
                  {t("agent.feedbackLink")}
                </Link>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
