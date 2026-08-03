"use client";

import {
  createContext,
  useCallback,
  useContext,
  useState,
  useEffect,
  useRef,
  type ReactNode,
} from "react";
import { X, CheckCircle, AlertTriangle, Info, AlertCircle } from "lucide-react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";

type ToastType = "success" | "error" | "warning" | "info";

interface Toast {
  id: number;
  type: ToastType;
  message: string;
  exiting?: boolean;
}

interface ToastContextValue {
  toast: (type: ToastType, message: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

const iconMap: Record<ToastType, typeof CheckCircle> = {
  success: CheckCircle,
  error: AlertCircle,
  warning: AlertTriangle,
  info: Info,
};

const colorMap: Record<ToastType, string> = {
  success: "border-[var(--tag-green-fg)] bg-[var(--tag-green-bg)]",
  error: "border-[var(--tag-rose-fg)] bg-[var(--tag-rose-bg)]",
  warning: "border-[var(--tag-orange-fg)] bg-[var(--tag-orange-bg)]",
  info: "border-[var(--tag-blue-fg)] bg-[var(--tag-blue-bg)]",
};

const iconColorMap: Record<ToastType, string> = {
  success: "text-[var(--tag-green-fg)]",
  error: "text-[var(--tag-rose-fg)]",
  warning: "text-[var(--tag-orange-fg)]",
  info: "text-[var(--tag-blue-fg)]",
};

let nextId = 0;
const TOAST_DURATION = 4000;
const EXIT_ANIMATION_MS = 200;

function getExitAnimationMs() {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return EXIT_ANIMATION_MS;
  }
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches ? 0 : EXIT_ANIMATION_MS;
}

function ToastItem({ toast, onRemove }: { toast: Toast; onRemove: (id: number) => void }) {
  const t = useTranslations();
  const [visible, setVisible] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    // Trigger enter animation
    requestAnimationFrame(() => setVisible(true));

    // Auto-dismiss
    timerRef.current = setTimeout(() => {
      setVisible(false);
      setTimeout(() => onRemove(toast.id), getExitAnimationMs());
    }, TOAST_DURATION);

    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [toast.id, onRemove]);

  const Icon = iconMap[toast.type];
  const assertive = toast.type === "error" || toast.type === "warning";

  return (
    <div
      role={assertive ? "alert" : "status"}
      aria-live={assertive ? "assertive" : "polite"}
      className={cn(
        "flex w-full items-start gap-3 rounded-lg border bg-card p-3 shadow-md transition-[opacity,transform] duration-200 motion-reduce:translate-x-0 motion-reduce:transition-none",
        colorMap[toast.type],
        visible ? "translate-x-0 opacity-100" : "translate-x-4 opacity-0"
      )}
    >
      <Icon className={cn("h-5 w-5 shrink-0 mt-0.5", iconColorMap[toast.type])} />
      <p className="flex-1 text-sm text-foreground">{toast.message}</p>
      <button
        type="button"
        aria-label={t("common.close")}
        onClick={() => {
          setVisible(false);
          setTimeout(() => onRemove(toast.id), getExitAnimationMs());
        }}
        className="inline-flex size-6 shrink-0 items-center justify-center rounded-md text-muted-foreground outline-none transition-colors duration-150 hover:bg-background/60 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring [@media(pointer:coarse)]:size-11"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const addToast = useCallback((type: ToastType, message: string) => {
    const id = ++nextId;
    setToasts((prev) => [...prev, { id, type, message }]);
  }, []);

  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ toast: addToast }}>
      {children}
      {/* Fixed toast container - top right */}
      <div
        className="fixed left-4 right-4 top-4 z-[100] flex flex-col gap-2 sm:left-auto sm:max-w-sm"
      >
        {toasts.map((t) => (
          <ToastItem key={t.id} toast={t} onRemove={removeToast} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    // Return a no-op fallback when used outside provider (SSR or missing wrapper)
    return { toast: () => {} };
  }
  return ctx;
}
