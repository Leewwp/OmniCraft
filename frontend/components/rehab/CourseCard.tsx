"use client";

import { Clock, BookOpen } from "lucide-react";
import { useTranslations } from "next-intl";

interface CourseCardProps {
  /** 后端按 locale 本地化的课程标题（SP-26-C #782）。 */
  title: string;
  /** 违规码（raw slug），仅以 title 提示保留可查性，不做展示文案。 */
  violationType: string;
  minReadingSec: number;
  rewardPoints: number;
  isActive: boolean;
  elapsed: number;
  children: React.ReactNode;
}

export function CourseCard({
  title,
  violationType,
  minReadingSec,
  rewardPoints,
  isActive,
  elapsed,
  children,
}: CourseCardProps) {
  const t = useTranslations();

  return (
    <div className="rounded-md border border-border bg-card p-4">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0 flex-1">
          <h3 className="text-sm font-semibold" title={violationType}>
            {title}
          </h3>
          <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-1">
              <Clock className="h-3 w-3" />
              {t("rehab.minReading", { sec: minReadingSec })}
            </span>
            <span className="inline-flex items-center gap-1">
              <BookOpen className="h-3 w-3" />
              {t("rehab.rewardPoints", { pts: rewardPoints })}
            </span>
          </div>
          {isActive && (
            <div className="mt-3 space-y-1">
              <div className="relative h-2 w-full rounded-full bg-muted/40">
                <div
                  className="absolute inset-y-0 left-0 rounded-full bg-accent transition-[width] duration-1000"
                  style={{
                    width: `${Math.min((elapsed / minReadingSec) * 100, 100)}%`,
                  }}
                />
              </div>
              <p className="text-xs text-muted-foreground">
                {t("rehab.readingProgress", { elapsed, required: minReadingSec })}
              </p>
            </div>
          )}
        </div>
        <div className="shrink-0">{children}</div>
      </div>
    </div>
  );
}
