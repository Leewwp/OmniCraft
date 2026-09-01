"use client";
/**
 * 【原型专用，随时可删】讨论区 tab：讨论卡片列表 + 讨论帖详情浮层（含回帖）。
 * 讨论帖详情浮层是本次重构新增交互：Esc / 浏览器后退 / 遮罩点击关闭。
 */
import { useState } from "react";
import { Flame, MessageCircle, Pin, Send, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { useToast } from "@/components/ui/Toast";
import { t } from "./copy";
import type { Discussion, Reply } from "./mock-data";

function AvatarDot({ name }: { name: string }) {
  return (
    <span
      aria-hidden="true"
      className="inline-flex size-6 shrink-0 items-center justify-center rounded-full bg-accent-subtle text-xs font-semibold text-accent-emphasis"
    >
      {name.slice(0, 1)}
    </span>
  );
}

export function DiscussionList({
  items,
  showHotScore,
  onOpen,
}: {
  items: Discussion[];
  showHotScore: boolean;
  onOpen: (d: Discussion) => void;
}) {
  const { toast } = useToast();
  if (items.length === 0) {
    return (
      <EmptyState
        icon={MessageCircle}
        title={t("hub.discussion.emptyTitle")}
        description={t("hub.discussion.emptyDesc")}
        action={<Button onClick={() => toast("info", t("common.prototypeOnly"))}>{t("hub.discussion.emptyAction")}</Button>}
      />
    );
  }
  return (
    <div className="flex flex-col gap-2">
      {items.map((d) => (
        <button
          key={d.id}
          type="button"
          onClick={() => onOpen(d)}
          className="flex items-start justify-between gap-4 rounded-lg border border-border bg-canvas-default p-4 text-left shadow-[var(--elevation-1)] transition-[border-color,box-shadow] duration-150 hover:border-border-strong hover:shadow-[var(--elevation-2)]"
        >
          <div className="min-w-0">
            <h3 className="flex items-center gap-1.5 text-sm font-semibold">
              {d.pinned && (
                <span className="inline-flex h-5 shrink-0 items-center gap-0.5 rounded-full bg-[var(--tag-orange-bg)] px-1.5 text-xs font-medium text-[var(--tag-orange-fg)]">
                  <Pin className="size-3" aria-hidden="true" />
                  {t("hub.discussion.pinned")}
                </span>
              )}
              <span className="line-clamp-1">{d.title}</span>
            </h3>
            <p className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
              <span>{d.author}</span>
              <span className="inline-flex items-center gap-1">
                <MessageCircle className="size-3" aria-hidden="true" />
                {t("hub.discussion.replyCount", { count: d.replyCount })}
              </span>
              <span>{t("hub.discussion.lastReply", { time: d.lastReplyDisplay })}</span>
            </p>
          </div>
          {showHotScore && (
            <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-[var(--tag-rose-bg)] px-2 py-0.5 text-xs font-medium text-[var(--tag-rose-fg)]">
              <Flame className="size-3" aria-hidden="true" />
              {t("hub.discussion.hotScore", { score: d.hot.toFixed(1) })}
            </span>
          )}
        </button>
      ))}
    </div>
  );
}

export function DiscussionOverlay({
  thread,
  extraReplies,
  onReply,
  onClose,
}: {
  thread: Discussion;
  extraReplies: Reply[];
  onReply: (threadId: string, body: string) => void;
  onClose: () => void;
}) {
  const { toast } = useToast();
  const [draft, setDraft] = useState("");
  const replies = [...thread.replies, ...extraReplies];
  const total = thread.replyCount + extraReplies.length;

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const body = draft.trim();
    if (!body) return;
    onReply(thread.id, body);
    setDraft("");
    toast("success", t("hub.discussion.replyPosted"));
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" role="dialog" aria-modal="true" aria-label={thread.title}>
      <div className="absolute inset-0 bg-black/50" onClick={onClose} aria-hidden="true" />
      <div className="relative flex max-h-[85vh] w-full max-w-2xl flex-col overflow-hidden rounded-lg border border-border bg-canvas-default shadow-[var(--elevation-3)]">
        <button
          type="button"
          onClick={onClose}
          aria-label={t("common.close")}
          className="absolute right-3 top-3 z-10 inline-flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors duration-150 hover:bg-muted hover:text-foreground"
        >
          <X className="size-4" aria-hidden="true" />
        </button>
        <div className="min-h-0 flex-1 overflow-y-auto p-5">
          <h2 className="flex items-start gap-2 pr-8 text-xl font-semibold leading-snug">
            {thread.pinned && (
              <span className="mt-1 inline-flex h-5 shrink-0 items-center gap-0.5 rounded-full bg-[var(--tag-orange-bg)] px-1.5 text-xs font-medium text-[var(--tag-orange-fg)]">
                <Pin className="size-3" aria-hidden="true" />
                {t("hub.discussion.pinned")}
              </span>
            )}
            {thread.title}
          </h2>
          <p className="mt-1.5 text-xs text-muted-foreground">
            {thread.author} · {t("hub.discussion.started", { time: thread.createdDisplay })}
          </p>
          <div className="mt-3 space-y-2">
            {thread.body.map((para, i) => (
              <p key={i} className="text-sm leading-relaxed text-foreground/90">
                {para}
              </p>
            ))}
          </div>
          <div className="mt-5 border-t border-border pt-4">
            <h3 className="text-sm font-semibold">{t("hub.discussion.replies", { count: total })}</h3>
            <ul className="mt-3 space-y-4">
              {replies.map((r, i) => (
                <li key={i} className="flex gap-2.5">
                  <AvatarDot name={r.author} />
                  <div className="min-w-0">
                    <p className="text-xs">
                      <span className="font-semibold">{r.author}</span>
                      <span className="ml-2 text-muted-foreground">{r.time}</span>
                    </p>
                    <p className="mt-0.5 text-sm leading-relaxed text-foreground/90">{r.body}</p>
                  </div>
                </li>
              ))}
            </ul>
          </div>
        </div>
        <form onSubmit={submit} className="flex items-center gap-2 border-t border-border bg-canvas-default p-3">
          <Input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder={t("hub.discussion.replyPlaceholder")}
            aria-label={t("hub.discussion.replyPlaceholder")}
            className="min-h-9 flex-1"
          />
          <Button type="submit" size="sm" className="min-h-9 gap-1 px-3" disabled={!draft.trim()}>
            <Send className="size-3.5" aria-hidden="true" />
            {t("hub.discussion.replySend")}
          </Button>
        </form>
      </div>
    </div>
  );
}
