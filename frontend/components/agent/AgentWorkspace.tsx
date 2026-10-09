"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations, useLocale } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { ArrowDown, BookOpen, Brain, History } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { Composer } from "@/components/ui/composer";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { useToast } from "@/components/ui/Toast";
import { useAuthGate } from "@/components/auth/AuthGateProvider";
import { AgentGuestExhaustedCard } from "@/components/agent/AgentGuestExhaustedCard";
import { useContentDetailOverlay } from "@/components/content/use-content-detail-overlay";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { getBrowserApiBase } from "@/lib/server-api";
import {
  startAgentStream,
  AgentStreamError,
  type AgentStreamCitation,
  type AgentStreamEvent,
} from "@/lib/agent-stream";
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
import {
  handoverFirstRoundTerminal,
  invalidateFirstRoundHandover,
  takeOverFirstRoundHandover,
} from "@/lib/agent-first-round-handover";
import {
  fetchGuestJson,
  fetchGuestQuota,
  isGuestQuotaTerminal,
  saveGuestHandover,
  stripAuthorizationFetch,
  takeGuestHandover,
  GUEST_AGENT_ENDPOINTS,
  GuestRequestError,
} from "@/lib/agent-guest";
import { AgentCitationsSidebar } from "@/components/agent/AgentCitationsSidebar";
import { isProviderDegradation } from "@/lib/agent";
import { useDelayedUnmount } from "@/lib/use-delayed-unmount";
import { usePersistentState } from "@/lib/use-persistent-state";
import {
  AgentConversationSidebar,
  type AgentConversationSummary,
} from "@/components/agent/AgentConversationSidebar";
import { renderAgentTurnBlocks, renderAgentTerminalTail, type AgentTurnBlockDeps } from "@/components/agent/AgentTurnBlocks";
const SIDEBAR_STORAGE_KEY = "agentSidebarCollapsed";
/** #539：深度思考开关持久化（localStorage，随会话恢复用户偏好）。 */
const DEEP_THINK_STORAGE_KEY = "agentDeepThink";
/** #545：模型偏好持久化（注册表 id；失效 id 由选项列表校验兜底）。 */
const MODEL_STORAGE_KEY = "agentModelPref";
/* #806 A3：侧栏/深度思考两处磁盘编码原样保留（"collapsed"/"expanded"、
   "on"/"off"），避免重置既有用户偏好；模型偏好因与异步注册表校验耦合
   （失效 id 须「不应用但不擦除」）不走 usePersistentState。 */
const parseSidebarStored = (raw: string) => raw === "collapsed";
const serializeSidebar = (collapsed: boolean) => (collapsed ? "collapsed" : "expanded");
const parseDeepThinkStored = (raw: string) => raw === "on";
const serializeDeepThink = (on: boolean) => (on ? "on" : "off");
const STICKY_BOTTOM_THRESHOLD = 80;
/** 输入自动增高上限：约 8 行（leading-6 = 24px × 8 + 上下 padding）后转内部滚动。 */

