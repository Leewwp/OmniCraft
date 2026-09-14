"use client";

import { StudioSidebar } from "./StudioSidebar";

/* SP-19 G1-1：顶栏由 (headered) 布局统一渲染，本组件不再自带 Header（防双渲染）；
 * 高度扣减 52px 与全局 --header-h 一致，语义不变。 */
export function StudioLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-[calc(100vh-52px)]">
      <StudioSidebar />
      <main className="flex-1 overflow-y-auto bg-background pb-16 sm:pb-0">
        <div className="mx-auto max-w-[1280px] px-6 py-6">
          {children}
        </div>
      </main>
    </div>
  );
}
