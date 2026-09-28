const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export interface PublicFeatures {
  web_agent_enabled: boolean;
  payment_enabled: boolean;
  creator_support_enabled: boolean;
  desktop_deploy_enabled: boolean;
}

export interface PublicCaptcha {
  provider: string;
  prefix: string;
  scene_id: string;
  region: string;
}

export interface PublicClient {
  download_enabled: boolean;
  download_url: string;
  latest_version: string;
}

export interface PublicLegal {
  current_terms_version: string;
  current_privacy_version: string;
}

export interface PublicUpload {
  image_gallery_min_items: number;
  image_gallery_max_items: number;
  video_gallery_min_items: number;
  video_gallery_max_items: number;
}

export interface PublicCollaboration {
  max_invitees_per_publish: number;
}

/** 发布类型顺序（T25/FIX-41：跟随运营配置，空数组=未配置走前端兜底） */
export interface PublicPublish {
  type_order_original?: string[];
  type_order_fanwork?: string[];
}

/** 每类型上传大小上限（T25/FIX-41：脱敏数值，替代前端硬编码） */
export interface PublicUploadLimits {
  video_max_mb?: number;
  image_max_mb?: number;
  text_max_mb?: number;
  mod_max_mb?: number;
  sheet_music_max_mb?: number;
}

/**
 * 内容类型注册表投影（#687 additive）：content_types 全量行。
 * zones = 可发布区；form = 发布表单形态；client_accept = 服务端推导的
 * 文件选择器 accept（含 unrestricted 族群时为 "*"）。
 */
export interface PublicContentTypeEntry {
  key: string;
  zones: string[];
  form: string;
  upload_file_types: string[];
  judge_eligible: boolean;
  client_accept?: string;
  attachment_policy?: { required_any_of: string[] };
}

/**
 * 上传族群能力投影（#687 安全字段）：extensions 为 null = unrestricted
 * （由 MIME 规则决定，服务端权威）；数组 = 显式扩展名白名单。max_mb 为
 * 经 limit key 动态解析的活值。
 */
export interface PublicUploadFileTypeEntry {
  key: string;
  extensions: string[] | null;
  max_mb: number;
}

/** 评论折叠阈值（T47/FIX-29c：点踩/点赞比 ≥ 阈值默认折叠） */
export interface PublicSocial {
  comment_fold_threshold?: number;
}

export interface PublicConfig {
  features: PublicFeatures;
  captcha: PublicCaptcha;
  client: PublicClient;
  legal: PublicLegal;
  upload: PublicUpload;
  collaboration: PublicCollaboration;
  publish?: PublicPublish;
  limits?: PublicUploadLimits;
  social?: PublicSocial;
  /** 内容类型注册表（#687 additive；缺省走内置兜底） */
  content_types?: PublicContentTypeEntry[];
  upload_file_types?: PublicUploadFileTypeEntry[];
  /** Object delivery domain; empty when delivery is not configured. */
  oss_domain: string;
}

/** 评论折叠阈值兜底：与 config.yaml social.comment_fold_threshold 基线一致 */
export const COMMENT_FOLD_THRESHOLD_FALLBACK = 0.30;

export function commentFoldThreshold(config: PublicConfig | null | undefined): number {
  const value = config?.social?.comment_fold_threshold;
  return typeof value === "number" && value > 0 && value < 1 ? value : COMMENT_FOLD_THRESHOLD_FALLBACK;
}

/**
 * 高踩比判定（business-rules：点踩/点赞 比 ≥ 阈值 → 默认折叠）。
 * 点赞为 0 且有点踩时视为比例无穷大，同样折叠。
 */
export function isHighDislikeRatio(likes: number, dislikes: number, threshold: number): boolean {
  if (dislikes <= 0) return false;
  if (likes <= 0) return true;
  return dislikes / likes >= threshold;
}

/**
 * 按 content_type 取上传大小上限（MB）；配置缺失或为 0 时回退既有默认。
 * 默认值与 config.yaml limits 基线一致，仅作配置不可用时的兜底。
 */
