import { HomePageClient } from "@/components/home/HomePageClient";
import { BackToTopButton } from "@/components/shared/BackToTopButton";
import { getBrowserApiBase } from "@/lib/server-api";
import type { HomePageData } from "@/lib/home-page-data";

// 二创区页视图（T1 #853）：`/`（登录态）与 `/fanworks` 共用。
// data=null = SSR 首屏不可用（如匿名落地页上完成客户端登录后切到的
// 二创页），走 HomePageClient 客户端加载路径（与筛选切换重挂同路径）。
export function FanworksHomeView({ data }: { data: HomePageData | null }) {
  return (
    <>
      <HomePageClient
        apiBase={getBrowserApiBase()}
        initialIPs={data?.ips ?? []}
        initialContents={data?.contents ?? []}
        initialContentTotal={data?.total ?? null}
        ssrFirstPage={data !== null}
      />
      <BackToTopButton />
    </>
  );
}
