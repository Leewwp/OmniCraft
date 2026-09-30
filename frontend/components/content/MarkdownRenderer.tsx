"use client";

import { isValidElement, useCallback, useEffect, useLayoutEffect, useRef, useState, type ComponentProps, type ReactNode } from "react";
import ReactMarkdown, { Components } from "react-markdown";
import rehypeHighlight from "rehype-highlight";
import remarkGfm from "remark-gfm";
import { Check, Copy } from "lucide-react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { isAllowedImageSrc, useImageHostAllowlist } from "@/lib/image-guard";

/** FT-5 (#697) 角标标题小卡数据：number 与正文 [n] 同一轮内全局编号体系
 * （目标查找与存储的锚）。displayNumber（#719）= 渲染层展示号——可见引用
 * 列表按渲染顺序连续重映射的「文字」，角标显示与读屏用它；未提供时回退
 * 原全局编号（旧行为）。 */
export interface CitationBadgeInfo {
  number: number;
  displayNumber?: number;
  title: string;
  excerpt?: string;
  kind: "content" | "ip";
}

interface MarkdownRendererProps {
  content: string;
  className?: string;
  /** A-06 行内引用锚定：提供后把正文中的 [n] 角标渲染为可点击角标。
   *  FT-5：提供 citations 时命中编号渲染为标题小卡（点击回调传轮内全局
   *  编号，1 基）；未提供时回退数字角标 + citationCount 上限（旧行为）。
   *  纯展示层，不改变服务端复验语义。 */
  onCitationRef?: (index: number) => void;
  citationCount?: number;
  citations?: CitationBadgeInfo[];
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
        className="absolute right-2 top-2 inline-flex size-7 items-center justify-center rounded-md border border-border bg-canvas-default text-fg-muted opacity-0 transition-opacity duration-150 hover:text-foreground focus:outline-none focus-visible:ring-1 focus:ring-ring focus-visible:opacity-100 group-hover/code:opacity-100"
      >
        {copied ? <Check className="h-3.5 w-3.5" aria-hidden="true" /> : <Copy className="h-3.5 w-3.5" aria-hidden="true" />}
      </button>
    </div>
  );
}

/* FT-5 (#697)：行内引用角标的标题小卡形态。桌面悬停浮窗复用 UserHoverCard
   的延时/定位/上翻模式（200ms 开关延迟、fixed 定位、视口钳制、下方不足上
   翻——小卡默认浮于角标上方、上方不足落下方、Esc 关闭、滚动跟随）；触屏
   （hover:none）不弹浮窗，点按直接走跳转。 */
const CITATION_POPOVER_WIDTH = 280;
const CITATION_POPOVER_EST_HEIGHT = 140;
const CITATION_HOVER_DELAY_MS = 200;
const CITATION_VIEWPORT_MARGIN = 8;

function truncateBadgeTitle(title: string, maxRunes = 10): string {
  const runes = Array.from(title.trim());
  if (runes.length <= maxRunes) return runes.join("");
  return `${runes.slice(0, maxRunes).join("")}…`;
}