export function uploadMaxMBForType(config: PublicConfig | null | undefined, contentType: string): number {
  const fallback: Record<string, number> = {
    mod: 500,
    sheet_music: 50,
    video: 300,
    image: 20,
    text: 10,
  };
  const limits = config?.limits;
  const fromConfig: Record<string, number | undefined> = {
    mod: limits?.mod_max_mb,
    sheet_music: limits?.sheet_music_max_mb,
    video: limits?.video_max_mb,
    image: limits?.image_max_mb,
    text: limits?.text_max_mb,
  };
  const value = fromConfig[contentType];
  return typeof value === "number" && value > 0 ? value : (fallback[contentType] ?? 20);
}

// Runtime flag flips (e.g. admin toggling web_agent_enabled) must reach the
// client within a demo-visible window, so the cache carries a short TTL.
const PUBLIC_CONFIG_TTL_MS = 5 * 60 * 1000;

let cachedConfig: PublicConfig | null = null;
let cachedAt = 0;

export async function fetchPublicConfig(): Promise<PublicConfig> {
  if (cachedConfig && Date.now() - cachedAt < PUBLIC_CONFIG_TTL_MS) return cachedConfig;

  const res = await fetch(`${API_URL}/api/v1/config/public`, {
    credentials: "include",
  });
  if (!res.ok) {
    throw new Error(`failed to fetch public config: ${res.status}`);
  }
  const data = (await res.json()) as PublicConfig;
  cachedConfig = data;
  cachedAt = Date.now();
  return data;
}

export function clearPublicConfigCache(): void {
  cachedConfig = null;
  cachedAt = 0;
}

// ---------------------------------------------------------------------------
// 内容类型注册表读取层（#687）
//
// FALLBACK_* 是全前端唯一被豁免的完整类型枚举点（source-contract gate
// 允许清单在案）：仅当后端 /config/public 尚未投影注册表（旧后端/离线）
// 时生效，与后端 config.DefaultContentRegistry 同基线。声明序为
// image…other，其 fanwork 子列 = 既有 IP hub 筛选顺序，保证兜底渲染与
// 今日观感一致。
// ---------------------------------------------------------------------------

export const FALLBACK_CONTENT_TYPE_ENTRIES: PublicContentTypeEntry[] = [
  { key: "image", zones: ["original", "fanwork"], form: "media", upload_file_types: ["image"], judge_eligible: true },
  { key: "article", zones: ["original", "fanwork"], form: "text", upload_file_types: [], judge_eligible: true },
  { key: "video", zones: ["original", "fanwork"], form: "media", upload_file_types: ["video"], judge_eligible: true },
  { key: "audio", zones: ["original", "fanwork"], form: "file", upload_file_types: ["text"], judge_eligible: true },
  { key: "mod", zones: ["fanwork"], form: "file", upload_file_types: ["mod"], judge_eligible: false },
  { key: "prompt", zones: ["fanwork"], form: "text", upload_file_types: [], judge_eligible: true },
  { key: "template", zones: ["original"], form: "file", upload_file_types: ["text"], judge_eligible: true },
  { key: "sheet_music", zones: ["original", "fanwork"], form: "file", upload_file_types: ["sheet_music"], judge_eligible: true },
  { key: "other", zones: ["original", "fanwork"], form: "text", upload_file_types: [], judge_eligible: true },
];

const FALLBACK_UPLOAD_FILE_TYPE_ENTRIES: PublicUploadFileTypeEntry[] = [
  { key: "video", extensions: null, max_mb: 300 },
  { key: "image", extensions: null, max_mb: 20 },
  { key: "avatar", extensions: null, max_mb: 20 },
  { key: "text", extensions: null, max_mb: 10 },
  { key: "mod", extensions: null, max_mb: 500 },
  { key: "sheet_music", extensions: [".mid", ".midi", ".xml", ".mxl", ".mscz", ".mscx", ".pdf"], max_mb: 50 },
];