export interface AgentWorkspaceProps {
  initialConversationId?: number;
  /** A-07：外部入口（搜索页「问 AI 助手」/agent?q=）带来的首轮预填问题，仅首挂载生效。 */
  initialQuery?: string;
  onCitationOpen?: (citation: AgentCitation) => void;
  /** #854：工作台表面。guest = 匿名设备身份（游客端点 + 余量 caption +
   *  用尽转化卡）；缺省 user = 既有受保护形态，行为零变化。 */
  variant?: "user" | "guest";
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
/* FT-5 (#697)：引用池 → 角标小卡数据（编号缺失的历史行回退位置序）的展示
   层映射住 AgentTurnBlocks（transcript 渲染域，toCitationBadgeInfo 在用
   副本）；工作台侧 2026-09-12 复制副本已随 #795 核实删除。 */

export function AgentWorkspace({
  initialConversationId,
  initialQuery,
  onCitationOpen,
  variant = "user",
}: AgentWorkspaceProps) {
  const isGuest = variant === "guest";
  const t = useTranslations();
  // #723：请求者语言（zh/en），随每轮请求携带。
  const siteLocale = useLocale();
  // SP-21 T4：管理员可从会话轮直接跳转链路详情（SSE trace_id ↔ 落库一致）。
  const { user: authUser } = useAuth();
  const isAdmin = authUser?.role === "admin";
  const { toast } = useToast();
  /* #854：SP-17 登录浮窗——只续做尚未执行的动作（登录后由新会话执行）。 */
  const { requireAuth } = useAuthGate();
  const apiBase = getBrowserApiBase();

  /* #854：游客余量（服务端权威）。null = 引导查询未返回；exhausted 后
     工作台以转化卡替换输入区，不再向游客端点发起生成。 */
  const [guestRemaining, setGuestRemaining] = useState<number | null>(null);
  const [guestExhausted, setGuestExhausted] = useState(false);
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
  /* #795：活动轮的实例内镜像——done 事件路径需要同步读到最新轮（流回调闭包
     与 setState updater 都读不到提交后的状态），首轮交接的「保存先于导航」
     才拿得到终态本体。所有活动轮变更统一经 commitLiveTurn/updateLiveTurn
     同步 ref+state（ref 只读于事件路径，渲染仍走 state）。 */
  const activeTurnRef = useRef<AgentTurn | null>(null);
  const commitLiveTurn = useCallback((next: AgentTurn | null) => {
    activeTurnRef.current = next;
    setActiveTurn(next);
  }, []);
  const updateLiveTurn = useCallback(
    (update: (previous: AgentTurn | null) => AgentTurn | null) => {
      commitLiveTurn(update(activeTurnRef.current));
    },
    [commitLiveTurn],
  );
  /* #854：终局后向服务端回读余量（caption 递减以服务端权威为准）。 */
  const refreshGuestQuota = useCallback(async () => {
    if (!isGuest) return;
    try {
      const state = await fetchGuestQuota();
      setGuestRemaining(state.remaining);
      setGuestExhausted(state.exhausted || state.remaining <= 0);
    } catch {
      /* 回读失败保持当前值；下一次终局再刷新 */
    }
  }, [isGuest]);

  const streaming = activeTurn?.streaming ?? false;
  const [confirmDeleteId, setConfirmDeleteId] = useState<number | null>(null);
  const [collapsed, setCollapsed] = usePersistentState<boolean>({
    storageKey: SIDEBAR_STORAGE_KEY,
    fallback: false,
    parse: parseSidebarStored,
    serialize: serializeSidebar,
  });
  const [drawerOpen, setDrawerOpen] = useState(false);
  /* #721：抽屉动效（延迟卸载走完退出动画）+ 关闭后焦点返回触发按钮。 */
  const historyTriggerRef = useRef<HTMLButtonElement | null>(null);
  const citationsTriggerRef = useRef<HTMLButtonElement | null>(null);
  const closeHistoryDrawer = useCallback(() => {
    setDrawerOpen(false);
    historyTriggerRef.current?.focus();
  }, []);
  const historyDrawerMounted = useDelayedUnmount(drawerOpen, 220);
  /* FT-4：参考来源侧栏——保存源数组引用（同一回答再点即收起），渲染时映射。 */
  const [panelSource, setPanelSource] = useState<AgentStreamCitation[] | null>(null);
  const citationsPanel = panelSource ? panelSource.map(toAgentCitation) : null;
  /* #721：关闭引用侧栏（Esc/遮罩/X/再次点击）后焦点返回打开它的入口按钮。 */
  const closeCitationsPanel = useCallback(() => {
    setPanelSource(null);
    citationsTriggerRef.current?.focus();
  }, []);
  const [showJumpToLatest, setShowJumpToLatest] = useState(false);

  const transcriptRef = useRef<HTMLDivElement>(null);
  const composerRef = useRef<HTMLTextAreaElement>(null);
  const controllerRef = useRef<AbortController | null>(null);
  /* FT-3：首轮 done→replace(/agent/c/{id}) 的待导航标记（会话 id 到 done
     才产生；push 会让「发完首条按返回」退回空页）。 */
  const pendingFirstRoundNavRef = useRef(false);
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
      const data = isGuest
        ? await fetchGuestJson<{ conversations?: AgentConversationSummary[] }>(
            GUEST_AGENT_ENDPOINTS.conversations,
          )
        : await api.get<{ conversations?: AgentConversationSummary[] }>(
            "/api/v1/agent/conversations",
          );
      setConversations(data.conversations ?? []);
    } catch (error) {
      setConversationsLoadError(true);
      silentError(error, { component: "AgentWorkspace", action: "list conversations" });
    } finally {
      setConversationsLoading(false);
    }
  }, [isGuest]);

  useEffect(() => {
    void loadConversations();
  }, [loadConversations]);

  /* #854：游客引导（余量查询兼设备身份发放）。失败（含 401 设备无效）
     一律按用尽降级到转化卡——服务端 fail-closed 时客户端绝不重试生成。 */
  useEffect(() => {
    if (!isGuest) return;
    let cancelled = false;
    fetchGuestQuota()
      .then((state) => {
        if (cancelled) return;
        setGuestRemaining(state.remaining);
        setGuestExhausted(state.exhausted || state.remaining <= 0);
      })
      .catch(() => {
        if (cancelled) return;
        setGuestRemaining(0);
        setGuestExhausted(true);
      });
    return () => {
      cancelled = true;
    };
  }, [isGuest]);

  /* FT-3：流中离开（路由切走/组件卸载）补 abort——SSE 连接不再残留。
     #854：登录成功切面即卸载本实例，同一 effect 兜底取消在途游客流。 */
  useEffect(() => () => controllerRef.current?.abort(), []);

  /* #854：登录续做——账号工作台挂载时消费游客交接（consume-once），
     在账号新会话里自动执行尚未执行的草稿；不迁移任何游客历史。 */
  useEffect(() => {
    if (isGuest) return;
    const pending = takeGuestHandover();
    if (pending !== null) {
      startTurn(pending);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  /* #539：深度思考开关——默认关（快、省 token），开启后请求带 deep_think，
     由后端映射到 provider 的思考控制（MiniMax M3 thinking.type）。
     持久化经 usePersistentState（#806 A3，编码 "on"/"off" 不变）。 */
  const [deepThink, setDeepThink] = usePersistentState<boolean>({
    storageKey: DEEP_THINK_STORAGE_KEY,
    fallback: false,
    parse: parseDeepThinkStored,
    serialize: serializeDeepThink,
  });

  /* #545：模型选择——拉取注册表（>1 供给才渲染选择器；拉取失败静默降级为
     单供给形态，不打扰对话主链路）。偏好持久 localStorage，失效 id 丢弃。 */
  const [modelOptions, setModelOptions] = useState<{ id: string; display_name: string }[]>([]);
  const [modelPref, setModelPref] = useState("");
  useEffect(() => {
    const modelsPromise = isGuest
      ? fetchGuestJson<{ models?: { id: string; display_name: string }[] }>(GUEST_AGENT_ENDPOINTS.models)
      : api.get<{ models?: { id: string; display_name: string }[] }>("/api/v1/agent/models");
    modelsPromise
      .then((data) => {
        const options = data.models ?? [];
        setModelOptions(options);
        const saved = window.localStorage.getItem(MODEL_STORAGE_KEY);
        if (saved && options.some((option) => option.id === saved)) setModelPref(saved);
      })
      .catch(() => {});
  }, [isGuest]);
  function toggleDeepThink() {
    setDeepThink(!deepThink);
  }
  const deepThinkToggle = (
    <button
      type="button"
      aria-pressed={deepThink}
      aria-label={t("agent.workspace.deepThink")}
      title={t("agent.workspace.deepThinkHint")}
      onClick={toggleDeepThink}
      /* #725 底部控件带收紧：开关视觉降级（h-6 紧凑形态，行为/可达性不变）。 */
      className={cn(
        "inline-flex h-6 items-center gap-1 rounded-md px-1.5 text-[11px] transition-colors duration-150 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
        /* FT-2：默认白底黑字；选中仅背景变化，字色不变 */
        deepThink ? "bg-primary/10 text-fg-default" : "bg-canvas-default text-fg-default hover:bg-canvas-subtle",
      )}
    >
      <Brain className="h-3 w-3" aria-hidden="true" />
      <span>{t("agent.workspace.deepThink")}</span>
    </button>
  );
  const modelSelector =
    modelOptions.length > 1 ? (
      <div className="w-fit">
        <Select
          aria-label={t("agent.workspace.modelLabel")}
          value={modelPref || modelOptions[0].id}
          onChange={(event) => {
            setModelPref(event.target.value);
            window.localStorage.setItem(MODEL_STORAGE_KEY, event.target.value);
          }}
          /* #725：模型选择器同步紧凑形态（#844 换共享 Select，pr-6 保箭头留白）。 */
          className="h-6 rounded-md border-border-default bg-canvas-default px-1 pr-6 text-[11px] text-fg-default focus-visible:ring-1 focus-visible:ring-ring"
        >
          {modelOptions.map((option) => (
            <option key={option.id} value={option.id}>
              {option.display_name}
            </option>
          ))}
        </Select>
      </div>
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
    const messagesPromise = isGuest
      ? fetchGuestJson<{ messages?: AgentHistoryMessageDTO[] }>(
          GUEST_AGENT_ENDPOINTS.conversation(activeId),
        )
      : api.get<{ messages?: AgentHistoryMessageDTO[] }>(
          `/api/v1/agent/conversations/${activeId}`,
        );
    messagesPromise
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
          commitLiveTurn(null);
        } else {
          /* 重挂载路径（FT-3 首轮 replace → key={id} 重挂载，内存活动轮已清空）：
             首轮终态由 #795 交接 module 接管——历史 DTO 不落追问/用量/trace，
             不回填则追问 chips 在每个新会话首轮必丢（#715）。takeOver 只在非空
             历史并入树尾时消费记录一次（空历史不消费、取消/失败到不了这里——
             cancelled 守卫）；同实例合并不消费记录：replace 总会发生，旧实例
             的合并结果随卸载丢弃，重挂载后的新实例才是最终态。 */
          setTurns(takeOverFirstRoundHandover(activeId, mapped));
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setMessagesLoadError(true);
          silentError(error, { component: "AgentWorkspace", action: "load conversation" });
          /* FT-3：深链他人/已删会话 404 → 落回空态入口（错误横幅随重挂载消失）。
             #795：目标会话已不存在 = 交接记录明确失效，先清理再回退。
             #854：游客 410（7 天过期）同路径回退，不读作存在性探测。 */
          const notFound = error instanceof ApiRequestError && error.status === 404;
          const guestGone = error instanceof GuestRequestError && error.status === 410;
          if (notFound || guestGone) {
            invalidateFirstRoundHandover(activeId);
            router.replace("/agent");
          }
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

  /* #715/#795：首轮终态的交接保存已移入 done 事件的受控路径（handleStreamEvent
     内 handoverFirstRoundTerminal：先保存、后 replace）——不依赖卸载前可能来
     不及执行的 effect，导航 adapter 即刻卸载旧实例也不丢记录。 */

  /* 续问轮终局（done/error/stop/关流）：commit 进树后清空活动轮——语义等价
     旧「尾行终稿替换 + 轮级态保留」（终态随轮入树，树尾轮渲染到下一轮开始）。 */
  useEffect(() => {
    if (!activeTurn || !activeTurn.settled || activeTurn.firstRound) return;
    setTurns((previous) => [...previous, activeTurn]);
    commitLiveTurn(null);
  }, [activeTurn, commitLiveTurn]);

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
    /* #795：离开当前会话 = 其未消费的首轮交接记录明确失效（防迟到回载/
       日后重开时并入过期终态）。 */
    if (activeId !== null && activeId !== id) invalidateFirstRoundHandover(activeId);
    commitLiveTurn(null);
    setActiveId(id);
    setDrawerOpen(false);
    /* FT-3：切会话即导航（URL 承载会话身份；同 id 不重复压栈）。 */
    if (!initialConversationId || id !== initialConversationId) {
      router.push(`/agent/c/${id}`);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeId, commitLiveTurn]);

  const handleNewConversation = useCallback(() => {
    if (streaming) return;
    fallbackRequestRef.current += 1;
    /* #795：离开目标会话回空态 = 交接记录明确失效。 */
    if (activeId !== null) invalidateFirstRoundHandover(activeId);
    setActiveId(null);
    setTurns([]);
    commitLiveTurn(null);
    setDrawerOpen(false);
    /* FT-3：新建会话回空态入口路由（已在 /agent 时同路由导航为 no-op）。 */
    router.push("/agent");
    focusComposer();
  }, [streaming, activeId, commitLiveTurn]);

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
     滚动到底部引用卡片，与其它页面「点链接开浮窗」的契约不一致）。FT-5
     (#697)：ref 为轮内全局编号——服务端已把角标编号与引用池对齐（跨检索
     累计、剔除不压缩），优先按 number 命中；历史行/旧轮次无 number 时回退
     位置序（ref-1）。citations 由调用方按轮传入（活动轮进行中的答案回落轮
     内流式引用；历史等无引用轮传空数组即不响应）。zone="ip" 的角标与卡片
     同分流（Q5）。 */
  const handleCitationRef = useCallback(
    (ref: number, citations: AgentStreamCitation[]) => {
      const citation = citations.find((item) => item.number === ref) ?? citations[ref - 1];
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
      /* #795：会话已删除 = 其交接记录永久失效（含侧栏删除非当前会话）。 */
      invalidateFirstRoundHandover(id);
      if (activeId === id) {
        fallbackRequestRef.current += 1;
        setActiveId(null);
        setTurns([]);
        commitLiveTurn(null);
        /* FT-3：删的是当前会话 → 落回空态入口（不留死 URL）。 */
        router.replace("/agent");
        focusComposer();
      }
      toast("success", t("agent.workspace.deleteSuccess"));
      void loadConversations();
    } catch (error) {
      silentError(error, { component: "AgentWorkspace", action: "delete conversation" });
      toast("error", t("agent.workspace.deleteFailed"));
    }
  }, [activeId, commitLiveTurn, confirmDeleteId, loadConversations, toast, t]);

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
      updateLiveTurn((previous) => (previous ? applyKeywordFallbackCitations(previous, citations) : previous));
    } catch (error) {
      silentError(error, { component: "AgentWorkspace", action: "keyword fallback" });
      if (fallbackRequestRef.current === requestId) {
        updateLiveTurn((previous) => (previous ? applyKeywordFallbackCitations(previous, []) : previous));
      }
    }
  }, [updateLiveTurn]);

  /* live 事件：形状归约进 TurnModel reducer（纯函数）；此处只留回合策略副作用
     （会话 id 写入/会话列表刷新/AbortController/关键词回退发起/首轮交接）。 */
  const handleStreamEvent = useCallback(
    (event: AgentStreamEvent, turnQuery: string) => {
      /* #795：归约走 ref 镜像（updater 读不到提交后状态，事件路径需要同步
         拿到终态本体做交接）。 */
      const live = activeTurnRef.current;
      const next = live !== null ? reduceAgentTurn(live, event) : null;
      if (next !== live) commitLiveTurn(next);
      if (event.type === "done") {
        if (event.conversation_id) {
          setActiveId(event.conversation_id);
          void loadConversations();
          if (isGuest) void refreshGuestQuota();
          /* FT-3 + #795：首轮会话 id 到 done 才产生——先把终态交接记录写入
             module（跨 key 重挂载存活的唯一载体），再 replace 写入会话 URL
             （push 会让「发完首条按返回」退回空页）。保存必须先于导航：
             真实 replace 会即刻卸载本实例，不能靠卸载前来不及执行的 effect。 */
          if (pendingFirstRoundNavRef.current) {
            pendingFirstRoundNavRef.current = false;
            if (next !== null && next.settled && next.firstRound) {
              handoverFirstRoundTerminal(
                event.conversation_id,
                next.terminal,
                () => router.replace(`/agent/c/${event.conversation_id}`),
              );
            } else {
              router.replace(`/agent/c/${event.conversation_id}`);
            }
          }
        }
        return;
      }
      if (event.type === "error") {
        controllerRef.current?.abort();
        /* provider 降级撤答：关键词回退与降级终态同 tick 发起（agent-turn.ts
           applyError 契约「回退请求由调用方副作用发起」）。改由 effect 驱动会晚一拍——
           降级文案先渲染而回退引用未落，依赖同步断言的既有测试稳定红（#663 后时序回归；
           #684 B+ 裁决：触发时序维持组件同 tick 双处，不重开；降级判定唯一真源 =
           lib/agent.ts isProviderDegradation，terminal.needsKeywordFallback = 回退
           挂起记录非触发真源）。 */
        if (isProviderDegradation(event)) {
          void loadKeywordFallback(turnQuery, fallbackRequestRef.current);
        }
      }
    },
    // refreshGuestQuota 恒等性是 isGuest 的纯函数（useCallback([isGuest])），
    // 补进 deps 只满足 exhaustive-deps，重建时机零变化。
    [commitLiveTurn, loadConversations, loadKeywordFallback, isGuest, refreshGuestQuota],
  );

  /* 发起一轮对话（A-01 续写契约）：上下文由服务端组装，客户端只带
     conversation_id + message。regenerate 复用同一入口且不重复落提问行。
     残留活动轮（首轮终态、回载未落地）先 commit 进树再开新轮。 */
  function startTurn(query: string) {
    const body: Record<string, unknown> = {
      message: query,
      context: { surface: "global" },
      deep_think: deepThink,
      // #723：请求者语言随轮携带（站内工具 usage_guide 跟随）。
      locale: siteLocale,
    };
    if (modelPref) body.model = modelPref;
    if (activeId !== null) body.conversation_id = activeId;
    fallbackRequestRef.current += 1;
    if (activeTurn) {
      setTurns((previous) => [...previous, activeTurn]);
    }
    pendingFirstRoundNavRef.current = activeId === null;
    commitLiveTurn(createAgentTurn(query, { id: newLocalTurnId(), firstRound: activeId === null }));

    const controller = new AbortController();
    controllerRef.current = controller;
    /* #854：游客流走专用 fetch 包装——剥离一切 Authorization（游客面只认
       匿名设备身份，凭证会被 403 拒绝而非降级）。 */
    const fetchImpl = isGuest ? stripAuthorizationFetch : fetch;
    /* GUEST_AGENT_ENDPOINTS.* 是含 /api/v1 的完整 API 路径（供 guestFetch）；
       startAgentStream 的调用方负责 apiBase（已含 /api/v1）+ 相对路径。 */
    const streamPath = isGuest ? "/agent/guest/chat/stream" : "/agent/chat/stream";
    void startAgentStream(fetchImpl, `${apiBase}${streamPath}`, body, {
      onEvent: (event) => handleStreamEvent(event, query),
      onError: (error) => {
        const code = error instanceof AgentStreamError ? error.code : undefined;
        if (isGuest && isGuestQuotaTerminal(code)) {
          /* 服务端权威用尽（并发抢占/计数丢失）：终局后以转化卡替换输入区。 */
          setGuestRemaining(0);
          setGuestExhausted(true);
        }
        updateLiveTurn((previous) =>
          previous ? reduceAgentTurn(previous, { type: "error", error_code: code }) : previous,
        );
      },
      onClose: () => {
        updateLiveTurn((previous) => (previous ? closeAgentStream(previous) : previous));
      },
    }, controller.signal);
  }

  function handleSend(overrideMessage?: string) {
    const trimmed = (overrideMessage ?? input).trim();
    if (!trimmed || streaming) return;
    if (isGuest && guestExhausted) {
      /* 用尽后的输入动作尚未执行：挂到登录浮窗的续做上，账号新会话首
         轮执行；禁止重放任何已消费的游客请求。 */
      openLoginContinuation(trimmed);
      return;
    }
    if (overrideMessage === undefined) setInput("");
    startTurn(trimmed);
  }

  /* #854：登录墙转化——requireAuth 的 pendingAction 只保存「尚未执行的
     草稿」；登录成功后双态壳切到账号工作台，由其挂载时 take 消费并
     自动发送（consume-once，刷新不重放）。 */
  function openLoginContinuation(draft: string) {
    requireAuth(() => {
      if (draft.trim() !== "") saveGuestHandover(draft);
      setInput("");
    });
  }

  function handleStop() {
    controllerRef.current?.abort();
    updateLiveTurn((previous) => (previous ? stopAgentTurn(previous) : previous));
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
      commitLiveTurn(null);
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
     空态下主区不渲染标题、主体居中渲染引导 + 输入框（同一表单组件的
     两种布局形态；FT-2 起两态同形）。 */
  const emptyConversation =
    !messagesLoading && !messagesLoadError && turns.length === 0 && activeTurn === null;

  /* 渲染顺序：树内轮 + 活动轮；终态尾部只挂「末轮且无活动轮跟随」（树内）
     或活动轮本身——等价旧轮级态在下一轮开始时清空的语义。 */
  const renderedTurns = activeTurn ? [...turns, activeTurn] : turns;
  const actionsTurnId = lastAnswerTurnId(renderedTurns);

  /* #755：轮块/终态尾部渲染拆至 AgentTurnBlocks（transcript 域），闭包
     依赖收拢为显式 deps——两函数名与调用点保持不变，行为零变化。 */
  const turnDeps: AgentTurnBlockDeps = {
    t,
    streaming,
    isAdmin,
    actionsTurnId,
    panelSource,
    citationsTriggerRef,
    setPanelSource,
    handleCitationRef,
    handleCopyMessage,
    handleRegenerate,
    handleFollowUpFill,
  };
  const renderTurnBlocks = (turn: AgentTurn, options: { isLive: boolean }) =>
    renderAgentTurnBlocks(turnDeps, turn, options);
  const renderTerminalTail = (turn: AgentTurn, isLive: boolean) =>
    renderAgentTerminalTail(turnDeps, turn, isLive);

  /* #417 F6b：输入区切换到公共 Composer（#413 F6a 产出）——发送/停止按钮
     内嵌右下角背景融合；Enter 发送、Shift+Enter 换行、自动增高 208 上限、
     isComposing 防护随组件内建；URL 预填与流式停止行为保持。 */
  /* FT-2（#694）两态同形：空态与会话态 rows=1、max-w-3xl(768px) 居中、
     默认宽度/高度不随形态切换变化（autoresize 208 上限机制不变）。 */
  const guestComposerLocked = isGuest && guestExhausted;
  const composerNode = (
    <>
      {guestComposerLocked ? (
        <AgentGuestExhaustedCard onLogin={() => openLoginContinuation(input)} />
      ) : null}
      <Composer
        ref={composerRef}
        value={input}
        onChange={setInput}
        onSubmit={() => handleSend()}
        keyMode="enter"
        /* #725：空态文本区默认 3 行量级（两态同形不变；自动增高/208px 封顶
           机制由 Composer 既有实现承担）。 */
        rows={3}
        expandMobileHit
        ariaLabel={t("agent.workspace.composerLabel")}
        placeholder={t("agent.workspace.inputPlaceholder")}
        submitLabel={t("agent.workspace.sendMessage")}
        submitDisabled={!input.trim() || streaming || guestComposerLocked}
        disabled={streaming || guestComposerLocked}
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
      {/* #854（Q21）：游客余量只在输入框下沿出现；Header/落地页一律不露轮数。 */}
      {isGuest ? (
        !guestComposerLocked && guestRemaining !== null ? (
          <p
            className="mt-1.5 px-1 text-xs text-fg-muted"
            data-testid="guest-quota-caption"
          >
            {t("agent.guest.caption", { remaining: guestRemaining })}
          </p>
        ) : null
      ) : (
        <p className="mt-1.5 px-1 text-xs text-fg-muted">{t("agent.workspace.composerHint")}</p>
      )}
    </>
  );
  const renderComposer = (emptyVariant: boolean) =>
    emptyVariant ? (
      /* 空态：外层（空态引导区）已是 max-w-3xl，这里满宽填充 */
      <div className="w-full">{composerNode}</div>
    ) : (
      /* 会话态：钉底容器内 max-w-3xl 居中（与消息列同宽） */
      <div className="shrink-0 bg-canvas-default p-3">
        <div className="mx-auto w-full max-w-3xl">{composerNode}</div>
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
          readOnly={isGuest}
          onToggleCollapse={() => {
            setCollapsed(!collapsed);
          }}
          onSelect={handleSelectConversation}
          onNewConversation={handleNewConversation}
          onRename={handleRename}
          onTogglePin={handleTogglePin}
          onDelete={(id) => setConfirmDeleteId(id)}
        />
      </div>

      {/* #721：滑入滑出（含关闭方向）+ 遮罩淡出——延迟卸载走完退出动画；
          退出帧剥离 dialog 语义/指针事件（不干扰可达性树与后续交互）。 */}
      {historyDrawerMounted && (
        <div
          className="fixed inset-0 z-50 min-[701px]:hidden"
          {...(drawerOpen ? { role: "dialog", "aria-modal": true } : { "aria-hidden": true, "inert": true as never })}
          aria-label={t("agent.workspace.sidebarLabel")}
          onKeyDown={(event) => {
            if (event.key === "Escape") closeHistoryDrawer();
          }}
        >
          <button
            type="button"
            tabIndex={drawerOpen ? undefined : -1}
            aria-label={t("agent.workspace.closeConversations")}
            className={cn(
              "absolute inset-0 bg-black/50 transition-opacity duration-200 motion-reduce:transition-none",
              drawerOpen ? "opacity-100" : "opacity-0",
            )}
            onClick={closeHistoryDrawer}
          />
          <div
            className={cn(
              /* #752：面板 flex 化（对齐桌面 wrapper 的 min-[701px]:flex 模式）——
                 aside 作为 flex item 沿主轴获得面板 h-full 的受限高度，面板 →
                 aside → 内层 flex（flex-1 min-h-0）→ nav（flex-1 min-h-0
                 overflow-y-auto）高度链闭合，会话列表恢复真实滚动。 */
              "relative flex h-full w-[85vw] max-w-[320px] bg-card shadow-md transition-transform duration-200 ease-out motion-reduce:transition-none [&_aside]:w-full [&_aside]:border-r-0",
              drawerOpen ? "translate-x-0" : "-translate-x-full",
            )}
          >
            <AgentConversationSidebar
              conversations={conversations}
              activeId={activeId}
              collapsed={false}
              loading={conversationsLoading}
              disabled={streaming}
              readOnly={isGuest}
              onToggleCollapse={closeHistoryDrawer}
              onSelect={handleSelectConversation}
              onNewConversation={handleNewConversation}
              onRename={handleRename}
              onTogglePin={handleTogglePin}
              onDelete={(id) => setConfirmDeleteId(id)}
              onRequestClose={closeHistoryDrawer}
            />
          </div>
        </div>
      )}

      <section
        aria-label={t("agent.workspace.transcriptLabel")}
        className="relative flex min-w-0 flex-1 flex-col min-[701px]:flex-row min-[701px]:border-l min-[701px]:border-border-default"
      >
        {/* #721：≤700px 主区纵向布局——标题栏为顶部横条（做薄），对话内容
            与底部输入框全宽不再被挤压；≥701px 维持横向现状。 */}
        <header className="flex h-12 w-full shrink-0 items-center gap-2 px-2 min-[701px]:h-14 min-[701px]:w-auto">
          <button
            type="button"
            ref={historyTriggerRef}
            aria-label={t("agent.workspace.openConversations")}
            onClick={() => setDrawerOpen(true)}
            className="inline-flex size-11 shrink-0 items-center justify-center rounded-md text-fg-muted transition-colors hover:bg-canvas-subtle hover:text-foreground focus:outline-none focus-visible:ring-1 focus:ring-ring min-[701px]:hidden"
          >
            {/* #721：历史记录语义的时钟类图标（原通用菜单图标与右上角侧栏
                图标撞语义）。 */}
            <History className="h-4 w-4" aria-hidden="true" />
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
              className="min-w-0 flex-1 truncate rounded-md border border-border-default bg-canvas-default px-2 py-1 text-sm font-semibold text-fg-default focus:border-border-strong focus:outline-none"
            />
          ) : (
            <h1
              className="min-w-0 flex-1 cursor-text truncate rounded-md px-1 py-0.5 text-sm font-semibold text-fg-default hover:bg-canvas-subtle focus:outline-none focus-visible:ring-1 focus-visible:ring-ring"
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
          /* #416 O2 空态形态（FT-2 居中修订）：引导内容（顺序文案不变）+
             输入框垂直居中；点击示例气泡直接发送 */
          <div className="flex min-h-0 flex-1 flex-col items-center justify-center overflow-y-auto px-4 py-8 text-center">
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
                    className="inline-flex items-center rounded-full border border-border-default bg-card px-3 py-1.5 text-sm text-fg-muted transition-colors duration-150 hover:border-border-strong hover:bg-canvas-subtle hover:text-foreground focus:outline-none focus-visible:ring-1 focus:ring-ring"
                  >
                    {t(key)}
                  </button>
                </li>
              ))}
            </ul>
            <div className="mt-8 w-full max-w-3xl text-left">{renderComposer(true)}</div>
          </div>
        ) : (
          <div className="flex min-w-0 flex-1 flex-col">
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
          </div>
        )}
        <AgentCitationsSidebar
          open={citationsPanel !== null}
          onClose={closeCitationsPanel}
          citations={citationsPanel ?? []}
          onOpen={handleCitationOpen}
        />
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

