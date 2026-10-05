import {
  mergeTerminalIntoLastTurn,
  type AgentTurn,
  type AgentTurnTerminal,
} from "@/lib/agent-turn";

/**
 * #795 首轮交接（first-round handover）module：集中持有「首轮终态保存 →
 * replace(/agent/c/{id}) → key 重挂载后的历史接管」规则。
 *
 * - 记录住模块级 Map：/agent → /agent/c/[id] 的 key={id} 重挂载会清空实例
 *   内状态（活动轮/树），只有模块级存储跨重挂载存活；真实整页刷新重置
 *   （与历史轮终态缺省契约一致——追问/用量/trace 不落历史 DTO，落库合同
 *   不变）。
 * - 保存先于导航：handover 先写记录再调 navigate——导航 adapter（真实
 *   Next.js replace）即刻卸载旧实例也不能丢记录。副作用只发生在调用方的
 *   受控事件路径（done 事件回调），不依赖卸载前可能来不及执行的 effect。
 * - 历史接管以服务端消息为事实源：takeOver 只在非空历史并入树尾时消费
 *   记录、且只消费一次；空历史不消费（记录保留待后续接管，不提前弹出
 *   静默丢弃终态）；历史请求取消/失败由调用方 cancelled 守卫拦截，根本
 *   不会到达消费点。
 * - 明确失效路径（切换会话/新建会话/删除会话/深链 404）由调用方调
 *   invalidate 清理记录：交接目标已不再有效时，防止日后重开同会话时并入
 *   过期终态、或迟到回载窗口内污染当前视图。
 * - 本 module 不拥有流或请求管理：unmount abort、SSE 生命周期、历史请求
 *   与 AbortController 仍由工作台接线。
 */
const firstRoundTerminals = new Map<number, AgentTurnTerminal>();

/**
 * 已终结首轮的交接：先保存终态记录，再触发导航。navigate 必须是写入会话
 * URL 的受控导航（真实 Next.js router.replace）——调用次序是本 module 的
 * 核心合同：导航引发的同步卸载不允许发生在保存之前。同一会话重复交接时
 * 最新终态覆盖旧记录（以最后到达的 done 为准）。
 */
export function handoverFirstRoundTerminal(
  conversationId: number,
  terminal: AgentTurnTerminal,
  navigate: () => void,
): void {
  firstRoundTerminals.set(conversationId, terminal);
  navigate();
}

/**
 * 历史接管（重挂载路径）：服务端历史行为事实源，交接记录只把追问/用量/
 * trace 等不落库字段并入树尾轮。无记录时原样返回（引用不变）；空历史不
 * 消费记录；消费即弹出——同一记录只供最终接管使用一次，后续重挂载回归
 * 历史轮终态缺省（落库契约）。同实例合并不经过本函数（工作台直接读活动
 * 轮终态，不消费记录——replace 总会发生，重挂载后的新实例才是最终态）。
 */
export function takeOverFirstRoundHandover(
  conversationId: number,
  historyTurns: AgentTurn[],
): AgentTurn[] {
  if (historyTurns.length === 0) return historyTurns;
  const terminal = firstRoundTerminals.get(conversationId);
  if (terminal === undefined) return historyTurns;
  firstRoundTerminals.delete(conversationId);
  return mergeTerminalIntoLastTurn(historyTurns, terminal);
}

/**
 * 明确失效清理：交接目标失效（用户已切换/离开会话、会话已删除、深链 404）
 * 时移除记录。未消费的记录不再可能被有效接管，留着只会在日后重开同会话
 * 时并入过期终态。
 */
export function invalidateFirstRoundHandover(conversationId: number): void {
  firstRoundTerminals.delete(conversationId);
}

/** 测试隔离钩子：模块级 Map 跨用例存活，单测文件须在用例间复位。 */
export function resetFirstRoundHandoverStore(): void {
  firstRoundTerminals.clear();
}
