"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";

export interface SourceOriginalPickerValue {
  id: number;
  title: string;
}

interface SourceOriginalPickerProps {
  value: SourceOriginalPickerValue | null;
  onChange: (value: SourceOriginalPickerValue | null) => void;
  placeholder: string;
  searchLabel: string;
  loadingLabel: string;
}

export function SourceOriginalPicker({
  value,
  onChange,
  placeholder,
  searchLabel,
  loadingLabel,
}: SourceOriginalPickerProps) {
  const [query, setQuery] = useState(value?.title ?? "");
  const [options, setOptions] = useState<SourceOriginalPickerValue[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (value) {
      setQuery(value.title);
    }
  }, [value]);

  useEffect(() => {
    const trimmed = query.trim();
    if (!trimmed || (value && trimmed === value.title)) {
      setOptions([]);
      setLoading(false);
      return;
    }

    let cancelled = false;
    const params = new URLSearchParams({
      zone: "original",
      q: trimmed,
      limit: "8",
    });

    setLoading(true);
    api
      .get<{ items?: SourceOriginalPickerValue[]; contents?: SourceOriginalPickerValue[] }>(
        `/api/v1/contents/search?${params.toString()}`,
      )
      .then((data) => {
        if (!cancelled) {
          setOptions(data.items ?? data.contents ?? []);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setOptions([]);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [query, value]);

  function handleQueryChange(next: string) {
    setQuery(next);
    if (value && next !== value.title) {
      onChange(null);
    }
  }

  function handleSelect(option: SourceOriginalPickerValue) {
    onChange(option);
    setQuery(option.title);
    setOptions([]);
  }

  return (
    <div className="space-y-2">
      <Input
        aria-label={searchLabel}
        value={query}
        onChange={(event) => handleQueryChange(event.target.value)}
        placeholder={placeholder}
      />
      {loading && <p className="text-xs text-muted-foreground">{loadingLabel}</p>}
      {options.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {options.map((option) => (
            <Button
              key={option.id}
              type="button"
              variant="outline"
              size="sm"
              onClick={() => handleSelect(option)}
            >
              {option.title}
            </Button>
          ))}
        </div>
      )}
    </div>
  );
}
