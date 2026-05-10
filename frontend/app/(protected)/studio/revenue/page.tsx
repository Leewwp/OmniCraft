"use client";

import { DollarSign } from "lucide-react";

export default function StudioRevenuePage() {
  return (
    <div>
      <h1 className="mb-1 text-xl font-bold text-foreground">收益数据</h1>
      <p className="mb-6 text-sm text-muted-foreground">查看创作收益和数据分析</p>
      <div className="rounded-lg border border-border bg-card p-12 text-center">
        <DollarSign className="mx-auto mb-3 h-8 w-8 text-muted-foreground" />
        <p className="mb-2 text-sm font-medium text-foreground">收益功能即将开放</p>
        <p className="text-sm text-muted-foreground">创作者激励计划正在筹备中，敬请期待</p>
      </div>
    </div>
  );
}