function CitationBadge({ info, onJump }: { info: CitationBadgeInfo; onJump: () => void }) {
  const t = useTranslations();
  const [open, setOpen] = useState(false);
  const [hoverCapable, setHoverCapable] = useState(false);
  const [position, setPosition] = useState<{ left: number; top: number } | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const cardRef = useRef<HTMLSpanElement>(null);
  const openTimerRef = useRef<number | null>(null);
  const closeTimerRef = useRef<number | null>(null);

  useEffect(() => {
    if (typeof window.matchMedia !== "function") return;
    const media = window.matchMedia("(hover: hover) and (pointer: fine)");
    const sync = () => setHoverCapable(media.matches);
    sync();
    media.addEventListener("change", sync);
    return () => media.removeEventListener("change", sync);
  }, []);

  const clearTimers = useCallback(() => {
    if (openTimerRef.current !== null) window.clearTimeout(openTimerRef.current);
    if (closeTimerRef.current !== null) window.clearTimeout(closeTimerRef.current);
    openTimerRef.current = null;
    closeTimerRef.current = null;
  }, []);

  const computePosition = useCallback(() => {
    const trigger = triggerRef.current;
    if (!trigger) return null;
    const rect = trigger.getBoundingClientRect();
    if (rect.bottom < 0 || rect.top > window.innerHeight || rect.right < 0 || rect.left > window.innerWidth) {
      return null;
    }
    const visibleLeft = CITATION_VIEWPORT_MARGIN;
    const visibleRight = window.innerWidth - CITATION_VIEWPORT_MARGIN;
    const visibleTop = CITATION_VIEWPORT_MARGIN;
    const visibleBottom = window.innerHeight - CITATION_VIEWPORT_MARGIN;
    const anchoredLeft = rect.left + rect.width / 2 - CITATION_POPOVER_WIDTH / 2;
    const left = Math.min(Math.max(visibleLeft, anchoredLeft), Math.max(visibleLeft, visibleRight - CITATION_POPOVER_WIDTH));
    // 默认浮于角标上方；上方不足落下方（卡高优先用实测值，首开退回估算）。
    const cardHeight = cardRef.current?.offsetHeight || CITATION_POPOVER_EST_HEIGHT;
    const aboveTop = rect.top - cardHeight - CITATION_VIEWPORT_MARGIN;
    const top = aboveTop >= visibleTop ? aboveTop : Math.min(visibleBottom - cardHeight, rect.bottom + CITATION_VIEWPORT_MARGIN);
    return { left, top };
  }, []);

  useLayoutEffect(() => {
    if (!open) return;
    const next = computePosition();
    if (!next) {
      setOpen(false);
      return;
    }
    setPosition(next);
  }, [open, computePosition]);

  const showNow = useCallback(() => {
    clearTimers();
    setPosition(computePosition());
    setOpen(true);
  }, [clearTimers, computePosition]);

  const scheduleOpen = useCallback(() => {
    clearTimers();
    openTimerRef.current = window.setTimeout(showNow, CITATION_HOVER_DELAY_MS);
  }, [clearTimers, showNow]);

  const scheduleClose = useCallback(() => {
    clearTimers();
    closeTimerRef.current = window.setTimeout(() => setOpen(false), CITATION_HOVER_DELAY_MS);
  }, [clearTimers]);

  const closeNow = useCallback(() => {
    clearTimers();
    setOpen(false);
  }, [clearTimers]);

  useEffect(() => {
    if (!open) return;
    function onScrollOrResize() {
      const next = computePosition();
      if (!next) {
        setOpen(false);
        return;
      }
      setPosition(next);
    }
    window.addEventListener("scroll", onScrollOrResize, true);
    window.addEventListener("resize", onScrollOrResize);
    return () => {
      window.removeEventListener("scroll", onScrollOrResize, true);
      window.removeEventListener("resize", onScrollOrResize);
    };
  }, [open, computePosition]);

  /* 卸载即清定时器：残留的开关延迟回调会在卸载后触发无 context 的重渲染
     （use-intl ENVIRONMENT_FALLBACK），并挂起测试事件循环。 */
  useEffect(() => clearTimers, [clearTimers]);

  useEffect(() => {
    if (!open) return;
    function onKey(event: KeyboardEvent) {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopPropagation();
      closeNow();
    }
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [open, closeNow]);

  /* #719 展示号：文字与读屏用展示号（缺省回退原全局编号），点击命中
     （onJump）仍由调用方以原编号解析目标。 */
  const shownNumber = info.displayNumber ?? info.number;
  return (
    <>
      <button
        type="button"
        ref={triggerRef}
        aria-label={t("markdown.citationJump", { index: shownNumber })}
        onClick={onJump}
        {...(hoverCapable
          ? { onPointerEnter: scheduleOpen, onPointerLeave: scheduleClose, onFocus: showNow, onBlur: scheduleClose }
          : {})}
        className="mx-0.5 inline-flex h-4 max-w-40 -translate-y-1 items-center justify-center gap-0.5 rounded-sm border border-accent-emphasis/0 bg-accent-subtle px-1.5 align-baseline text-[0.7em] font-semibold text-accent-emphasis transition-colors duration-150 hover:border-accent-emphasis hover:bg-accent-subtle focus:outline-none focus-visible:ring-1 focus:ring-ring"
      >
        <span aria-hidden className="text-[0.85em] opacity-70">{shownNumber}</span>
        <span className="truncate">{truncateBadgeTitle(info.title)}</span>
      </button>
      {open && position && (
        <span
          ref={cardRef}
          role="tooltip"
          {...(hoverCapable ? { onPointerEnter: clearTimers, onPointerLeave: scheduleClose } : {})}
          style={{ position: "fixed", left: position.left, top: position.top, width: CITATION_POPOVER_WIDTH, zIndex: 60 }}
          className="pointer-events-auto flex flex-col gap-1 rounded-md border border-border-default bg-card p-3 shadow-lg"
        >
          <span className="flex items-center gap-1.5 text-sm font-medium text-accent-emphasis">
            <span className="text-xs font-normal text-fg-muted">{shownNumber}</span>
            <span className="line-clamp-2">{info.title}</span>
            {info.kind === "ip" && (
              <span className="ml-auto shrink-0 rounded border border-border-default px-1.5 py-0.5 text-xs font-normal text-fg-muted">
                {t("agent.citations.zoneIP")}
              </span>
            )}
          </span>
          {info.excerpt && (
            <span className="line-clamp-3 text-xs text-fg-muted">{info.excerpt}</span>
          )}
          <span className="text-xs text-fg-muted/80">{t("markdown.citationView")}</span>
        </span>
      )}
    </>
  );
}

export function MarkdownRenderer({ content, className, onCitationRef, citationCount, citations }: MarkdownRendererProps) {
  const t = useTranslations();
  const citationMode = onCitationRef !== undefined;
  const source = citationMode ? withCitationAnchors(content) : content;
  // SP-25 FR-07（中-1）：流式窗口渲染层图片白名单——见 lib/image-guard.ts。
  const ossDomain = useImageHostAllowlist();

  const renderers: Components = {
    // #723 溢出防御：生成内容的长表格包横向滚动容器（表自身不撑宽页面）。
    table({ children, ...props }) {
      return (
        <div className="w-full overflow-x-auto">
          <table {...props}>{children}</table>
        </div>
      );
    },
    img({ src, alt, ...props }) {
      if (!isAllowedImageSrc(typeof src === "string" ? src : undefined, ossDomain)) {
        return (
          <span
            role="img"
            aria-label={t("markdown.imageBlocked")}
            className="inline-flex items-center rounded border border-border bg-muted/50 px-1.5 py-0.5 text-xs text-fg-muted"
          >
            {t("markdown.imageBlocked")}
          </span>
        );
      }
      // eslint-disable-next-line @next/next/no-img-element -- 模型输出的动态
      // 图源不受 next/image 优化管线管理，白名单已在渲染层收紧。
      return <img src={src} alt={alt} loading="lazy" {...props} />;
    },
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
      /* 行内引用角标：由 withCitationAnchors 生成的 #cite-n 链接。FT-5：提供
       * citations 时按轮内全局编号命中渲染为标题小卡；未命中（越界/未提供）
       * 回退旧数字角标（citationCount 上限）或纯文本 sup。 */
      if (citationMode && href?.startsWith("#cite-")) {
        const index = Number.parseInt(href.slice(6), 10);
        if (Number.isInteger(index) && index > 0) {
          const badge = citations?.find((item) => item.number === index);
          if (badge) {
            return <CitationBadge info={badge} onJump={() => onCitationRef?.(index)} />;
          }
          if (citations === undefined && (citationCount === undefined || index <= citationCount)) {
            return (
              <button
                type="button"
                aria-label={t("markdown.citationJump", { index })}
                onClick={() => onCitationRef?.(index - 1)}
                className="mx-0.5 inline-flex h-4 min-w-4 -translate-y-1 items-center justify-center rounded-sm border border-accent-emphasis/0 bg-accent-subtle px-1 align-baseline text-[0.7em] font-semibold text-accent-emphasis transition-colors duration-150 hover:border-accent-emphasis hover:bg-accent-subtle focus:outline-none focus-visible:ring-1 focus:ring-ring"
              >
                {index}
              </button>
            );
          }
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
        "prose prose-sm max-w-none break-words dark:prose-invert",
        "prose-headings:text-foreground prose-p:text-foreground/90 prose-a:text-accent-primary",
        "prose-code:rounded prose-code:border prose-code:border-border prose-code:bg-muted/50 prose-code:px-1 prose-code:py-0.5 prose-code:text-sm prose-code:font-mono",
        "prose-pre:rounded-md prose-pre:border prose-pre:border-border prose-pre:bg-muted/30 prose-pre:",
        "prose-img:rounded-md prose-img:border prose-img:border-border",
        "prose-blockquote:border-l-accent-primary prose-blockquote:text-muted-foreground",
        "prose-table:border prose-table:border-border prose-th:border prose-th:border-border prose-th:bg-muted/30 prose-th:px-3 prose-th:py-2 prose-td:border prose-td:border-border prose-td:px-3 prose-td:py-2",
        className,
      )}
    >
      <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeHighlight]} components={renderers}>
        {source}
      </ReactMarkdown>
    </div>
  );
}
