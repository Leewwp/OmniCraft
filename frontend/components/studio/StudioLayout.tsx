"use client";

import { StudioSidebar } from "./StudioSidebar";
import { Header } from "@/components/layout/Header";

export function StudioLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <Header />
      <div className="flex h-[calc(100vh-52px)]">
        <StudioSidebar />
        <main className="flex-1 overflow-y-auto bg-background">
          <div className="mx-auto max-w-[1280px] px-6 py-6">
            {children}
          </div>
        </main>
      </div>
    </>
  );
}
