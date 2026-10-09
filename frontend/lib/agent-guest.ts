// #854 匿名 Agent 面（T2）前端侧纯逻辑：guest 端点、余量状态、终局判定与
// 登录续做交接。全部走普通 fetch 且绝不携带 Authorization——游客面只服务
// 匿名设备身份，任何凭证（有效与否）都会被服务端 403 拒绝，而非降级。

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export const GUEST_AGENT_ENDPOINTS = {
  models: "/api/v1/agent/guest/models",
  quota: "/api/v1/agent/guest/quota",
  conversations: "/api/v1/agent/guest/conversations",
  conversation: (id: number) => `/api/v1/agent/guest/conversations/${id}`,
  chatStream: "/api/v1/agent/guest/chat/stream",
} as const;

export interface GuestQuotaState {
  remaining: number;
  maxTurns: number;
  exhausted: boolean;
}

export interface GuestApiError {
  code: string;
  message: string;
  status: number;
}

export class GuestRequestError extends Error {
  constructor(
    public code: string,
    message: string,
    public status: number,
  ) {
    super(message);
    this.name = "GuestRequestError";
  }
}

/** 游客端点的裸 fetch：credentials include（设备 cookie + CSRF 双提交），
 *  永不附带 Authorization。非 2xx 抛 GuestRequestError（code 来自响应体）。 */
async function guestFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    credentials: "include",
  });
  if (!res.ok) {
    let code = "UNKNOWN_ERROR";
    let message = res.statusText;
    try {
      const body = (await res.json()) as { code?: string; message?: string };
      if (body.code) code = body.code;
      if (body.message) message = body.message;
    } catch {
      /* 非 JSON 错误体保持 statusText */
    }
    throw new GuestRequestError(code, message, res.status);
  }
  return (await res.json()) as T;
}

/** SSE 直连用的 fetch 包装：剥离调用方已附加的 Authorization 头
 *  （startAgentStream 默认带 token；游客面只认匿名设备身份）。 */
export const stripAuthorizationFetch: typeof fetch = (input, init) => {
  if (!init?.headers) return fetch(input, init);
  const headers = new Headers(init.headers as HeadersInit);
  headers.delete("Authorization");
  return fetch(input, { ...init, headers });
};

/** 游客读端点的共享裸 fetch（列表/会话回放/模型列表）：同 guestFetch 语义。 */
export function fetchGuestJson<T>(path: string): Promise<T> {
  return guestFetch<T>(path);
}

/** 余量查询兼身份引导：无设备 cookie 时服务端在此端点发放并初始化（读路径
 *  引导），因此工作台挂载即调用本函数是正常流程的第一步。 */
export async function fetchGuestQuota(): Promise<GuestQuotaState> {
  const data = await guestFetch<{
    remaining: number;
    max_turns: number;
    exhausted: boolean;
  }>(GUEST_AGENT_ENDPOINTS.quota);
  return {
    remaining: data.remaining,
    maxTurns: data.max_turns,
    exhausted: data.exhausted,
  };
}

/** 游客面终局余量错误码：这些码之后工作台必须呈现转化卡（登录墙），
 *  且不得再对游客端点发起生成请求。 */
const GUEST_QUOTA_TERMINAL_CODES = new Set([
  "GUEST_QUOTA_EXHAUSTED",
  "GUEST_QUOTA_STATE_LOST",
]);

export function isGuestQuotaTerminal(code?: string): boolean {
  return typeof code === "string" && GUEST_QUOTA_TERMINAL_CODES.has(code);
}

/**
 * 登录续做交接（consume-once）：游客被登录墙拦下的「尚未执行的动作」
 * （草稿消息）在登录成功后的账号新会话中执行。sessionStorage 承载、
 * take 即清——刷新/重放不二次执行已消费的游客请求；账号历史不迁移。
 */
const GUEST_HANDOVER_KEY = "omnicraft.guestAgentHandover";

export function saveGuestHandover(message: string): void {
  if (typeof window === "undefined") return;
  try {
    window.sessionStorage.setItem(
      GUEST_HANDOVER_KEY,
      JSON.stringify({ message, savedAt: Date.now() }),
    );
  } catch {
    /* 存储不可用时静默放弃（续做是增强，不是承诺） */
  }
}

export function takeGuestHandover(): string | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.sessionStorage.getItem(GUEST_HANDOVER_KEY);
    if (!raw) return null;
    window.sessionStorage.removeItem(GUEST_HANDOVER_KEY);
    const parsed = JSON.parse(raw) as { message?: unknown };
    if (typeof parsed.message === "string" && parsed.message.trim() !== "") {
      return parsed.message;
    }
  } catch {
    /* 畸形记录按不存在处理 */
  }
  return null;
}

/** 双态判定（纯函数）：/agent 表面由恢复完成后的真实身份与特性开关决定。
 *  待定 = 中性壳；登录用户 = 既有受保护工作台（webAgent 门照旧）；
 *  游客 = 总闸开启才给游客工作台，否则登录引导降级。 */
export type AgentSurface = "pending" | "user" | "guest" | "login-guide";

export function resolveAgentSurface(
  isLoading: boolean,
  hasUser: boolean,
  features: { web_agent_enabled: boolean; guest_agent_enabled: boolean },
): AgentSurface {
  if (isLoading) return "pending";
  if (hasUser) return features.web_agent_enabled ? "user" : "login-guide";
  return features.web_agent_enabled && features.guest_agent_enabled
    ? "guest"
    : "login-guide";
}

/** 入口可见性（纯函数）：Header/落地页共用——登录用户看 webAgent 门，
 *  游客看 guest 总闸；轮数从不进入口（Q21）。 */
export function agentEntryVisible(
  hasUser: boolean,
  userVerified: boolean,
  features: { web_agent_enabled: boolean; guest_agent_enabled: boolean },
): boolean {
  if (hasUser) return features.web_agent_enabled && userVerified;
  return features.web_agent_enabled && features.guest_agent_enabled;
}
