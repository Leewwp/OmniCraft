"use client";

import { useEffect, useId, useState } from "react";
import { useTranslations } from "next-intl";
import { Loader2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";

export type SourceContentZone = "original" | "fanwork";

export interface SourceContent {
  id: number;
  title: string;
  zone: SourceContentZone;
}

interface SourceContentPickerProps {
  sourceKind: SourceContentZone;
  selected?: SourceContent;
  disabled?: boolean;
  onSelect: (content?: SourceContent) => void;
}

interface SearchResultItem {
  id?: unknown;
  title?: unknown;
  zone?: unknown;
  status?: unknown;
  deleted_at?: unknown;
  author?: { deleted_at?: unknown; is_banned?: unknown };
}

const SEARCH_DEBOUNCE_MS = 300;
const SEARCH_LIMIT = 8;

function isSelectableSource(item: SearchResultItem, sourceKind: SourceContentZone): item is SourceContent {
  if (typeof item.id !== "number" || item.id <= 0) return false;
  if (typeof item.title !== "string" || item.title.trim() === "") return false;
  if (item.zone !== sourceKind) return false;
  if (item.deleted_at) return false;
  if (item.status !== undefined && item.status !== null && item.status !== "published") return false;
  if (item.author && (item.author.deleted_at || item.author.is_banned)) return false;
  return true;
}

export function SourceContentPicker({
  sourceKind,
  selected,
  disabled = false,
  onSelect,
}: SourceContentPickerProps) {
  const t = useTranslations("sourceContentPicker");
  const inputId = useId();
  const listboxId = `${inputId}-listbox`;
  const [query, setQuery] = useState(selected?.title ?? "");
  const [results, setResults] = useState<SourceContent[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchFailed, setSearchFailed] = useState(false);
  const [highlighted, setHighlighted] = useState<number | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    setQuery(selected?.title ?? "");
  }, [selected]);

  useEffect(() => {
    const trimmed = query.trim();
    if (!trimmed || trimmed === selected?.title) {
      setResults([]);
      setLoading(false);
      setSearchFailed(false);
      setHighlighted(null);
      return;
    }

    setLoading(true);
    setSearchFailed(false);
    const timer = setTimeout(() => {
      let cancelled = false;
      const params = new URLSearchParams({ zone: sourceKind, q: trimmed, limit: String(SEARCH_LIMIT) });
      api
        .get<{ items?: SearchResultItem[] }>(`/api/v1/contents/search?${params.toString()}`)
        .then((data) => {
          if (cancelled) return;
          setResults(
            (data.items ?? [])
              .filter((item) => isSelectableSource(item, sourceKind))
              .map((item) => ({ id: item.id, title: item.title, zone: item.zone })),
          );
          setHighlighted(null);
        })
        .catch(() => {
          if (cancelled) return;
          setResults([]);
          setSearchFailed(true);
          setHighlighted(null);
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
      return () => {
        cancelled = true;
      };
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [query, sourceKind, selected?.title, attempt]);

  function handleQueryChange(next: string) {
    setQuery(next);
    if (selected && next.trim() !== selected.title) {
      onSelect(undefined);
    }
  }

  function handleSelect(content: SourceContent) {
    setQuery(content.title);
    setResults([]);
    setHighlighted(null);
    onSelect(content);
  }

  function handleKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (results.length === 0) return;
      setHighlighted((current) => {
        if (event.key === "ArrowDown") {
          return current === null ? 0 : (current + 1) % results.length;
        }
        return current === null ? results.length - 1 : (current - 1 + results.length) % results.length;
      });
    } else if (event.key === "Enter") {
      if (highlighted !== null && results[highlighted]) {
        event.preventDefault();
        handleSelect(results[highlighted]);
      }
    } else if (event.key === "Escape") {
      setResults([]);
      setHighlighted(null);
    }
  }

  const showResults = !loading && !searchFailed && results.length > 0;
  const showEmpty = !loading && !searchFailed && query.trim() !== "" && query.trim() !== selected?.title && results.length === 0;

  return (
    <div className="space-y-2">
      <label htmlFor={inputId} className="mb-1.5 block text-sm font-medium text-foreground">
        {t(`${sourceKind}.label`)}
      </label>
      <Input
        id={inputId}
        role="combobox"
        aria-expanded={showResults}
        aria-controls={listboxId}
        aria-activedescendant={highlighted !== null && showResults ? `${listboxId}-option-${highlighted}` : undefined}
        value={query}
        onChange={(event) => handleQueryChange(event.target.value)}
        onKeyDown={handleKeyDown}
        disabled={disabled}
        placeholder={t(`${sourceKind}.placeholder`)}
      />
      {loading && (
        <div role="status" className="flex items-center gap-2 text-xs text-muted-foreground">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          {t("search.loading")}
        </div>
      )}
      {!loading && searchFailed && (
        <div role="alert" className="flex items-center gap-2 text-xs text-destructive">
          <span>{t("error.searchFailed")}</span>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setAttempt((next) => next + 1)}
          >
            {t("error.retry")}
          </Button>
        </div>
      )}
      {showEmpty && (
        <div role="status" className="text-xs text-muted-foreground">
          {t("search.empty")}
        </div>
      )}
      {showResults && (
        <ul
          id={listboxId}
          role="listbox"
          aria-label={t(`a11y.results${sourceKind === "original" ? "Original" : "Fanwork"}`)}
          className="max-h-56 overflow-y-auto rounded-md border border-border bg-card py-1"
        >
          {results.map((result, index) => (
            <li
              key={result.id}
              id={`${listboxId}-option-${index}`}
              role="option"
              aria-selected={selected?.id === result.id}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => handleSelect(result)}
              onMouseEnter={() => setHighlighted(index)}
              className={cn(
                "cursor-pointer px-3 py-2 text-sm",
                highlighted === index && "bg-muted",
              )}
            >
              <span className="font-medium text-foreground">{result.title}</span>
              <span className="ml-2 text-xs text-muted-foreground">{t(`${result.zone}.label`)}</span>
            </li>
          ))}
        </ul>
      )}
      {selected && (
        <div className="flex items-center justify-between gap-2 rounded-md border border-border bg-muted/40 px-3 py-2">
          <div className="min-w-0">
            <p className="text-xs text-muted-foreground">{t("selected.label")}</p>
            <p className="truncate text-sm font-medium text-foreground">{selected.title}</p>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onSelect(undefined)}
            disabled={disabled}
            aria-label={t("selected.clear")}
            className="shrink-0"
          >
            <X className="h-4 w-4" />
          </Button>
        </div>
      )}
    </div>
  );
}
