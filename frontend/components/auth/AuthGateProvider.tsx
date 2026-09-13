"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  ReactNode,
  ComponentType,
} from "react";
import { AUTH_REQUIRED_EVENT, AuthRequiredDetail, emitAuthRequired, setAuthGateActive } from "@/lib/auth-gate";
import { LoginModal, LoginModalProps } from "@/components/auth/LoginModal";

// SP-17/T2 (#491)：全站未登录门。requireAuth(pendingAction?) 打开登录浮窗，
// 登录成功自动关浮窗并续做原动作；setPortalContainer 供原生 <dialog> top
// layer 场景（内容详情浮窗）把浮窗渲染进 dialog 内部，避免被压层。

interface AuthGateContextValue {
  requireAuth: (pendingAction?: () => void | Promise<void>) => void;
  setPortalContainer: (container: HTMLElement | null) => void;
  /** 详情浮窗 onCancel 据此让位：浮窗打开时 Esc 只关浮窗不关详情。 */
  isGateOpen: boolean;
}

const AuthGateContext = createContext<AuthGateContextValue | null>(null);

export function useAuthGate() {
  const ctx = useContext(AuthGateContext);
  if (ctx) return ctx;
  // 兜底：provider 不可达（纯组件单测 / 异常树）时直走事件桥——其它 provider
  // 实例在场则照常开浮窗，完全无 provider 时由桥内兜底带 redirect 跳 /login，
  // 与旧版硬跳语义对齐，不静默丢功能。
  return {
    requireAuth: (pendingAction?: () => void | Promise<void>) =>
      emitAuthRequired({ pendingAction }),
    setPortalContainer: () => {},
    isGateOpen: false,
  };
}

interface AuthGateProviderProps {
  children: ReactNode;
  /** 测试注入口（RegisterPageContent CaptchaComponent 先例）。 */
  captchaComponent?: LoginModalProps["captchaComponent"];
}

export function AuthGateProvider({ children, captchaComponent }: AuthGateProviderProps) {
  const [open, setOpen] = useState(false);
  const [portalContainer, setPortalContainer] = useState<HTMLElement | null>(null);
  /* pendingAction 只走 ref：函数直接 setState 会触发 React eagerState 优化
     （函数 payload 被当 updater 立即调用，动作在登录前就执行——实测踩坑）。 */
  const pendingActionRef = useRef<(() => void | Promise<void>) | null>(null);
  /* 焦点归还目标：requireAuth 触发瞬间（点击处理器内）的 activeElement，
     而非浮窗渲染完成时的焦点——后者已被浮窗自身的 autofocus 污染。 */
  const restoreFocusRef = useRef<HTMLElement | null>(null);
  /* 事件监听器（[] 依赖）读取的开闭镜像：浮窗已开时迟到的 auth-required
     事件（后台轮询 401 → 刷新失败）不得清掉用户正等待续做的 pendingAction。 */
  const openRef = useRef(false);

  const captureTriggerFocus = useCallback(() => {
    restoreFocusRef.current =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;
  }, []);

  const requireAuth = useCallback((action?: () => void | Promise<void>) => {
    captureTriggerFocus();
    pendingActionRef.current = action ?? null;
    openRef.current = true;
    setOpen(true);
  }, [captureTriggerFocus]);

  useEffect(() => {
    setAuthGateActive(true);
    const handleAuthRequired = (event: Event) => {
      // 浮窗已开：忽略重复事件——事件桥触发无 pendingAction，落到这里会把
      // 首个触发挂上的续做动作清空（登录成功后无事可做），首个触发优先。
      if (openRef.current) return;
      const detail = (event as CustomEvent<AuthRequiredDetail>).detail;
      // 事件桥触发（api 401）无 UI 触发元：清除上次残留的归还目标。
      restoreFocusRef.current = null;
      pendingActionRef.current = detail?.pendingAction ?? null;
      openRef.current = true;
      setOpen(true);
    };
    window.addEventListener(AUTH_REQUIRED_EVENT, handleAuthRequired);
    return () => {
      window.removeEventListener(AUTH_REQUIRED_EVENT, handleAuthRequired);
      setAuthGateActive(false);
    };
  }, []);

  const handleClose = useCallback(() => {
    openRef.current = false;
    setOpen(false);
    pendingActionRef.current = null;
  }, []);

  const handleSuccess = useCallback(() => {
    openRef.current = false;
    setOpen(false);
    const action = pendingActionRef.current;
    pendingActionRef.current = null;
    if (action) {
      void Promise.resolve()
        .then(action)
        .catch(() => {
          /* 续做失败由各门自身错误态呈现，不在门层兜底 */
        });
    }
  }, []);

  const value = useMemo(
    () => ({ requireAuth, setPortalContainer, isGateOpen: open }),
    [requireAuth, open],
  );

  return (
    <AuthGateContext.Provider value={value}>
      {children}
      <LoginModal
        open={open}
        onClose={handleClose}
        onSuccess={handleSuccess}
        portalContainer={portalContainer}
        captchaComponent={captchaComponent}
        restoreFocusRef={restoreFocusRef}
      />
    </AuthGateContext.Provider>
  );
}
