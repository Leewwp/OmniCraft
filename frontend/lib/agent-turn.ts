import { normalizeAgentCitation } from "@/lib/agent";
import type { AgentStreamCitation, AgentStreamEvent, AgentStreamTool } from "@/lib/agent-stream";

/**
 * 回答回合模型（#663，与 CONTEXT.md「回答回合」词条同源）：一次提问到终局
 * 事件的完整周期 = 提问行 + think/tools 相块 + 正文（含随答案持久化的引用）
 * + 终态字段。live SSE 事件与历史行是同一模型的两个入口（两个 adapter =
 * 真 seam）；本模块零 next-intl 依赖，空态/占位文案由调用方注入。
 */

/** 思考/工具相块：按到达顺序排列；连续同类事件合并进同一段。 */
export type AgentTurnSegment =
  | { kind: "think"; content: string }
  | { kind: "tools"; tools: AgentStreamTool[] };

/** 轮终态字段：done/error/stop 终裁后填充。追问/用量/trace 不落库，历史轮无此数据。 */
export interface AgentTurnTerminal {
  answerKind: string | null;
  degraded: boolean;
  /** 轮级引用列表（轮尾渲染）；done 引用随答案持久化后清空，provider 降级的关键词回退写这里。 */
  citations: AgentStreamCitation[];
  /** SP-15 B #435：grounded 轮推荐追问（done 携带，v1 不落库）。 */
  followUps: string[];
  usage: { prompt_tokens: number; completion_tokens: number } | null;
  traceId: string | null;
  /** #610 空轮：no_evidence 且零成功工具（零调用或全部失败）。 */
  emptyNoEvidence: boolean;
  /** provider 降级待关键词回退：applyError 降级分支置位，回退结果落轮
      （applyKeywordFallbackCitations）清除。回退请求本身由组件从原始 error
      事件同 tick 发起（#678 时序契约，非读本字段）——本字段目前只是回退
      挂起的状态记录，仅测试断言消费（双真源收口待 #684 裁决）。 */
  needsKeywordFallback: boolean;
  stopped: boolean;
  error: boolean;
  errorCode: string | null;
}

export interface AgentTurn {
  /** 历史轮 = 服务端行 id；本地轮 = `live-` 前缀唯一串（无模块级计数器）。 */
  id: string;
  /** 提问行（完整回合含提问）。 */
  query: string;
  segments: AgentTurnSegment[];
  /** 正文（done 终裁前的流式增量 / 终裁后的服务端终稿）。 */
  answer: string;
  /** done 终稿随答案持久化的引用（行内 [n] 角标与答案下引用卡的数据源）。 */
  answerCitations?: AgentStreamCitation[];
  /** 历史 moderation 脱敏行：渲染占位文案而非答案。 */
  moderationBlocked?: boolean;
  streaming: boolean;
  /** 终局已到（done/error/stop/关流）：续问轮在终局后 commit 进树、清空活动轮。 */
  settled: boolean;
  /** 首轮（发起时会话 id 尚未产生）：done 后活到历史回载替换树；续问轮 done 即 commit。 */
  firstRound: boolean;
  terminal: AgentTurnTerminal;
}

export function createAgentTurn(
  query: string,
  options: { id: string; firstRound: boolean },
): AgentTurn {
  return {
    id: options.id,
    query,
    segments: [],
    answer: "",
    streaming: true,
    settled: false,
    firstRound: options.firstRound,
    terminal: {
      answerKind: null,
      degraded: false,
      citations: [],
      followUps: [],
      usage: null,
      traceId: null,
      emptyNoEvidence: false,
      needsKeywordFallback: false,
      stopped: false,
      error: false,
      errorCode: null,
    },
  };
}

function withTerminal(turn: AgentTurn, patch: Partial<AgentTurnTerminal>): AgentTurn {
  return { ...turn, terminal: { ...turn.terminal, ...patch } };
}

