"use client";

import { cn } from "@/lib/utils";
import type { LucideIcon } from "lucide-react";

interface AdminMetricCardProps {
  label: string;
  value: number | string;
  icon: LucideIcon;
  variant?: "default" | "warning" | "danger";
  loading?: boolean;
}

export function AdminMetricCard({ label, value, icon: Icon, variant = "default", loading }: AdminMetricCardProps) {
  return (
    <div className="rounded-md border border-border bg-card p-4">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-xs font-medium text-muted-foreground">{label}</p>
          {loading ? (
            <div className="mt-1 h-7 w-16 animate-pulse rounded bg-muted" />
          ) : (
            <p className={cn(
              "mt-1 text-2xl font-bold tracking-tight",
              variant === "danger" && "text-red-600",
              variant === "warning" && "text-amber-600",
            )}>
              {value}
            </p>
          )}
        </div>
        <div className={cn(
          "flex h-10 w-10 items-center justify-center rounded-md",
          variant === "danger" && "bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400",
          variant === "warning" && "bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400",
          variant === "default" && "bg-muted text-muted-foreground",
        )}>
          <Icon className="h-5 w-5" />
        </div>
      </div>
    </div>
  );
}
