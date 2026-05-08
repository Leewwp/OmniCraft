"use client";

import { useTranslations } from "next-intl";

interface DiffViewerProps {
  baseText: string;
  proposedText: string;
}

function splitLines(value: string): string[] {
  return value.replace(/\r\n/g, "\n").split("\n");
}

export function DiffViewer({ baseText, proposedText }: DiffViewerProps) {
  const t = useTranslations();
  const left = splitLines(baseText);
  const right = splitLines(proposedText);
  const maxLen = Math.max(left.length, right.length);

  return (
    <div className="overflow-hidden rounded-md border border-border bg-card ">
      <div className="grid grid-cols-2 border-b border-border bg-muted/30 text-xs font-medium">
        <div className="border-r border-border px-3 py-2">{t('pr.originalVersion')}</div>
        <div className="px-3 py-2">{t('pr.prVersion')}</div>
      </div>

      <div className="max-h-[420px] overflow-auto">
        {Array.from({ length: maxLen }).map((_, index) => {
          const l = left[index] ?? "";
          const r = right[index] ?? "";
          const changed = l !== r;

          return (
            <div key={index} className="grid grid-cols-2 border-b border-border text-xs">
              <pre
                className={`overflow-x-auto border-r border-border px-3 py-2 whitespace-pre-wrap ${
                  changed ? "bg-rose-50/50" : ""
                }`}
              >
                {l}
              </pre>
              <pre
                className={`overflow-x-auto px-3 py-2 whitespace-pre-wrap ${
                  changed ? "bg-emerald-50/50" : ""
                }`}
              >
                {r}
              </pre>
            </div>
          );
        })}
      </div>
    </div>
  );
}
