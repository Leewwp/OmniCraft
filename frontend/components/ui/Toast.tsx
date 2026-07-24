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
  success: "border-green-500/30 bg-green-50 dark:bg-green-950/30",
  error: "border-destructive/30 bg-red-50 dark:bg-red-950/30",
  warning: "border-yellow-500/30 bg-yellow-50 dark:bg-yellow-950/30",
  info: "border-blue-500/30 bg-blue-50 dark:bg-blue-950/30",
};

const iconColorMap: Record<ToastType, string> = {
  success: "text-green-600 dark:text-green-400",
  error: "text-destructive",
  warning: "text-yellow-600 dark:text-yellow-400",
  info: "text-blue-600 dark:text-blue-400",
};

let nextId = 0;
const TOAST_DURATION = 4000;
const EXIT_ANIMATION_MS = 300;

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
      setTimeout(() => onRemove(toast.id), EXIT_ANIMATION_MS);
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
        "flex w-full max-w-sm items-start gap-3 rounded-md border p-3 shadow-md transition-all duration-300",
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
          setTimeout(() => onRemove(toast.id), EXIT_ANIMATION_MS);
        }}
        className="inline-flex size-6 shrink-0 items-center justify-center rounded text-muted-foreground hover:text-foreground [@media(pointer:coarse)]:size-11"
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
        className="fixed right-4 top-4 z-[100] flex flex-col gap-2"
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
