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

export interface PublicConfig {
  features: PublicFeatures;
  captcha: PublicCaptcha;
  client: PublicClient;
  legal: PublicLegal;
  upload: PublicUpload;
  collaboration: PublicCollaboration;
  /** Object delivery domain; empty when delivery is not configured. */
  oss_domain: string;
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
