"use client";

import { cn } from "@/lib/utils";

export interface ContentType {
  value: string;
  icon: string;
  label: string;
  description: string;
}

interface ContentTypeGridProps {
  types: ContentType[];
  selected?: string | null;
  onSelect: (type: string) => void;
}

export function ContentTypeGrid({
  types,
  selected,
  onSelect,
}: ContentTypeGridProps) {
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
      {types.map((type) => {
        const isSelected = selected === type.value;
        return (
          <button
            key={type.value}
            type="button"
            onClick={() => onSelect(type.value)}
            className={cn(
              "flex flex-col items-center gap-3 rounded-lg border p-5 text-center transition-all duration-150",
              isSelected
                ? "border-accent-emphasis bg-accent-subtle ring-2 ring-accent-emphasis/20"
                : "border-border bg-card hover:border-border/80 hover:bg-muted/50"
            )}
          >
            <span className="text-3xl">{type.icon}</span>
            <div>
              <div className="text-sm font-semibold text-foreground">
                {type.label}
              </div>
              <div className="mt-1 text-xs text-muted-foreground">
                {type.description}
              </div>
            </div>
          </button>
        );
      })}
    </div>
  );
}
