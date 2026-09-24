"use client";

import { Fragment, useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import Link from "next/link";
import { AlertCircle, ArrowDown, BookOpen, Brain, Copy, Menu, RotateCw } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Composer } from "@/components/ui/composer";
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
} from "@/lib/agent-stream";
import { MarkdownRenderer } from "@/components/content/MarkdownRenderer";
import { toAgentCitation, type AgentCitation } from "@/lib/agent";
import {
  applyKeywordFallbackCitations,
  closeAgentStream,
  createAgentTurn,
  mapAgentHistoryToTurns,
  mergeTerminalIntoLastTurn,
  reduceAgentTurn,
  stopAgentTurn,
  type AgentHistoryMessageDTO,
  type AgentTurn,
} from "@/lib/agent-turn";
import { AgentCitationList } from "@/components/agent/AgentCitationList";
import { AgentThinkingBlock } from "@/components/agent/AgentThinkingBlock";
import { AgentToolStatus } from "@/components/agent/AgentToolStatus";
import {
  AgentConversationSidebar,
  type AgentConversationSummary,
} from "@/components/agent/AgentConversationSidebar";
import { AgentFollowUpChips } from "@/components/agent/AgentFollowUpChips";
const SIDEBAR_STORAGE_KEY = "agentSidebarCollapsed";
/** #539：深度思考开关持久化（localStorage，随会话恢复用户偏好）。 */
const DEEP_THINK_STORAGE_KEY = "agentDeepThink";
/** #545：模型偏好持久化（注册表 id；失效 id 由选项列表校验兜底）。 */
const MODEL_STORAGE_KEY = "agentModelPref";
const STICKY_BOTTOM_THRESHOLD = 80;
/** 输入自动增高上限：约 8 行（leading-6 = 24px × 8 + 上下 padding）后转内部滚动。 */

export interface AgentWorkspaceProps {
  initialConversationId?: number;
  /** A-07：外部入口（搜索页「问 AI 助手」/agent?q=）带来的首轮预填问题，仅首挂载生效。 */
  initialQuery?: string;
  onCitationOpen?: (citation: AgentCitation) => void;
}

