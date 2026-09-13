// SP-17/T2 (#491)：auth-required 事件桥——api.ts 这类非 React 模块在 401
// 刷新失败时通知 AuthGateProvider 打开登录浮窗（带 pendingAction 自动续做）。
// 门未挂载（极端路径：provider 尚未 mount / 纯工具页）时兜底带 redirect 跳
// /login，保持旧「会话失效回登录页」语义且不丢当前路径。

export const AUTH_REQUIRED_EVENT = "omnicraft:auth-required";

export interface AuthRequiredDetail {
  pendingAction?: () => void | Promise<void>;
}

let authGateActive = false;

/** AuthGateProvider mount/unmount 时切换；api.ts 据此决定事件 vs 跳页兜底。 */
export function setAuthGateActive(active: boolean) {
  authGateActive = active;
}

export function emitAuthRequired(detail?: AuthRequiredDetail) {
  if (typeof window === "undefined") return;
  if (!authGateActive) {
    const current = window.location.pathname + window.location.search;
    window.location.href = `/login?redirect=${encodeURIComponent(current)}`;
    return;
  }
  window.dispatchEvent(new window.CustomEvent<AuthRequiredDetail>(AUTH_REQUIRED_EVENT, { detail }));
}
