// 访客落地页「精选陈列」选槽纯函数（票 #853 / spec §12.2）。
//
// 口径（spec Q7 + §十二）：
// - 主序保持服务端返回顺序（contents sort=hot / ips sort=most_contents），
//   前端不发明热度权重、不重排服务端主序；
// - 缺封面项过滤、按 id 去重（先到先得 = 主序优先）；
// - 两区均有数据时混排（区内保序的轮流交错），并携带分区徽标所需 zone；
//   一侧不足由另一侧与 newest 补槽；
// - 固定槽位（作品 10 + IP 10）；补完仍 <4 项 → 返回空数组，该子区整段隐藏
//   （宁可少一段不留空壳）。
// 固定输入下输出确定（去重与选槽无随机性）。

export const SHOWCASE_WORK_SLOTS = 10;
export const SHOWCASE_IP_SLOTS = 10;
export const SHOWCASE_MIN_VISIBLE = 4;

export interface ShowcaseWorkInput {
  id: number;
  title?: string;
  zone?: string | null;
  cover_image_url?: string | null;
}

export interface ShowcaseIpInput {
  id: number;
  name?: string;
  cover_url?: string | null;
}

export interface ShowcaseWork {
  id: number;
  title: string;
  zone: "fanwork" | "original";
  coverUrl: string;
}

export interface ShowcaseIp {
  id: number;
  name: string;
  coverUrl: string;
}

interface SlotItem {
  id: number;
  coverUrl: string;
}

function hasCover(cover: string | null | undefined): cover is string {
  return typeof cover === "string" && cover.trim() !== "";
}

/** 区内保序的两列轮流交错（A B A B …；短列耗尽后接长列剩余）。 */
function interleave<T>(first: T[], second: T[]): T[] {
  const merged: T[] = [];
  const max = Math.max(first.length, second.length);
  for (let i = 0; i < max; i += 1) {
    if (i < first.length) merged.push(first[i]);
    if (i < second.length) merged.push(second[i]);
  }
  return merged;
}

function normalizeWork(item: ShowcaseWorkInput, fallbackZone: "fanwork" | "original"): ShowcaseWork | null {
  if (!hasCover(item.cover_image_url)) return null;
  const zone = item.zone === "original" ? "original" : fallbackZone;
  return {
    id: item.id,
    title: item.title ?? "",
    zone,
    coverUrl: item.cover_image_url as string,
  };
}

/**
 * 作品子区选槽：两区 hot 主序混排 → newest 补槽 → 去重/封顶 → <4 隐藏。
 */
export function buildWorkShowcase(lists: {
  fanworkHot: ShowcaseWorkInput[];
  originalHot: ShowcaseWorkInput[];
  fanworkNewest: ShowcaseWorkInput[];
  originalNewest: ShowcaseWorkInput[];
  slots?: number;
}): ShowcaseWork[] {
  const fanworkHot = lists.fanworkHot
    .map((item) => normalizeWork(item, "fanwork"))
    .filter((item): item is ShowcaseWork => item !== null);
  const originalHot = lists.originalHot
    .map((item) => normalizeWork(item, "original"))
    .filter((item): item is ShowcaseWork => item !== null);
  const fanworkNewest = lists.fanworkNewest
    .map((item) => normalizeWork(item, "fanwork"))
    .filter((item): item is ShowcaseWork => item !== null);
  const originalNewest = lists.originalNewest
    .map((item) => normalizeWork(item, "original"))
    .filter((item): item is ShowcaseWork => item !== null);

  const selected = selectSlots<ShowcaseWork>(
    interleave(fanworkHot, originalHot),
    interleave(fanworkNewest, originalNewest),
    lists.slots ?? SHOWCASE_WORK_SLOTS,
  );
  return selected;
}

/**
 * IP 子区选槽：most_contents 主序 → newest 补槽 → 同一套过滤/去重/隐藏规则。
 * 注意：IP 接口没有真实热度序（sort=hot 实际走默认 newest，spec §12.2），
 * 因此主序只用 most_contents。
 */
export function buildIpShowcase(lists: {
  primary: ShowcaseIpInput[];
  newest: ShowcaseIpInput[];
  slots?: number;
}): ShowcaseIp[] {
  const normalize = (item: ShowcaseIpInput): ShowcaseIp | null => {
    if (!hasCover(item.cover_url)) return null;
    return { id: item.id, name: item.name ?? "", coverUrl: item.cover_url as string };
  };
  const primary = lists.primary.map(normalize).filter((item): item is ShowcaseIp => item !== null);
  const newest = lists.newest.map(normalize).filter((item): item is ShowcaseIp => item !== null);
  return selectSlots<ShowcaseIp>(primary, newest, lists.slots ?? SHOWCASE_IP_SLOTS);
}

function selectSlots<T extends SlotItem>(primary: T[], fill: T[], slots: number): T[] {
  const seen = new Set<number>();
  const picked: T[] = [];
  const push = (item: T) => {
    if (seen.has(item.id)) return;
    seen.add(item.id);
    picked.push(item);
  };
  for (const item of primary) {
    if (picked.length >= slots) break;
    push(item);
  }
  for (const item of fill) {
    if (picked.length >= slots) break;
    push(item);
  }
  if (picked.length < SHOWCASE_MIN_VISIBLE) return [];
  return picked;
}
