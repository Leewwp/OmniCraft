import { absoluteUrl } from "@/lib/site-url";
import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { FanworksHomeView } from "@/components/home/FanworksHomeView";
import { fetchHomePageData } from "@/lib/home-page-data";

// /fanworks（T1 #853，spec §12.2 二创入口补闭环）：
// 公开二创区——直接复用原 `/` 的二创渲染与数据逻辑（不复制业务实现），
// 两种身份均可访问。落地页三大页面章的二创卡、简介链接、触屏进入按钮
// 全部指向本路由，消除游客点回落地页的自循环；登录用户的 `/` 与本页
// 内容一致（同一 FanworksHomeView）。

export async function generateMetadata(): Promise<Metadata> {
  // 自原 `/` 页迁移的 metadata（原页已由 (landing) 双态页与 /fanworks 承接）。
  const t = await getTranslations();
  const siteUrl = absoluteUrl(""); // SP-05 低-41：占位域出清，env 缺失走相对路径
  return {
    title: t("home.heroSubtitle"),
    description: t("home.heroDescription"),
    openGraph: {
      title: t("home.heroSubtitle"),
      description: t("home.heroTitle"),
      images: [{ url: `${siteUrl}/og-default.png`, width: 1200, height: 630 }],
      type: "website",
    },
    twitter: {
      card: "summary_large_image",
      title: `OmniCraft ${t("nav.siteName")}`,
      description: t("home.heroTitle"),
    },
  };
}

export default async function FanworksPage() {
  const data = await fetchHomePageData();
  return <FanworksHomeView data={data} />;
}
