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
    firstRound: options.firstRound,
    terminal: {
      answerKind: null,
      degraded: false,
      citations: [],
      followUps: [],
      usage: null,
      traceId: null,
      emptyNoEvidence: false,
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
      answer: "",
      terminal: { ...turn.terminal, error: false, degraded: true },
    };
  }
  return {
    ...turn,
    streaming: false,
    terminal: { ...turn.terminal, error: true, errorCode: event.error_code ?? null },
  };
}

/** 停止按钮：保留已流出的思考/工具/半答，标记停止终态。 */
export function stopAgentTurn(turn: AgentTurn): AgentTurn {
  return { ...turn, streaming: false, terminal: { ...turn.terminal, stopped: true } };
}

/** 流在无 done/error 事件时关闭（传输层结束）：仅收敛 streaming 标志。 */
export function closeAgentStream(turn: AgentTurn): AgentTurn {
  return { ...turn, streaming: false };
}

/** provider 降级关键词回退结果写轮级引用（纯函数；请求由调用方发起）。 */
export function applyKeywordFallbackCitations(
  turn: AgentTurn,
  citations: AgentStreamCitation[],
): AgentTurn {
  return withTerminal(turn, { citations });
}
