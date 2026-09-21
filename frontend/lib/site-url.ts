/**
 * SP-25 FR-05（低-41）：站点绝对 URL 的唯一出口。NEXT_PUBLIC_SITE_URL 缺失
 * （本地开发/未配置部署）时降级为相对路径——绝不再输出
 * "https://omnicraft.com" 占位域（该域名非本项目所有，会让 sitemap/robots/
 * OG 全部指向错误站点）。
 */
export const SITE_BASE_URL = (process.env.NEXT_PUBLIC_SITE_URL ?? "").replace(/\/+$/, "");

/** 站内路径 → 绝对 URL（已配置站点域）或相对路径（未配置）。 */
export function absoluteUrl(path: string): string {
  return SITE_BASE_URL ? `${SITE_BASE_URL}${path}` : path;
}
