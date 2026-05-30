"use client";

import { useTranslations } from "next-intl";

interface CourseContentProps {
  contentI18n?: Record<string, string>;
  violationType: string;
}

export function CourseContent({ contentI18n, violationType }: CourseContentProps) {
  const t = useTranslations();

  return (
    <div className="min-w-0 flex-1">
      <h3 className="text-sm font-semibold">
        {contentI18n?.zh || violationType}
      </h3>
      <p className="mt-1 text-xs text-muted-foreground">
        {contentI18n?.en || t("rehab.courseContent")}
      </p>
    </div>
  );
}
