"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";

interface MergeEditorProps {
  baseText: string;
  proposedText: string;
  onChange?: (value: string) => void;
}

export function MergeEditor({ baseText, proposedText, onChange }: MergeEditorProps) {
  const t = useTranslations();
  const [merged, setMerged] = useState(proposedText || baseText);

  useEffect(() => {
    setMerged(proposedText || baseText);
  }, [baseText, proposedText]);

  useEffect(() => {
    onChange?.(merged);
  }, [merged, onChange]);

  return (
    <div className="space-y-3 rounded-md border border-border bg-card p-4 shadow-none">
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-3">
        <div className="space-y-1">
          <p className="text-xs font-medium text-muted-foreground">{t('pr.originalVersion')}</p>
          <textarea readOnly value={baseText} className="h-56 w-full rounded-md border border-border bg-muted/20 p-2 text-xs" />
        </div>
        <div className="space-y-1">
          <p className="text-xs font-medium text-muted-foreground">{t('pr.prVersion')}</p>
          <textarea
            readOnly
            value={proposedText}
            className="h-56 w-full rounded-md border border-border bg-muted/20 p-2 text-xs"
          />
        </div>
        <div className="space-y-1">
          <p className="text-xs font-medium text-muted-foreground">{t('pr.mergedResult')}</p>
          <textarea
            value={merged}
            onChange={(e) => setMerged(e.target.value)}
            className="h-56 w-full rounded-md border border-border bg-background p-2 text-xs"
          />
        </div>
      </div>

      <div className="flex justify-end">
        <Button
          size="sm"
          variant="outline"
          onClick={() => {
            void navigator.clipboard.writeText(merged);
          }}
        >
          {t('pr.copyResult')}
        </Button>
      </div>
    </div>
  );
}