/** 注册表 content_types 行（投影缺失时走内置兜底，永不 undefined） */
export function contentTypeEntries(config: PublicConfig | null | undefined): PublicContentTypeEntry[] {
  const projected = config?.content_types;
  return projected && projected.length > 0 ? projected : FALLBACK_CONTENT_TYPE_ENTRIES;
}

/** 注册表 upload_file_types 行（安全字段投影；缺省走内置兜底） */
export function uploadFileTypeEntries(config: PublicConfig | null | undefined): PublicUploadFileTypeEntry[] {
  const projected = config?.upload_file_types;
  return projected && projected.length > 0 ? projected : FALLBACK_UPLOAD_FILE_TYPE_ENTRIES;
}

export function contentTypeMap(config: PublicConfig | null | undefined): Record<string, PublicContentTypeEntry> {
  const map: Record<string, PublicContentTypeEntry> = {};
  for (const entry of contentTypeEntries(config)) map[entry.key] = entry;
  return map;
}

export function uploadFileTypeMap(config: PublicConfig | null | undefined): Record<string, PublicUploadFileTypeEntry> {
  const map: Record<string, PublicUploadFileTypeEntry> = {};
  for (const entry of uploadFileTypeEntries(config)) map[entry.key] = entry;
  return map;
}

/**
 * 某区可发布的类型键（保持注册表声明序）。zone ∈ original | fanwork。
 */
export function zoneContentKeys(config: PublicConfig | null | undefined, zone: string): string[] {
  return contentTypeEntries(config)
    .filter((entry) => entry.zones.includes(zone))
    .map((entry) => entry.key);
}

/**
 * 类型的发布表单形态；未知类型回退 "text"（与既有 FILE_PRIMARY.includes
 * 语义等价：未知类型走文本主形态，不崩）。
 */
export function formForContentType(config: PublicConfig | null | undefined, contentType: string): "text" | "file" | "media" {
  const entry = contentTypeMap(config)[contentType];
  return (entry?.form as "text" | "file" | "media") ?? "text";
}

/** 文件主形态 = form 为 file 或 media（等价旧 FILE_PRIMARY_TYPES 判定） */
export function isFilePrimaryContentType(config: PublicConfig | null | undefined, contentType: string): boolean {
  return formForContentType(config, contentType) !== "text";
}

/** 类型附件集的 accept 属性（服务端推导优先；兜底注册表现算） */
export function clientAcceptForContentType(config: PublicConfig | null | undefined, contentType: string): string | undefined {
  const entry = contentTypeMap(config)[contentType];
  if (!entry) return undefined;
  if (entry.client_accept !== undefined) return entry.client_accept;
  if (entry.upload_file_types.length === 0) return undefined;
  const families = uploadFileTypeMap(config);
  const parts: string[] = [];
  for (const familyKey of entry.upload_file_types) {
    const family = families[familyKey];
    if (!family) continue;
    if (family.extensions === null) return "*";
    parts.push(...family.extensions);
  }
  return parts.sort().join(",");
}

/**
 * 扩展名→族群推导合同（#687 前端侧）：显式扩展名匹配优先（注册表歧义
 * 不变量保证唯一）；无显式匹配时唯一 unrestricted 族群兜底；null = 该
 * 内容类型不支持此文件。服务端 presign/verify 仍按 MIME 权威校验。
 */
export function deriveUploadFamilyForExtension(config: PublicConfig | null | undefined, contentType: string, fileName: string): string | null {
  const entry = contentTypeMap(config)[contentType];
  if (!entry) return null;
  const dot = fileName.lastIndexOf(".");
  const normalized = dot >= 0 ? fileName.slice(dot).toLowerCase() : "";
  if (!normalized) return null;
  const families = uploadFileTypeMap(config);
  let fallback: string | null = null;
  for (const familyKey of entry.upload_file_types) {
    const family = families[familyKey];
    if (!family) continue;
    if (family.extensions !== null) {
      if (family.extensions.some((ext) => ext.toLowerCase() === normalized)) return familyKey;
    } else if (fallback !== null) {
      return null; // >1 unrestricted 族群：歧义，拒绝
    } else {
      fallback = familyKey;
    }
  }
  return fallback;
}
