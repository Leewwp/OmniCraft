import type { MetadataRoute } from "next";

export default function robots(): MetadataRoute.Robots {
  const baseUrl = process.env.NEXT_PUBLIC_SITE_URL || "https://omnicraft.com";
  return {
    rules: [
      {
        userAgent: "*",
        allow: "/",
        disallow: ["/admin/", "/api/", "/dashboard/", "/judge/", "/publish/", "/settings/", "/history/", "/messages/", "/appeals/", "/rehab/"],
      },
    ],
    sitemap: `${baseUrl}/sitemap.xml`,
  };
}
