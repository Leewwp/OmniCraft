"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import {
  installOverlayHistoryController,
  registerOverlayHistoryNavigator,
} from "@/lib/overlay-history";

/** D3 #860：详情弹窗 history 状态机的常驻接管者（根布局挂载一次）。
 *  1. 安装全局 popstate 控制器——浮层关闭后浏览器 forward/back 命中死会话
 *     记录时（浮层组件已卸载、无人订阅）由本 bridge 的导航器接管；
 *  2. 注册 SPA 导航器：router.replace 到记录的规范路径，让宿主详情页在
 *     当前地址渲染（地址与可见内容一致），同时截掉其后残留的死记录。
 *  未注册时 lib/overlay-history 回退 location.replace，行为一致。 */
export function OverlayHistoryBridge() {
  const router = useRouter();
  useEffect(() => {
    installOverlayHistoryController();
    return registerOverlayHistoryNavigator((href) => router.replace(href));
  }, [router]);
  return null;
}
