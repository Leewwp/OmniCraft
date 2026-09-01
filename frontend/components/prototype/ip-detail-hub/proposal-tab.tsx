"use client";
/**
 * 【原型专用，随时可删】提案投票 tab：提案卡片（diff + 双色进度条 + 投票/锁定）
 * + 未关注引导面板 + 四状态筛选（含改动历史时间线）+ 空态 CTA。
 */
import { Clock, Lightbulb, ThumbsDown, ThumbsUp } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { t } from "./copy";
import { CoverDiff, IntroDiff, ProposalStatusBadge, TagDiff, SearchEmpty } from "./bits";
import {
  PROPOSAL_CONFIG,
  type Proposal,
  type ProposalStatusFilter,
} from "./mock-data";

function proposalFields(p: Proposal): string[] {
  const fields: string[] = [];
  if (p.introDiff) fields.push("intro");
  if (p.coverDiff) fields.push("cover");
  if (p.tagChange) fields.push("tags");
  return fields;
}

/** 提案卡片：头部 → 字段标签 → diff 区 → 结果/操作条。 */
export function ProposalCard({
  proposal,
  myVote,
  following,
  gateOpen,
  onGateOpen,
  onVote,
  onFollow,
  variant = "list",
}: {
  proposal: Proposal;
  myVote?: "for" | "against";
  following: boolean;
  gateOpen: boolean;
  onGateOpen: () => void;
  onVote: (vote: "for" | "against") => void;
  onFollow: () => void;
  variant?: "list" | "history";
}) {
  const { votesFor, votesAgainst } = proposal;
  const total = votesFor + votesAgainst;
  const forPct = total > 0 ? Math.round((votesFor / total) * 100) : 0;
  const thresholdPct = Math.round(PROPOSAL_CONFIG.passThreshold * 100);

  return (
    <article className="rounded-lg border border-border bg-canvas-default p-4 shadow-[var(--elevation-1)]">
      {/* 头部：提案人 + 时间 + 状态 */}
      <header className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">
          {t("proposal.startedAt", { name: proposal.proposer, time: proposal.startedDisplay })}
        </p>
        <ProposalStatusBadge status={proposal.status} />
      </header>

      {/* 字段标签 */}
      <div className="mt-2.5 flex flex-wrap gap-1.5">
        {proposalFields(proposal).map((field) => (
          <span
            key={field}
            className="inline-flex h-5 items-center rounded-full bg-canvas-subtle px-2 text-xs font-medium text-muted-foreground"
          >
            {t(`proposal.field.${field}`)}
          </span>
        ))}
      </div>

      {/* diff 区 */}
      <div className="mt-3 space-y-3">
        {proposal.introDiff && <IntroDiff segments={proposal.introDiff} />}
        {proposal.coverDiff && <CoverDiff oldStyle={proposal.coverDiff.oldStyle} newStyle={proposal.coverDiff.newStyle} />}
        {proposal.tagChange && <TagDiff mode={proposal.tagChange.mode} tag={proposal.tagChange.tag} />}
      </div>

      {/* 结案结果（已通过 / 已否决 / 改动历史） */}
      {proposal.status !== "open" && (
        <footer className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-border pt-3 text-xs text-muted-foreground">
          <span className={cn(votesFor >= votesAgainst ? "text-[var(--tag-green-fg)]" : "text-destructive", "font-medium")}>
            {t("proposal.result", {
              for: votesFor,
              against: votesAgainst,
              rate: total > 0 ? Math.round((votesFor / total) * 100) : 0,
            })}
          </span>
          {variant === "history" && proposal.effectiveDisplay ? (
            <span>{t("proposal.adoptedAt", { time: proposal.effectiveDisplay })}</span>
          ) : (
            proposal.closedDisplay && (
              <span>
                {t("proposal.closedAt", {
                  status: proposal.status === "adopted" ? "结案生效" : "否决",
                  time: proposal.closedDisplay,
                })}
              </span>
            )
          )}
        </footer>
      )}

      {/* 操作条（进行中） */}
      {proposal.status === "open" && (
        <div className="mt-3 border-t border-border pt-3">
          {/* 双色进度条 + 门槛刻度 */}
          <div className="relative h-2 overflow-visible rounded-full bg-border" aria-hidden="true">
            {total > 0 && (
              <>
                <div
                  className="absolute inset-y-0 left-0 rounded-l-full bg-[var(--tag-green-fg)] transition-[width] duration-150"
                  style={{ width: `${forPct}%` }}
                />
                <div
                  className="absolute inset-y-0 rounded-r-full bg-destructive transition-[width,left] duration-150"
                  style={{ left: `${forPct}%`, width: `${100 - forPct}%` }}
                />
              </>
            )}
            {/* 门槛刻度：通过线位置 */}
            <div
              className="absolute -inset-y-1 w-0.5 bg-foreground/50"
              style={{ left: `${thresholdPct}%` }}
              title={t("proposal.passLine", { threshold: thresholdPct })}
            />
          </div>
          <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
            <span>
              <span className="font-medium text-[var(--tag-green-fg)]">{t("proposal.votesFor", { count: votesFor })}</span>
              {" · "}
              <span className="font-medium text-destructive">{t("proposal.votesAgainst", { count: votesAgainst })}</span>
            </span>
            <span className="inline-flex items-center gap-1">
              <Clock className="size-3" aria-hidden="true" />
              {t("proposal.deadline", { days: proposal.deadlineDays ?? 0 })}
              {" · "}
              {t("proposal.meta", {
                minVotes: PROPOSAL_CONFIG.minVotes,
                threshold: thresholdPct,
              })}
            </span>
          </div>

          {/* 投票按钮 / 已投锁定 / 未关注引导 */}
          {gateOpen ? (
            <div className="mt-3 flex flex-wrap items-center justify-between gap-3 rounded-md border border-border bg-canvas-subtle p-3">
              <div className="min-w-0">
                <p className="text-sm font-medium">{t("proposal.gate.title")}</p>
                <p className="mt-0.5 text-xs text-muted-foreground">{t("proposal.gate.desc")}</p>
              </div>
              <Button size="sm" className="min-h-9 shrink-0 gap-1 px-3" onClick={onFollow}>
                {t("proposal.gate.action")}
              </Button>
            </div>
          ) : (
            <>
              <div className="mt-3 flex gap-2">
                <Button
                  variant="outline"
                  className={cn(
                    "min-h-9 flex-1 gap-1.5 border-[var(--tag-green-fg)]/40 text-[var(--tag-green-fg)] hover:bg-[var(--tag-green-bg)] hover:text-[var(--tag-green-fg)]",
                    myVote === "for" && "border-transparent bg-[var(--tag-green-fg)] text-white hover:bg-[var(--tag-green-fg)] hover:text-white",
                  )}
                  disabled={myVote != null}
                  onClick={() => (following ? onVote("for") : onGateOpen())}
                >
                  <ThumbsUp className="size-3.5" aria-hidden="true" />
                  {myVote === "for" ? t("proposal.myVoteFor") : t("proposal.voteFor")}
                </Button>
                <Button
                  variant="outline"
                  className={cn(
                    "min-h-9 flex-1 gap-1.5 border-destructive/40 text-destructive hover:bg-destructive/5 hover:text-destructive",
                    myVote === "against" && "border-transparent bg-destructive text-white hover:bg-destructive hover:text-white",
                  )}
                  disabled={myVote != null}
                  onClick={() => (following ? onVote("against") : onGateOpen())}
                >
                  <ThumbsDown className="size-3.5" aria-hidden="true" />
                  {myVote === "against" ? t("proposal.myVoteAgainst") : t("proposal.voteAgainst")}
                </Button>
              </div>
              {myVote != null && <p className="mt-1.5 text-right text-xs text-muted-foreground">{t("proposal.lockedNote")}</p>}
            </>
          )}
        </div>
      )}
    </article>
  );
}

