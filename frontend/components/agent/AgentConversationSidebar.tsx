"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { MoreHorizontal, PanelLeftClose, PanelLeftOpen, Pin, PinOff, Plus, Trash2 } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

export interface AgentConversationSummary {
  id: number;
  context_type?: string;
  title?: string | null;
  pinned_at?: string | null;
  created_at?: string;
  updated_at?: string;
}

interface AgentConversationSidebarProps {
  conversations: AgentConversationSummary[];
  activeId?: number | null;
  collapsed: boolean;
  loading?: boolean;
  onToggleCollapse: () => void;
  onSelect: (id: number) => void;
  onNewConversation: () => void;
  /** A-06 侧边栏 ⋯ 菜单三交互（对接 A-01 PATCH / DELETE API，由外壳实现）。 */
  onRename?: (id: number, title: string) => void;
  onTogglePin?: (id: number, pinned: boolean) => void;
  onDelete?: (id: number) => void;
  onRequestClose?: () => void;
  disabled?: boolean;
}

/** PATCH 契约同款长度上限（service.ConversationTitleMaxRunes = 50）。 */
const TITLE_MAX_RUNES = 50;

type ConversationGroupKey = "pinned" | "today" | "yesterday" | "earlier";

function groupKey(conversation: AgentConversationSummary): ConversationGroupKey {
  if (conversation.pinned_at) return "pinned";
  const updatedAt = conversation.updated_at;
  if (!updatedAt) return "earlier";
  const date = new Date(updatedAt);
  if (Number.isNaN(date.getTime())) return "earlier";
  const now = new Date();
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const startOfYesterday = startOfToday - 86400000;
  const time = date.getTime();
  if (time >= startOfToday) return "today";
  if (time >= startOfYesterday) return "yesterday";
  return "earlier";
}

const GROUP_KEYS: ConversationGroupKey[] = ["pinned", "today", "yesterday", "earlier"];

/**
 * Agent 工作台会话侧栏（A1.6 + A-06）：折叠按钮 → 全宽「开启新对话」→ 会话历史
 * （置顶分组置顶，其余按时间分组）。每项悬停 ⋯ 菜单 = 重命名 / 置顶(取消置顶) /
 * 删除；重命名内联输入（Enter 提交 / Esc 取消）。折叠态持久化由外壳负责。
 */
