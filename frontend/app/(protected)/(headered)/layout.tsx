import { Header } from "@/components/layout/Header";

// SP-19 G1-1：受保护工作台页统一顶栏（route group，URL 不变）。
// 只渲染 Header 不带 Footer（工作台形态）。admin 仍留在 (protected) 直下
// 用自己的侧边栏后台布局、不经本组——但 2026-10-04 SP-26 A-4（#780）推翻了
// G1-1「admin 无顶栏」的取向：admin 布局现自带全站 <Header />（详见其文件注释）。
export default function HeaderedLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1">{children}</main>
    </div>
  );
}
