"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { AlertCircle, BookOpen, Loader2, Menu, RotateCw, Send, Trash2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { useToast } from "@/components/ui/Toast";
import { useContentDetailOverlay } from "@/components/content/use-content-detail-overlay";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { getBrowserApiBase } from "@/lib/server-api";
import {
  startAgentStream,
  type AgentStreamCitation,
  type AgentStreamEvent,
  type AgentStreamTool,
} from "@/lib/agent-stream";
import { toAgentCitation, type AgentCitation } from "@/lib/agent";
import { AgentCitationCard } from "@/components/agent/AgentCitationCard";
import { AgentToolStatus } from "@/components/agent/AgentToolStatus";
import {
  AgentConversationSidebar,
  type AgentConversationSummary,
} from "@/components/agent/AgentConversationSidebar";
import { cn } from "@/lib/utils";

const SIDEBAR_STORAGE_KEY = "agentSidebarCollapsed";
const MAX_CONTEXT_MESSAGES = 10;
const STICKY_BOTTOM_THRESHOLD = 80;

export interface AgentWorkspaceProps {
  initialConversationId?: number;
  onCitationOpen?: (citation: AgentCitation) => void;
}

interface WorkspaceMessage {
  id: number;
  role: "user" | "assistant";
  content: string;
}

interface AgentMessageDTO {
  id: number;
  role: string;
  content?: string | null;
}

let nextMessageId = 1;

const SUGGESTION_KEYS = [
  "agent.workspace.suggestionLayout",
  "agent.workspace.suggestionMusic",
  "agent.workspace.suggestionMod",
] as const;

/**
 * Agent 工作台外壳（ui-spec `## Page: /agent`）：会话侧栏 + 主对话区。
 * 复用服务端流式契约（POST /api/v1/agent/chat/stream，surface=global）与
 * 会话生命周期契约（新对话不确认；清空历史 ConfirmModal + owner-scoped
 * DELETE，失败保留消息并回归触发焦点）。回答中的引用打开共享
 * ContentDetailOverlay（source=agent-citation），关闭后焦点回到引用卡片。
 */
