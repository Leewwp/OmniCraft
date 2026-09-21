import type { MetadataRoute } from "next";
import { absoluteUrl } from "@/lib/site-url";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: "*",
        allow: "/",
        // SP-25 低-41：补齐遗漏的受保护路径（/dashboard 重定向 stub、/agent
        // 工作台、/feedback/mine 个人反馈）。
        disallow: ["/admin/", "/api/", "/studio/", "/judge/", "/publish/", "/settings/", "/history/", "/messages/", "/appeals/", "/rehab/", "/dashboard/", "/agent/", "/feedback/mine"],
      },
    ],
    sitemap: absoluteUrl("/sitemap.xml"),
  };
}
