/**
 * 附件预览资格中立策略（C1 #920）：扫描门判定自 AttachmentPreview 组件下移
 * 至 lib 中立模块——overlay-media（浮层可预览媒体链构建）与 AttachmentPreview
 * （独立详情页预览分发）均从此处取用，消除 lib → 组件 的运行时反向依赖
 * （overlay-media ↔ AttachmentPreview 环的前半段）。lib 模块不得运行时
 * （value）import 组件；type-only 引用不受限。
 */

/** 扫描门放行的状态（后端非 clean 不签发 oss_url；这里双保险渲染状态卡）。 */
export function isPreviewBlockedByScan(scanStatus?: string): boolean {
  if (!scanStatus) return false;
  return scanStatus !== "clean" && scanStatus !== "not_required";
}
