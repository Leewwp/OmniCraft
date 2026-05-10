"use client";

import { Users } from "lucide-react";

export default function StudioContributorsPage() {
  return (
    <div>
      <h1 className="mb-1 text-xl font-bold text-foreground">贡献者</h1>
      <p className="mb-6 text-sm text-muted-foreground">管理对你内容的贡献者</p>
      <div className="rounded-lg border border-border bg-card p-12 text-center">
        <Users className="mx-auto mb-3 h-8 w-8 text-muted-foreground" />
        <p className="text-muted-foreground">贡献者列表将在此处显示</p>
      </div>
    </div>
  );
}