function appendThink(turn: AgentTurn, delta: string): AgentTurn {
  const segments = [...turn.segments];
  const last = segments[segments.length - 1];
  if (last && last.kind === "think") {
    segments[segments.length - 1] = { kind: "think", content: last.content + delta };
  } else {
    segments.push({ kind: "think", content: delta });
  }
  return { ...turn, segments };
}

function appendTool(turn: AgentTurn, tool: AgentStreamTool): AgentTurn {
  const segments = [...turn.segments];
  const last = segments[segments.length - 1];
  if (last && last.kind === "tools") {
    segments[segments.length - 1] = { kind: "tools", tools: [...last.tools, tool] };
  } else {
    segments.push({ kind: "tools", tools: [tool] });
  }
  return { ...turn, segments };
}

/**
 * live 入口：SSE 事件 reducer（纯函数，无渲染可直测）。判定条件逐字迁移旧
 * 组件内实现（done 终裁三分支 / 空轮启发式 / 追问挂载条件），不改语义；
 * 会话列表刷新、activeId 写入、关键词回退请求与 AbortController 等副作用
 * 留在调用方（回合传输策略，非回合形状职责）。
 */
export function reduceAgentTurn(turn: AgentTurn, event: AgentStreamEvent): AgentTurn {
  switch (event.type) {
    case "start": {
      if (!event.trace_id) return turn;
      return withTerminal(turn, { traceId: event.trace_id });
    }
    case "think_delta": {
      if (!event.delta) return turn;
      return appendThink(turn, event.delta);
    }
    case "tool_status": {
      if (!event.tool) return turn;
      return appendTool(turn, event.tool);
    }
    case "delta": {
      if (!event.delta) return turn;
      return { ...turn, answer: turn.answer + event.delta };
    }
    case "citation": {
      if (!event.citation) return turn;
      return withTerminal(turn, { citations: [...turn.terminal.citations, event.citation] });
    }
    case "usage": {
      const usage = event.usage as { prompt_tokens?: unknown; completion_tokens?: unknown } | undefined;
      if (
        usage &&
        typeof usage.prompt_tokens === "number" &&
        typeof usage.completion_tokens === "number"
      ) {
        return withTerminal(turn, {
          usage: { prompt_tokens: usage.prompt_tokens, completion_tokens: usage.completion_tokens },
        });
      }
      return turn;
    }
    case "done":
      return applyDone(turn, event);
    case "error":
      return applyError(turn, event);
    default:
      return turn;
  }
}

type DoneEvent = Extract<AgentStreamEvent, { type: "done" }>;

/** done 终裁（A-02 v2 三分支，逐字迁移旧判定）：no_evidence/degraded 撤下已流出
 *  正文；正常轮 answer 存在则以服务端终稿替换并随答案持久化引用；answer 为空
 *  即无正文（旧实现的空泡删除等价于本模型 answer=""）。 */
function applyDone(turn: AgentTurn, event: DoneEvent): AgentTurn {
  let next: AgentTurn = {
    ...turn,
    streaming: false,
    settled: true,
    terminal: {
      ...turn.terminal,
      traceId: event.trace_id ?? turn.terminal.traceId,
      usage: event.usage ?? turn.terminal.usage,
      citations: event.citations ?? turn.terminal.citations,
      degraded: Boolean(event.degraded),
      answerKind: event.answer_kind ?? null,
    },
  };
  if (event.degraded || event.answer_kind === "no_evidence") {
    next = { ...next, answer: "" };
  } else if (typeof event.answer === "string") {
    next = { ...next, answer: event.answer };
    if (event.citations && event.citations.length > 0) {
      /* 引用已随答案持久化：清掉轮级临时引用，避免同轮双渲染（provider-error
         兜底的引用仍走轮级列表）。 */
      next = {
        ...next,
        answerCitations: event.citations,
        terminal: { ...next.terminal, citations: [] },
      };
    }
  }
  /* #610 空轮：no_evidence 且没有任何成功执行的工具（零调用或全部失败）。 */
  const emptyNoEvidence =
    event.answer_kind === "no_evidence" &&
    !(event.tools ?? []).some((tool) => tool.status === "success");
  const followUps =
    event.answer_kind === "grounded_content" && !event.degraded && event.follow_ups
      ? event.follow_ups
      : [];
  return withTerminal(next, { emptyNoEvidence, followUps });
}

