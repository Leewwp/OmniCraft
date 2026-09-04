import { api } from "@/lib/api";

// 通知深链统一映射（FIX-31b）：各 target_type 归位到正确页面。
// 讨论通知借 Discussion.ip_id 二跳到 IP Hub 的讨论浮层（#290 后讨论无独立
// 子路由）；查询失败/404 容错回 /messages，不阻塞点击反馈。
export interface NotificationTarget {
  type?: string;
  target_type?: string;
  target_id?: number;
}

export function getNotificationUrl(n: NotificationTarget): string {
  if (n.target_type && n.target_id) {
    switch (n.target_type) {
      case "content":
      case "comment":
        // 评论通知的 target 即所属内容（social_service Notify 传 content.ID）
        return `/content/${n.target_id}`;
      case "pr":
        return "/studio/pr-requests";
      case "user":
        return `/user/${n.target_id}`;
      case "ip":
        // 共治提案通知深链到提案 tab（#290 story 36）
        if (n.type?.startsWith("ip_proposal_")) return `/ip/${n.target_id}?tab=proposals`;
        return `/ip/${n.target_id}`;
      case "appeal":
        return "/appeals";
      case "report":
        return "/appeals?tab=reports";
      case "feedback_ticket":
        return "/feedback/mine";
      case "message":
        return "/messages?tab=messages";
      default:
        return "/messages";
    }
  }
  return "/messages";
}

// 讨论通知需 ip_id 才能落到 /ip/{ipId}?tab=discussions&d={id}；同步映射覆盖
// 不了，二跳一次查询（404/网络失败容错回 /messages）。
export async function resolveNotificationHref(n: NotificationTarget): Promise<string> {
  if (n.target_type === "discussion" && n.target_id) {
    try {
      const data = await api.get<{ discussion?: { ip_id?: number | null } }>(
        `/api/v1/social/discussions/${n.target_id}`
      );
      const ipId = data.discussion?.ip_id;
      if (ipId && ipId > 0) return `/ip/${ipId}?tab=discussions&d=${n.target_id}`;
    } catch {
      // 容错：讨论已删/不可达时回消息中心
    }
    return "/messages";
  }
  return getNotificationUrl(n);
}
