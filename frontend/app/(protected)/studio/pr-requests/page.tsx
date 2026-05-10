"use client";

import { useTranslations } from "next-intl";
import { GitPullRequest } from "lucide-react";

export default function StudioPRRequestsPage() {
  const t = useTranslations();

  return (
    <div>
      <h1 className="mb-1 text-xl font-bold text-foreground">PR 管理</h1>
      <p className="mb-6 text-sm text-muted-foreground">管理内容协同修改请求</p>
      <div className="rounded-lg border border-border bg-card p-12 text-center">
        <GitPullRequest className="mx-auto mb-3 h-8 w-8 text-muted-foreground" />
        <p className="text-muted-foreground">PR 请求将在此处显示</p>
      </div>
    </div>
  );
}
