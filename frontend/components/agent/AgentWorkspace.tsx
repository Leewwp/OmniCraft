"use client";

import { Fragment, useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { AlertCircle, BookOpen, Copy, Loader2, Menu, RotateCw, Send, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { useToast } from "@/components/ui/Toast";
import { useContentDetailOverlay } from "@/components/content/use-content-detail-overlay";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { getBrowserApiBase } from "@/lib/server-api";
import {
  startAgentStream,
  AgentStreamError,
  type AgentStreamCitation,
  type AgentStreamEvent,
  type AgentStreamTool,
} from "@/lib/agent-stream";
import { MarkdownRenderer } from "@/components/content/MarkdownRenderer";
import { toAgentCitation, type AgentCitation } from "@/lib/agent";
import { AgentCitationList } from "@/components/agent/AgentCitationList";
import { AgentThinkingBlock } from "@/components/agent/AgentThinkingBlock";
import { AgentToolStatus } from "@/components/agent/AgentToolStatus";
import {
  AgentConversationSidebar,
  type AgentConversationSummary,
} from "@/components/agent/AgentConversationSidebar";
const SIDEBAR_STORAGE_KEY = "agentSidebarCollapsed";
const STICKY_BOTTOM_THRESHOLD = 80;
/** 输入自动增高上限：约 8 行（leading-6 = 24px × 8 + 上下 padding）后转内部滚动。 */
const COMPOSER_MAX_HEIGHT = 208;

export interface AgentWorkspaceProps {
  initialConversationId?: number;
  onCitationOpen?: (citation: AgentCitation) => void;
}

interface WorkspaceMessage {
  id: number;
  role: "user" | "assistant";
  content: string;
  /** A-02：think 行独立成消息（流式与历史回放同构），仅展示层。 */
  phase?: "think";
  moderationBlocked?: boolean;
}

interface AgentMessageDTO {
  id: number;
  role: string;
  content?: string | null;
  phase?: string;
  moderation?: string;
}

let nextMessageId = 1;

const SUGGESTION_KEYS = [
  "agent.workspace.suggestionLayout",
  "agent.workspace.suggestionMusic",
  "agent.workspace.suggestionMod",
] as const;

/**
 * Agent 工作台外壳（ui-spec `## Page: /agent`，A-06 DeepSeek 化）：全局导航下
 * 「会话历史栏 + 主对话区」。请求体走 A-01 续写契约（{conversation_id?, message}，
 * 上下文由服务端按 token 预算组装）；三层生成形态 = 思考折叠区（流式展开→完成
 * 折叠）+ 工具步骤区（检索词/命中数）+ 逐字正文（SSE v2）；行内 [n] 角标锚定到
 * 引用卡片（纯展示层）。侧边栏 ⋯ 菜单 = 重命名/置顶/删除（PATCH/DELETE owner-scoped）。
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
  const [turnThinking, setTurnThinking] = useState("");
  const [turnTools, setTurnTools] = useState<AgentStreamTool[]>([]);
  const [turnCitations, setTurnCitations] = useState<AgentStreamCitation[]>([]);
  const [turnDegraded, setTurnDegraded] = useState(false);
  const [lastAnswerKind, setLastAnswerKind] = useState<string | null>(null);
  const [turnErrorCode, setTurnErrorCode] = useState<string | null>(null);
  const [turnTraceId, setTurnTraceId] = useState<string | null>(null);
  const [turnUsage, setTurnUsage] = useState<{ prompt_tokens: number; completion_tokens: number } | null>(null);
  const [confirmDeleteId, setConfirmDeleteId] = useState<number | null>(null);
  const [collapsed, setCollapsed] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [showJumpToLatest, setShowJumpToLatest] = useState(false);
  const [highlightedCitation, setHighlightedCitation] = useState<number | null>(null);

  const transcriptRef = useRef<HTMLDivElement>(null);
  const composerRef = useRef<HTMLTextAreaElement>(null);
  const controllerRef = useRef<AbortController | null>(null);
  const activeQueryRef = useRef("");
  const fallbackRequestRef = useRef(0);
  const atBottomRef = useRef(true);
  /** 当前轮 answer 消息的客户端 id：工具步骤/思考块渲染在它之前（DeepSeek 顺序）。 */
  const turnAnswerIdRef = useRef<number | null>(null);
  const highlightTimerRef = useRef<number | null>(null);
  const turnCitationsRef = useRef<AgentStreamCitation[]>([]);

  useEffect(() => {
    turnCitationsRef.current = turnCitations;
  }, [turnCitations]);

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

  /* 输入自动增高：内容变化时贴着内容长高，8 行封顶转内部滚动。 */
  useEffect(() => {
    const composer = composerRef.current;
    if (!composer) return;
    composer.style.height = "auto";
    composer.style.height = `${Math.min(composer.scrollHeight, COMPOSER_MAX_HEIGHT)}px`;
  }, [input]);

  /* 引用高亮计时器清理。 */
  useEffect(() => {
    return () => {
      if (highlightTimerRef.current !== null) window.clearTimeout(highlightTimerRef.current);
    };
  }, []);

  /* 选中会话时加载服务端历史；新对话清空本地消息。think 行（phase="think"）
     以思考折叠块回放；A-05 blocked 行渲染占位提示。注意：done 事件会把新会话
     id 写入 activeId，此处不得重置轮内状态（citations/tools 属于刚完成的轮）。 */
  useEffect(() => {
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
        /* 服务端 think 行接管历史回放：清掉轮内思考态，避免与流式思考块双渲染。 */
        setTurnThinking("");
        setMessages(
          (data.messages ?? [])
            .filter(
              (message) =>
                message.role === "user" ||
                message.phase === "think" ||
                message.moderation === "blocked" ||
                (message.content ?? "").trim() !== "",
            )
            .map((message) =>
              message.moderation === "blocked"
                ? {
                    id: message.id,
                    role: "assistant" as const,
                    content: t("agent.workspace.messageHiddenByModeration"),
                    moderationBlocked: true,
                  }
                : message.phase === "think"
                  ? {
                      id: message.id,
                      role: "assistant" as const,
                      content: message.content ?? "",
                      phase: "think" as const,
                    }
                  : {
                      id: message.id,
                      role: message.role === "user" ? ("user" as const) : ("assistant" as const),
                      content: message.content ?? "",
                    },
            ),
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeId]);

  /* 仅停留在底部附近时自动跟随流式内容；向上阅读后停止抢滚动。 */
  useEffect(() => {
    if (!atBottomRef.current) return;
    const transcript = transcriptRef.current;
    if (transcript) transcript.scrollTop = transcript.scrollHeight;
  }, [messages, turnThinking, turnTools]);

  /* Esc 关闭移动端会话抽屉（不离开工作台）。 */
  useEffect(() => {
    if (!drawerOpen) return;
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setDrawerOpen(false);
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [drawerOpen]);

  function resetTurnExtras() {
    setTurnThinking("");
    setTurnTools([]);
    setTurnCitations([]);
    setTurnDegraded(false);
    setTurnError(false);
    setStoppedNotice(false);
    setLastAnswerKind(null);
    setTurnErrorCode(null);
    setTurnTraceId(null);
    setTurnUsage(null);
    setHighlightedCitation(null);
    turnAnswerIdRef.current = null;
  }

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
    fallbackRequestRef.current += 1;
    resetTurnExtras();
    setActiveId(id);
    setDrawerOpen(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleNewConversation = useCallback(() => {
    if (streaming) return;
    fallbackRequestRef.current += 1;
    setActiveId(null);
    setMessages([]);
    resetTurnExtras();
    setDrawerOpen(false);
    focusComposer();
  }, [streaming]);

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

  /* 行内 [n] 角标 → 高亮并滚动到对应引用卡片（纯展示层映射）。 */
  const handleCitationRef = useCallback((index: number) => {
    const citations = turnCitationsRef.current;
    if (index < 0 || index >= citations.length) return;
    const target = document.getElementById(`agent-citation-${index}`);
    target?.scrollIntoView({ behavior: "smooth", block: "nearest" });
    target?.focus({ preventScroll: true });
    setHighlightedCitation(index);
    if (highlightTimerRef.current !== null) window.clearTimeout(highlightTimerRef.current);
    highlightTimerRef.current = window.setTimeout(() => setHighlightedCitation(null), 1800);
  }, []);

  const handleRename = useCallback(
    async (id: number, title: string) => {
      try {
        const data = await api.patch<{ conversation?: AgentConversationSummary }>(
          `/api/v1/agent/conversations/${id}`,
          { title },
        );
        if (data.conversation) {
          setConversations((previous) =>
            previous.map((item) => (item.id === id ? { ...item, ...data.conversation } : item)),
          );
        }
      } catch (error) {
        silentError(error, { component: "AgentWorkspace", action: "rename conversation" });
        toast("error", t("agent.workspace.renameFailed"));
      }
    },
    [toast, t],
  );

  const handleTogglePin = useCallback(
    async (id: number, pinned: boolean) => {
      try {
        await api.patch(`/api/v1/agent/conversations/${id}`, { pinned });
        await loadConversations();
      } catch (error) {
        silentError(error, { component: "AgentWorkspace", action: "toggle pin" });
        toast("error", t("agent.workspace.pinFailed"));
      }
    },
    [loadConversations, toast, t],
  );

  const handleDeleteConfirm = useCallback(async () => {
    const id = confirmDeleteId;
    if (id === null) return;
    setConfirmDeleteId(null);
    try {
      await api.delete(`/api/v1/agent/conversations/${id}`);
      if (activeId === id) {
        fallbackRequestRef.current += 1;
        setActiveId(null);
        setMessages([]);
        resetTurnExtras();
        focusComposer();
      }
      toast("success", t("agent.workspace.deleteSuccess"));
      void loadConversations();
    } catch (error) {
      silentError(error, { component: "AgentWorkspace", action: "delete conversation" });
      toast("error", t("agent.workspace.deleteFailed"));
    }
  }, [activeId, confirmDeleteId, loadConversations, toast, t]);

  const loadKeywordFallback = useCallback(async (query: string, requestId: number) => {
    const trimmed = query.trim();
    if (!trimmed) return;
    try {
      const params = new URLSearchParams({ q: trimmed, page: "1", page_size: "10" });
      const data = await api.get<{ items?: unknown[]; contents?: unknown[] }>(
        `/api/v1/contents/search?${params.toString()}`,
      );
      if (fallbackRequestRef.current !== requestId) return;
      const rawItems = data.items ?? data.contents ?? [];
      const citations: AgentStreamCitation[] = [];
      const seen = new Set<number>();
      for (const raw of rawItems) {
        if (!raw || typeof raw !== "object") continue;
        const item = raw as Record<string, unknown>;
        const contentId = item.id;
        const title = item.title;
        const zone = item.zone;
        if (
          typeof contentId !== "number" ||
          !Number.isInteger(contentId) ||
          contentId <= 0 ||
          seen.has(contentId) ||
          typeof title !== "string" ||
          title.trim() === "" ||
          (zone !== "original" && zone !== "fanwork")
        ) {
          continue;
        }
        const citation: AgentStreamCitation = {
          content_id: contentId,
          title: title.trim(),
          zone,
        };
        const excerpt = item.excerpt ?? item.description;
        if (typeof excerpt === "string" && excerpt.trim() !== "") {
          citation.excerpt = excerpt.trim();
        }
        seen.add(contentId);
        citations.push(citation);
      }
      setTurnCitations(citations);
    } catch (error) {
      silentError(error, { component: "AgentWorkspace", action: "keyword fallback" });
      if (fallbackRequestRef.current === requestId) setTurnCitations([]);
    }
  }, []);

  const handleStreamEvent = useCallback(
    (event: AgentStreamEvent) => {
      switch (event.type) {
        case "start": {
          if (event.trace_id) setTurnTraceId(event.trace_id);
          break;
        }
        case "think_delta": {
          const delta = event.delta;
          if (!delta) break;
          setTurnThinking((previous) => previous + delta);
          break;
        }
        case "usage": {
          const usage = event.usage as { prompt_tokens?: unknown; completion_tokens?: unknown } | undefined;
          if (
            usage &&
            typeof usage.prompt_tokens === "number" &&
            typeof usage.completion_tokens === "number"
          ) {
            setTurnUsage({ prompt_tokens: usage.prompt_tokens, completion_tokens: usage.completion_tokens });
          }
          break;
        }
        case "delta": {
          if (!event.delta) break;
          const delta = event.delta;
          setMessages((previous) => {
            const next = [...previous];
            const last = next[next.length - 1];
            if (last && last.role === "assistant" && last.phase !== "think") {
              next[next.length - 1] = { ...last, content: last.content + delta };
            } else {
              const id = nextMessageId++;
              turnAnswerIdRef.current = id;
              next.push({ id, role: "assistant", content: delta });
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
          if (event.trace_id) setTurnTraceId(event.trace_id);
          if (event.conversation_id) {
            setActiveId(event.conversation_id);
            void loadConversations();
          }
          if (event.usage) setTurnUsage(event.usage);
          if (event.citations) setTurnCitations(event.citations);
          setTurnDegraded(Boolean(event.degraded));
          /* A-02 v2：done 是终态裁决。no_evidence/degraded 撤下已流出正文
             （模型总结不得展示）；正常轮 answer 存在则以服务端终稿替换已流出
             正文，answer 为空则移除空泡。 */
          if (event.degraded || event.answer_kind === "no_evidence") {
            turnAnswerIdRef.current = null;
            setMessages((previous) => {
              const next = [...previous];
              const last = next[next.length - 1];
              if (last && last.role === "assistant" && last.phase !== "think") next.splice(next.length - 1, 1);
              return next;
            });
          } else if (typeof event.answer === "string") {
            const finalAnswer: string = event.answer;
            if (finalAnswer === "") turnAnswerIdRef.current = null;
            setMessages((previous) => {
              const next = [...previous];
              const last = next[next.length - 1];
              if (last && last.role === "assistant" && last.phase !== "think") {
                if (finalAnswer === "") {
                  next.splice(next.length - 1, 1);
                } else {
                  next[next.length - 1] = { ...last, content: finalAnswer };
                }
              }
              return next;
            });
          }
          setLastAnswerKind(event.answer_kind ?? null);
          setStreaming(false);
          break;
        }
        case "error": {
          if (event.degraded && event.degraded_reason === "provider_error") {
            setTurnError(false);
            setTurnDegraded(true);
            turnAnswerIdRef.current = null;
            setMessages((previous) => {
              const next = [...previous];
              const last = next[next.length - 1];
              if (last && last.role === "assistant" && last.phase !== "think") next.splice(next.length - 1, 1);
              return next;
            });
            void loadKeywordFallback(activeQueryRef.current, fallbackRequestRef.current);
            setStreaming(false);
            controllerRef.current?.abort();
            break;
          }
          setTurnErrorCode(event.error_code ?? null);
          setTurnError(true);
          setStreaming(false);
          controllerRef.current?.abort();
          break;
        }
        default:
          break;
      }
    },
    [loadConversations, loadKeywordFallback],
  );

  /* 发起一轮对话（A-01 续写契约）：上下文由服务端组装，客户端只带
     conversation_id + message。regenerate 复用同一入口且不重复落用户行。 */
  function startTurn(query: string) {
    const body: Record<string, unknown> = {
      message: query,
      context: { surface: "global" },
    };
    if (activeId !== null) body.conversation_id = activeId;
    activeQueryRef.current = query;
    fallbackRequestRef.current += 1;
    resetTurnExtras();
    setStreaming(true);

    const controller = new AbortController();
    controllerRef.current = controller;
    void startAgentStream(fetch, `${apiBase}/agent/chat/stream`, body, {
      onEvent: handleStreamEvent,
      onError: (error) => {
        setTurnErrorCode(error instanceof AgentStreamError ? error.code ?? null : null);
        setTurnError(true);
        setStreaming(false);
      },
      onClose: () => setStreaming(false),
    }, controller.signal);
  }

  function handleSend(overrideMessage?: string) {
    const trimmed = (overrideMessage ?? input).trim();
    if (!trimmed || streaming) return;
    setMessages((previous) => [...previous, { id: nextMessageId++, role: "user", content: trimmed }]);
    if (overrideMessage === undefined) setInput("");
    startTurn(trimmed);
  }

  function handleStop() {
    controllerRef.current?.abort();
    setStoppedNotice(true);
    setStreaming(false);
  }

  /* 重新生成：保留到最后一跳用户消息为止的历史，撤下其后的 think/answer 行重发。 */
  function handleRegenerate() {
    if (streaming) return;
    let lastUserIndex = -1;
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      if (messages[index].role === "user") {
        lastUserIndex = index;
        break;
      }
    }
    if (lastUserIndex < 0) return;
    const query = messages[lastUserIndex].content;
    setMessages(messages.slice(0, lastUserIndex + 1));
    startTurn(query);
  }

  async function handleCopyMessage(content: string) {
    if (content.trim() === "") return;
    try {
      await navigator.clipboard.writeText(content);
      toast("success", t("agent.workspace.copySuccess"));
    } catch {
      toast("error", t("agent.workspace.copyFailed"));
    }
  }

  const activeConversation = conversations.find((conversation) => conversation.id === activeId) ?? null;
  const headerTitle =
    activeId === null
      ? t("agent.workspace.newConversation")
      : activeConversation?.title?.trim() || `${t("agent.workspace.untitled")} #${activeId}`;
  const turnExtrasPresent = turnThinking !== "" || turnTools.length > 0;
  const lastAnswerIndex = messages.map((message) => message.role).lastIndexOf("assistant");
  const lastMessageIsUser = messages[messages.length - 1]?.role === "user";

  function renderTurnExtrasBefore(message: WorkspaceMessage, index: number) {
    /* 当前轮的 answer 消息前渲染 思考块+工具步骤。done 后服务端历史回载会替换
       流式 answer 行（客户端 id 失联）——回退锚定到轮尾最后一条 answer 行；
       ref 为 null（no_evidence/degraded 撤答或流中尚无 answer）时由 map 后的
       轮尾兜底块渲染，此处不重复。 */
    const isTurnAnswer =
      turnAnswerIdRef.current !== null && message.id === turnAnswerIdRef.current;
    const isOrphanAnchor =
      turnAnswerIdRef.current !== null &&
      !isTurnAnswer &&
      index === lastAnswerIndex &&
      !streaming &&
      message.role === "assistant" &&
      message.phase !== "think" &&
      !message.moderationBlocked;
    if (!isTurnAnswer && !isOrphanAnchor) return null;
    return (
      <Fragment key={`turn-extras-${message.id}`}>
        {turnThinking !== "" && <AgentThinkingBlock content={turnThinking} streaming={streaming} />}
        {turnTools.length > 0 && <AgentToolStatus tools={turnTools} live={streaming} />}
      </Fragment>
    );
  }

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
          onRename={handleRename}
          onTogglePin={handleTogglePin}
          onDelete={(id) => setConfirmDeleteId(id)}
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
          <div className="relative h-full w-[85vw] max-w-[320px] bg-card shadow-md">
            <AgentConversationSidebar
              conversations={conversations}
              activeId={activeId}
              collapsed={false}
              loading={conversationsLoading}
              disabled={streaming}
              onToggleCollapse={() => setDrawerOpen(false)}
              onSelect={handleSelectConversation}
              onNewConversation={handleNewConversation}
              onRename={handleRename}
              onTogglePin={handleTogglePin}
              onDelete={(id) => setConfirmDeleteId(id)}
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
          ) : messages.length === 0 && !streaming && !turnExtrasPresent ? (
            <section className="mx-auto flex max-w-md flex-col items-center px-4 pt-24 text-center">
              <div className="flex size-14 items-center justify-center rounded-full bg-accent-subtle text-accent-emphasis">
                <BookOpen className="size-6" aria-hidden="true" />
              </div>
              <h2 className="mt-4 text-base font-medium text-fg-default">
                {t("agent.workspace.emptyTitle")}
              </h2>
              <p className="mt-2 text-sm text-fg-muted">{t("agent.workspace.emptyDescription")}</p>
              <ul className="mt-5 flex flex-wrap items-center justify-center gap-2">
                {SUGGESTION_KEYS.map((key) => (
                  <li key={key}>
                    <button
                      type="button"
                      onClick={() => handleSend(t(key))}
                      className="inline-flex items-center rounded-full border border-border-default bg-card px-3 py-1.5 text-sm text-fg-muted transition-colors duration-150 hover:border-border-strong hover:bg-canvas-subtle hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                    >
                      {t(key)}
                    </button>
                  </li>
                ))}
              </ul>
            </section>
          ) : (
            <div className="mx-auto flex max-w-3xl flex-col gap-3">
              {messages.map((message, index) => (
                <Fragment key={`${message.id}-${message.role}-${message.phase ?? "body"}`}>
                  {renderTurnExtrasBefore(message, index)}
                  {message.phase === "think" ? (
                    <AgentThinkingBlock content={message.content} streaming={false} />
                  ) : message.role === "user" ? (
                    <div className="ml-auto max-w-[85%] whitespace-pre-wrap rounded-md bg-primary px-3 py-2 text-sm text-primary-foreground">
                      {message.content}
                    </div>
                  ) : message.moderationBlocked ? (
                    <div className="max-w-[85%] rounded-md border border-border-default bg-card px-3 py-2 text-sm text-fg-muted">
                      {message.content}
                    </div>
                  ) : (
                    <div className="group/message max-w-[85%] rounded-md bg-canvas-subtle px-3 py-2 text-sm">
                      {message.content ? (
                        // 受控渲染：react-markdown 未接 rehype-raw，原始 HTML 一律转义（T20 核验）
                        <MarkdownRenderer
                          content={message.content}
                          onCitationRef={handleCitationRef}
                          citationCount={turnCitations.length}
                        />
                      ) : (
                        <span aria-label={t("agent.a11y.streamStatus")}>
                          <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
                        </span>
                      )}
                      {!streaming && message.content && index === lastAnswerIndex && (
                        <div className="mt-1.5 flex items-center gap-1 opacity-0 transition-opacity duration-150 group-hover/message:opacity-100 focus-within:opacity-100">
                          <button
                            type="button"
                            aria-label={t("agent.workspace.copyMessage")}
                            onClick={() => void handleCopyMessage(message.content)}
                            className="inline-flex size-7 items-center justify-center rounded-md text-fg-muted transition-colors hover:bg-canvas-default hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                          >
                            <Copy className="h-3.5 w-3.5" aria-hidden="true" />
                          </button>
                          <button
                            type="button"
                            aria-label={t("agent.workspace.regenerate")}
                            onClick={handleRegenerate}
                            className="inline-flex size-7 items-center justify-center rounded-md text-fg-muted transition-colors hover:bg-canvas-default hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                          >
                            <RotateCw className="h-3.5 w-3.5" aria-hidden="true" />
                          </button>
                        </div>
                      )}
                    </div>
                  )}
                </Fragment>
              ))}

              {/* 轮内尚无 answer 消息时（纯思考/工具中或提前停止），轮尾兜底渲染。 */}
              {turnAnswerIdRef.current === null && (streaming || turnExtrasPresent) && (
                <Fragment key="turn-extras-tail">
                  {turnThinking !== "" && <AgentThinkingBlock content={turnThinking} streaming={streaming} />}
                  {turnTools.length > 0 && <AgentToolStatus tools={turnTools} live={streaming} />}
                </Fragment>
              )}

              {!streaming && (turnUsage || turnTraceId) && (
                <details className="max-w-[85%] rounded-md border border-border-default bg-card px-3 py-1.5 text-xs text-fg-muted">
                  <summary className="cursor-pointer select-none">{t("agent.workspace.turnDetails")}</summary>
                  {turnUsage && (
                    <p className="mt-1.5">
                      {t("agent.workspace.turnUsage", {
                        prompt: turnUsage.prompt_tokens,
                        completion: turnUsage.completion_tokens,
                      })}
                    </p>
                  )}
                  {turnTraceId && (
                    <p className="mt-1">
                      {t("agent.workspace.traceLabel")}: <span className="font-mono">{turnTraceId}</span>
                    </p>
                  )}
                </details>
              )}

              {lastAnswerKind === "no_evidence" && (
                <div className="flex items-start gap-2 rounded-md border border-border-default bg-card px-3 py-2 text-sm text-fg-default">
                  <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-fg-muted" aria-hidden="true" />
                  <div>
                    <p className="font-medium">{t("agent.noEvidence.title")}</p>
                    <p className="mt-1 text-xs text-fg-muted">{t("agent.noEvidence.description")}</p>
                    {activeQueryRef.current && (
                      <Link
                        href={`/search?q=${encodeURIComponent(activeQueryRef.current)}`}
                        className="mt-2 inline-flex min-h-11 items-center text-sm font-medium text-accent-emphasis underline-offset-2 hover:underline focus:outline-none focus:ring-2 focus:ring-ring"
                      >
                        {t("agent.noEvidence.searchCta")}
                      </Link>
                    )}
                  </div>
                </div>
              )}

              {turnDegraded && (
                <div className="flex items-start gap-2 rounded-md border border-border-default bg-card px-3 py-2 text-sm text-fg-default">
                  <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-fg-muted" aria-hidden="true" />
                  <div>
                    <p className="font-medium">{t("agent.degraded.title")}</p>
                    <p className="mt-1 text-xs text-fg-muted">{t("agent.degraded.description")}</p>
                    {activeQueryRef.current && (
                      <Link
                        href={`/search?q=${encodeURIComponent(activeQueryRef.current)}`}
                        className="mt-2 inline-flex min-h-11 items-center text-sm font-medium text-accent-emphasis underline-offset-2 hover:underline focus:outline-none focus:ring-2 focus:ring-ring"
                      >
                        {t("agent.noEvidence.searchCta")}
                      </Link>
                    )}
                  </div>
                </div>
              )}

              <AgentCitationList
                citations={turnCitations.map(toAgentCitation)}
                onOpen={handleCitationOpen}
                highlightedIndex={highlightedCitation}
              />

              {stoppedNotice && (
                <p className="text-xs text-fg-muted">{t("agent.workspace.stoppedNotice")}</p>
              )}

              {turnError && lastMessageIsUser && (
                <div className="flex items-start gap-2 rounded-md border border-border-destructive bg-card px-3 py-2 text-sm text-fg-default">
                  <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" aria-hidden="true" />
                  <div className="flex-1">
                    {turnErrorCode === "AGENT_RATE_LIMIT_EXCEEDED" ? (
                      <>
                        <p className="font-medium">{t("agent.workspace.rateLimitTitle")}</p>
                        <p className="mt-1 text-xs text-fg-muted">{t("agent.workspace.rateLimitHint")}</p>
                      </>
                    ) : (
                      <p className="font-medium">{t("agent.workspace.errorTitle")}</p>
                    )}
                    {turnTraceId && (
                      <p className="mt-1 text-xs text-fg-muted">
                        {t("agent.workspace.traceLabel")}: <span className="font-mono">{turnTraceId}</span>
                      </p>
                    )}
                  </div>
                  {turnErrorCode !== "AGENT_RATE_LIMIT_EXCEEDED" && (
                    <Button variant="outline" size="sm" className="h-9" onClick={handleRegenerate}>
                      <RotateCw className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
                      {t("agent.workspace.errorRetry")}
                    </Button>
                  )}
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
              rows={1}
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
              style={{ maxHeight: COMPOSER_MAX_HEIGHT }}
              className="min-h-11 flex-1 resize-none self-auto overflow-y-auto rounded-md border border-border-default bg-canvas-default px-3 py-2 text-sm leading-6 text-fg-default placeholder:text-fg-muted focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring disabled:opacity-60"
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
        open={confirmDeleteId !== null}
        onOpenChange={(open) => {
          if (!open) setConfirmDeleteId(null);
        }}
        title={t("agent.workspace.deleteConfirmTitle")}
        description={t("agent.workspace.deleteConfirmDescription")}
        confirmLabel={t("agent.workspace.deleteConfirmAction")}
        onConfirm={() => void handleDeleteConfirm()}
      />
    </main>
  );
}
