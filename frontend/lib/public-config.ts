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

export interface PublicConfig {
  features: PublicFeatures;
  captcha: PublicCaptcha;
  client: PublicClient;
  legal: PublicLegal;
  upload: PublicUpload;
  collaboration: PublicCollaboration;
  publish?: PublicPublish;
  limits?: PublicUploadLimits;
  /** Object delivery domain; empty when delivery is not configured. */
  oss_domain: string;
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
