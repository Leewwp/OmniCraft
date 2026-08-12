"use client";

import { Select as SelectPrimitive } from "@base-ui/react/select";
import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";

export interface SortSelectOption {
  value: string;
  label: string;
}

export interface SortSelectProps {
  className?: string;
  value: string;
  options: SortSelectOption[];
  onChange: (value: string) => void;
  ariaLabel: string;
}

/** Shared accessible sort control (trigger + positioned listbox). See ui-spec Component: SortSelect. */
export function SortSelect({ className, value, options, onChange, ariaLabel }: SortSelectProps) {
  return (
    <SelectPrimitive.Root
      value={value}
      modal={false}
      items={options.map((option) => ({ value: option.value, label: option.label }))}
      onValueChange={(next) => {
        if (next != null) onChange(String(next));
      }}
    >
      <SelectPrimitive.Trigger
        aria-label={ariaLabel}
        className={cn(
          "inline-flex items-center gap-2 rounded-md border border-border-default bg-canvas-default px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-accent-emphasis",
          className,
        )}
      >
        <SelectPrimitive.Value />
        <SelectPrimitive.Icon className="flex items-center">
          <ChevronDown className="h-4 w-4 text-fg-muted" aria-hidden="true" />
        </SelectPrimitive.Icon>
      </SelectPrimitive.Trigger>
      <SelectPrimitive.Portal>
        <SelectPrimitive.Positioner
          side="bottom"
          sideOffset={4}
          align="start"
          alignItemWithTrigger={false}
          className="isolate z-50 outline-none"
        >
          <SelectPrimitive.Popup className="z-50 max-h-(--available-height) min-w-(--anchor-width) max-w-[calc(100vw-2rem)] overflow-y-auto rounded-md border border-border-default bg-canvas-default p-1 shadow-md outline-none">
            <SelectPrimitive.List>
              {options.map((option) => (
                <SelectPrimitive.Item
                  key={option.value}
                  value={option.value}
                  className="relative flex cursor-pointer items-center rounded-md px-3 py-1.5 text-sm text-foreground select-none outline-none data-highlighted:bg-accent-subtle data-highlighted:text-accent-emphasis data-selected:bg-accent-subtle data-selected:text-accent-emphasis"
                >
                  <SelectPrimitive.ItemText>{option.label}</SelectPrimitive.ItemText>
                </SelectPrimitive.Item>
              ))}
            </SelectPrimitive.List>
          </SelectPrimitive.Popup>
        </SelectPrimitive.Positioner>
      </SelectPrimitive.Portal>
    </SelectPrimitive.Root>
  );
}
