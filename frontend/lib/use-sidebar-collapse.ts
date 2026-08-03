"use client";

import { useCallback, useEffect, useState } from "react";

export const PUBLIC_SIDEBAR_STORAGE_KEY = "sidebarCollapsed";
export const STUDIO_SIDEBAR_STORAGE_KEY = "studio_sidebar_collapsed";
export const ADMIN_SIDEBAR_STORAGE_KEY = "admin_sidebar_collapsed";

interface UseSidebarCollapseOptions {
  storageKey: string;
}

type CollapseStateUpdate = boolean | ((current: boolean) => boolean);

export function useSidebarCollapse({
  storageKey,
}: UseSidebarCollapseOptions) {
  const [collapsed, setCollapsedState] = useState(false);

  useEffect(() => {
    try {
      const stored = window.localStorage.getItem(storageKey);
      if (stored !== null) {
        setCollapsedState(stored === "true");
      }
    } catch {
      // Keep the in-memory default when storage is unavailable.
    }
  }, [storageKey]);

  const setCollapsed = useCallback(
    (update: CollapseStateUpdate) => {
      setCollapsedState((current) => {
        const next = typeof update === "function" ? update(current) : update;
        try {
          window.localStorage.setItem(storageKey, String(next));
        } catch {
          // Persistence is best-effort; navigation remains usable in memory.
        }
        return next;
      });
    },
    [storageKey],
  );

  const toggle = useCallback(() => {
    setCollapsed((current) => !current);
  }, [setCollapsed]);

  return { collapsed, setCollapsed, toggle };
}