export function AgentWorkspace({ initialConversationId, onCitationOpen }: AgentWorkspaceProps) {
  const t = useTranslations();
  const { toast } = useToast();
  const apiBase = getBrowserApiBase();

  const [conversations, setConversations] = useState<AgentConversationSummary[]>([]);
  const [conversationsLoading, setConversationsLoading] = useState(true);
  const [conversationsLoadError, setConversationsLoadError] = useState(false);
  const [activeId, setActiveId] = useState<number | null>(initialConversationId ?? null);
  const [messages, setMessages] = useState<WorkspaceMessage[]>([]);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [messagesLoadError, setMessagesLoadError] = useState(false);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const [turnError, setTurnError] = useState(false);
  const [stoppedNotice, setStoppedNotice] = useState(false);
  const [turnTools, setTurnTools] = useState<AgentStreamTool[]>([]);
  const [turnCitations, setTurnCitations] = useState<AgentStreamCitation[]>([]);
  const [lastAnswerKind, setLastAnswerKind] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [showJumpToLatest, setShowJumpToLatest] = useState(false);

  const transcriptRef = useRef<HTMLDivElement>(null);
  const composerRef = useRef<HTMLTextAreaElement>(null);
  const controllerRef = useRef<AbortController | null>(null);
  const atBottomRef = useRef(true);

  /* 共享浮层入口控制器：Agent 引用入口只保留来源参数差异。 */
  const { open: openCitationOverlay, overlayElement } = useContentDetailOverlay({
    source: "agent-citation",
  });

  const loadConversations = useCallback(async () => {
    setConversationsLoading(true);
    setConversationsLoadError(false);
    try {
      const data = await api.get<{ conversations?: AgentConversationSummary[] }>(
        "/api/v1/agent/conversations",
      );
      setConversations(data.conversations ?? []);
    } catch (error) {
      setConversationsLoadError(true);
      silentError(error, { component: "AgentWorkspace", action: "list conversations" });
    } finally {
      setConversationsLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadConversations();
  }, [loadConversations]);

  /* 侧栏折叠状态持久化（A1.6）。 */
  useEffect(() => {
    setCollapsed(window.localStorage.getItem(SIDEBAR_STORAGE_KEY) === "collapsed");
  }, []);

  /* 选中会话时加载服务端历史；新对话清空本地消息。 */
  useEffect(() => {
    setStoppedNotice(false);
    if (activeId === null) {
      setMessages([]);
      setMessagesLoading(false);
      setMessagesLoadError(false);
      return;
    }
    let cancelled = false;
    setMessagesLoading(true);
    setMessagesLoadError(false);
    api
      .get<{ messages?: AgentMessageDTO[] }>(`/api/v1/agent/conversations/${activeId}`)
      .then((data) => {
        if (cancelled) return;
        setMessages(
          (data.messages ?? []).map((message) => ({
            id: message.id,
            role: message.role === "user" ? "user" : "assistant",
            content: message.content ?? "",
          })),
        );
      })
      .catch((error) => {
        if (!cancelled) {
          setMessagesLoadError(true);
          silentError(error, { component: "AgentWorkspace", action: "load conversation" });
        }
      })
      .finally(() => {
        if (!cancelled) setMessagesLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [activeId]);

  /* 仅停留在底部附近时自动跟随流式内容；向上阅读后停止抢滚动。 */
  useEffect(() => {
    if (!atBottomRef.current) return;
    const transcript = transcriptRef.current;
    if (transcript) transcript.scrollTop = transcript.scrollHeight;
  }, [messages]);

  /* Esc 关闭移动端会话抽屉（不离开工作台）。 */
  useEffect(() => {
    if (!drawerOpen) return;
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setDrawerOpen(false);
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [drawerOpen]);

  function focusComposer() {
    window.requestAnimationFrame(() => {
      composerRef.current?.focus({ preventScroll: true });
    });
  }

  function handleTranscriptScroll() {
    const transcript = transcriptRef.current;
    if (!transcript) return;
    const nearBottom =
      transcript.scrollHeight - transcript.scrollTop - transcript.clientHeight <
      STICKY_BOTTOM_THRESHOLD;
    atBottomRef.current = nearBottom;
    setShowJumpToLatest(!nearBottom);
  }

  function scrollToLatest() {
    const transcript = transcriptRef.current;
    if (!transcript) return;
    transcript.scrollTop = transcript.scrollHeight;
    atBottomRef.current = true;
    setShowJumpToLatest(false);
  }

  const handleSelectConversation = useCallback((id: number) => {
    setActiveId(id);
    setDrawerOpen(false);
  }, []);

  const handleNewConversation = useCallback(() => {
    if (streaming) return;
    setActiveId(null);
    setMessages([]);
    setTurnCitations([]);
    setTurnTools([]);
    setTurnError(false);
    setStoppedNotice(false);
    setLastAnswerKind(null);
    setDrawerOpen(false);
    focusComposer();
  }, [streaming]);

  const handleClearConfirm = useCallback(async () => {
    if (activeId === null) return;
    try {
      await api.delete(`/api/v1/agent/conversations/${activeId}`);
      setActiveId(null);
      setMessages([]);
      setTurnCitations([]);
      setTurnTools([]);
      setTurnError(false);
      setStoppedNotice(false);
      setLastAnswerKind(null);
      toast("success", t("agent.workspace.clearHistorySuccess"));
      void loadConversations();
      focusComposer();
    } catch (error) {
      silentError(error, { component: "AgentWorkspace", action: "clear conversation" });
      toast("error", t("agent.workspace.clearHistoryFailed"));
    }
  }, [activeId, loadConversations, toast, t]);

  const handleCitationOpen = useCallback(
    (citation: AgentCitation, trigger: HTMLElement) => {
      openCitationOverlay(
        { contentId: citation.contentId, zone: citation.zone },
        trigger,
      );
      onCitationOpen?.(citation);
    },
    [onCitationOpen, openCitationOverlay],
  );

  const handleStreamEvent = useCallback(
    (event: AgentStreamEvent) => {
      switch (event.type) {
        case "delta": {
          if (!event.delta) break;
          const delta = event.delta;
          setMessages((previous) => {
            const next = [...previous];
            const last = next[next.length - 1];
            if (last && last.role === "assistant") {
              next[next.length - 1] = { ...last, content: last.content + delta };
            } else {
              next.push({ id: nextMessageId++, role: "assistant", content: delta });
            }
            return next;
          });
          break;
        }
        case "tool_status": {
          const tool = event.tool;
          if (tool) setTurnTools((previous) => [...previous, tool]);
          break;
        }
        case "citation": {
          const citation = event.citation;
          if (citation) {
            setTurnCitations((previous) => [...previous, citation]);
          }
          break;
        }
        case "done": {
          if (event.conversation_id) {
            setActiveId(event.conversation_id);
            void loadConversations();
          }
          if (event.citations) setTurnCitations(event.citations);
          setLastAnswerKind(event.answer_kind ?? null);
          setStreaming(false);
          break;
        }
        case "error": {
          setTurnError(true);
          setStreaming(false);
          controllerRef.current?.abort();
          break;
        }
        default:
          break;
      }
    },
    [loadConversations],
  );

  function handleSend(overrideMessage?: string) {
    const trimmed = (overrideMessage ?? input).trim();
    if (!trimmed || streaming) return;

    const userMessage: WorkspaceMessage = {
      id: nextMessageId++,
      role: "user",
      content: trimmed,
    };
    const history = [...messages, userMessage];
    setMessages(history);
    if (overrideMessage === undefined) setInput("");
    setTurnError(false);
    setStoppedNotice(false);
    setTurnTools([]);
    setTurnCitations([]);
    setLastAnswerKind(null);
    setStreaming(true);

    const controller = new AbortController();
    controllerRef.current = controller;
    void startAgentStream(
      fetch,
      `${apiBase}/agent/chat/stream`,
      {
        messages: history
          .slice(-MAX_CONTEXT_MESSAGES)
          .map((message) => ({ role: message.role, content: message.content })),
        context: { surface: "global" },
      },
      {
        onEvent: handleStreamEvent,
        onError: () => {
          setTurnError(true);
          setStreaming(false);
        },
        onClose: () => setStreaming(false),
      },
      controller.signal,
    );
  }

  function handleStop() {
    controllerRef.current?.abort();
    setStoppedNotice(true);
    setStreaming(false);
  }

  function handleRetry() {
    if (messages.length === 0) return;
    const lastUserMessage = [...messages].reverse().find((message) => message.role === "user");
    if (lastUserMessage) {
      setMessages((previous) => previous.slice(0, -1));
      handleSend(lastUserMessage.content);
    }
  }

  const lastMessageIsUser = messages[messages.length - 1]?.role === "user";
  const headerTitle =
    activeId === null
      ? t("agent.workspace.newConversation")
      : `${t("agent.workspace.untitled")} #${activeId}`;

  return (
    <main
      aria-label={t("agent.workspace.sidebarLabel")}
      className="flex h-[calc(100dvh-var(--header-h))] w-full overflow-hidden bg-canvas-default"
    >
      <div className="hidden min-[701px]:flex">
        <AgentConversationSidebar
          conversations={conversations}
          activeId={activeId}
          collapsed={collapsed}
          loading={conversationsLoading}
          disabled={streaming}
          onToggleCollapse={() => {
            const next = !collapsed;
            setCollapsed(next);
            window.localStorage.setItem(SIDEBAR_STORAGE_KEY, next ? "collapsed" : "expanded");
          }}
          onSelect={handleSelectConversation}
          onNewConversation={handleNewConversation}
        />
      </div>

      {drawerOpen && (
        <div className="fixed inset-0 z-50 min-[701px]:hidden" role="dialog" aria-modal="true" aria-label={t("agent.workspace.sidebarLabel")}>
          <button
            type="button"
            aria-label={t("agent.workspace.closeConversations")}
            className="absolute inset-0 bg-black/50"
            onClick={() => setDrawerOpen(false)}
          />
          <div className="relative h-full w-[85vw] max-w-[320px] bg-canvas-default shadow-md">
            <AgentConversationSidebar
              conversations={conversations}
              activeId={activeId}
              collapsed={false}
              loading={conversationsLoading}
              disabled={streaming}
              onToggleCollapse={() => setDrawerOpen(false)}
              onSelect={handleSelectConversation}
              onNewConversation={handleNewConversation}
              onRequestClose={() => setDrawerOpen(false)}
            />
          </div>
        </div>
      )}

      <section
        aria-label={t("agent.workspace.transcriptLabel")}
        className="relative flex min-w-0 flex-1 flex-col border-l border-border-default"
      >
        <header className="flex h-14 shrink-0 items-center gap-2 border-b border-border-default px-2">
          <button
            type="button"
            aria-label={t("agent.workspace.openConversations")}
            onClick={() => setDrawerOpen(true)}
            className="inline-flex size-11 shrink-0 items-center justify-center rounded-md text-fg-muted transition-colors hover:bg-canvas-subtle hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring min-[701px]:hidden"
          >
            <Menu className="h-4 w-4" aria-hidden="true" />
          </button>
          <h1 className="min-w-0 flex-1 truncate px-1 text-sm font-semibold text-fg-default">
            {headerTitle}
          </h1>
          {activeId !== null && !streaming && (
            <Button
              variant="ghost"
              size="sm"
              aria-label={t("agent.workspace.clearHistory")}
              onClick={() => setConfirmOpen(true)}
              className="h-10 shrink-0 text-fg-muted hover:text-destructive"
            >
              <Trash2 className="h-4 w-4" aria-hidden="true" />
              <span className="hidden sm:inline">{t("agent.workspace.clearHistory")}</span>
            </Button>
          )}
        </header>

        <div
          ref={transcriptRef}
          role="log"
          aria-live="polite"
          aria-label={t("agent.workspace.transcriptLabel")}
          data-slot="agent-transcript"
          onScroll={handleTranscriptScroll}
          className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-4"
        >
          {messagesLoading ? (
            <div className="space-y-3" aria-busy="true">
              <div className="h-10 w-2/3 animate-pulse rounded bg-canvas-subtle" />
              <div className="ml-auto h-10 w-2/3 animate-pulse rounded bg-canvas-subtle" />
              <div className="h-10 w-3/4 animate-pulse rounded bg-canvas-subtle" />
            </div>
          ) : messagesLoadError ? (
            <div className="mx-auto mt-16 max-w-sm rounded-md border border-border-destructive px-4 py-3 text-sm text-fg-default">
              {t("agent.workspace.conversationLoadFailed")}
            </div>
          ) : messages.length === 0 && !streaming ? (
            <section className="mx-auto flex max-w-md flex-col items-center px-4 pt-24 text-center">
              <div className="flex size-14 items-center justify-center rounded-full bg-accent-subtle text-accent-emphasis">
                <BookOpen className="size-6" aria-hidden="true" />
              </div>
              <h2 className="mt-4 text-base font-medium text-fg-default">
                {t("agent.workspace.emptyTitle")}
              </h2>
              <p className="mt-2 text-sm text-fg-muted">{t("agent.workspace.emptyDescription")}</p>
              <ul className="mt-5 flex flex-col gap-2">
                {SUGGESTION_KEYS.map((key) => (
                  <li key={key}>
                    <button
                      type="button"
                      onClick={() => handleSend(t(key))}
                      className="inline-flex w-full items-center justify-center rounded-md border border-border-default bg-canvas-default px-3 py-2 text-sm text-fg-muted transition-colors hover:bg-canvas-subtle hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                    >
                      {t(key)}
                    </button>
                  </li>
                ))}
              </ul>
            </section>
          ) : (
            <div className="mx-auto flex max-w-3xl flex-col gap-3">
              {messages.map((message) => (
                <div
                  key={`${message.id}-${message.role}`}
                  className={cn(
                    "max-w-[85%] whitespace-pre-wrap rounded-md px-3 py-2 text-sm",
                    message.role === "user"
                      ? "ml-auto bg-accent text-white"
                      : "bg-canvas-subtle text-fg-default",
                  )}
                >
                  {message.content || (
                    <span aria-label={t("agent.a11y.streamStatus")}>
                      <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
                    </span>
                  )}
                </div>
              ))}

              {turnTools.length > 0 && <AgentToolStatus tools={turnTools} />}

              {lastAnswerKind === "no_evidence" && (
                <div className="flex items-start gap-2 rounded-md border border-border-default bg-canvas-default px-3 py-2 text-sm text-fg-default">
                  <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-fg-muted" aria-hidden="true" />
                  <div>
                    <p className="font-medium">{t("agent.noEvidence.title")}</p>
                    <p className="mt-1 text-xs text-fg-muted">{t("agent.noEvidence.description")}</p>
                  </div>
                </div>
              )}

              {turnCitations.length > 0 && (
                <section aria-label={t("agent.citations.title")} className="mt-1">
                  <div className="flex items-baseline gap-2">
                    <h3 className="text-sm font-medium text-fg-default">{t("agent.citations.title")}</h3>
                    <span className="text-xs text-fg-muted">
                      {t("agent.citations.count", { count: turnCitations.length })}
                    </span>
                  </div>
                  <ul className="mt-2 grid gap-2 sm:grid-cols-2">
                    {turnCitations.map((citation, index) => (
                      <li key={`${citation.content_id}-${index}`}>
                        <AgentCitationCard
                          citation={toAgentCitation(citation)}
                          index={index}
                          onOpen={handleCitationOpen}
                        />
                      </li>
                    ))}
                  </ul>
                </section>
              )}

              {stoppedNotice && (
                <p className="text-xs text-fg-muted">{t("agent.workspace.stoppedNotice")}</p>
              )}

              {turnError && lastMessageIsUser && (
                <div className="flex items-start gap-2 rounded-md border border-border-destructive bg-canvas-default px-3 py-2 text-sm text-fg-default">
                  <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" aria-hidden="true" />
                  <div className="flex-1">
                    <p className="font-medium">{t("agent.workspace.errorTitle")}</p>
                  </div>
                  <Button variant="outline" size="sm" className="h-9" onClick={handleRetry}>
                    <RotateCw className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
                    {t("agent.workspace.errorRetry")}
                  </Button>
                </div>
              )}
            </div>
          )}
        </div>

        {showJumpToLatest && !streaming && (
          <div className="pointer-events-none absolute bottom-24 left-1/2 z-10 -translate-x-1/2">
            <Button
              variant="outline"
              size="sm"
              className="pointer-events-auto h-9"
              onClick={scrollToLatest}
            >
              <X className="mr-1.5 h-3.5 w-3.5 rotate-45" aria-hidden="true" />
              {t("agent.workspace.jumpToLatest")}
            </Button>
          </div>
        )}

        <form
          onSubmit={(event) => {
            event.preventDefault();
            handleSend();
          }}
          className="shrink-0 border-t border-border-default bg-canvas-default p-3"
        >
          <div className="flex items-end gap-2">
            <textarea
              ref={composerRef}
              rows={2}
              aria-label={t("agent.workspace.composerLabel")}
              placeholder={t("agent.workspace.inputPlaceholder")}
              value={input}
              onChange={(event) => setInput(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter" && !event.shiftKey) {
                  event.preventDefault();
                  handleSend();
                }
              }}
              disabled={streaming}
              className="min-h-11 flex-1 resize-none rounded-md border border-border-default bg-canvas-default px-3 py-2 text-sm text-fg-default placeholder:text-fg-muted focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring disabled:opacity-60"
            />
            {streaming ? (
              <Button
                type="button"
                size="sm"
                className="h-11 w-11 shrink-0 p-0"
                aria-label={t("agent.workspace.stopGenerating")}
                onClick={handleStop}
              >
                <X className="h-4 w-4" aria-hidden="true" />
              </Button>
            ) : (
              <Button
                type="submit"
                size="sm"
                className="h-11 w-11 shrink-0 p-0"
                aria-label={t("agent.workspace.sendMessage")}
                disabled={!input.trim()}
              >
                <Send className="h-4 w-4" aria-hidden="true" />
              </Button>
            )}
          </div>
          <p className="mt-1.5 px-1 text-xs text-fg-muted">{t("agent.workspace.composerHint")}</p>
        </form>
      </section>

      {overlayElement}

      <ConfirmModal
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t("agent.workspace.clearHistoryConfirmTitle")}
        description={t("agent.workspace.clearHistoryConfirmDescription")}
        confirmLabel={t("agent.workspace.clearHistoryConfirmAction")}
        onConfirm={() => handleClearConfirm()}
      />
    </main>
  );
}
