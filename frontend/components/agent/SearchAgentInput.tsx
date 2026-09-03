"use client";

import { useState, useRef, useCallback } from "react";
import { useTranslations } from "next-intl";
import { Search, Loader2, Sparkles } from "lucide-react";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface SearchAgentInputProps {
  onResults?: (results: Record<string, unknown>[], query: string) => void;
  onKeywordFallback?: (query: string) => void;
  className?: string;
}

export function SearchAgentInput({ onResults, onKeywordFallback, className }: SearchAgentInputProps) {
  const t = useTranslations();
  const [mode, setMode] = useState<"keyword" | "agent">("keyword");
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [downgradeNotice, setDowngradeNotice] = useState("");
  const abortRef = useRef<AbortController | null>(null);

  const doKeywordSearch = useCallback((q: string) => {
    onResults?.([], q);
    onKeywordFallback?.(q);
  }, [onResults, onKeywordFallback]);

  async function handleSearch() {
    const trimmed = query.trim();
    if (!trimmed) return;

    setLoading(true);
    setError("");
    setDowngradeNotice("");
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    try {
      if (mode === "agent") {
        const res = await api.post<{ results?: Record<string, unknown>[] }>("/api/v1/agent/search", {
          query: trimmed,
        });
        if (!controller.signal.aborted) {
          const results = res.results ?? [];
          if (results.length > 0) {
            onResults?.(results, trimmed);
          } else {
            setDowngradeNotice(t("agent.searchAgentNoResults"));
            doKeywordSearch(trimmed);
          }
        }
      } else {
        doKeywordSearch(trimmed);
      }
    } catch (e) {
      if (!controller.signal.aborted) {
        if (e instanceof ApiRequestError) {
          const downgradeCodes = ["UNAUTHORIZED", "FORBIDDEN", "TOKEN_EXPIRED", "FEATURE_DISABLED", "AGENT_ERROR"];
          if (downgradeCodes.includes(e.code) || e.status === 429 || e.status >= 500) {
            setDowngradeNotice(t("agent.searchAgentDowngrade"));
            doKeywordSearch(trimmed);
          } else {
            setError(e.message);
          }
        } else {
          setDowngradeNotice(t("agent.searchAgentDowngrade"));
          doKeywordSearch(trimmed);
        }
        silentError(e, { component: 'SearchAgentInput', action: 'handleSearch' });
      }
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  }

  return (
    <div className={cn("space-y-2", className)}>
      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleSearch();
            }}
            placeholder={
              mode === "agent"
                ? t("agent.searchAgentPlaceholder")
                : t("agent.searchKeywordPlaceholder")
            }
            className="w-full rounded-md border border-border bg-background py-2.5 pl-10 pr-4 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
          />
          {mode === "agent" && (
            <Sparkles className="absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-purple-400" />
          )}
        </div>
        <Button size="sm" onClick={handleSearch} disabled={loading || !query.trim()}>
          {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
        </Button>
      </div>

      <div className="flex items-center gap-2">
        <button
          type="button"
          className={cn(
            "inline-flex items-center gap-1 rounded px-2 py-1 text-xs",
            mode === "agent"
              ? "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400"
              : "text-muted-foreground hover:text-foreground",
          )}
          onClick={() => setMode("agent")}
        >
          <Sparkles className="h-3 w-3" />
          {t("agent.aiSearch")}
        </button>
        <span className="text-xs text-muted-foreground">/</span>
        <button
          type="button"
          className={cn(
            "inline-flex items-center gap-1 rounded px-2 py-1 text-xs",
            mode === "keyword" && "bg-muted text-foreground",
            mode !== "keyword" && "text-muted-foreground hover:text-foreground",
          )}
          onClick={() => setMode("keyword")}
        >
          <Search className="h-3 w-3" />
          {t("agent.keywordSearch")}
        </button>
      </div>

      {downgradeNotice && (
        <p className="text-xs text-amber-600 dark:text-amber-400">{downgradeNotice}</p>
      )}
      {error && <p className="text-xs text-destructive">{error}</p>}
    </div>
  );
}
