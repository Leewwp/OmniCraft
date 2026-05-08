"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Sparkles, Loader2, Check } from "lucide-react";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface UploadAssistResult {
  suggested_title?: string;
  suggested_description?: string;
  suggested_tags?: string[];
  suggested_category?: string;
}

interface AgentUploadAssistPanelProps {
  uploadedFiles: string[];
  contentId?: number;
  title?: string;
  description?: string;
  contentType?: string;
  onFill?: (data: UploadAssistResult) => void;
  className?: string;
}

export function AgentUploadAssistPanel({
  uploadedFiles,
  contentId,
  title,
  description,
  contentType,
  onFill,
  className,
}: AgentUploadAssistPanelProps) {
  const t = useTranslations();
  const [result, setResult] = useState<UploadAssistResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  async function handleAnalyze() {
    if (uploadedFiles.length === 0) return;
    setLoading(true);
    setError("");
    try {
      const data = await api.post<UploadAssistResult>(
        "/api/v1/agent/upload-assist",
        {
          title: title || "",
          description: description || "",
          filename: uploadedFiles[0] || "",
          content_type: contentType || "other",
        },
      );
      setResult(data);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t("common.operationFailed"));
    } finally {
      setLoading(false);
    }
  }

  function handleFill() {
    if (result && onFill) onFill(result);
  }

  return (
    <div className={cn("rounded-md border border-border bg-card p-4 space-y-3", className)}>
      <div className="flex items-center gap-2">
        <Sparkles className="h-4 w-4 text-purple-500" />
        <span className="text-sm font-medium">{t("agent.uploadAssistTitle")}</span>
      </div>

      <p className="text-xs text-muted-foreground">
        {t("agent.uploadAssistDesc")}
      </p>

      {error && <p className="text-xs text-destructive">{error}</p>}

      {result ? (
        <div className="space-y-2">
          {result.suggested_title && (
            <div className="text-xs">
              <span className="text-muted-foreground">{t("agent.suggestedTitle")}:</span>{" "}
              <span className="font-medium">{result.suggested_title}</span>
            </div>
          )}
          {result.suggested_description && (
            <div className="text-xs">
              <span className="text-muted-foreground">{t("agent.suggestedDescription")}:</span>{" "}
              <span className="font-medium line-clamp-2">{result.suggested_description}</span>
            </div>
          )}
          {result.suggested_tags && result.suggested_tags.length > 0 && (
            <div className="flex flex-wrap items-center gap-1">
              <span className="text-xs text-muted-foreground">{t("agent.suggestedTags")}:</span>
              {result.suggested_tags.map((tag) => (
                <span
                  key={tag}
                  className="inline-block rounded bg-purple-100 px-1.5 py-0.5 text-xs font-medium text-purple-700 dark:bg-purple-900/30 dark:text-purple-400"
                >
                  {tag}
                </span>
              ))}
            </div>
          )}
          <Button size="sm" variant="outline" onClick={handleFill}>
            <Check className="mr-1 h-3.5 w-3.5" />
            {t("agent.autoFill")}
          </Button>
        </div>
      ) : (
        <Button size="sm" variant="outline" onClick={handleAnalyze} disabled={loading}>
          {loading ? (
            <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" />
          ) : (
            <Sparkles className="mr-1 h-3.5 w-3.5" />
          )}
          {t("agent.analyze")}
        </Button>
      )}
    </div>
  );
}
