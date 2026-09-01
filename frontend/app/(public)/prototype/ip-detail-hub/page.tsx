/**
 * 【原型专用路由，随时可删】IP 详情页「贴吧式社区枢纽」交互原型。
 * 生产构建下整路由 404（notFound 兜底），不会出现在正式产物中。
 * 原型说明见 components/prototype/ip-detail-hub/ip-hub-prototype.tsx 顶部注释。
 */
import { Suspense } from "react";
import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { IpHubPrototype } from "@/components/prototype/ip-detail-hub/ip-hub-prototype";

export const metadata: Metadata = {
  title: "原型 · IP 详情页社区枢纽",
  robots: { index: false, follow: false },
};

export default function IpDetailHubPrototypePage() {
  if (process.env.NODE_ENV === "production") {
    notFound();
  }
  return (
    <Suspense fallback={null}>
      <IpHubPrototype />
    </Suspense>
  );
}
