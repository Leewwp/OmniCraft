"use client";

import { useState, useRef } from "react";
import { useTranslations } from "next-intl";
import { Search, Loader2, Sparkles, ArrowLeft } from "lucide-react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface SearchResult {
  content_item_id: number;
  title: string;
  score?: number;
  summary?: string;
}

interface SearchAgentInputProps {
  onResults?: (results: SearchResult[], query: string) => void;
  className?: string;
}

export function SearchAgentInput({ onResults, className }: SearchAgentInputProps) {
  const t = useTranslations();
  const [mode, setMode] = useState<"keyword" | "agent">("agent");
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const abortRef = useRef<AbortController | null>(null);

  async function handleSearch() {
    const trimmed = query.trim();
    if (!trimmed) return;

    setLoading(true);
    setError("");
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    try {
      if (mode === "agent") {
        const res = await api.post<{ results?: SearchResult[] }>("/api/v1/agent/search", {
          query: trimmed,
        });
        if (!controller.signal.aborted) {
          onResults?.(res.results ?? [], trimmed);
        }
      } else {
        // Keyword search fallback — redirect or pass to parent
        onResults?.([], trimmed);
      }
    } catch (e) {
      if (!controller.signal.aborted) {
        setError((e as Error).message || t("common.operationFailed"));
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
          <ArrowLeft className="h-3 w-3" />
          {t("agent.keywordSearch")}
        </button>
      </div>

      {error && <p className="text-xs text-destructive">{error}</p>}
    </div>
  );
}
