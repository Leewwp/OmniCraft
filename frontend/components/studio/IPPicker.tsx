"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";

export interface IPPickerValue {
  id: number;
  name: string;
}

interface IPPickerProps {
  value: IPPickerValue | null;
  onChange: (value: IPPickerValue | null) => void;
  placeholder: string;
  searchLabel: string;
  loadingLabel: string;
}

export function IPPicker({ value, onChange, placeholder, searchLabel, loadingLabel }: IPPickerProps) {
  const [query, setQuery] = useState(value?.name ?? "");
  const [options, setOptions] = useState<IPPickerValue[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (value) {
      setQuery(value.name);
    }
  }, [value]);

  useEffect(() => {
    const trimmed = query.trim();
    if (!trimmed || (value && trimmed === value.name)) {
      setOptions([]);
      setLoading(false);
      return;
    }

    let cancelled = false;
    setLoading(true);
    api
      .get<{ ips?: IPPickerValue[]; items?: IPPickerValue[] }>(`/api/v1/ips?q=${encodeURIComponent(trimmed)}`)
      .then((data) => {
        if (!cancelled) {
          setOptions(data.ips ?? data.items ?? []);
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
    if (value && next !== value.name) {
      onChange(null);
    }
  }

  function handleSelect(option: IPPickerValue) {
    onChange(option);
    setQuery(option.name);
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
              {option.name}
            </Button>
          ))}
        </div>
      )}
    </div>
  );
}
