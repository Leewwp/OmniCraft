"use client";

import { useSyncExternalStore } from "react";

/**
 * 浮层族视口缝（C2 #921）：断点语义有名有主的唯一来源。
 *
 * - twoColumn（960）：浮层双栏界——统一版式壳层（面板/工具栏分形）、
 *   Layer 桌面分支、ContentDetail creatorFirst 生效门、MediaGallery 移动分端
 *   （mobileLayout = !twoColumn，旧 959 反写消灭）。
 * - pcTier（1100）：ui-spec 全局三档 PC 档界——连续浏览 reach-end 门
 *   （<1100 才放行）、ContentDetail 行内关联行降级判定。
 *
 * 实现 = useSyncExternalStore + 模块级 MQL 缓存（SurfaceStage 先例）：
 * MediaQueryList 按 query 字符串缓存、按 window.matchMedia 函数身份失效
 * （测试更换桩自动重建）；同 query 的 N 个调用点复用同一 MQL 对象、各自直
 * 订阅，不做广播/引用计数层。
 *
 * getServerSnapshot 固定 false：ContentDetail / MediaGallery 会在独立详情页
 * SSR 树中预渲染（original 页直渲染、fanwork 页经 Host），挂载时同步读
 * matchMedia 会造成服务端 false vs 客户端 true 的水合分裂；uSES 的 server
 * snapshot 路径首帧语义与旧 useState+useEffect 逐一等价（ContentDetail 首帧
 * false、MediaGallery 首帧 mobileLayout=true），水合后快照差异由 uSES 的
 * 客户端重渲染消化。无 matchMedia 环境（SSR/测试）默认 { false, false }，
 * 与五处旧实现的缺省值逐一等价。
 */

/** 浮层双栏界断点（≥960 命中）。 */
export const TWO_COLUMN_QUERY = "(min-width: 960px)";
/** ui-spec 全局三档 PC 档界断点（≥1100 命中）。 */
export const PC_TIER_QUERY = "(min-width: 1100px)";

const mqlCache = new Map<string, MediaQueryList>();
let cachedMatchMedia: ((query: string) => MediaQueryList) | null = null;

function getMql(query: string): MediaQueryList | null {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return null;
  const matchMedia = window.matchMedia;
  if (cachedMatchMedia !== matchMedia) {
    cachedMatchMedia = matchMedia;
    mqlCache.clear();
  }
  let mql = mqlCache.get(query);
  if (!mql) {
    mql = matchMedia(query);
    mqlCache.set(query, mql);
  }
  return mql;
}

/** uSES server snapshot：SSR/水合首帧恒 false（见模块注释）。 */
export function overlayViewportServerSnapshot(): false {
  return false;
}

function createSnapshot(query: string): () => boolean {
  return () => Boolean(getMql(query)?.matches);
}

function createSubscribe(query: string): (onChange: () => void) => () => void {
  return (onChange: () => void) => {
    const mql = getMql(query);
    if (!mql || typeof mql.addEventListener !== "function") return () => undefined;
    mql.addEventListener("change", onChange);
    return () => {
      mql.removeEventListener("change", onChange);
    };
  };
}

const getTwoColumn = createSnapshot(TWO_COLUMN_QUERY);
const subscribeTwoColumn = createSubscribe(TWO_COLUMN_QUERY);
const getPcTier = createSnapshot(PC_TIER_QUERY);
const subscribePcTier = createSubscribe(PC_TIER_QUERY);

export interface OverlayViewport {
  /** ≥960：浮层双栏界（统一版式壳层与媒体分端同源）。 */
  twoColumn: boolean;
  /** ≥1100：ui-spec 全局三档 PC 档界。 */
  pcTier: boolean;
}

/** 浮层族视口语义唯一入口（五处定值订阅 + 两个事件时助手全部归一于此）。 */
export function useOverlayViewport(): OverlayViewport {
  const twoColumn = useSyncExternalStore(
    subscribeTwoColumn,
    getTwoColumn,
    overlayViewportServerSnapshot,
  );
  const pcTier = useSyncExternalStore(
    subscribePcTier,
    getPcTier,
    overlayViewportServerSnapshot,
  );
  return { twoColumn, pcTier };
}
