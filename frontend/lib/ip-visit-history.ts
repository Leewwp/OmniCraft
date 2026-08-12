import { api, getAccessToken } from "@/lib/api";
import { silentError } from "@/lib/error-handler";

export interface RecentIPItem {
  id: number;
  name: string;
}

interface AccountIPVisit {
  ip: { id: number; name: string; slug?: string; description?: string; cover_url?: string; category?: string };
  visited_at: string;
}

const RECENT_IP_KEY = "recent_ips";
const RECENT_IP_CAP = 6;

export interface MergeResult {
  accepted: number[];
  discarded: number[];
}

function dedupeCap(items: RecentIPItem[]): RecentIPItem[] {
  const seen = new Set<number>();
  const out: RecentIPItem[] = [];
  for (const item of items) {
    if (seen.has(item.id)) continue;
    seen.add(item.id);
    out.push(item);
    if (out.length >= RECENT_IP_CAP) break;
  }
  return out;
}

// readLocalRecentIps parses the legacy local structure (most recent first,
// deduplicated, capped at six). Malformed or missing entries are ignored so a
// corrupt key never breaks the home page.
export function readLocalRecentIps(): RecentIPItem[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = localStorage.getItem(RECENT_IP_KEY);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    const items: RecentIPItem[] = [];
    for (const entry of parsed) {
      if (
        entry &&
        typeof entry === "object" &&
        typeof (entry as RecentIPItem).id === "number" &&
        typeof (entry as RecentIPItem).name === "string"
      ) {
        items.push({ id: (entry as RecentIPItem).id, name: (entry as RecentIPItem).name });
      }
    }
    return dedupeCap(items);
  } catch {
    return [];
  }
}

export function writeLocalRecentIps(items: RecentIPItem[]): void {
  if (typeof window === "undefined") return;
  try {
    localStorage.setItem(RECENT_IP_KEY, JSON.stringify(dedupeCap(items)));
  } catch {
    // Storage failures must never break navigation.
  }
}

// recordIpVisit writes the visit locally for every user and, when signed in,
// also fires a best-effort server record. The local write always happens
// first so a failed API call can never lose the visit; the server call is
// fire-and-forget and the local record doubles as the retry source.
export function recordIpVisit(item: RecentIPItem): void {
  writeLocalRecentIps([item, ...readLocalRecentIps()]);
  if (!getAccessToken()) return;
  api
    .put(`/api/v1/users/me/ip-visits/${item.id}`, {})
    .catch((e) => silentError(e, { component: "ip-visit-history", action: "recordIpVisit" }));
}

// mergeLocalIpsIntoAccount idempotently upserts the local anonymous history
// into the signed-in account. Local items are removed only for ids the server
// explicitly acknowledged (accepted or discarded); network errors, 401, 400
// and 500 all retain the local records for a later retry. Returns null when
// there is nothing to merge or the attempt failed.
export async function mergeLocalIpsIntoAccount(): Promise<MergeResult | null> {
  const local = readLocalRecentIps();
  if (local.length === 0 || !getAccessToken()) return null;
  try {
    const data = await api.post<{ accepted_ip_ids: number[]; discarded_ip_ids: number[] }>(
      "/api/v1/users/me/ip-visits/merge",
      {
        visits: local.map((item) => ({ ip_id: item.id, visited_at: new Date().toISOString() })),
      }
    );
    const accepted = data.accepted_ip_ids ?? [];
    const discarded = data.discarded_ip_ids ?? [];
    const acknowledged = new Set([...accepted, ...discarded]);
    if (acknowledged.size > 0) {
      writeLocalRecentIps(local.filter((item) => !acknowledged.has(item.id)));
    }
    return { accepted, discarded };
  } catch (e) {
    silentError(e, { component: "ip-visit-history", action: "mergeLocalIpsIntoAccount" });
    return null;
  }
}

// loadRecentIps returns the recent list for the current session: account
// history for signed-in users (falling back to local when the server cannot
// be reached or holds nothing while local context exists) and local history
// for anonymous users.
export async function loadRecentIps(): Promise<RecentIPItem[]> {
  const local = readLocalRecentIps();
  if (!getAccessToken()) return local;
  try {
    const data = await api.get<{ items: AccountIPVisit[]; limit: number }>("/api/v1/users/me/ip-visits");
    const serverItems = (data.items ?? []).map((visit) => ({ id: visit.ip.id, name: visit.ip.name }));
    if (serverItems.length > 0) return serverItems;
    return local;
  } catch (e) {
    silentError(e, { component: "ip-visit-history", action: "loadRecentIps" });
    return local;
  }
}
