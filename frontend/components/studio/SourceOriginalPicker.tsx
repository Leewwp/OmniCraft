"use client";

import { SourceContentPicker } from "@/components/studio/SourceContentPicker";

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

export function SourceOriginalPicker({ value, onChange }: SourceOriginalPickerProps) {
  return (
    <SourceContentPicker
      sourceKind="original"
      selected={value ? { ...value, zone: "original" } : undefined}
      onSelect={(content) => onChange(content ? { id: content.id, title: content.title } : null)}
    />
  );
}
