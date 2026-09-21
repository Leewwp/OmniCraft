"use client";

import { useEffect, useState } from "react";

/**
 * SP-25 FR-07（中-1 渲染层兜底）：模型输出里的图片只在平台自有图源上加载。
 * 后端 SanitizeImageURLs 覆盖终稿与落库原文，但 SSE 流式 delta 阶段浏览器
 * 会逐帧渲染未消毒的增量——此处按域名白名单把非平台 <img> 替换为占位文案。
 *
 * 白名单三来源：相对路径（站内代理/数据图）、*.aliyuncs.com（平台 OSS 基线，
 * 与 next.config images 白名单一致）、/config/public 的 oss_domain（配送域，
 * 加载后生效；请求经模块级 promise 去重，流式逐帧渲染不会反复拉取）。
 */

let ossDomainPromise: Promise<string> | null = null;

function fetchOssDomain(): Promise<string> {
  if (!ossDomainPromise) {
    ossDomainPromise = Promise.resolve()
      .then(() => {
        // 测试运行时（jsdom 无全局 fetch）同步 ReferenceError 会击穿渲染，
        // 先探测再调用。
        if (typeof fetch !== "function") {
          return "";
        }
        return fetch("/api/v1/config/public")
          .then((res) => (res.ok ? res.json() : null))
          .then((cfg: { oss_domain?: string } | null) => cfg?.oss_domain ?? "")
          .catch(() => "");
      })
      .catch(() => "");
  }
  return ossDomainPromise;
}

function hostnameOf(src: string): string {
  try {
    return new URL(src).hostname.toLowerCase();
  } catch {
    return "";
  }
}

/** 判断图片 src 是否允许加载（纯函数，便于测试）。 */
export function isAllowedImageSrc(src: string | undefined, ossDomain: string): boolean {
  if (!src) {
    return false;
  }
  // 站内相对路径与协议相对按同源处理。
  if (src.startsWith("/") && !src.startsWith("//")) {
    return true;
  }
  const host = hostnameOf(src);
  if (!host) {
    return false;
  }
  if (host === "localhost" || host === "127.0.0.1") {
    return true;
  }
  if (host.endsWith(".aliyuncs.com")) {
    return true;
  }
  const configured = ossDomain.trim().toLowerCase();
  if (configured && (host === configured || src.startsWith(`https://${configured}/`) || src.startsWith(`http://${configured}/`))) {
    return true;
  }
  return false;
}

/**
 * 读取平台图源白名单（oss_domain）。返回 "" 表示尚未加载——此时静态基线
 * （相对路径 + aliyuncs.com）仍然生效，配送域图片在配置到达后经重渲染放行。
 */
export function useImageHostAllowlist(): string {
  const [ossDomain, setOssDomain] = useState("");
  useEffect(() => {
    let alive = true;
    void fetchOssDomain().then((domain) => {
      if (alive) {
        setOssDomain(domain);
      }
    });
    return () => {
      alive = false;
    };
  }, []);
  return ossDomain;
}
