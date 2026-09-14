/* SP-19 G1-3：IP 分类词表单一事实源（Q14/Q18-A 决议）。
 *
 * 11 类（顺序即 UI 展示序）；literature 为存量转正类目（原 hidden 值直接
 * 保留），vtuber 补正式标签（原被别名到「其他」），gaming/video 由回填迁移
 * 并入 game/other（0NN_ip_category_backfill.sql）。
 *
 * 词表不定稿（Q14）：拓展 = 本常量 + backend/config.yaml ip_categories
 * allowlist + i18n 键（zh/en 的 ipCategory.*）三处同步小 PR；机制说明见
 * docs/GLOSSARY.md「IP 分类」条目。 */

export interface IPCategoryDef {
  slug: string;
  labelKey: string;
}

/** 11 个正式分类（发布表单 / 标签显示用，无「全部」）。 */
export const IP_CATEGORIES: IPCategoryDef[] = [
  { slug: "game", labelKey: "ipCategory.game" },
  { slug: "film_tv", labelKey: "ipCategory.film_tv" },
  { slug: "anime", labelKey: "ipCategory.anime" },
  { slug: "manga", labelKey: "ipCategory.manga" },
  { slug: "novel", labelKey: "ipCategory.novel" },
  { slug: "literature", labelKey: "ipCategory.literature" },
  { slug: "music", labelKey: "ipCategory.music" },
  { slug: "variety", labelKey: "ipCategory.variety" },
  { slug: "short_drama", labelKey: "ipCategory.short_drama" },
  { slug: "vtuber", labelKey: "ipCategory.vtuber" },
  { slug: "other", labelKey: "ipCategory.other" },
];

/** 含「全部」入口的浏览筛选清单（IP 库 pills / 首页 chips 用）。 */
export const IP_CATEGORY_FILTERS: IPCategoryDef[] = [
  { slug: "", labelKey: "home.allIps" },
  ...IP_CATEGORIES.map((c) => ({ slug: c.slug, labelKey: c.labelKey })),
];

/** slug → labelKey；未知 slug 兜底到「其他」（存量脏值不崩 UI）。 */
export function ipCategoryLabelKey(slug: string | undefined | null): string {
  return IP_CATEGORIES.find((c) => c.slug === slug)?.labelKey ?? "ipCategory.other";
}