/** 本地轮 id：crypto UUID（无模块级计数器——跨会话重挂载不漂移）。 */
function newLocalTurnId(): string {
  return `live-${typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(36).slice(2)}`}`;
}

/**
 * 末个 assistant 源块的定位（旧 lastAnswerIndex 的轮树等价）：按渲染顺序扫过
 * 各轮相块（think/tools/占位/正文），仅当末块为正文块时挂消息操作行——
 * think/tools 行与 moderation 占位行在旧实现里同样不挂操作行。
 */
function lastAnswerTurnId(turns: AgentTurn[]): string | null {
  let candidate: string | null = null;
  let lastBlockIsAnswer = false;
  for (const turn of turns) {
    if (turn.segments.length > 0) lastBlockIsAnswer = false;
    if (turn.moderationBlocked) {
      lastBlockIsAnswer = false;
    } else if (turn.answer !== "") {
      lastBlockIsAnswer = true;
      candidate = turn.id;
    }
  }
  return lastBlockIsAnswer ? candidate : null;
}

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
 * #663：树 = Turn[]（TurnModel 双入口装配），活动轮单一状态独立渲染；首轮
 * 活动轮活到历史回载替换树之后（在途不闪空），续问轮 done 终局后 commit 进树。
 */
export function AgentWorkspace({ initialConversationId, initialQuery, onCitationOpen }: AgentWorkspaceProps) {
  const t = useTranslations();
  // SP-21 T4：管理员可从会话轮直接跳转链路详情（SSE trace_id ↔ 落库一致）。
  const { user: authUser } = useAuth();
  const isAdmin = authUser?.role === "admin";
  const { toast } = useToast();
  const apiBase = getBrowserApiBase();

  const [conversations, setConversations] = useState<AgentConversationSummary[]>([]);
  const [conversationsLoading, setConversationsLoading] = useState(true);
  const [conversationsLoadError, setConversationsLoadError] = useState(false);
  const [activeId, setActiveId] = useState<number | null>(initialConversationId ?? null);
  const [turns, setTurns] = useState<AgentTurn[]>([]);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [messagesLoadError, setMessagesLoadError] = useState(false);
  const [input, setInput] = useState(() => (initialQuery ?? "").trim());
  /* 活动轮单一状态：live 事件经 reduceAgentTurn 归约；轮内展示状态
     （思考/工具/引用/降级/空轮/错误码/trace/用量/追问/停止）全部住在这里。 */
  const [activeTurn, setActiveTurn] = useState<AgentTurn | null>(null);
  const streaming = activeTurn?.streaming ?? false;
  const [confirmDeleteId, setConfirmDeleteId] = useState<number | null>(null);
  const [collapsed, setCollapsed] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [showJumpToLatest, setShowJumpToLatest] = useState(false);

  const transcriptRef = useRef<HTMLDivElement>(null);
  const composerRef = useRef<HTMLTextAreaElement>(null);
  const controllerRef = useRef<AbortController | null>(null);
  /** provider 降级关键词回退的请求代（防过期响应写回新轮）。 */
  const fallbackRequestRef = useRef(0);
  const atBottomRef = useRef(true);

  /* 共享浮层入口控制器：Agent 引用入口只保留来源参数差异。 */
  const { open: openCitationOverlay, overlayElement } = useContentDetailOverlay({
    source: "agent-citation",
  });
  /* SP-19 G2-1（Q5）：IP 引用不走内容浮层，router.push 落 /ip/[id] 详情页。 */
  const router = useRouter();

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

  /* #539：深度思考开关——默认关（快、省 token），开启后请求带 deep_think，
     由后端映射到 provider 的思考控制（MiniMax M3 thinking.type）。 */
  const [deepThink, setDeepThink] = useState(false);
  useEffect(() => {
    setDeepThink(window.localStorage.getItem(DEEP_THINK_STORAGE_KEY) === "on");
  }, []);

  /* #545：模型选择——拉取注册表（>1 供给才渲染选择器；拉取失败静默降级为
     单供给形态，不打扰对话主链路）。偏好持久 localStorage，失效 id 丢弃。 */
  const [modelOptions, setModelOptions] = useState<{ id: string; display_name: string }[]>([]);
  const [modelPref, setModelPref] = useState("");
  useEffect(() => {
    api
      .get<{ models?: { id: string; display_name: string }[] }>("/api/v1/agent/models")
      .then((data) => {
        const options = data.models ?? [];
        setModelOptions(options);
        const saved = window.localStorage.getItem(MODEL_STORAGE_KEY);
        if (saved && options.some((option) => option.id === saved)) setModelPref(saved);
      })
      .catch(() => {});
  }, []);
  function toggleDeepThink() {
    const next = !deepThink;
    window.localStorage.setItem(DEEP_THINK_STORAGE_KEY, next ? "on" : "off");
    setDeepThink(next);
  }
  const deepThinkToggle = (
    <button
      type="button"
      aria-pressed={deepThink}
      aria-label={t("agent.workspace.deepThink")}
      title={t("agent.workspace.deepThinkHint")}
      onClick={toggleDeepThink}
      className={cn(
        "inline-flex h-7 items-center gap-1 rounded-md px-2 text-xs transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        deepThink ? "bg-primary/10 text-primary" : "text-fg-subtle hover:text-fg-default",
      )}
    >
      <Brain className="h-3.5 w-3.5" aria-hidden="true" />
      <span>{t("agent.workspace.deepThink")}</span>
    </button>
  );
  const modelSelector =
    modelOptions.length > 1 ? (
      <select
        aria-label={t("agent.workspace.modelLabel")}
        value={modelPref || modelOptions[0].id}
        onChange={(event) => {
          setModelPref(event.target.value);
          window.localStorage.setItem(MODEL_STORAGE_KEY, event.target.value);
        }}
        className="h-7 rounded-md border border-border-default bg-canvas-default px-1.5 text-xs text-fg-default focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        {modelOptions.map((option) => (
          <option key={option.id} value={option.id}>
            {option.display_name}
          </option>
        ))}
      </select>
    ) : null;

  /* 选中会话时加载服务端历史（server-authoritative：moderation 脱敏与跨端一致
     的权威方向）；新对话清空轮树。done 事件会把新会话 id 写入 activeId 触发本
     effect——首轮活动轮活到此处：历史行接管树之后活动轮清空，其终态字段
     （追问/用量/trace/通知）并入树尾轮跨重载存活（消灭在途闪烁窗口）。 */
  useEffect(() => {
    if (activeId === null) {
      setTurns([]);
      setMessagesLoading(false);
      setMessagesLoadError(false);
      return;
    }
    let cancelled = false;
    setMessagesLoading(true);
    setMessagesLoadError(false);
    api
      .get<{ messages?: AgentHistoryMessageDTO[] }>(`/api/v1/agent/conversations/${activeId}`)
      .then((data) => {
        if (cancelled) return;
        const mapped = mapAgentHistoryToTurns(
          data.messages ?? [],
          t("agent.workspace.messageHiddenByModeration"),
        );
        /* 闭包内 activeTurn 与 activeId 同批更新（done 事件一次 setState 提交），
           此处即终局活动轮。 */
        const pending = activeTurn;
        if (pending && pending.settled && pending.firstRound && mapped.length > 0) {
          setTurns(mergeTerminalIntoLastTurn(mapped, pending.terminal));
          setActiveTurn(null);
        } else {
          setTurns(mapped);
        }
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

  /* 续问轮终局（done/error/stop/关流）：commit 进树后清空活动轮——语义等价
     旧「尾行终稿替换 + 轮级态保留」（终态随轮入树，树尾轮渲染到下一轮开始）。 */
  useEffect(() => {
    if (!activeTurn || !activeTurn.settled || activeTurn.firstRound) return;
    setTurns((previous) => [...previous, activeTurn]);
    setActiveTurn(null);
  }, [activeTurn]);

  /* 仅停留在底部附近时自动跟随流式内容；向上阅读后停止抢滚动。 */
  useEffect(() => {
    if (!atBottomRef.current) return;
    const transcript = transcriptRef.current;
    if (transcript) transcript.scrollTop = transcript.scrollHeight;
  }, [turns, activeTurn]);

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
    fallbackRequestRef.current += 1;
    setActiveTurn(null);
    setActiveId(id);
    setDrawerOpen(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleNewConversation = useCallback(() => {
    if (streaming) return;
    fallbackRequestRef.current += 1;
    setActiveId(null);
    setTurns([]);
    setActiveTurn(null);
    setDrawerOpen(false);
    focusComposer();
  }, [streaming]);

  const handleCitationOpen = useCallback(
    (citation: AgentCitation, trigger: HTMLElement) => {
      /* SP-19 G2-1（Q5）：IP 引用卡点击分流到 IP 详情页，不开内容浮层。 */
      if (citation.zone === "ip") {
        router.push(`/ip/${citation.contentId}`);
      } else {
        openCitationOverlay(
          { contentId: citation.contentId, zone: citation.zone },
          trigger,
        );
      }
      onCitationOpen?.(citation);
    },
    [onCitationOpen, openCitationOverlay, router],
  );

  /* 行内 [n] 角标 → 直接打开共享内容浮层（2026-09-06 实测修复：原先只高亮
     滚动到底部引用卡片，与其它页面「点链接开浮窗」的契约不一致）。index 为
     0 基；citations 由调用方按轮传入（活动轮进行中的答案回落轮内流式引用；
     历史等无引用轮传空数组即不响应）。zone="ip" 的角标与卡片同分流（Q5）。 */
  const handleCitationRef = useCallback(
    (index: number, citations: AgentStreamCitation[]) => {
      const citation = citations[index];
      if (!citation || citation.content_id <= 0) return;
      if (citation.zone === "ip") {
        router.push(`/ip/${citation.content_id}`);
        return;
      }
      openCitationOverlay(
        {
          contentId: citation.content_id,
          zone: citation.zone === "fanwork" ? "fanwork" : "original",
        },
        null,
      );
    },
    [openCitationOverlay, router],
  );

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
        setTurns([]);
        setActiveTurn(null);
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
      setActiveTurn((previous) => (previous ? applyKeywordFallbackCitations(previous, citations) : previous));
    } catch (error) {
      silentError(error, { component: "AgentWorkspace", action: "keyword fallback" });
      if (fallbackRequestRef.current === requestId) {
        setActiveTurn((previous) => (previous ? applyKeywordFallbackCitations(previous, []) : previous));
      }
    }
  }, []);

  /* live 事件：形状归约进 TurnModel reducer（纯函数）；此处只留回合策略副作用
     （会话 id 写入/会话列表刷新/AbortController）。 */
  const handleStreamEvent = useCallback(
    (event: AgentStreamEvent) => {
      setActiveTurn((previous) => (previous ? reduceAgentTurn(previous, event) : previous));
      if (event.type === "done") {
        if (event.conversation_id) {
          setActiveId(event.conversation_id);
          void loadConversations();
        }
        return;
      }
      if (event.type === "error") {
        controllerRef.current?.abort();
      }
    },
    [loadConversations],
  );

  /* provider 降级关键词回退：由轮终态的待回退标记驱动（标记随回退结果落轮
     清除）；查询取自轮自身，无跨状态查询 ref。 */
  useEffect(() => {
    if (!activeTurn?.terminal.needsKeywordFallback) return;
    void loadKeywordFallback(activeTurn.query, fallbackRequestRef.current);
  }, [activeTurn, loadKeywordFallback]);

  /* 发起一轮对话（A-01 续写契约）：上下文由服务端组装，客户端只带
     conversation_id + message。regenerate 复用同一入口且不重复落提问行。
     残留活动轮（首轮终态、回载未落地）先 commit 进树再开新轮。 */
  function startTurn(query: string) {
    const body: Record<string, unknown> = {
      message: query,
      context: { surface: "global" },
      deep_think: deepThink,
    };
    if (modelPref) body.model = modelPref;
    if (activeId !== null) body.conversation_id = activeId;
    fallbackRequestRef.current += 1;
    if (activeTurn) {
      setTurns((previous) => [...previous, activeTurn]);
    }
    setActiveTurn(createAgentTurn(query, { id: newLocalTurnId(), firstRound: activeId === null }));

    const controller = new AbortController();
    controllerRef.current = controller;
    void startAgentStream(fetch, `${apiBase}/agent/chat/stream`, body, {
      onEvent: handleStreamEvent,
      onError: (error) => {
        const code = error instanceof AgentStreamError ? error.code : undefined;
        setActiveTurn((previous) =>
          previous ? reduceAgentTurn(previous, { type: "error", error_code: code }) : previous,
        );
      },
      onClose: () => {
        setActiveTurn((previous) => (previous ? closeAgentStream(previous) : previous));
      },
    }, controller.signal);
  }

  function handleSend(overrideMessage?: string) {
    const trimmed = (overrideMessage ?? input).trim();
    if (!trimmed || streaming) return;
    if (overrideMessage === undefined) setInput("");
    startTurn(trimmed);
  }

  function handleStop() {
    controllerRef.current?.abort();
    setActiveTurn((previous) => (previous ? stopAgentTurn(previous) : previous));
  }

  /* SP-15 B #435：追问药丸点击 = 仅填入并聚焦 composer，不自动发送
     （用户回车确认，防误触）。 */
  function handleFollowUpFill(query: string) {
    setInput(query);
    composerRef.current?.focus({ preventScroll: true });
  }

  /* 重新生成：活动轮残留（首轮终态、回载未落地）直接重发；否则撤下树尾整轮
     （提问行由新一轮活动轮接替渲染）重发同一查询。 */
  function handleRegenerate() {
    if (streaming) return;
    if (activeTurn) {
      const query = activeTurn.query;
      setActiveTurn(null);
      startTurn(query);
      return;
    }
    if (turns.length === 0) return;
    const last = turns[turns.length - 1];
    setTurns(turns.slice(0, -1));
    startTurn(last.query);
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
  /* #416 O2：主区标题只来自会话标题（与左侧列表同源）；未选会话或空态
     一律不渲染标题（「开启新对话」固定文案退出主区）。 */
  const headerTitle =
    activeConversation?.title?.trim() || `${t("agent.workspace.untitled")} #${activeId}`;

  const [editingTitle, setEditingTitle] = useState(false);
  const [titleDraft, setTitleDraft] = useState("");

  /* #416 O2：标题原地编辑——复用侧栏重命名契约（非空、≤50 字符）；
     Enter/失焦保存、Esc 取消、空白不保存恢复原标题。 */
  function startTitleEdit() {
    if (activeId === null || emptyConversation) return;
    setEditingTitle(true);
    setTitleDraft(activeConversation?.title ?? "");
  }

  function commitTitleEdit() {
    setEditingTitle(false);
    const trimmed = titleDraft.trim().slice(0, 50);
    if (activeId === null || trimmed === "" || trimmed === activeConversation?.title) return;
    void handleRename(activeId, trimmed);
  }

  function cancelTitleEdit() {
    setEditingTitle(false);
  }

  /* #416 O2：空态判定 = 当前会话无任何轮次（含未选会话与已选空会话）。
     空态下主区不渲染标题、主体中部偏下渲染引导 + 大号输入框（同一表单
     组件的两种布局形态）。 */
  const emptyConversation =
    !messagesLoading && !messagesLoadError && turns.length === 0 && activeTurn === null;

  /* 渲染顺序：树内轮 + 活动轮；终态尾部只挂「末轮且无活动轮跟随」（树内）
     或活动轮本身——等价旧轮级态在下一轮开始时清空的语义。 */
  const renderedTurns = activeTurn ? [...turns, activeTurn] : turns;
  const actionsTurnId = lastAnswerTurnId(renderedTurns);

  /* 单轮块渲染：提问行 + 思考/工具相块（流式展开→完成折叠）+ 正文与引用。
     终态尾部由 renderTerminalTail 以独立兄弟元素渲染（换树窗口 DOM 节点
     不随轮 key 重建——追问药丸跨历史回载可点的语义保证）。 */
  function renderTurnBlocks(turn: AgentTurn, options: { isLive: boolean }) {
    const terminal = turn.terminal;
    const badgeCitations = turn.answerCitations ?? (options.isLive ? terminal.citations : []);
    return (
      <Fragment key={turn.id}>
        <div className="ml-auto max-w-[85%] whitespace-pre-wrap rounded-md bg-primary px-3 py-2 text-sm text-primary-foreground">
          {turn.query}
        </div>
        {turn.segments.map((segment, index) =>
          segment.kind === "think" ? (
            <AgentThinkingBlock
              key={`${turn.id}-segment-${index}`}
              content={segment.content}
              streaming={options.isLive && turn.streaming}
            />
          ) : (
            <AgentToolStatus
              key={`${turn.id}-segment-${index}`}
              tools={segment.tools}
              live={options.isLive && turn.streaming}
            />
          ),
        )}
        {turn.moderationBlocked ? (
          <div className="max-w-[85%] rounded-md border border-border-default bg-card px-3 py-2 text-sm text-fg-muted">
            {turn.answer}
          </div>
        ) : turn.answer !== "" ? (
          <>
            <div className="group/message max-w-[85%] rounded-md bg-canvas-subtle px-3 py-2 text-sm">
              {/* 受控渲染：react-markdown 未接 rehype-raw，原始 HTML 一律转义（T20 核验） */}
              <MarkdownRenderer
                content={turn.answer}
                onCitationRef={(citationIndex) => handleCitationRef(citationIndex, badgeCitations)}
                citationCount={badgeCitations.length}
              />
              {!streaming && actionsTurnId === turn.id && (
                <div className="mt-1.5 flex items-center gap-1 opacity-0 transition-opacity duration-150 group-hover/message:opacity-100 focus-within:opacity-100">
                  <button
                    type="button"
                    aria-label={t("agent.workspace.copyMessage")}
                    onClick={() => void handleCopyMessage(turn.answer)}
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
            {/* 引用随消息持久化（2026-09-06 实测修复）：每条有引用的
                回答消息下方都保留跳转入口，不再随下一轮开始而消失。 */}
            {turn.answerCitations && turn.answerCitations.length > 0 && (
              <AgentCitationList
                citations={turn.answerCitations.map(toAgentCitation)}
                onOpen={handleCitationOpen}
              />
            )}
          </>
        ) : null}
      </Fragment>
    );
  }

  /* 终态尾部（独立兄弟元素，稳定 DOM 位置）：挂当前轮——活动轮优先（树内
     末轮终态在有新活动轮时不渲染，等价旧轮级态在下一轮开始时清空）。 */
  function renderTerminalTail(turn: AgentTurn, isLive: boolean) {
    const terminal = turn.terminal;
    return (
      <>
            {/* SP-15 B #435：轮内推荐追问——当前轮答案（及其引用）下方的
                动作药丸；住轮终态，历史重载不冲掉、下轮开始清空。 */}
            {!streaming && terminal.followUps.length > 0 && (
              <AgentFollowUpChips followUps={terminal.followUps} onFill={handleFollowUpFill} />
            )}

            {!streaming && (terminal.usage || terminal.traceId) && (
              <details className="max-w-[85%] rounded-md border border-border-default bg-card px-3 py-1.5 text-xs text-fg-muted">
                <summary className="cursor-pointer select-none">{t("agent.workspace.turnDetails")}</summary>
                {terminal.usage && (
                  <p className="mt-1.5">
                    {t("agent.workspace.turnUsage", {
                      prompt: terminal.usage.prompt_tokens,
                      completion: terminal.usage.completion_tokens,
                    })}
                  </p>
                )}
                {terminal.traceId && (
                  <p className="mt-1">
                    {t("agent.workspace.traceLabel")}:{" "}
                    {isAdmin ? (
                      <Link
                        href={`/admin/traces/${terminal.traceId}`}
                        className="font-mono text-indigo-600 underline-offset-2 hover:underline dark:text-indigo-400"
                      >
                        {terminal.traceId}
                      </Link>
                    ) : (
                      <span className="font-mono">{terminal.traceId}</span>
                    )}
                  </p>
                )}
              </details>
            )}

            {/* #610 空轮（no_evidence 且零工具执行）：专门空态文案，
                替代「已深度思考」旁的近空白气泡。 */}
            {terminal.emptyNoEvidence && (
              <div className="flex max-w-[85%] items-start gap-2 rounded-md border border-border-default bg-card px-3 py-2 text-sm text-fg-default">
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-fg-muted" aria-hidden="true" />
                <div>
                  <p className="font-medium">{t("agent.emptyTurn.title")}</p>
                  <p className="mt-1 text-xs text-fg-muted">{t("agent.emptyTurn.description")}</p>
                </div>
              </div>
            )}

            {terminal.answerKind === "no_evidence" && !terminal.emptyNoEvidence && (
              <div className="flex max-w-[85%] items-start gap-2 rounded-md border border-border-default bg-card px-3 py-2 text-sm text-fg-default">
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-fg-muted" aria-hidden="true" />
                <div>
                  <p className="font-medium">{t("agent.noEvidence.title")}</p>
                  <p className="mt-1 text-xs text-fg-muted">{t("agent.noEvidence.description")}</p>
                  {turn.query && (
                    <Link
                      href={`/search?q=${encodeURIComponent(turn.query)}`}
                      className="mt-2 inline-flex min-h-11 items-center text-sm font-medium text-accent-emphasis underline-offset-2 hover:underline focus:outline-none focus:ring-2 focus:ring-ring"
                    >
                      {t("agent.noEvidence.searchCta")}
                    </Link>
                  )}
                </div>
              </div>
            )}

            {terminal.degraded && (
              <div className="flex max-w-[85%] items-start gap-2 rounded-md border border-border-default bg-card px-3 py-2 text-sm text-fg-default">
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-fg-muted" aria-hidden="true" />
                <div>
                  <p className="font-medium">{t("agent.degraded.title")}</p>
                  <p className="mt-1 text-xs text-fg-muted">{t("agent.degraded.description")}</p>
                  {turn.query && (
                    <Link
                      href={`/search?q=${encodeURIComponent(turn.query)}`}
                      className="mt-2 inline-flex min-h-11 items-center text-sm font-medium text-accent-emphasis underline-offset-2 hover:underline focus:outline-none focus:ring-2 focus:ring-ring"
                    >
                      {t("agent.noEvidence.searchCta")}
                    </Link>
                  )}
                </div>
              </div>
            )}

            <AgentCitationList
              citations={terminal.citations.map(toAgentCitation)}
              onOpen={handleCitationOpen}
            />

            {terminal.stopped && (
              <p className="text-xs text-fg-muted">{t("agent.workspace.stoppedNotice")}</p>
            )}

            {terminal.error && turn.answer === "" && (
              <div className="flex items-start gap-2 rounded-md border border-border-destructive bg-card px-3 py-2 text-sm text-fg-default">
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" aria-hidden="true" />
                <div className="flex-1">
                  {terminal.errorCode === "AGENT_RATE_LIMIT_EXCEEDED" ? (
                    <>
                      <p className="font-medium">{t("agent.workspace.rateLimitTitle")}</p>
                      <p className="mt-1 text-xs text-fg-muted">{t("agent.workspace.rateLimitHint")}</p>
                    </>
                  ) : (
                    <p className="font-medium">{t("agent.workspace.errorTitle")}</p>
                  )}
                  {terminal.traceId && (
                    <p className="mt-1 text-xs text-fg-muted">
                      {t("agent.workspace.traceLabel")}: <span className="font-mono">{terminal.traceId}</span>
                    </p>
                  )}
                </div>
                {terminal.errorCode !== "AGENT_RATE_LIMIT_EXCEEDED" && (
                  <Button variant="outline" size="sm" className="h-9" onClick={handleRegenerate}>
                    <RotateCw className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
                    {t("agent.workspace.errorRetry")}
                  </Button>
                )}
              </div>
            )}
      </>
    );
  }

  /* #416 O2：同一表单的两种布局形态——空态 = 大号输入框（rows 4、宽占比
     更大）随引导区；会话态 = 底部常规形态（rows 1）。发送按钮与按键语义
     两形态一致（发送按钮改造属 #417，本轮不动）。 */
  /* #417 F6b：输入区切换到公共 Composer（#413 F6a 产出）——发送/停止按钮
     内嵌右下角背景融合；Enter 发送、Shift+Enter 换行、自动增高 208 上限、
     isComposing 防护随组件内建；URL 预填与流式停止行为保持。 */
  const renderComposer = (emptyVariant: boolean) => (
    <div className={emptyVariant ? "w-full" : "shrink-0 bg-canvas-default p-3"}>
      <Composer
        ref={composerRef}
        value={input}
        onChange={setInput}
        onSubmit={() => handleSend()}
        keyMode="enter"
        rows={emptyVariant ? 4 : 1}
        ariaLabel={t("agent.workspace.composerLabel")}
        placeholder={t("agent.workspace.inputPlaceholder")}
        submitLabel={t("agent.workspace.sendMessage")}
        submitDisabled={!input.trim() || streaming}
        disabled={streaming}
        stopLabel={streaming ? t("agent.workspace.stopGenerating") : undefined}
        onStop={streaming ? handleStop : undefined}
        leading={modelSelector ? (
          <>
            {deepThinkToggle}
            {modelSelector}
          </>
        ) : (
          deepThinkToggle
        )}
      />
      <p className="mt-1.5 px-1 text-xs text-fg-muted">{t("agent.workspace.composerHint")}</p>
    </div>
  );
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
        <header className="flex h-14 shrink-0 items-center gap-2 px-2">
          <button
            type="button"
            aria-label={t("agent.workspace.openConversations")}
            onClick={() => setDrawerOpen(true)}
            className="inline-flex size-11 shrink-0 items-center justify-center rounded-md text-fg-muted transition-colors hover:bg-canvas-subtle hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring min-[701px]:hidden"
          >
            <Menu className="h-4 w-4" aria-hidden="true" />
          </button>
          {/* #416 O2：空态不渲染标题（消除与侧栏「开启新对话」的语义重复）；
              会话态标题与会话列表同源，点击进入原地编辑 */}
          {emptyConversation || activeId === null ? null : editingTitle ? (
            <input
              autoFocus
              value={titleDraft}
              onChange={(event) => setTitleDraft(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  commitTitleEdit();
                } else if (event.key === "Escape") {
                  event.preventDefault();
                  cancelTitleEdit();
                }
              }}
              onBlur={commitTitleEdit}
              aria-label={t("agent.workspace.editTitleLabel")}
              maxLength={50}
              className="min-w-0 flex-1 truncate rounded-md border border-border-default bg-canvas-default px-2 py-1 text-sm font-semibold text-fg-default focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring"
            />
          ) : (
            <h1
              className="min-w-0 flex-1 cursor-text truncate rounded-md px-1 py-0.5 text-sm font-semibold text-fg-default hover:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-ring"
              title={t("agent.workspace.editTitleLabel")}
              tabIndex={0}
              onClick={startTitleEdit}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  startTitleEdit();
                }
              }}
            >
              {headerTitle}
            </h1>
          )}
        </header>

        {emptyConversation ? (
          /* #416 O2 空态形态：主体中部偏下 = 引导内容（顺序文案不变）+ 大号
             输入框；点击示例气泡直接发送 */
          <div className="flex min-h-0 flex-1 flex-col items-center justify-end overflow-y-auto px-4 pb-[12vh] pt-8 text-center">
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
            <div className="mt-8 w-full max-w-2xl text-left">{renderComposer(true)}</div>
          </div>
        ) : (
          <>
            <div
              ref={transcriptRef}
              role="log"
              aria-live="polite"
              aria-label={t("agent.workspace.transcriptLabel")}
              data-slot="agent-transcript"
              onScroll={handleTranscriptScroll}
              className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-4"
            >
          {/* 首轮 done→历史回载在途：已有内容（树轮/活动轮）持续渲染不闪空，
              骨架屏只在无可示内容时出现（#663 显性体验改善）。 */}
          {messagesLoadError ? (
            <div className="mx-auto mt-16 max-w-sm rounded-md border border-border-destructive px-4 py-3 text-sm text-fg-default">
              {t("agent.workspace.conversationLoadFailed")}
            </div>
          ) : messagesLoading && turns.length === 0 && activeTurn === null ? (
            <div className="space-y-3" aria-busy="true">
              <div className="h-10 w-2/3 animate-pulse rounded bg-canvas-subtle" />
              <div className="ml-auto h-10 w-2/3 animate-pulse rounded bg-canvas-subtle" />
              <div className="h-10 w-3/4 animate-pulse rounded bg-canvas-subtle" />
            </div>
          ) : (
            <div className="mx-auto flex max-w-3xl flex-col gap-3">
              {turns.map((turn) => renderTurnBlocks(turn, { isLive: false }))}
              {activeTurn && renderTurnBlocks(activeTurn, { isLive: true })}
              {(activeTurn ?? turns[turns.length - 1] ?? null) &&
                renderTerminalTail(activeTurn ?? turns[turns.length - 1], activeTurn !== null)}
            </div>
          )}
            </div>

            {showJumpToLatest && !streaming && (
              <div className="pointer-events-none absolute bottom-32 left-1/2 z-10 -translate-x-1/2">
                <Button
                  variant="outline"
                  size="sm"
                  className="pointer-events-auto h-9"
                  onClick={scrollToLatest}
                >
                  <ArrowDown className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
                  {t("agent.workspace.jumpToLatest")}
                </Button>
              </div>
            )}

            {renderComposer(false)}
          </>
        )}
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
