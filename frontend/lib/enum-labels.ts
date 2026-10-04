/* SP-26-C（#782）：内部 slug → 展示词集中映射单一事实源。
 *
 * 背景：history 条目类型、studio/admin 表格枚举列此前直接渲染后端
 * slug（template/under_review/SUCCESS…），中文界面混排英文内部标识。
 * 本模块以 i18n label key 为杠杆（zh/en 双语走 messages 的 enums.*
 * 命名空间；IP 分类继续走 lib/ip-categories.ts 的 ipCategory.*），
 * 命名一律取自 zh.json 现有词汇，不新造近义词。
 *
 * 词表拓展 = 本常量 + zh/en 的 enums.* 三处同步小 PR。未知 slug 不崩
 * UI：helper 原样返回（admin 诊断场景可见原始值，等同技术场景豁免）。 */

export type TranslateFn = (key: string) => string;

function labelFor(t: TranslateFn, keys: Record<string, string>, value: string | undefined | null): string {
  const key = keys[value ?? ""];
  return key ? t(key) : (value ?? "—");
}

/** content_type（含 #688/#689/#690 族群键位；filter slug "text" 不在此列，
 *  筛选 chips 各自维护 value，仅 label 词与本表对齐）。 */
const CONTENT_TYPE_LABEL_KEYS: Record<string, string> = {
  article: "enums.contentType.article",
  image: "enums.contentType.image",
  video: "enums.contentType.video",
  audio: "enums.contentType.audio",
  template: "enums.contentType.template",
  sheet_music: "enums.contentType.sheet_music",
  mod: "enums.contentType.mod",
  prompt: "enums.contentType.prompt",
  document: "enums.contentType.document",
  model3d: "enums.contentType.model3d",
  "3d_print": "enums.contentType.3d_print",
  other: "enums.contentType.other",
};

/** 分区（原创/二创）。 */
const ZONE_LABEL_KEYS: Record<string, string> = {
  original: "enums.zone.original",
  fanwork: "enums.zone.fanwork",
};

/** 用户角色（复用 admin.feedback.admin/user 词条）。 */
const ROLE_LABEL_KEYS: Record<string, string> = {
  admin: "enums.role.admin",
  user: "enums.role.user",
};

/** 内容/举报/反馈/IP 共用状态词（under_review=审核中、pending=待处理、
 *  resolved=已解决、dismissed=已驳回，#782 词表拍板值）。pending（举报）与
 *  open（反馈工单）是两个独立键：zh 文案同为「待处理」是有意同义词，en 区分
 *  Pending/Open——语义分道时改键文案即可，勿合并键。 */
const STATUS_LABEL_KEYS: Record<string, string> = {
  draft: "enums.status.draft",
  pending: "enums.status.pending",
  under_review: "enums.status.under_review",
  published: "enums.status.published",
  banned: "enums.status.banned",
  approved: "enums.status.approved",
  rejected: "enums.status.rejected",
  resolved: "enums.status.resolved",
  dismissed: "enums.status.dismissed",
  open: "enums.status.open",
  in_progress: "enums.status.in_progress",
  closed: "enums.status.closed",
  reopened: "enums.status.reopened",
};

/** 举报对象类型。 */
const REPORT_TARGET_LABEL_KEYS: Record<string, string> = {
  content: "enums.reportTarget.content",
  comment: "enums.reportTarget.comment",
};

/** 反馈工单优先级（#782 验收口径：admin 页面无英文 slug 直出）。 */
const PRIORITY_LABEL_KEYS: Record<string, string> = {
  low: "enums.priority.low",
  normal: "enums.priority.normal",
  high: "enums.priority.high",
  urgent: "enums.priority.urgent",
};

/** 反馈工单分类（对齐用户侧 feedback.cat_* 词条）。 */
const FEEDBACK_CATEGORY_LABEL_KEYS: Record<string, string> = {
  web_bug: "enums.feedbackCategory.web_bug",
  desktop_deploy: "enums.feedbackCategory.desktop_deploy",
  content_or_community: "enums.feedbackCategory.content_or_community",
  account_or_security: "enums.feedbackCategory.account_or_security",
  agent_quality: "enums.feedbackCategory.agent_quality",
  feature_request: "enums.feedbackCategory.feature_request",
  other: "enums.feedbackCategory.other",
};

/** Agent trace 运行状态。 */
const TRACE_STATUS_LABEL_KEYS: Record<string, string> = {
  RUNNING: "enums.traceStatus.RUNNING",
  SUCCESS: "enums.traceStatus.SUCCESS",
  ERROR: "enums.traceStatus.ERROR",
  CANCELLED: "enums.traceStatus.CANCELLED",
};

/** Agent 回答类型（grounded_content=基于内容引用 等）。 */
const ANSWER_KIND_LABEL_KEYS: Record<string, string> = {
  grounded_content: "enums.answerKind.grounded_content",
  no_evidence: "enums.answerKind.no_evidence",
  conversational: "enums.answerKind.conversational",
  publish_suggestion: "enums.answerKind.publish_suggestion",
  other: "enums.answerKind.other",
};

export function contentTypeLabel(t: TranslateFn, contentType: string | undefined | null): string {
  return labelFor(t, CONTENT_TYPE_LABEL_KEYS, contentType);
}

export function zoneLabel(t: TranslateFn, zone: string | undefined | null): string {
  return labelFor(t, ZONE_LABEL_KEYS, zone);
}

export function roleLabel(t: TranslateFn, role: string | undefined | null): string {
  return labelFor(t, ROLE_LABEL_KEYS, role);
}

export function statusLabel(t: TranslateFn, status: string | undefined | null): string {
  return labelFor(t, STATUS_LABEL_KEYS, status);
}

export function reportTargetLabel(t: TranslateFn, targetType: string | undefined | null): string {
  return labelFor(t, REPORT_TARGET_LABEL_KEYS, targetType);
}

export function priorityLabel(t: TranslateFn, priority: string | undefined | null): string {
  return labelFor(t, PRIORITY_LABEL_KEYS, priority);
}

export function feedbackCategoryLabel(t: TranslateFn, category: string | undefined | null): string {
  return labelFor(t, FEEDBACK_CATEGORY_LABEL_KEYS, category);
}

export function traceStatusLabel(t: TranslateFn, status: string | undefined | null): string {
  return labelFor(t, TRACE_STATUS_LABEL_KEYS, status);
}

export function answerKindLabel(t: TranslateFn, answerKind: string | undefined | null): string {
  return labelFor(t, ANSWER_KIND_LABEL_KEYS, answerKind);
}
