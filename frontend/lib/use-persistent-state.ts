"use client";

import { useCallback, useEffect, useRef, useState } from "react";

interface UsePersistentStateOptions<T> {
  storageKey: string;
  /** 首帧（hydration 安全）与 storage 不可用/缺键时的取值。 */
  fallback: T;
  /** 从存储原文解析出状态值；返回值应与 fallback 同型。 */
  parse: (raw: string) => T;
  /** 状态值序列化为存储原文；磁盘编码由调用方定义并保持稳定。 */
  serialize: (value: T) => string;
}

/**
 * localStorage 持久化状态（#806 A3）：
 * - hydration 安全：首帧渲染 fallback，存储值在 effect 后置应用，
 *   SSR 标记与服务端 HTML 不因客户端存储抖动；
 * - setter 先持久化再更新状态，读写全程 try/catch——存储不可用
 *   （隐私模式/配额）时退化为纯内存状态，导航主链路不受影响。
 * parse/serialize 经 latest-ref 取用：调用方传内联箭头函数也不会让
 * 读效应每帧重跑或让 update 失稳。
 */
export function usePersistentState<T>({
  storageKey,
  fallback,
  parse,
  serialize,
}: UsePersistentStateOptions<T>) {
  const [value, setValue] = useState<T>(fallback);
  const parseRef = useRef(parse);
  const serializeRef = useRef(serialize);
  parseRef.current = parse;
  serializeRef.current = serialize;

  useEffect(() => {
    try {
      const raw = window.localStorage.getItem(storageKey);
      if (raw !== null) {
        setValue(parseRef.current(raw));
      }
    } catch {
      // Keep the in-memory default when storage is unavailable.
    }
  }, [storageKey]);

  const update = useCallback(
    (next: T) => {
      try {
        window.localStorage.setItem(storageKey, serializeRef.current(next));
      } catch {
        // Persistence is best-effort; the value still applies in memory.
      }
      setValue(next);
    },
    [storageKey],
  );

  return [value, update] as const;
}
