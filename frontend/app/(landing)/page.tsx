import { cookies } from "next/headers";
import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { RootAuthGate } from "@/components/landing/RootAuthGate";
import { Header } from "@/components/layout/Header";
import { Footer } from "@/components/layout/Footer";
import { FanworksHomeView } from "@/components/home/FanworksHomeView";
import { fetchHomePageData } from "@/lib/home-page-data";

// `/` 双态页（票 #853 / spec §12.2）：
// - 双态由 AuthProvider 恢复完成后的真实身份决定；SSR 只读 refresh cookie
//   的存在性作初始提示——不轮换、不把「有 cookie」当登录成功；
// - refresh cookie 不存在 ⇒ 恢复必然失败 ⇒ 必为匿名：跳过二创取数、
//   SSR 直接渲染落地页；
// - 有 cookie ⇒ 待定中性壳直到恢复完成（伪/过期 cookie 归匿名落地页）；
// - 已登录 = 原二创页（与 /fanworks 同一实现，现状零变化）。
// 一切内容深链永不重定向；不做「已看过」cookie。

const REFRESH_COOKIE_NAMES = ["__Host-refresh_token", "refresh_token"];

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("landing");
  return {
    title: t("metaTitle"),
    description: t("metaDescription"),
  };
}

export default async function LandingRootPage() {
  const cookieStore = await cookies();
  const hasRefreshCookie = REFRESH_COOKIE_NAMES.some((name) => cookieStore.has(name));

  const homeData = hasRefreshCookie ? await fetchHomePageData() : null;

  return (
    <RootAuthGate
      hasRefreshCookie={hasRefreshCookie}
      authenticatedView={
        /* 已登录表面 = 原 (public) 壳 + 二创页（Header/Footer 与 /fanworks 一致，
           满足「登录 `/` 现状零变化」；落地页分支则只有落地页自身顶栏）。 */
        <div className="flex min-h-screen flex-col">
          <Header />
          <main className="flex-1">
            <FanworksHomeView data={homeData} />
          </main>
          <Footer />
        </div>
      }
    />
  );
}
