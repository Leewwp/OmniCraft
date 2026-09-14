"use client";

import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { Info, Megaphone } from "lucide-react";
import { UserHoverCard } from "@/components/social/UserHoverCard";
import { FollowButton } from "@/components/social/FollowButton";
import { MarkdownRenderer } from "@/components/content/MarkdownRenderer";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/* 通知详情条目（SP-18 #509 §4.1，B 站式三行结构）：触发者（头像/用户名接
   UserHoverCard，#505 吸收）+ 动作词 + 相对时间 / 动作载荷摘录 / 原内容引用
   块（target_summary，点击跳原内容）/ 分类操作行。未读 = 左侧 3px accent
   竖条 + 用户名/标题 semibold；已读手动（hover 出标记已读，不自动）。 */

export interface DecoratedNotification {
  id: number;
  type: string;
  channel: string;
  title?: string;
  body?: string;
  is_read: boolean;
  target_type?: string;
  target_id?: number;
  sender_id?: number;
  created_at: string;
  sender?: { id: number; username: string; avatar_url: string; bio?: string } | null;
  target_summary?: { kind: string; title?: string; url?: string } | null;
}

interface NotificationDetailItemProps {
  notification: DecoratedNotification;
  onMarkRead: (id: number) => void;
}

function formatRelative(iso: string, locale: string): string {
  const ts = Date.parse(iso);
  if (Number.isNaN(ts)) return "";
  const diffSec = Math.max(0, Math.floor((Date.now() - ts) / 1000));
  const rtf = new Intl.RelativeTimeFormat(locale === "en" ? "en" : "zh", { numeric: "auto" });
  if (diffSec < 60) return locale === "en" ? "just now" : "刚刚";
  if (diffSec < 3600) return rtf.format(-Math.floor(diffSec / 60), "minute");
  if (diffSec < 86400) return rtf.format(-Math.floor(diffSec / 3600), "hour");
  if (diffSec < 86400 * 30) return rtf.format(-Math.floor(diffSec / 86400), "day");
  return new Date(ts).toLocaleDateString(locale === "en" ? "en-US" : "zh-CN");
}

const KIND_LABEL_KEYS: Record<string, string> = {
  content: "messages.detail.kindContent",
  comment: "messages.detail.kindContent",
  discussion: "messages.detail.kindDiscussion",
  ip: "messages.detail.kindIP",
  pr: "messages.detail.kindPR",
  user: "messages.detail.kindUser",
  appeal: "messages.detail.kindAppeal",
  report: "messages.detail.kindReport",
  feedback_ticket: "messages.detail.kindFeedback",
  message: "messages.detail.kindMessage",
};

