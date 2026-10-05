"use client";

import { useCallback } from "react";

import { usePersistentState } from "@/lib/use-persistent-state";

export const PUBLIC_SIDEBAR_STORAGE_KEY = "sidebarCollapsed";
export const STUDIO_SIDEBAR_STORAGE_KEY = "studio_sidebar_collapsed";
export const ADMIN_SIDEBAR_STORAGE_KEY = "admin_sidebar_collapsed";

interface UseSidebarCollapseOptions {
  storageKey: string;
}

type CollapseStateUpdate = boolean | ((current: boolean) => boolean);

/* 磁盘编码 "true"/"false"（#806 A3 前即如此，保持不变以免重置既有偏好）。 */
const parseCollapseStored = (raw: string) => raw === "true";
const serializeCollapse = (collapsed: boolean) => String(collapsed);

/**
 * 通用布尔折叠 hook：持久化语义全部委托 usePersistentState
 * （#806 A3 起两个 localStorage hook 不再各写一份读写样板），
 * 本体只保留函数式更新与 toggle 便捷层。
 */
export function useSidebarCollapse({
  storageKey,
}: UseSidebarCollapseOptions) {
  const [collapsed, setCollapsedValue] = usePersistentState<boolean>({
    storageKey,
    fallback: false,
    parse: parseCollapseStored,
    serialize: serializeCollapse,
  });

  const setCollapsed = useCallback(
    (update: CollapseStateUpdate) => {
      setCollapsedValue(
        typeof update === "function" ? update(collapsed) : update,
      );
    },
    [collapsed, setCollapsedValue],
  );

  const toggle = useCallback(() => {
    setCollapsed((current) => !current);
  }, [setCollapsed]);

  return { collapsed, setCollapsed, toggle };
}
