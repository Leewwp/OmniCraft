"use client";

import { useEffect, useState } from "react";

import { fetchPublicConfig, type PublicConfig } from "./public-config";

/**
 * 客户端公开配置 hook（#687）：首帧返回 null（读取层全部访问器对
 * null/undefined 走内置兜底 = 既有行为），配置到达后触发重渲染。取数
 * 失败静默保持兜底——/config/public 属增强信息，不阻塞页面。
 */
export function usePublicConfig(): PublicConfig | null {
  const [config, setConfig] = useState<PublicConfig | null>(null);
  useEffect(() => {
    let cancelled = false;
    fetchPublicConfig()
      .then((cfg) => {
        if (!cancelled) setConfig(cfg);
      })
      .catch(() => {
        // 兜底语义继续生效
      });
    return () => {
      cancelled = true;
    };
  }, []);
  return config;
}
