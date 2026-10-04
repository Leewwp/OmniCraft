"use client";

import { useTranslations } from "next-intl";
import { MarkdownRenderer } from "@/components/content/MarkdownRenderer";

interface CourseContentProps {
  /** 后端按 locale 本地化的课程标题与正文（SP-26-C #782）。 */
  title: string;
  content: string;
  /** 违规码（raw slug），仅以 title 提示保留可查性，不做展示文案。 */
  violationType: string;
}

export function CourseContent({ title, content, violationType }: CourseContentProps) {
  const t = useTranslations();

  return (
    <div className="min-w-0 flex-1">
      <h3 className="text-sm font-semibold" title={violationType}>
        {title}
      </h3>
      <div className="mt-1 text-xs text-muted-foreground">
        {content ? (
          <MarkdownRenderer content={content} />
        ) : (
          t("rehab.courseContent")
        )}
      </div>
    </div>
  );
}