type ErrorEvent = Extract<AgentStreamEvent, { type: "error" }>;

/** error 终局：provider_error 降级撤答（关键词回退请求由调用方副作用发起）；
    其余错误置错误态（横幅渲染条件 = 错误且无正文，由渲染层判定）。 */
function applyError(turn: AgentTurn, event: ErrorEvent): AgentTurn {
  if (event.degraded && event.degraded_reason === "provider_error") {
    return {
      ...turn,
      streaming: false,
      settled: true,
      answer: "",
      terminal: { ...turn.terminal, error: false, degraded: true, needsKeywordFallback: true },
    };
  }
  return {
    ...turn,
    streaming: false,
    settled: true,
    terminal: { ...turn.terminal, error: true, errorCode: event.error_code ?? null },
  };
}

/** 停止按钮：保留已流出的思考/工具/半答，标记停止终态。 */
export function stopAgentTurn(turn: AgentTurn): AgentTurn {
  return { ...turn, streaming: false, settled: true, terminal: { ...turn.terminal, stopped: true } };
}

/** 流在无 done/error 事件时关闭（传输层结束）：仅收敛 streaming 标志。 */
export function closeAgentStream(turn: AgentTurn): AgentTurn {
  return { ...turn, streaming: false, settled: true };
}

/** provider 降级关键词回退结果写轮级引用并清除待回退标记（纯函数）。 */
export function applyKeywordFallbackCitations(
  turn: AgentTurn,
  citations: AgentStreamCitation[],
): AgentTurn {
  return withTerminal(turn, { citations, needsKeywordFallback: false });
}

/* ---------- 历史入口（#538 起为纯函数，#663 自 lib/agent-history.ts 整体迁入） ---------- */

/** 历史端点消息行（GET /api/v1/agent/conversations/:id 的 messages[]）。
 *  #538：phase="tools" 行带持久化工具步骤摘要（tools[]），与流式
 *  tool_status 事件同构；moderation 行由端点做内容脱敏（content 省略）。 */
export interface AgentHistoryMessageDTO {
  id: number;
  role: string;
  content?: string | null;
  phase?: string;
  moderation?: string;
  /** 端点原始引用（可能畸形；由 normalizeAgentCitation 剔除后才进渲染行）。 */
  citations?: unknown[];
  tools?: AgentStreamTool[];
}

/** 行级渲染形态（历史回放与既有测试复用；think/tools phase 行按序保留）。 */
export interface AgentHistoryWorkspaceMessage {
  id: number | string;
  role: "user" | "assistant";
  content: string;
  /** A-02：think 行独立成消息（仅展示层）；#538：tools 行回放工具步骤条。 */
  phase?: "think" | "tools";
  moderationBlocked?: boolean;
  /** 2026-09-06 实测修复：引用随答案消息持久化（历史回放的跳转入口）。 */
  citations?: AgentStreamCitation[];
  /** #538：phase="tools" 行的步骤摘要（AgentToolStatus live=false 渲染）。 */
  tools?: AgentStreamTool[];
}

/** 服务端历史行 → 行级渲染行（原 lib/agent-history.ts 逐字迁入）。
 *  - think/tools phase 行按原顺序保留（与流式轮内渲染同构）；
 *  - blocked 行替换为占位文案（moderationBlocked 标记）；
 *  - 空内容非 phase 行剔除（no_evidence/degraded 撤答不留空泡）。 */
