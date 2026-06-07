"use client";

import { Select } from "@/components/ui/select";

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
    ariaLabel?: string;
  }[];
}

export function AdminFilterBar({ filters }: AdminFilterBarProps) {
  return (
    <div className="flex flex-wrap items-center gap-3">
      {filters.map((f) => (
        <div key={f.key} className="w-fit min-w-40">
          <Select
            aria-label={f.ariaLabel ?? f.allLabel}
            className="px-3 py-1.5 text-sm"
            value={f.value}
            onChange={(e) => f.onChange(e.target.value)}
          >
            <option value="">{f.allLabel}</option>
            {f.options.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </Select>
        </div>
      ))}
    </div>
  );
}
