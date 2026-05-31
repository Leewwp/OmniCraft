"use client";

import { useState, useRef } from "react";
import { useTranslations } from "next-intl";
import { BookOpen, ChevronDown, ChevronUp, Loader2, RotateCcw, HelpCircle, Flag } from "lucide-react";
import { useSSE } from "@/lib/useSSE";
import { MarkdownRenderer } from "@/components/content/MarkdownRenderer";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import Link from "next/link";

interface UsageGuidePanelProps {
  contentId: number;
  className?: string;
}

export function UsageGuidePanel({ contentId, className }: UsageGuidePanelProps) {
  const t = useTranslations();
  const [expanded, setExpanded] = useState(false);
  const [content, setContent] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState("");
  const contentRef = useRef("");

  const { streaming, start, stop } = useSSE({
    onMessage: (delta) => {
      contentRef.current += delta;
      setContent(contentRef.current);
    },
    onClose: () => {
      setLoaded(true);
      setError("");
    },
    onError: () => {
      setError(t("agent.guideError"));
    },
  });

  function toggle() {
    if (!expanded && !loaded) {
      fetchGuide();
    }
    setExpanded(!expanded);
  }

  function fetchGuide() {
    setError("");
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
    start(`${apiUrl}/api/v1/agent/usage-guide/${contentId}?stream=true`);
  }

  function handleRetry() {
    contentRef.current = "";
    setContent("");
    setLoaded(false);
    fetchGuide();
  }

  return (
    <div className={cn("rounded-md border border-border bg-card", className)}>
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
            <div className="prose prose-sm max-w-none dark:prose-invert">
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
