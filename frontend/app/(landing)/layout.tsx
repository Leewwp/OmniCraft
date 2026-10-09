import type { Viewport } from "next";
import "./landing.css";

// 落地页路由组（T1 #853）：/（匿名）独用一套壳——只有落地页自身顶栏，
// 不带产品 Header/Footer（其他公开路由保留 (public) 原壳）。
// 根 layout 继续提供 next-intl / next-themes / AuthProvider（双态门依赖）。

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
};

export default function LandingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <>{children}</>;
}