export function AgentConversationSidebar({
  conversations,
  activeId,
  collapsed,
  loading,
  onToggleCollapse,
  onSelect,
  onNewConversation,
  onRename,
  onTogglePin,
  onDelete,
  onRequestClose,
  disabled,
}: AgentConversationSidebarProps) {
  const t = useTranslations();
  const [renamingId, setRenamingId] = useState<number | null>(null);
  const [renameDraft, setRenameDraft] = useState("");

  const groups = useMemo(() => {
    const buckets = new Map<ConversationGroupKey, AgentConversationSummary[]>();
    for (const key of GROUP_KEYS) buckets.set(key, []);
    for (const conversation of conversations) {
      buckets.get(groupKey(conversation))?.push(conversation);
    }
    return GROUP_KEYS.map((key) => ({
      key,
      label: t(`agent.workspace.group${key[0].toUpperCase()}${key.slice(1)}`),
      items: buckets.get(key) ?? [],
    })).filter((group) => group.items.length > 0);
  }, [conversations, t]);

  const collapseLabel = onRequestClose
    ? t("agent.workspace.closeConversations")
    : t("agent.workspace.collapseSidebar");

  const expandLabel = onRequestClose
    ? t("agent.workspace.closeConversations")
    : t("agent.workspace.expandSidebar");

  function startRename(conversation: AgentConversationSummary) {
    setRenamingId(conversation.id);
    setRenameDraft(conversation.title ?? "");
  }

  function commitRename(id: number) {
    const trimmed = renameDraft.trim();
    setRenamingId(null);
    if (trimmed !== "") onRename?.(id, trimmed.slice(0, TITLE_MAX_RUNES));
  }

  if (collapsed) {
    return (
      <aside
        aria-label={t("agent.workspace.sidebarLabel")}
        className="flex w-14 shrink-0 flex-col items-center gap-2 border-r border-border-default bg-canvas-default py-3"
      >
        <button
          type="button"
          title={expandLabel}
          aria-label={expandLabel}
          onClick={onRequestClose ?? onToggleCollapse}
          className="inline-flex size-11 items-center justify-center rounded-md text-fg-muted transition-colors hover:bg-canvas-subtle hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
        >
          <PanelLeftOpen className="h-4 w-4" aria-hidden="true" />
        </button>
        <button
          type="button"
          title={t("agent.workspace.newConversation")}
          aria-label={t("agent.workspace.newConversation")}
          onClick={onNewConversation}
          disabled={disabled}
          className="inline-flex size-11 items-center justify-center rounded-md text-fg-muted transition-colors hover:bg-canvas-subtle hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring disabled:pointer-events-none disabled:opacity-50"
        >
          <Plus className="h-4 w-4" aria-hidden="true" />
        </button>
      </aside>
    );
  }

  return (
    <aside
      aria-label={t("agent.workspace.sidebarLabel")}
      className="flex w-64 shrink-0 flex-col border-r border-border-default bg-canvas-default"
    >
      <div className="flex h-14 shrink-0 items-center justify-between border-b border-border-default px-2">
        <span className="px-2 text-sm font-medium text-fg-default">
          {t("agent.workspace.sidebarLabel")}
        </span>
        <button
          type="button"
          aria-label={t("agent.workspace.collapseSidebar")}
          onClick={onToggleCollapse}
          className="inline-flex size-11 items-center justify-center rounded-md text-fg-muted transition-colors hover:bg-canvas-subtle hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
        >
          <PanelLeftClose className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>

      <div className="shrink-0 px-3 pb-2 pt-2">
        <button
          type="button"
          onClick={onNewConversation}
          disabled={disabled}
          className="inline-flex h-10 w-full items-center justify-center gap-1.5 rounded-md border border-border-default bg-canvas-default px-3 text-sm font-medium text-foreground transition-colors hover:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-ring disabled:pointer-events-none disabled:opacity-50"
        >
          <Plus className="h-4 w-4" aria-hidden="true" />
          {t("agent.workspace.newConversation")}
        </button>
      </div>

      <nav className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-2 pb-3">
        {loading && conversations.length === 0 ? (
          <div className="space-y-2 px-1 py-2" aria-busy="true">
            <div className="h-4 w-20 animate-pulse rounded bg-canvas-subtle" />
            <div className="h-10 w-full animate-pulse rounded bg-canvas-subtle" />
            <div className="h-10 w-full animate-pulse rounded bg-canvas-subtle" />
          </div>
        ) : groups.length === 0 ? (
          <p className="px-2 py-4 text-xs text-fg-muted">{t("agent.workspace.emptyConversations")}</p>
        ) : (
          <ul className="space-y-3">
            {groups.map((group) => (
              <li key={group.key}>
                <p className="px-2 pb-1 text-xs font-medium text-fg-muted">{group.label}</p>
                <ul className="space-y-0.5">
                  {group.items.map((conversation) => {
                    const active = conversation.id === activeId;
                    const renaming = renamingId === conversation.id;
                    const pinned = Boolean(conversation.pinned_at);
                    return (
                      <li key={conversation.id} className="group/item">
                        {renaming ? (
                          <input
                            autoFocus
                            aria-label={t("agent.workspace.renameInputLabel", { id: conversation.id })}
                            value={renameDraft}
                            maxLength={TITLE_MAX_RUNES * 2}
                            onChange={(event) => setRenameDraft(event.target.value)}
                            onKeyDown={(event) => {
                              if (event.key === "Enter") {
                                event.preventDefault();
                                commitRename(conversation.id);
                              } else if (event.key === "Escape") {
                                event.preventDefault();
                                setRenamingId(null);
                              }
                            }}
                            onBlur={() => commitRename(conversation.id)}
                            className="h-10 w-full rounded-md border border-ring bg-canvas-default px-2 text-sm text-fg-default focus:outline-none focus:ring-2 focus:ring-ring"
                          />
                        ) : (
                          <div
                            className={cn(
                              "flex h-10 w-full items-center gap-1 rounded-md pl-2 pr-1 text-left text-sm transition-colors",
                              active
                                ? "bg-accent-subtle font-medium text-accent-emphasis"
                                : "text-fg-default hover:bg-canvas-subtle",
                            )}
                          >
                            <button
                              type="button"
                              aria-label={
                                conversation.title
                                  ? conversation.title
                                  : t("agent.a11y.conversationItem", { id: conversation.id })
                              }
                              aria-current={active ? "page" : undefined}
                              onClick={() => onSelect(conversation.id)}
                              disabled={disabled}
                              className="flex min-w-0 flex-1 items-center gap-1.5 self-stretch rounded-md py-2 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50"
                            >
                              {pinned && <Pin className="h-3 w-3 shrink-0" aria-hidden="true" />}
                              <span className="truncate">
                                {conversation.title?.trim() || t("agent.workspace.untitled")}
                              </span>
                              <time
                                dateTime={conversation.updated_at}
                                className="ml-auto shrink-0 pl-1 text-xs font-normal text-fg-muted"
                              >
                                {formatTime(conversation.updated_at)}
                              </time>
                            </button>
                            <DropdownMenu>
                              <DropdownMenuTrigger
                                aria-label={t("agent.workspace.menuLabel", { id: conversation.id })}
                                className="inline-flex size-7 shrink-0 items-center justify-center rounded-md text-fg-muted transition-colors hover:bg-canvas-default hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring opacity-0 group-hover/item:opacity-100 focus-visible:opacity-100 data-[popup-open]:opacity-100"
                              >
                                <MoreHorizontal className="h-4 w-4" aria-hidden="true" />
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end">
                                <DropdownMenuItem onClick={() => startRename(conversation)}>
                                  {t("agent.workspace.menuRename")}
                                </DropdownMenuItem>
                                <DropdownMenuItem
                                  onClick={() => onTogglePin?.(conversation.id, !pinned)}
                                >
                                  {pinned
                                    ? t("agent.workspace.menuUnpin")
                                    : t("agent.workspace.menuPin")}
                                </DropdownMenuItem>
                                <DropdownMenuItem
                                  variant="destructive"
                                  onClick={() => onDelete?.(conversation.id)}
                                >
                                  <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
                                  {t("agent.workspace.menuDelete")}
                                </DropdownMenuItem>
                              </DropdownMenuContent>
                            </DropdownMenu>
                          </div>
                        )}
                      </li>
                    );
                  })}
                </ul>
              </li>
            ))}
          </ul>
        )}
      </nav>

      <p className="shrink-0 border-t border-border-default px-3 py-2 text-xs text-fg-muted">
        {t("agent.workspace.privacyHint")}
      </p>
    </aside>
  );
}

function formatTime(updatedAt?: string): string {
  if (!updatedAt) return "";
  const date = new Date(updatedAt);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit" }).format(date);
}
