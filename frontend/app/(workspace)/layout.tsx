import { Header } from "@/components/layout/Header";

// #854：/agent 双主体壳（route group，URL 不变）。与 (protected)/(headered)
// 同构的顶栏形态，但不带登录守卫——登录用户与游客都从这里进入工作台；
// 表面判定在 AgentWorkspaceGate（真实身份 + 特性开关）。其余 (protected)
// 路由守卫不放宽。
export default function WorkspaceLayout({
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
