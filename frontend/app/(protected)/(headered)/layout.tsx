import { Header } from "@/components/layout/Header";

// SP-19 G1-1：受保护工作台页统一顶栏（route group，URL 不变）。
// 只渲染 Header 不带 Footer（工作台形态）；admin 留在 (protected) 直下
// 用自己的侧边栏后台布局，不经本组。
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
