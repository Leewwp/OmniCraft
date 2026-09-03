"use client";

import { isValidElement, type ComponentProps, type ReactNode, useState } from "react";
import ReactMarkdown, { Components } from "react-markdown";
import rehypeHighlight from "rehype-highlight";
import { Check, Copy } from "lucide-react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";

interface MarkdownRendererProps {
  content: string;
  className?: string;
  /** A-06 行内引用锚定：提供后把正文中的 [n] 角标渲染为可点击 sup 角标，
   *  点击回调序号（1 基）；citationCount 之外的角标渲染为纯文本。纯展示层，
   *  不改变服务端复验语义。 */
  onCitationRef?: (index: number) => void;
  citationCount?: number;
}

/** 把句末 [1][2] 角标转为 markdown 链接 [[1]](#cite-1)，由 a 渲染器接管为
 *  可点击角标。避开真实链接 [1](url)、引用定义 [1]: 与图片 ![1]。 */
function withCitationAnchors(content: string): string {
  return content.replace(
    /(?<!!)\[(\d{1,2})\](?![:(\[])/g,
    (match, digits: string) => `[[${digits}]](#cite-${digits})`,
  );
}

/** 把 React 子树还原为纯文本（代码块复制用）。 */
function nodeToText(node: ReactNode): string {
  if (node === null || node === undefined || typeof node === "boolean") return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(nodeToText).join("");
  if (isValidElement(node)) {
    const props = node.props as { children?: ReactNode };
    return nodeToText(props.children);
  }
  return "";
}

function CodeBlock({ className, children, ...props }: ComponentProps<"code">) {
  const t = useTranslations();
  const [copied, setCopied] = useState(false);

  async function handleCopy(event: React.MouseEvent<HTMLButtonElement>) {
    event.stopPropagation();
    const text = nodeToText(children);
    if (text === "") return;
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      /* 剪贴板不可用（权限/非安全上下文）时静默失败，按钮回到可复制态。 */
    }
  }

  return (
    <div className="group/code relative">
      <pre className="overflow-x-auto rounded-md border border-border bg-muted/30 p-4">
        <code className={cn("text-sm font-mono", className)} {...props}>
          {children}
        </code>
      </pre>
      <button
        type="button"
        aria-label={t("markdown.copyCode")}
        onClick={handleCopy}
        className="absolute right-2 top-2 inline-flex size-7 items-center justify-center rounded-md border border-border bg-canvas-default text-fg-muted opacity-0 transition-opacity duration-150 hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring focus-visible:opacity-100 group-hover/code:opacity-100"
      >
        {copied ? <Check className="h-3.5 w-3.5" aria-hidden="true" /> : <Copy className="h-3.5 w-3.5" aria-hidden="true" />}
      </button>
    </div>
  );
}

export function MarkdownRenderer({ content, className, onCitationRef, citationCount }: MarkdownRendererProps) {
  const t = useTranslations();
  const citationMode = onCitationRef !== undefined;
  const source = citationMode ? withCitationAnchors(content) : content;

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
      return <CodeBlock className={className} {...props}>{children}</CodeBlock>;
    },
    a({ href, children, ...props }) {
      /* 行内引用角标：由 withCitationAnchors 生成的 #cite-n 链接。 */
      if (citationMode && href?.startsWith("#cite-")) {
        const index = Number.parseInt(href.slice(6), 10);
        if (Number.isInteger(index) && index > 0 && (citationCount === undefined || index <= citationCount)) {
          return (
            <button
              type="button"
              aria-label={t("markdown.citationJump", { index })}
              onClick={() => onCitationRef?.(index - 1)}
              className="mx-0.5 inline-flex h-4 min-w-4 -translate-y-1 items-center justify-center rounded-sm bg-accent-subtle px-1 align-baseline text-[0.7em] font-semibold text-accent-emphasis transition-colors duration-150 hover:bg-accent-emphasis hover:text-accent-emphasis-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            >
              {index}
            </button>
          );
        }
        return <sup className="text-[0.7em] text-fg-muted">{children}</sup>;
      }
      /* 外链安全：仅 http(s) 绝对地址开新标签并断开 opener；站内相对链接原样。 */
      const isExternal = typeof href === "string" && /^https?:\/\//i.test(href);
      return (
        <a
          href={href}
          {...(isExternal ? { target: "_blank", rel: "noopener noreferrer" } : {})}
          {...props}
        >
          {children}
        </a>
      );
    },
  };

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
      <ReactMarkdown rehypePlugins={[rehypeHighlight]} components={renderers}>
        {source}
      </ReactMarkdown>
    </div>
  );
}
