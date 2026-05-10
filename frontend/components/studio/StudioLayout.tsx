"use client";

import { StudioSidebar } from "./StudioSidebar";

export function StudioLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-[calc(100vh-52px)]">
      <StudioSidebar />
      <main className="flex-1 overflow-y-auto bg-background">
        <div className="mx-auto max-w-[1280px] px-6 py-6">
          {children}
        </div>
      </main>
    </div>
  );
}
