"use client";

import { useTranslations } from "next-intl";

interface FilterOption {
  value: string;
  label: string;
}

interface AdminFilterBarProps {
  filters: {
    key: string;
    value: string;
    onChange: (v: string) => void;
    options: FilterOption[];
    allLabel: string;
  }[];
}

export function AdminFilterBar({ filters }: AdminFilterBarProps) {
  return (
    <div className="flex flex-wrap items-center gap-3">
      {filters.map((f) => (
        <select
          key={f.key}
          className="rounded-md border border-border bg-background px-3 py-1.5 text-sm"
          value={f.value}
          onChange={(e) => f.onChange(e.target.value)}
        >
          <option value="">{f.allLabel}</option>
          {f.options.map((opt) => (
            <option key={opt.value} value={opt.value}>{opt.label}</option>
          ))}
        </select>
      ))}
    </div>
  );
}
