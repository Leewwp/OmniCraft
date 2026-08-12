"use client";

import { useEffect, useId, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Loader2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";

export interface CollabUser {
  id: number;
  username: string;
  avatarUrl?: string;
}

interface CollabUserPickerProps {
  selectedUsers: Array<{ id: number; username: string; avatarUrl?: string }>;
  maxSelected: number;
  disabled?: boolean;
  onChange: (users: Array<{ id: number; username: string; avatarUrl?: string }>) => void;
}

interface SearchResultItem {
  id?: unknown;
  username?: unknown;
  avatar_url?: unknown;
}

const SEARCH_DEBOUNCE_MS = 300;
const SEARCH_LIMIT = 8;

function isSelectableUser(item: SearchResultItem): item is { id: number; username: string; avatar_url?: string } {
  if (typeof item.id !== "number" || item.id <= 0) return false;
  if (typeof item.username !== "string" || item.username.trim() === "") return false;
  return true;
}

export function CollabUserPicker({ selectedUsers, maxSelected, disabled = false, onChange }: CollabUserPickerProps) {
  const t = useTranslations("collabUserPicker");
  const inputId = useId();
  const listboxId = `${inputId}-listbox`;
  const inputRef = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<CollabUser[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchFailed, setSearchFailed] = useState(false);
  const [highlighted, setHighlighted] = useState<number | null>(null);
  const [attempt, setAttempt] = useState(0);

  const selectionUnavailable = maxSelected <= 0;
  const capReached = maxSelected > 0 && selectedUsers.length >= maxSelected;
  const searchBlocked = disabled || selectionUnavailable || capReached;

  useEffect(() => {
    const trimmed = query.trim();
    if (!trimmed || searchBlocked) {
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
      const params = new URLSearchParams({ q: trimmed, limit: String(SEARCH_LIMIT) });
      api
        .get<{ users?: SearchResultItem[] }>(`/api/v1/users/search?${params.toString()}`)
        .then((data) => {
          if (cancelled) return;
          setResults(
            (data.users ?? [])
              .filter(isSelectableUser)
              .map((item) => ({
                id: item.id,
                username: item.username,
                ...(item.avatar_url ? { avatarUrl: item.avatar_url } : {}),
              })),
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
  }, [query, searchBlocked, attempt]);

  function handleSelect(user: CollabUser) {
    const alreadySelected = selectedUsers.some((selected) => selected.id === user.id);
    if (alreadySelected || capReached || selectionUnavailable) return;
    onChange([...selectedUsers, user]);
    setQuery("");
    setResults([]);
    setHighlighted(null);
    inputRef.current?.focus();
  }

  function handleRemove(user: CollabUser) {
    if (disabled) return;
    onChange(selectedUsers.filter((selected) => selected.id !== user.id));
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
  const showEmpty = !loading && !searchFailed && query.trim() !== "" && results.length === 0 && !searchBlocked;

  return (
    <div className="space-y-2">
      <label htmlFor={inputId} className="mb-1.5 block text-sm font-medium text-foreground">
        {t("label")}
      </label>
      {selectedUsers.length > 0 && (
        <ul className="flex flex-wrap gap-2" aria-label={t("a11y.selectedList")}>
          {selectedUsers.map((user) => (
            <li
              key={user.id}
              className="flex items-center gap-1.5 rounded-full border border-border bg-muted/40 py-1 pl-1 pr-1 text-sm text-foreground"
            >
              {user.avatarUrl ? (
                <img src={user.avatarUrl} alt="" className="h-6 w-6 rounded-full object-cover" />
              ) : (
                <span aria-hidden="true" className="flex h-6 w-6 items-center justify-center rounded-full bg-muted text-xs text-muted-foreground">
                  {user.username.slice(0, 1).toUpperCase()}
                </span>
              )}
              <span className="min-w-0 max-w-[12rem] truncate">{user.username}</span>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                onClick={() => handleRemove(user)}
                disabled={disabled}
                aria-label={t("a11y.removeUser", { username: user.username })}
                className="shrink-0 rounded-full"
              >
                <X className="h-3.5 w-3.5" />
              </Button>
            </li>
          ))}
        </ul>
      )}
      {capReached && (
        <p className="text-xs text-muted-foreground" role="status">
          {t("selected.maxReached", { max: maxSelected })}
        </p>
      )}
      {selectionUnavailable && (
        <p className="text-xs text-muted-foreground" role="status">
          {t("disabled.unavailable")}
        </p>
      )}
      <Input
        ref={inputRef}
        id={inputId}
        role="combobox"
        aria-expanded={showResults}
        aria-controls={listboxId}
        aria-activedescendant={highlighted !== null && showResults ? `${listboxId}-option-${highlighted}` : undefined}
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        onKeyDown={handleKeyDown}
        disabled={searchBlocked}
        placeholder={t("search.placeholder")}
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
          aria-label={t("a11y.results")}
          className="max-h-56 overflow-y-auto rounded-md border border-border bg-card py-1"
        >
          {results.map((result, index) => {
            const alreadySelected = selectedUsers.some((selected) => selected.id === result.id);
            const rowDisabled = alreadySelected || capReached;
            return (
              <li
                key={result.id}
                id={`${listboxId}-option-${index}`}
                role="option"
                aria-selected={alreadySelected}
                aria-disabled={rowDisabled}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => {
                  if (!rowDisabled) handleSelect(result);
                }}
                onMouseEnter={() => setHighlighted(index)}
                className={cn(
                  "flex cursor-pointer items-center gap-2 px-3 py-2 text-sm",
                  highlighted === index && "bg-muted",
                  rowDisabled && "cursor-not-allowed opacity-60",
                )}
              >
                {result.avatarUrl ? (
                  <img src={result.avatarUrl} alt="" className="h-6 w-6 shrink-0 rounded-full object-cover" />
                ) : (
                  <span aria-hidden="true" className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-muted text-xs text-muted-foreground">
                    {result.username.slice(0, 1).toUpperCase()}
                  </span>
                )}
                <span className="min-w-0 flex-1 truncate font-medium text-foreground">{result.username}</span>
                {alreadySelected && (
                  <span className="shrink-0 text-xs text-muted-foreground">{t("duplicate.alreadySelected")}</span>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
