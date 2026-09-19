"use client";

/* SP-19 G3-3（#524，Q8-A 完整拟真）：发布页手机壳实时预览。
 *
 * #548：壳体固定纵向比例（280×560、2px 中性描边、正文区内滚动），
 * 侧挂 320px 预览列不再随内容塌缩。实时标题 + 作者行 + 正文（复用
 * MarkdownRenderer——与详情页完全同配置渲染，所见即真实详情效果）+
 * 底部互动栏静态示意 + 「预览效果仅供参考」脚注。夜间开关只切换预览
 * 容器明暗，不影响站点主题；卡片可折叠。 */

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Battery, ChevronDown, Heart, MessageCircle, Signal, Star, Wifi, Moon, Sun } from "lucide-react";
import { MarkdownRenderer } from "@/components/content/MarkdownRenderer";
import { cn } from "@/lib/utils";

interface PublishPhonePreviewProps {
  title: string;
  /** 正文 markdown（text 类主内容）。 */
  markdown?: string;
  /** 文件类简述（纯文本，pre-wrap 渲染，与详情页分流一致）。 */
  plainText?: string;
  authorName: string;
  className?: string;
}

export function PublishPhonePreview({ title, markdown, plainText, authorName, className }: PublishPhonePreviewProps) {
  const t = useTranslations("studio.preview");
  const [dark, setDark] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const initial = [...authorName][0] ?? "?";

  return (
    <div className={cn("rounded-lg border border-border bg-card", className)}>
      <div className="flex items-center justify-between px-3 py-2">
        <button
          type="button"
          onClick={() => setCollapsed((v) => !v)}
          aria-expanded={!collapsed}
          className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
        >
          <ChevronDown className={cn("h-3.5 w-3.5 transition-transform", collapsed && "-rotate-90")} />
          {t("cardTitle")}
        </button>
        <button
          type="button"
          onClick={() => setDark((v) => !v)}
          aria-pressed={dark}
          className="inline-flex items-center gap-1 rounded-full border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          {dark ? <Moon className="h-3.5 w-3.5" /> : <Sun className="h-3.5 w-3.5" />}
          {dark ? t("darkOn") : t("darkOff")}
        </button>
      </div>

      {!collapsed && (
        <div className="flex justify-center px-3 pb-4">
          {/* 手机壳（#548）：固定纵向比例 280×560，正文区内滚动（高度不随
              内容塌缩）；描边 2px 中性色（暗色态 zinc-700），替代原 6px 近黑
              粗边。窄容器（<1280px tab 预览）自适应宽度。 */}
          <div
            className={cn(
              "flex w-full max-w-[280px] flex-col overflow-hidden rounded-[1.75rem] border-2 shadow-md",
              dark ? "border-zinc-700 bg-zinc-950 text-zinc-100" : "border-zinc-300 bg-white text-zinc-900",
            )}
            style={{ height: 560 }}
          >
            {/* 状态栏（静态示意） */}
            <div className={cn("flex shrink-0 items-center justify-between px-5 pb-1 pt-2 text-[10px]", dark ? "text-zinc-400" : "text-zinc-500")}>
              <span>9:41</span>
              <span className="flex items-center gap-1">
                <Signal className="h-3 w-3" aria-hidden="true" />
                <Wifi className="h-3 w-3" aria-hidden="true" />
                <Battery className="h-3.5 w-3.5" aria-hidden="true" />
              </span>
            </div>

            {/* 标题（实时） */}
            <div className="shrink-0 px-5 pt-3">
              <h1 className="text-lg font-bold leading-snug break-words">{title || t("titlePlaceholder")}</h1>
              {/* 作者行（当前用户） */}
              <div className="mt-2 flex items-center gap-2">
                <span className={cn("flex h-6 w-6 items-center justify-center rounded-full text-[10px] font-semibold", dark ? "bg-zinc-800" : "bg-zinc-200")}>
                  {initial}
                </span>
                <span className="text-xs">{authorName}</span>
              </div>
            </div>

            {/* 正文：text 类走 MarkdownRenderer（与详情页同配置），文件类纯
                文本；flex-1 + overflow-y-auto = 壳内滚动。 */}
            <div className={cn("min-h-0 flex-1 overflow-y-auto px-5 pb-4 pt-3 text-sm", dark && "[&_.markdown-body]:text-zinc-100")}>
              {markdown !== undefined ? (
                markdown.trim() ? <MarkdownRenderer content={markdown} /> : <p className={cn("text-xs", dark ? "text-zinc-500" : "text-zinc-400")}>{t("bodyPlaceholder")}</p>
              ) : plainText ? (
                <p className="whitespace-pre-wrap leading-relaxed">{plainText}</p>
              ) : (
                <p className={cn("text-xs", dark ? "text-zinc-500" : "text-zinc-400")}>{t("bodyPlaceholder")}</p>
              )}
            </div>

            {/* 底部互动栏：静态示意 */}
            <div className={cn("flex shrink-0 items-center gap-5 border-t px-5 py-2.5 text-xs", dark ? "border-zinc-800 text-zinc-400" : "border-zinc-100 text-zinc-500")}>
              <span className="inline-flex items-center gap-1"><Heart className="h-3.5 w-3.5" aria-hidden="true" />128</span>
              <span className="inline-flex items-center gap-1"><Star className="h-3.5 w-3.5" aria-hidden="true" />56</span>
              <span className="inline-flex items-center gap-1"><MessageCircle className="h-3.5 w-3.5" aria-hidden="true" />12</span>
              <span className="ml-auto text-[10px]">{t("staticHint")}</span>
            </div>
          </div>
        </div>
      )}
      <p className="px-3 pb-2 text-center text-[10px] text-muted-foreground">{t("footnote")}</p>
    </div>
  );
}
