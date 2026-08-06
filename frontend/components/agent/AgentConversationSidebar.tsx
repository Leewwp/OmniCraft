"use client";

import { useMemo } from "react";
import { useTranslations } from "next-intl";
import { PanelLeftClose, PanelLeftOpen, Plus } from "lucide-react";
import { cn } from "@/lib/utils";

export interface AgentConversationSummary {
  id: number;
  context_type?: string;
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
  onRequestClose?: () => void;
  /** 会话全文搜索（A1.6）生产 API 由 Web Agent Productization 提供，本票不实现。 */
  disabled?: boolean;
}

type ConversationGroupKey = "today" | "yesterday" | "earlier";

function groupKey(updatedAt?: string): ConversationGroupKey {
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

const GROUP_KEYS: ConversationGroupKey[] = ["today", "yesterday", "earlier"];

/**
 * Agent 工作台会话侧栏（A1.6 契约）：折叠按钮 → 全宽「开启新对话」→ 会话历史
 * （保留非空时间分组）。可收为 56–64px 窄栏并持久化；会话全文搜索入口由
 * Web Agent Productization 计划提供生产数据源后接线。
 */
export function AgentConversationSidebar({
  conversations,
  activeId,
  collapsed,
  loading,
  onToggleCollapse,
  onSelect,
  onNewConversation,
  onRequestClose,
  disabled,
}: AgentConversationSidebarProps) {
  const t = useTranslations();

  const groups = useMemo(() => {
    const buckets = new Map<ConversationGroupKey, AgentConversationSummary[]>();
    for (const key of GROUP_KEYS) buckets.set(key, []);
    for (const conversation of conversations) {
      buckets.get(groupKey(conversation.updated_at))?.push(conversation);
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
                    return (
                      <li key={conversation.id}>
                        <button
                          type="button"
                          aria-label={t("agent.a11y.conversationItem", { id: conversation.id })}
                          aria-current={active ? "page" : undefined}
                          onClick={() => onSelect(conversation.id)}
                          disabled={disabled}
                          className={cn(
                            "flex h-10 w-full items-center justify-between gap-2 rounded-md px-2 text-left text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-ring disabled:pointer-events-none disabled:opacity-50",
                            active
                              ? "bg-accent-subtle font-medium text-accent-emphasis"
                              : "text-fg-default hover:bg-canvas-subtle",
                          )}
                        >
                          <span className="truncate">{t("agent.workspace.untitled")}</span>
                          <time
                            dateTime={conversation.updated_at}
                            className="shrink-0 text-xs text-fg-muted"
                          >
                            {formatTime(conversation.updated_at)}
                          </time>
                        </button>
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
