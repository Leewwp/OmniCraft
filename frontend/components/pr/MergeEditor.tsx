"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";

interface MergeEditorProps {
  baseText: string;
  proposedText: string;
  onChange?: (value: string) => void;
}

export function MergeEditor({ baseText, proposedText, onChange }: MergeEditorProps) {
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
          <p className="text-xs font-medium text-muted-foreground">原始版本</p>
          <textarea readOnly value={baseText} className="h-56 w-full rounded-md border border-border bg-muted/20 p-2 text-xs" />
        </div>
        <div className="space-y-1">
          <p className="text-xs font-medium text-muted-foreground">PR 提案</p>
          <textarea
            readOnly
            value={proposedText}
            className="h-56 w-full rounded-md border border-border bg-muted/20 p-2 text-xs"
          />
        </div>
        <div className="space-y-1">
          <p className="text-xs font-medium text-muted-foreground">手动合并结果</p>
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
          复制合并结果
        </Button>
      </div>
    </div>
  );
}
