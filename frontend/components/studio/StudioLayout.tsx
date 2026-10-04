"use client";

import { StudioSidebar } from "./StudioSidebar";

/* SP-19 G1-1：顶栏由 (headered) 布局统一渲染，本组件不再自带 Header（防双渲染）；
 * 高度扣减 52px 与全局 --header-h 一致，语义不变。 */
export function StudioLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-[calc(100vh-52px)]">
      <StudioSidebar />
      <main className="flex-1 overflow-y-auto bg-background pb-16 sm:pb-0">
        {/* SP-26 A-3（#780）：移动端左缘为浮动「展开侧边栏」按钮（fixed left-4
            top-[60px]，44×44，StudioSidebar 唯一移动入口）让位——页头左移会与
            按钮重叠遮挡 H1 首二字；≥701px 按钮隐藏，恢复对称 px-6。 */}
        <div className="mx-auto max-w-[1280px] pl-16 pr-6 py-6 min-[701px]:px-6">
          {children}
        </div>
      </main>
    </div>
  );
}