export function NotificationDetailItem({ notification: n, onMarkRead }: NotificationDetailItemProps) {
  const t = useTranslations();
  const locale = useLocale();
  const router = useRouter();

  const sender = n.sender;
  const summary = n.target_summary;
  const isSystemLike = n.channel === "system" || n.channel === "broadcast";
  const time = formatRelative(n.created_at, locale);

  const actionVerb = (() => {
    switch (n.channel) {
      case "reply":
        return t("messages.detail.repliedYou");
      case "like":
        return t("messages.detail.likedYourWork");
      case "follow":
        return t("messages.detail.followedYou");
      case "pr":
        return t("messages.detail.sentYouPR");
      default:
        return "";
    }
  })();

  const kindLabel = summary
    ? KIND_LABEL_KEYS[summary.kind]
      ? t(KIND_LABEL_KEYS[summary.kind])
      : summary.kind
    : "";

  /* 操作目标（§4.1 + Q2 裁决 A）：
     - 回复我的：回复/查看对话都跳原内容；kind=content 时带评论锚点 + 聚焦输入
       （?comments_focus=1 由 CommentSection 消费），kind=discussion 落讨论浮层；
     - 收到的赞：查看作品；PR：查看 PR；关注：真 FollowButton。 */
  const contentTarget =
    summary && (summary.kind === "content" || summary.kind === "comment")
      ? { base: summary.url ?? `/content/${n.target_id ?? 0}`, isContent: true }
      : summary?.kind === "discussion" && summary.url
        ? { base: summary.url, isContent: false }
        : null;

  function jumpReply() {
    if (!n.is_read) onMarkRead(n.id);
    if (!contentTarget) return;
    router.push(
      contentTarget.isContent ? `${contentTarget.base}?comments_focus=1#comments` : contentTarget.base,
    );
  }

  return (
    <article
      className={cn(
        "group relative flex gap-3 border-b border-border-default px-4 py-3",
        !n.is_read && "bg-accent-subtle/40",
      )}
      aria-label={n.title ?? n.body ?? ""}
    >
      {!n.is_read && <span aria-hidden="true" className="absolute inset-y-2 left-0 w-[3px] rounded-r bg-accent-emphasis" />}

      {/* 触发者头像：系统/广播 = 图标圆（非头像）；其余 = UserHoverCard 触发器 */}
      {isSystemLike ? (
        <span
          aria-hidden="true"
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-canvas-subtle text-fg-muted"
        >
          {n.channel === "broadcast" ? <Megaphone className="h-4 w-4" /> : <Info className="h-4 w-4" />}
        </span>
      ) : sender ? (
        <UserHoverCard
          userId={sender.id}
          username={sender.username}
          avatarUrl={sender.avatar_url || undefined}
          size={32}
          placement="dynamic"
        />
      ) : (
        <span aria-hidden="true" className="h-8 w-8 shrink-0 rounded-full bg-canvas-subtle" />
      )}

      <div className="min-w-0 flex-1 space-y-1.5">
        {/* 第一行：触发者 + 动作词 + 相对时间 */}
        <div className="flex min-w-0 items-center gap-1.5 text-sm">
          {sender && !isSystemLike ? (
            <UserHoverCard
              userId={sender.id}
              username={sender.username}
              placement="dynamic"
              showAvatar={false}
              className={cn(!n.is_read && "font-semibold")}
            />
          ) : (
            <span className={cn("truncate font-medium text-fg-default", !n.is_read && "font-semibold")}>{n.title}</span>
          )}
          {actionVerb && <span className="shrink-0 text-fg-muted">{actionVerb}</span>}
          {time && (
            <time className="ml-auto shrink-0 pl-2 text-xs text-fg-muted" dateTime={n.created_at}>
              {time}
            </time>
          )}
        </div>

        {/* 第二行：动作载荷——回复内容摘录（2 行）/ 系统与广播正文（3 行，沿
            NotificationList 的安全 Markdown 渲染语义）/ 关注者 bio 摘录。 */}
        {n.channel === "follow" && sender?.bio ? (
          <p className="line-clamp-1 text-sm text-fg-muted">{sender.bio}</p>
        ) : n.body && !isSystemLike && n.channel !== "like" ? (
          <MarkdownRenderer
            content={n.body}
            className="line-clamp-2 prose-p:my-0 prose-strong:text-fg-default text-sm"
          />
        ) : n.body && isSystemLike ? (
          <MarkdownRenderer
            content={n.body}
            className="line-clamp-3 prose-p:my-0 prose-strong:text-fg-default text-sm"
          />
        ) : null}

        {/* 第三行：原内容引用块（kind 徽标 + 标题，点击跳原内容） */}
        {summary?.title && summary.url && (
          <a
            href={summary.url}
            onClick={(event) => {
              event.preventDefault();
              if (!n.is_read) onMarkRead(n.id);
              router.push(summary.url!);
            }}
            className="flex max-w-md items-center gap-2 rounded-md border border-border-default bg-canvas-subtle px-3 py-1.5 text-xs text-fg-muted transition-colors hover:border-accent-emphasis/40 hover:text-fg-default focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {kindLabel && <span className="shrink-0 rounded bg-background px-1.5 py-0.5 text-[10px] text-fg-muted">{kindLabel}</span>}
            <span className="truncate text-fg-default">{summary.title}</span>
          </a>
        )}

        {/* 操作行 */}
        {!isSystemLike && (
          <div className="flex flex-wrap items-center gap-2 pt-0.5">
            {n.channel === "reply" && contentTarget && (
              <>
                <Button size="sm" variant="outline" className="h-8 px-3 text-xs" onClick={jumpReply}>
                  {t("messages.detail.replyAction")}
                </Button>
                <Button size="sm" variant="ghost" className="h-8 px-3 text-xs" onClick={jumpReply}>
                  {t("messages.detail.viewConversation")}
                </Button>
              </>
            )}
            {n.channel === "like" && contentTarget?.isContent && (
              <Button
                size="sm"
                variant="outline"
                className="h-8 px-3 text-xs"
                onClick={() => {
                  if (!n.is_read) onMarkRead(n.id);
                  router.push(contentTarget.base);
                }}
              >
                {t("messages.detail.viewWork")}
              </Button>
            )}
            {n.channel === "pr" && summary?.url && (
              <Button
                size="sm"
                variant="outline"
                className="h-8 px-3 text-xs"
                onClick={() => {
                  if (!n.is_read) onMarkRead(n.id);
                  router.push(summary.url!);
                }}
              >
                {t("messages.detail.viewPR")}
              </Button>
            )}
            {n.channel === "follow" && sender && <FollowButton targetType="user" targetId={sender.id} className="h-8" />}
          </div>
        )}
      </div>

      {/* 手动已读：hover 出现的「标记已读」小按钮（§5.3，默认不自动） */}
      {!n.is_read && (
        <Button
          size="sm"
          variant="ghost"
          className="h-7 shrink-0 self-start px-2 text-xs opacity-0 transition-opacity focus-visible:opacity-100 group-hover:opacity-100"
          onClick={() => onMarkRead(n.id)}
        >
          {t("messages.detail.markRead")}
        </Button>
      )}
    </article>
  );
}
