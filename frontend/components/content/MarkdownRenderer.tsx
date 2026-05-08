"use client";

import ReactMarkdown, { Components } from "react-markdown";
import { cn } from "@/lib/utils";

interface MarkdownRendererProps {
  content: string;
  className?: string;
}

export function MarkdownRenderer({ content, className }: MarkdownRendererProps) {
  return (
    <div
      className={cn(
        "prose prose-sm max-w-none dark:prose-invert",
        "prose-headings:text-foreground prose-p:text-foreground/90 prose-a:text-accent-primary",
        "prose-code:rounded prose-code:border prose-code:border-border prose-code:bg-muted/50 prose-code:px-1 prose-code:py-0.5 prose-code:text-sm prose-code:font-mono",
        "prose-pre:rounded-md prose-pre:border prose-pre:border-border prose-pre:bg-muted/30 prose-pre:",
        "prose-img:rounded-md prose-img:border prose-img:border-border",
        "prose-blockquote:border-l-accent-primary prose-blockquote:text-muted-foreground",
        "prose-table:border prose-table:border-border prose-th:border prose-th:border-border prose-th:bg-muted/30 prose-th:px-3 prose-th:py-2 prose-td:border prose-td:border-border prose-td:px-3 prose-td:py-2",
        className,
      )}
    >
      <ReactMarkdown components={renderers}>{content}</ReactMarkdown>
    </div>
  );
}

const renderers: Components = {
  code({ className, children, ...props }) {
    const isInline = !className;
    if (isInline) {
      return (
        <code className="rounded border border-border bg-muted/50 px-1 py-0.5 text-sm font-mono" {...props}>
          {children}
        </code>
      );
    }
    return (
      <pre className="overflow-x-auto rounded-md border border-border bg-muted/30 p-4 ">
        <code className={cn("text-sm font-mono", className)} {...props}>
          {children}
        </code>
      </pre>
    );
  },
};