/** 提案区主体：按状态筛选渲染列表 / 时间线 / 空态。 */
export function ProposalList({
  proposals,
  filter,
  following,
  votes,
  gateFor,
  searching = false,
  searchQ = "",
  onClearSearch,
  onGateOpen,
  onVote,
  onFollow,
  onCreateEntry,
}: {
  proposals: Proposal[];
  filter: ProposalStatusFilter;
  following: boolean;
  votes: Record<string, "for" | "against">;
  gateFor: string | null;
  /** 搜索态：结果已在传入的 proposals 里过滤完成 */
  searching?: boolean;
  searchQ?: string;
  onClearSearch?: () => void;
  onGateOpen: (id: string) => void;
  onVote: (id: string, vote: "for" | "against") => void;
  onFollow: () => void;
  onCreateEntry: () => void;
}) {
  if (searching && proposals.length === 0) {
    return <SearchEmpty q={searchQ} onClear={onClearSearch ?? (() => {})} />;
  }
  if (filter === "history") {
    const adopted = proposals
      .filter((p) => p.status === "adopted")
      .sort((a, b) => a.effectiveIdx - b.effectiveIdx);
    if (adopted.length === 0) {
      return <EmptyState icon={Lightbulb} title={t("proposal.empty.adopted")} />;
    }
    return (
      <div>
        <p className="mb-3 text-xs text-muted-foreground">{t("proposal.historyNote")}</p>
        <ol className="relative ml-2 border-l border-border">
          {adopted.map((p) => (
            <li key={p.id} className="relative pb-6 pl-6 last:pb-0">
              <span
                className="absolute -left-[7px] top-1 size-3.5 rounded-full border-2 border-background bg-[var(--tag-green-fg)]"
                aria-hidden="true"
              />
              <p className="mb-2 text-xs text-muted-foreground">
                {t("proposal.adoptedAt", { time: p.effectiveDisplay ?? "" })}
              </p>
              <ProposalCard
                proposal={p}
                variant="history"
                following={following}
                gateOpen={false}
                onGateOpen={() => onGateOpen(p.id)}
                onVote={(vote) => onVote(p.id, vote)}
                onFollow={onFollow}
              />
            </li>
          ))}
        </ol>
      </div>
    );
  }

  let list = proposals.filter((p) => p.status === filter);
  if (filter === "open") {
    // 默认排序：即将截止优先，其次最新（mock：按剩余天数升序）
    list = [...list].sort((a, b) => (a.deadlineDays ?? 99) - (b.deadlineDays ?? 99));
  }
  if (list.length === 0) {
    if (searching) {
      return (
        <EmptyState
          icon={Lightbulb}
          title={t("proposal.searchNoMatchInStatus", { q: searchQ })}
          action={
            onClearSearch && (
              <Button variant="outline" onClick={onClearSearch}>
                {t("hub.search.clear")}
              </Button>
            )
          }
        />
      );
    }
    if (filter === "open") {
      return (
        <div className="rounded-lg border border-dashed border-border bg-canvas-default px-4 py-10">
          <EmptyState
            icon={Lightbulb}
            title={t("proposal.empty.title")}
            description={t("proposal.empty.desc")}
            action={<Button onClick={onCreateEntry}>{t("proposal.empty.action")}</Button>}
          />
        </div>
      );
    }
    return <EmptyState icon={Lightbulb} title={filter === "adopted" ? t("proposal.empty.adopted") : t("proposal.empty.rejected")} />;
  }
  return (
    <div className="flex flex-col gap-3">
      {filter === "adopted" && <p className="text-xs text-muted-foreground">{t("proposal.adoptedNote")}</p>}
      {filter === "open" && <p className="text-xs text-muted-foreground">{t("proposal.openOrderNote")}</p>}
      {list.map((p) => (
        <ProposalCard
          key={p.id}
          proposal={p}
          myVote={votes[p.id] ?? p.myVote}
          following={following}
          gateOpen={gateFor === p.id}
          onGateOpen={() => onGateOpen(p.id)}
          onVote={(vote) => onVote(p.id, vote)}
          onFollow={onFollow}
        />
      ))}
    </div>
  );
}
