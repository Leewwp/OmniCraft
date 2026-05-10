"use client";

import { Tags } from "lucide-react";

export default function StudioTagSuggestionsPage() {
  return (
    <div>
      <h1 className="mb-1 text-xl font-bold text-foreground">标签建议</h1>
      <p className="mb-6 text-sm text-muted-foreground">审核用户提交的标签建议</p>
      <div className="rounded-lg border border-border bg-card p-12 text-center">
        <Tags className="mx-auto mb-3 h-8 w-8 text-muted-foreground" />
        <p className="text-muted-foreground">标签建议将在此处显示</p>
      </div>
    </div>
  );
}