export function mapAgentHistoryMessages(
  messages: AgentHistoryMessageDTO[],
  moderationPlaceholder: string,
): AgentHistoryWorkspaceMessage[] {
  return messages
    .filter(
      (message) =>
        message.role === "user" ||
        message.phase === "think" ||
        message.phase === "tools" ||
        message.moderation === "blocked" ||
        (message.content ?? "").trim() !== "",
    )
    .map((message): AgentHistoryWorkspaceMessage => {
      if (message.moderation === "blocked") {
        return {
          id: message.id,
          role: "assistant",
          content: moderationPlaceholder,
          moderationBlocked: true,
        };
      }
      if (message.phase === "think") {
        return {
          id: message.id,
          role: "assistant",
          content: message.content ?? "",
          phase: "think",
        };
      }
      if (message.phase === "tools") {
        return {
          id: message.id,
          role: "assistant",
          content: "",
          phase: "tools",
          tools: message.tools ?? [],
        };
      }
      const validCitations = (message.citations ?? []).filter(
        (citation): citation is AgentStreamCitation => normalizeAgentCitation(citation) !== null,
      );
      return {
        id: message.id,
        role: message.role === "user" ? "user" : "assistant",
        content: message.content ?? "",
        ...(validCitations.length > 0 ? { citations: validCitations } : {}),
      };
    });
}

function historyTurn(id: string, query: string): AgentTurn {
  return { ...createAgentTurn(query, { id, firstRound: false }), streaming: false, settled: true };
}

/** 历史入口：服务端历史行 → 回合树。行经 mapAgentHistoryMessages 过滤后按
 *  user 行分轮：think/tools 行入相块（连续同类行合并，与 live 入口同构）、
 *  blocked 行占位、正文行置答案（含随行持久化引用）。终态字段（追问/用量/
 *  trace/answer_kind 等）不落库，历史轮保持缺省——两入口的内容形状同一，
 *  终态差异是落库契约而非装配漂移。 */
export function mapAgentHistoryToTurns(
  messages: AgentHistoryMessageDTO[],
  moderationPlaceholder: string,
): AgentTurn[] {
  const rows = mapAgentHistoryMessages(messages, moderationPlaceholder);
  const turns: AgentTurn[] = [];
  let current: AgentTurn | null = null;
  const replaceCurrent = (turn: AgentTurn) => {
    current = turn;
    if (turns.length > 0) turns[turns.length - 1] = turn;
  };
  for (const row of rows) {
    if (row.role === "user") {
      current = historyTurn(String(row.id), row.content);
      turns.push(current);
      continue;
    }
    if (!current) {
      /* 防御：无提问行的头部行（服务端契约下不出现）归入空提问轮。 */
      current = historyTurn(`head-${row.id}`, "");
      turns.push(current);
    }
    if (row.phase === "think") {
      replaceCurrent(appendThink(current, row.content));
    } else if (row.phase === "tools") {
      for (const tool of row.tools ?? []) {
        current = appendTool(current, tool);
      }
      replaceCurrent(current);
    } else if (row.moderationBlocked) {
      replaceCurrent({ ...current, moderationBlocked: true, answer: row.content });
    } else {
      const answer = current.answer === "" ? row.content : `${current.answer}\n\n${row.content}`;
      replaceCurrent({
        ...current,
        answer,
        ...(!current.answerCitations && row.citations && row.citations.length > 0
          ? { answerCitations: row.citations }
          : {}),
      });
    }
  }
  return turns;
}

/** 首轮活动轮活到历史回载落地后的终态并入：把 live 轮的终态字段（追问/
 *  用量/trace/通知旗标）并入树尾轮——跨重载存活的语义等价旧轮级状态。 */
export function mergeTerminalIntoLastTurn(
  turns: AgentTurn[],
  terminal: AgentTurnTerminal,
): AgentTurn[] {
  if (turns.length === 0) return turns;
  const last = turns[turns.length - 1];
  return [...turns.slice(0, -1), { ...last, terminal: { ...last.terminal, ...terminal } }];
}
