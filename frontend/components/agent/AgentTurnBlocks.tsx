"use client";

/* #755（#718 裁决 go 项）：transcript 域自 AgentWorkspace 拆出——轮块
 * （提问行/思考/工具/正文引用）与终态尾部（追问/本轮详情/空轮/降级/错误/
 * 参考来源入口）的渲染集中于此；闭包依赖收拢为显式 AgentTurnBlockDeps，
 * 行为零变化（#663 特征样本锁形、agent-workspace 测试零改动）。 */

import { Fragment, useState } from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { AlertCircle, BookOpenText, Copy, Info, RotateCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { MarkdownRenderer, type CitationBadgeInfo } from "@/components/content/MarkdownRenderer";
import type { AgentStreamCitation } from "@/lib/agent-stream";
import {
  shouldShowThinkingPlaceholder,
  type AgentTurn,
} from "@/lib/agent-turn";
import { citationDisplayMapOf, remapCitationMarks } from "@/lib/agent";
import { AgentThinkingBlock } from "@/components/agent/AgentThinkingBlock";
import { AgentThinkingPlaceholder } from "@/components/agent/AgentThinkingPlaceholder";
import { AgentToolStatus } from "@/components/agent/AgentToolStatus";
import { AgentMetaPill } from "@/components/agent/AgentMetaPill";
import { AgentFollowUpChips } from "@/components/agent/AgentFollowUpChips";

export interface AgentTurnBlockDeps {
  t: ReturnType<typeof useTranslations>;
  streaming: boolean;
  isAdmin: boolean;
  /** 悬停复制/重生成动作行只挂最新答案轮（lastAnswerTurnId 推导）。 */
  actionsTurnId: string | null;
  panelSource: AgentStreamCitation[] | null;
  citationsTriggerRef: RefObject<HTMLButtonElement | null>;
  setPanelSource: Dispatch<SetStateAction<AgentStreamCitation[] | null>>;
  handleCitationRef: (citationRef: number, citations: AgentStreamCitation[]) => void;
  handleCopyMessage: (content: string) => void | Promise<void>;
  handleRegenerate: () => void;
  handleFollowUpFill: (query: string) => void;
}

/* FT-5 (#697)：引用池 → 角标小卡数据（编号缺失的历史行回退位置序，保持
   与旧数字角标相同的展示层映射）。 */
function toCitationBadgeInfo(citation: AgentStreamCitation, index: number): CitationBadgeInfo {
  return {
    number: citation.number ?? index + 1,
    title: citation.title,
    excerpt: citation.excerpt,
    kind: citation.zone === "ip" ? "ip" : "content",
  };
}

/* #715→#795：首轮终态跨路由重挂载的交接已收口到 lib/agent-first-round-handover
   （本文件 2026-09-12 的模块级 Map 副本从未被本域读写，经 #795 核实 caller 后
   删除）。 */

/* 单轮块渲染：提问行 + 思考/工具相块（流式展开→完成折叠）+ 正文与引用。
   终态尾部由 renderTerminalTail 以独立兄弟元素渲染（换树窗口 DOM 节点
   不随轮 key 重建——追问药丸跨历史回载可点的语义保证）。 */
export function renderAgentTurnBlocks(deps: AgentTurnBlockDeps, turn: AgentTurn, options: { isLive: boolean }) {
  const { t, streaming, actionsTurnId, panelSource, citationsTriggerRef, setPanelSource, handleCitationRef, handleCopyMessage, handleRegenerate } = deps;
  const terminal = turn.terminal;
  const badgeCitations = turn.answerCitations ?? (options.isLive ? terminal.citations : []);
  /* #719 展示号映射（单一来源）：正文角标 / 复制替换共用；侧栏按同一
     列表顺序渲染（位置即展示号），目标查找仍用原全局编号。 */
  const displayMap = citationDisplayMapOf(badgeCitations);
  const badgeInfos = badgeCitations.map((citation, index) => {
    const info = toCitationBadgeInfo(citation, index);
    return { ...info, displayNumber: displayMap.get(info.number) };
  });
  return (
    <Fragment key={turn.id}>
      <div className="ml-auto max-w-[85%] whitespace-pre-wrap rounded-md bg-primary px-3 py-2 text-sm text-primary-foreground">
        {turn.query}
      </div>
      {/* #727 发送后即时占位：首个可见内容（非空白 think/工具步骤/答案）
          同帧移除；终态由 streaming=false 覆盖（含空 done）。 */}
      {shouldShowThinkingPlaceholder(turn, options.isLive) && (
        <AgentThinkingPlaceholder startedAt={turn.startedAt} />
      )}
      {turn.segments.map((segment, index) =>
        segment.kind === "think" ? (
          <AgentThinkingBlock
            key={`${turn.id}-segment-${index}`}
            content={segment.content}
            streaming={options.isLive && turn.streaming}
          />
        ) : (
          <AgentToolStatus
            key={`${turn.id}-segment-${index}`}
            tools={segment.tools}
            live={options.isLive && turn.streaming}
          />
        ),
      )}
      {turn.moderationBlocked ? (
        <div className="max-w-[85%] rounded-md border border-border-default bg-card px-3 py-2 text-sm text-fg-muted">
          {turn.answer}
        </div>
      ) : turn.answer !== "" ? (
        <>
          <div className="group/message max-w-[85%] rounded-md bg-canvas-subtle px-3 py-2 text-sm">
            {/* 受控渲染：react-markdown 未接 rehype-raw，原始 HTML 一律转义（T20 核验）。
                #719 展示号：角标文字/读屏用 displayNumber（可见列表顺序连续），
                点击命中仍传原全局编号。 */}
            <MarkdownRenderer
              content={turn.answer}
              onCitationRef={(citationRef) => handleCitationRef(citationRef, badgeCitations)}
              citationCount={badgeCitations.length}
              citations={badgeInfos}
            />
            {!streaming && actionsTurnId === turn.id && (
              <div className="mt-1.5 flex items-center gap-1 opacity-0 transition-opacity duration-150 group-hover/message:opacity-100 focus-within:opacity-100">
                <button
                  type="button"
                  aria-label={t("agent.workspace.copyMessage")}
                  onClick={() => void handleCopyMessage(remapCitationMarks(turn.answer, displayMap))}
                  className="inline-flex size-7 items-center justify-center rounded-md text-fg-muted transition-colors hover:bg-canvas-default hover:text-foreground focus:outline-none focus-visible:ring-1 focus:ring-ring"
                >
                  <Copy className="h-3.5 w-3.5" aria-hidden="true" />
                </button>
                <button
                  type="button"
                  aria-label={t("agent.workspace.regenerate")}
                  onClick={handleRegenerate}
                  className="inline-flex size-7 items-center justify-center rounded-md text-fg-muted transition-colors hover:bg-canvas-default hover:text-foreground focus:outline-none focus-visible:ring-1 focus:ring-ring"
                >
                  <RotateCw className="h-3.5 w-3.5" aria-hidden="true" />
                </button>
              </div>
            )}
          </div>
          {/* 引用随消息持久化（2026-09-06 实测修复）：每条有引用的
              回答消息下方都保留跳转入口，不再随下一轮开始而消失。 */}
          {turn.answerCitations && turn.answerCitations.length > 0 && (
            <CitationsEntryButton
              count={turn.answerCitations.length}
              active={panelSource === turn.answerCitations}
              onToggle={() => setPanelSource(turn.answerCitations ?? [])}
              triggerRef={citationsTriggerRef}
            />
          )}
        </>
      ) : null}
    </Fragment>
  );
}


/* 终态尾部（独立兄弟元素，稳定 DOM 位置）：挂当前轮——活动轮优先（树内
   末轮终态在有新活动轮时不渲染，等价旧轮级态在下一轮开始时清空）。 */
export function renderAgentTerminalTail(deps: AgentTurnBlockDeps, turn: AgentTurn, isLive: boolean) {
  const { t, streaming, isAdmin, panelSource, citationsTriggerRef, setPanelSource, handleRegenerate, handleFollowUpFill } = deps;
  const terminal = turn.terminal;
  return (
    <>
          {/* SP-15 B #435：轮内推荐追问——当前轮答案（及其引用）下方的
              动作药丸；住轮终态，历史重载不冲掉、下轮开始清空。 */}
          {!streaming && terminal.followUps.length > 0 && (
            <AgentFollowUpChips followUps={terminal.followUps} onFill={handleFollowUpFill} />
          )}

          {!streaming && (terminal.usage || terminal.traceId) && (
            <TurnDetailsDisclosure usage={terminal.usage} traceId={terminal.traceId} isAdmin={isAdmin} />
          )}

          {/* #610 空轮（no_evidence 且零工具执行）：专门空态文案，
              替代「已深度思考」旁的近空白气泡。 */}
          {terminal.emptyNoEvidence && (
            <div className="flex max-w-[85%] items-start gap-2 rounded-md border border-border-default bg-card px-3 py-2 text-sm text-fg-default">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-fg-muted" aria-hidden="true" />
              <div>
                <p className="font-medium">{t("agent.emptyTurn.title")}</p>
                <p className="mt-1 text-xs text-fg-muted">{t("agent.emptyTurn.description")}</p>
              </div>
            </div>
          )}

          {terminal.answerKind === "no_evidence" && !terminal.emptyNoEvidence && (
            <div className="flex max-w-[85%] items-start gap-2 rounded-md border border-border-default bg-card px-3 py-2 text-sm text-fg-default">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-fg-muted" aria-hidden="true" />
              <div>
                <p className="font-medium">{t("agent.noEvidence.title")}</p>
                <p className="mt-1 text-xs text-fg-muted">{t("agent.noEvidence.description")}</p>
                {turn.query && (
                  <Link
                    href={`/search?q=${encodeURIComponent(turn.query)}`}
                    className="mt-2 inline-flex min-h-11 items-center text-sm font-medium text-accent-emphasis underline-offset-2 hover:underline focus:outline-none focus-visible:ring-1 focus:ring-ring"
                  >
                    {t("agent.noEvidence.searchCta")}
                  </Link>
                )}
              </div>
            </div>
          )}

          {terminal.degraded && (
            <div className="flex max-w-[85%] items-start gap-2 rounded-md border border-border-default bg-card px-3 py-2 text-sm text-fg-default">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-fg-muted" aria-hidden="true" />
              <div>
                <p className="font-medium">{t("agent.degraded.title")}</p>
                <p className="mt-1 text-xs text-fg-muted">{t("agent.degraded.description")}</p>
                {turn.query && (
                  <Link
                    href={`/search?q=${encodeURIComponent(turn.query)}`}
                    className="mt-2 inline-flex min-h-11 items-center text-sm font-medium text-accent-emphasis underline-offset-2 hover:underline focus:outline-none focus-visible:ring-1 focus:ring-ring"
                  >
                    {t("agent.noEvidence.searchCta")}
                  </Link>
                )}
              </div>
            </div>
          )}

          {terminal.citations.length > 0 && (
            <CitationsEntryButton
              count={terminal.citations.length}
              active={panelSource === terminal.citations}
              onToggle={() => setPanelSource(terminal.citations)}
              triggerRef={citationsTriggerRef}
            />
          )}

          {terminal.stopped && (
            <p className="text-xs text-fg-muted">{t("agent.workspace.stoppedNotice")}</p>
          )}

          {terminal.error && turn.answer === "" && (
            <div className="flex items-start gap-2 rounded-md border border-border-destructive bg-card px-3 py-2 text-sm text-fg-default">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" aria-hidden="true" />
              <div className="flex-1">
                {terminal.errorCode === "AGENT_RATE_LIMIT_EXCEEDED" ? (
                  <>
                    <p className="font-medium">{t("agent.workspace.rateLimitTitle")}</p>
                    <p className="mt-1 text-xs text-fg-muted">{t("agent.workspace.rateLimitHint")}</p>
                  </>
                ) : (
                  <p className="font-medium">{t("agent.workspace.errorTitle")}</p>
                )}
                {terminal.traceId && (
                  <p className="mt-1 text-xs text-fg-muted">
                    {t("agent.workspace.traceLabel")}: <span className="font-mono">{terminal.traceId}</span>
                  </p>
                )}
              </div>
              {terminal.errorCode !== "AGENT_RATE_LIMIT_EXCEEDED" && (
                <Button variant="outline" size="sm" className="h-9" onClick={handleRegenerate}>
                  <RotateCw className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
                  {t("agent.workspace.errorRetry")}
                </Button>
              )}
            </div>
          )}
    </>
  );
}


/** #750：本轮详情（用量/trace）从裸 details/summary 换成共享胶囊触发器——
    单一受控触发（button 不嵌 summary，杜绝双切换/嵌套交互），默认折叠、
    可反复开合，aria-expanded 与内容可见状态同源；usage/trace 展示条件、
    管理员 trace 链接语义不变。内部状态随挂载周期复位（换会话/换轮重开）。 */
function TurnDetailsDisclosure({ usage, traceId, isAdmin }: {
  usage: { prompt_tokens: number; completion_tokens: number } | null;
  traceId: string | null;
  isAdmin: boolean;
}) {
  const t = useTranslations();
  const [open, setOpen] = useState(false);
  return (
    <div className="max-w-[85%] space-y-1.5">
      <AgentMetaPill
        icon={Info}
        label={t("agent.workspace.turnDetails")}
        expanded={open}
        onClick={() => setOpen((value) => !value)}
      />
      {open && (
        <div className="rounded-md border-[1.5px] border-border-strong bg-canvas-default px-3 py-2 text-xs text-fg-muted">
          {usage && (
            <p>
              {t("agent.workspace.turnUsage", {
                prompt: usage.prompt_tokens,
                completion: usage.completion_tokens,
              })}
            </p>
          )}
          {traceId && (
            <p className={usage ? "mt-1" : undefined}>
              {t("agent.workspace.traceLabel")}:{" "}
              {isAdmin ? (
                <Link
                  href={`/admin/traces/${traceId}`}
                  className="font-mono text-indigo-600 underline-offset-2 hover:underline dark:text-indigo-400"
                >
                  {traceId}
                </Link>
              ) : (
                <span className="font-mono">{traceId}</span>
              )}
            </p>
          )}
        </div>
      )}
    </div>
  );
}

/** FT-4（#696）：回答底部「N 条参考来源」入口按钮（侧栏 toggle 通道之一；
    另一通道 = 侧栏 X）。原内联折叠列表退役。#719：统一 AgentMetaPill
    胶囊外观（点击仍开侧栏）。#763：补 max-w 包装——消息列表是 flex 列，
    胶囊作直接子项会被 stretch 拉满整行，包一层才与工具/思考块同形态。 */
function CitationsEntryButton({ count, active, onToggle, triggerRef }: {
  count: number;
  active: boolean;
  onToggle: () => void;
  triggerRef: React.RefObject<HTMLButtonElement | null>;
}) {
  const t = useTranslations();
  return (
    <div className="mt-1 max-w-[85%]">
      <AgentMetaPill
        icon={BookOpenText}
        label={t("agent.citations.entry", { count })}
        pressed={active}
        onClick={onToggle}
        buttonRef={triggerRef}
      />
    </div>
  );
}
