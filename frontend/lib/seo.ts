/**
 * 票 #950：canonical alternates 与详情页 JSON-LD（schema.org CreativeWork）
 * 的统一出口。locale 由 cookie 决定（URL 不带语言前缀，见 lib/locale.ts），
 * canonical 即页面自身唯一规范绝对 URL（经 lib/site-url 的 absoluteUrl），
 * 多 locale 共用同一 URL，不产生 canonical 自我冲突。
 */
import { absoluteUrl } from "./site-url";

export interface CreativeWorkLdInput {
  /** 作品标题 / IP 名（同时落 name 与 headline，覆盖两类结构化数据消费方） */
  name: string;
  /** 页面规范绝对 URL（与 alternates.canonical 一致） */
  url: string;
  /** ISO 时间（取页面已有数据的 updated_at；缺失则整键省略） */
  dateModified?: string;
}

export function buildCreativeWorkJsonLd(input: CreativeWorkLdInput): Record<string, string> {
  const node: Record<string, string> = {
    "@context": "https://schema.org",
    "@type": "CreativeWork",
    name: input.name,
    headline: input.name,
    url: input.url,
  };
  if (input.dateModified) {
    node.dateModified = input.dateModified;
  }
  return node;
}

/** JSON-LD 序列化：`<` 转义防内容里的 `</script>` 截断注入。 */
export function serializeJsonLd(node: Record<string, string>): string {
  return JSON.stringify(node).replace(/</g, "\\u003c");
}

/** 页面 metadata.alternates 出口：canonical 指向唯一规范绝对 URL。 */
export function canonicalAlternates(path: string): { canonical: string } {
  return { canonical: absoluteUrl(path) };
}
