"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Check, Clock, History, Minus, Plus, Search, ThumbsDown, ThumbsUp, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { FilterPills } from "@/components/ui/filter-pills";
import { Skeleton } from "@/components/ui/skeleton";
import { useAuth } from "@/contexts/AuthContext";
import { useToast } from "@/components/ui/Toast";
import { api, ApiRequestError } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";

interface ProposalFields {
  description_change?: string | null;
  cover_url_change?: string | null;
  tags_add?: string[] | string | null;
  tags_remove?: string[] | string | null;
}

interface Proposal extends ProposalFields {
  id: number;
  ip_id: number;
  proposer_id: number;
  proposer?: { id?: number; username?: string };
  status: "open" | "adopted" | "rejected";
  yes_votes: number;
  no_votes: number;
  created_at: string;
  deadline_at: string;
  closed_at?: string | null;
  effective_at?: string | null;
  my_vote?: "yes" | "no" | null;
}

interface VersionRow {
  id: number;
  proposal_id: number;
  yes_votes: number;
  no_votes: number;
  created_at: string;
}

const STATUS_FILTERS = [
  { value: "open", labelKey: "proposal.statusOpen" },
  { value: "adopted", labelKey: "proposal.statusAdopted" },
  { value: "rejected", labelKey: "proposal.statusRejected" },
  { value: "history", labelKey: "proposal.statusHistory" },
];

function parseTags(v: string[] | string | null | undefined): string[] {
  if (Array.isArray(v)) return v;
  if (typeof v === "string" && v.startsWith("[")) {
    try { return JSON.parse(v) as string[]; } catch { return []; }
  }
  return [];
}

// fetchProposals 落库后的已解析形态：tags 差异数组已从 JSON 字符串转为 string[]。
type ParsedProposal = Omit<Proposal, "tags_add" | "tags_remove"> & {
  tags_add: string[];
  tags_remove: string[];
};

function daysLeft(deadline: string): number {
  const ms = new Date(deadline).getTime() - Date.now();
  return Math.max(0, Math.ceil(ms / 86400000));
}

interface IPProposalsTabProps {
  ipId: number;
  apiBase: string;
  query: string;
  status: string;
  canCreate: boolean;
  // 当前资料（用于 diff 对比：旧简介文本 / 旧封面图）
  currentDescription?: string;
  ipCoverUrl?: string;
  onStatusChange: (next: string) => void;
  onFollowed?: () => void;
}

// 提案投票 tab：四状态筛选（含改动历史时间线）+ 提案卡片（diff + 双色进度条 +
// 门槛刻度 + 剩余时间 + 投票锁定态）+ 发起表单（简介/封面/标签）+ 未关注投票
// 引导面板。状态筛选由 URL query 驱动；门槛刻度取自后端 config。
export function IPProposalsTab({ ipId, apiBase, query, status, canCreate, currentDescription, ipCoverUrl, onStatusChange, onFollowed }: IPProposalsTabProps) {
  const t = useTranslations();
  const { user } = useAuth();
  const { toast } = useToast();
  const [proposals, setProposals] = useState<ParsedProposal[]>([]);
  const [versions, setVersions] = useState<VersionRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [voting, setVoting] = useState<number | null>(null);
  // 未达投票资格（未关注）的提案：页内展示一键关注引导（story 29）
  const [followBlockedId, setFollowBlockedId] = useState<number | null>(null);
  const [governance, setGovernance] = useState<{ minVotes: number; passThreshold: number }>({ minVotes: 10, passThreshold: 0.6 });
  // 请求序号守卫：快速切换筛选/搜索时丢弃过期响应
  const fetchSeqRef = useRef(0);

  const fetchProposals = useCallback(async (nextStatus: string) => {
    const seq = ++fetchSeqRef.current;
    setLoading(true);
    const params = new URLSearchParams();
    if (nextStatus !== "history") params.set("status", nextStatus);
    if (query) params.set("q", query);
    try {
      const res = await fetch(`${apiBase}/ips/${ipId}/proposals?${params.toString()}`, { cache: "no-store" });
      if (!res.ok) throw new Error("FETCH_FAILED");
      const data = (await res.json()) as {
        proposals?: Proposal[];
        min_votes?: number;
        pass_threshold?: number;
      };
      if (seq !== fetchSeqRef.current) return;
      setProposals((data.proposals ?? []).map((p) => ({
        ...p,
        tags_add: parseTags(p.tags_add),
        tags_remove: parseTags(p.tags_remove),
      })));
      if (typeof data.min_votes === "number" && typeof data.pass_threshold === "number") {
        setGovernance({ minVotes: data.min_votes, passThreshold: data.pass_threshold });
      }
    } catch {
      if (seq !== fetchSeqRef.current) return;
      setProposals([]);
    } finally {
      if (seq === fetchSeqRef.current) setLoading(false);
    }
  }, [apiBase, ipId, query]);

  const fetchVersions = useCallback(async () => {
    try {
      const res = await fetch(`${apiBase}/ips/${ipId}/versions`, { cache: "no-store" });
      if (!res.ok) throw new Error("FETCH_FAILED");
      const data = (await res.json()) as { versions?: VersionRow[] };
      setVersions(data.versions ?? []);
    } catch {
      setVersions([]);
    }
  }, [apiBase, ipId]);

  useEffect(() => {
    void fetchProposals(status);
    if (status === "history") void fetchVersions();
  }, [status, fetchProposals, fetchVersions]);

  async function vote(proposal: Proposal, value: "yes" | "no") {
    setVoting(proposal.id);
    try {
      const res = await api.post<{ proposal?: Proposal }>(
        `/api/v1/ips/${ipId}/proposals/${proposal.id}/vote`,
        { vote: value },
      );
      if (res.proposal) {
        const updated: ParsedProposal = {
          ...res.proposal,
          tags_add: parseTags(res.proposal.tags_add),
          tags_remove: parseTags(res.proposal.tags_remove),
        };
        setProposals((cur) => cur.map((p) => (p.id === proposal.id ? updated : p)));
      }
      toast("success", t("proposal.voteRecorded"));
    } catch (e) {
      silentError(e, { component: "IPProposalsTab", action: "vote" });
      // 未关注（资格门）→ 页内一键关注引导原地解锁（story 29）；其余走 toast
      if (e instanceof ApiRequestError && e.code === "PROPOSAL_NOT_ELIGIBLE") {
        setFollowBlockedId(proposal.id);
      } else {
        toast("error", t(getUserFacingErrorKey(e, "proposal.voteFailed")));
      }
    } finally {
      setVoting(null);
    }
  }

  return (
    <section className="space-y-3" aria-label={t('ip.hubTab_proposals')}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <FilterPills
          ariaLabel={t('proposal.filterLabel')}
          options={STATUS_FILTERS.map((s) => ({ value: s.value, label: t(s.labelKey) }))}
          value={status}
          onChange={onStatusChange}
          className="flex-1"
        />
        {status === "open" && canCreate && (
          <Button size="sm" variant="outline" className="gap-1.5" onClick={() => setCreating((v) => !v)}>
            {creating ? <X className="h-3.5 w-3.5" aria-hidden="true" /> : <Plus className="h-3.5 w-3.5" aria-hidden="true" />}
            {creating ? t("proposal.cancelCreate") : t("proposal.create")}
          </Button>
        )}
      </div>

      {creating && (
        <ProposalCreateForm
          ipId={ipId}
          onCreated={() => { setCreating(false); onStatusChange("open"); void fetchProposals("open"); }}
          onCancel={() => setCreating(false)}
        />
      )}

      {loading ? (
        <div className="space-y-2" aria-busy="true" aria-label={t('common.loading')}>
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="rounded-md border border-border bg-card p-4">
              <Skeleton className="h-5 w-1/2" />
              <Skeleton className="mt-3 h-16 w-full" />
            </div>
          ))}
        </div>
      ) : status === "history" ? (
        <HistoryTimeline versions={versions} />
      ) : proposals.length === 0 ? (
        <EmptyState
          icon={Search}
          title={query ? t('ip.hubNoSearchResult', { q: query }) : t('proposal.emptyTitle')}
          description={query ? t('ip.hubNoSearchHint') : t('proposal.emptyHint')}
          action={!query && canCreate ? (
            <Button size="sm" onClick={() => setCreating(true)}>{t('proposal.emptyCta')}</Button>
          ) : undefined}
        />
      ) : (
        <div className="space-y-3">
          {proposals.map((p) => (
            <ProposalCard
              key={p.id}
              proposal={p}
              minVotes={governance.minVotes}
              passThreshold={governance.passThreshold}
              voting={voting === p.id}
              loggedIn={!!user}
              currentDescription={currentDescription}
              ipCoverUrl={ipCoverUrl}
              followBlocked={followBlockedId === p.id}
              onFollowHintClose={() => setFollowBlockedId((cur) => (cur === p.id ? null : cur))}
              onVote={(v) => void vote(p, v)}
              onFollowed={onFollowed}
            />
          ))}
        </div>
      )}
    </section>
  );
}

function ProposalCard({
  proposal, minVotes, passThreshold, voting, loggedIn, currentDescription, ipCoverUrl, followBlocked, onFollowHintClose, onVote, onFollowed,
}: {
  proposal: ParsedProposal;
  minVotes: number;
  passThreshold: number;
  voting: boolean;
  loggedIn: boolean;
  currentDescription?: string;
  ipCoverUrl?: string;
  followBlocked: boolean;
  onFollowHintClose: () => void;
  onVote: (v: "yes" | "no") => void;
  onFollowed?: () => void;
}) {
  const t = useTranslations();
  const [diffExpanded, setDiffExpanded] = useState(false);
  const total = proposal.yes_votes + proposal.no_votes;
  const yesPct = total > 0 ? Math.round((proposal.yes_votes / total) * 100) : 0;
  const noPct = total > 0 ? 100 - yesPct : 0;
  const thresholdPct = Math.max(0, Math.min(100, Math.round(passThreshold * 100)));
  const voted = proposal.my_vote != null;
  const adopted = proposal.status === "adopted";
  const rejected = proposal.status === "rejected";
  const newDescription = proposal.description_change ?? null;
  const descriptionDiff = newDescription != null && currentDescription != null && newDescription !== currentDescription;
  const diffLong = (newDescription?.split("\n").length ?? 0) > 8;

  return (
    <article className="space-y-3 rounded-md border border-border bg-card p-4">
      <header className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>
          {proposal.proposer?.username ?? `#${proposal.proposer_id}`}
          {" · "}
          {new Date(proposal.created_at).toLocaleDateString()}
        </span>
        {proposal.status === "open" && (
          <span className="inline-flex items-center gap-1">
            <Clock className="h-3 w-3" aria-hidden="true" />
            {t('proposal.daysLeft', { count: daysLeft(proposal.deadline_at) })}
          </span>
        )}
        {adopted && <span className="font-medium text-emerald-600">{t('proposal.statusAdopted')}</span>}
        {rejected && <span className="font-medium text-destructive">{t('proposal.statusRejected')}</span>}
      </header>

      {/* 字段级 diff */}
      <div className="space-y-2">
        {newDescription != null && (
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-wider text-fg-subtle">{t('proposal.fieldDescription')}</p>
            {descriptionDiff && (
              <p className={`mt-1 whitespace-pre-wrap rounded-md bg-red-50 p-2 text-sm leading-relaxed text-red-700/90 line-through dark:bg-red-950/30 dark:text-red-300/80 ${diffExpanded ? "" : "line-clamp-4"}`}>
                {currentDescription}
              </p>
            )}
            <p className={`mt-1 whitespace-pre-wrap rounded-md bg-emerald-50 p-2 text-sm leading-relaxed text-emerald-800 underline decoration-emerald-500/60 decoration-1 underline-offset-2 dark:bg-emerald-950/30 dark:text-emerald-300 ${diffExpanded ? "" : "line-clamp-8"}`}>
              {newDescription}
            </p>
            {diffLong && (
              <button type="button" onClick={() => setDiffExpanded((v) => !v)} className="mt-1 text-xs text-accent-emphasis hover:underline">
                {diffExpanded ? t('proposal.diffCollapse') : t('proposal.diffExpand')}
              </button>
            )}
          </div>
        )}
        {proposal.cover_url_change != null && (
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-wider text-fg-subtle">{t('proposal.fieldCover')}</p>
            <div className="mt-1 flex flex-wrap items-center gap-3">
              <figure className="w-40">
                {/* 提案封面 URL 为用户输入的任意地址，不走 next/image 域名白名单 */}
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={ipCoverUrl || ""}
                  alt={t('proposal.coverOld')}
                  className="h-24 w-40 rounded-md border border-border object-cover"
                  onError={(e) => { e.currentTarget.style.visibility = "hidden"; }}
                />
                <figcaption className="mt-1 text-center text-[10px] text-muted-foreground">{t('proposal.coverOld')}</figcaption>
              </figure>
              <span aria-hidden="true" className="text-muted-foreground">→</span>
              <figure className="w-40">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={proposal.cover_url_change}
                  alt={t('proposal.coverNew')}
                  className="h-24 w-40 rounded-md border border-border object-cover"
                  onError={(e) => { e.currentTarget.style.visibility = "hidden"; }}
                />
                <figcaption className="mt-1 text-center text-[10px] text-muted-foreground">{t('proposal.coverNew')}</figcaption>
              </figure>
            </div>
          </div>
        )}
        {(proposal.tags_add?.length || 0) > 0 && (
          <div className="flex flex-wrap items-center gap-1.5">
            <p className="text-[11px] font-semibold uppercase tracking-wider text-fg-subtle">{t('proposal.fieldTags')}</p>
            {(proposal.tags_add ?? []).map((tag) => (
              <span key={`add-${tag}`} className="inline-flex items-center gap-1 rounded-full border border-emerald-300 bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700">
                <Plus className="h-3 w-3" aria-hidden="true" />{tag}
              </span>
            ))}
            {(proposal.tags_remove ?? []).map((tag) => (
              <span key={`rm-${tag}`} className="inline-flex items-center gap-1 rounded-full border border-red-300 bg-red-50 px-2 py-0.5 text-xs font-medium text-red-700 line-through">
                <Minus className="h-3 w-3" aria-hidden="true" />{tag}
              </span>
            ))}
          </div>
        )}
      </div>

      {/* 进度条 + 门槛刻度（赞成绿 / 反对红双色段） */}
      <div>
        <div className="relative h-2.5 w-full overflow-hidden rounded-full bg-muted">
          <div className="flex h-full">
            <div className="h-full bg-emerald-500 transition-all duration-150" style={{ width: `${yesPct}%` }} />
            <div className="h-full bg-red-400 transition-all duration-150" style={{ width: `${noPct}%` }} />
          </div>
          <div
            className="absolute top-0 h-full w-0.5 bg-foreground/60"
            style={{ left: `${thresholdPct}%` }}
            aria-hidden="true"
          />
        </div>
        <div className="mt-1 flex items-center justify-between text-xs text-muted-foreground">
          <span className="inline-flex items-center gap-1 text-emerald-600">
            <ThumbsUp className="h-3 w-3" aria-hidden="true" />{proposal.yes_votes}
          </span>
          <span>{t('proposal.voteScale', { yes: proposal.yes_votes, min: minVotes })}</span>
          <span className="inline-flex items-center gap-1 text-destructive">
            <ThumbsDown className="h-3 w-3" aria-hidden="true" />{proposal.no_votes}
          </span>
        </div>
      </div>

      {/* 操作条 */}
      {proposal.status === "open" && (
        <div className="flex flex-wrap items-center gap-2">
          {voted ? (
            <p className="text-xs text-muted-foreground">
              {t('proposal.votedAs', { choice: t(proposal.my_vote === "yes" ? "proposal.voteYes" : "proposal.voteNo") })}
            </p>
          ) : loggedIn ? (
            <>
              <Button size="sm" disabled={voting} onClick={() => onVote("yes")} className="gap-1.5">
                <ThumbsUp className="h-3.5 w-3.5" aria-hidden="true" />{t('proposal.voteYes')}
              </Button>
              <Button size="sm" variant="outline" disabled={voting} onClick={() => onVote("no")} className="gap-1.5">
                <ThumbsDown className="h-3.5 w-3.5" aria-hidden="true" />{t('proposal.voteNo')}
              </Button>
            </>
          ) : (
            <p className="text-xs text-muted-foreground">{t('proposal.followToVote')}</p>
          )}
          {followBlocked && (
            <FollowHint
              ipId={proposal.ip_id}
              onDone={() => { onFollowHintClose(); onFollowed?.(); }}
            />
          )}
        </div>
      )}
    </article>
  );
}

// 未关注者投票引导：点击投票按钮前若被拒（403）由 toast 提示；
// 这里提供页内一键关注面板（关注后原地解锁投票权）。
function FollowHint({ ipId, onDone }: { ipId: number; onDone: () => void }) {
  const t = useTranslations();
  const { toast } = useToast();
  const [busy, setBusy] = useState(false);

  async function follow() {
    setBusy(true);
    try {
      await api.post(`/api/v1/ips/${ipId}/follow`, {});
      toast("success", t("proposal.followedUnlocked"));
      onDone();
    } catch (e) {
      silentError(e, { component: "FollowHint", action: "follow" });
      toast("error", t(getUserFacingErrorKey(e, "proposal.followFailed")));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex items-center gap-2 rounded-md border border-border bg-accent-subtle/60 px-3 py-1.5">
      <span className="text-xs text-accent-emphasis">{t('proposal.followToVote')}</span>
      <Button size="sm" variant="outline" disabled={busy} onClick={() => void follow()}>{t('proposal.followNow')}</Button>
    </div>
  );
}

function HistoryTimeline({ versions }: { versions: VersionRow[] }) {
  const t = useTranslations();
  if (versions.length === 0) {
    return <EmptyState icon={History} title={t('proposal.historyEmpty')} description={t('proposal.historyEmptyHint')} />;
  }
  return (
    <ol className="relative space-y-4 border-l border-border pl-4">
      {versions.map((v) => (
        <li key={v.id} className="relative">
          <span className="absolute -left-[21px] top-1.5 h-2.5 w-2.5 rounded-full border-2 border-border bg-accent-emphasis" aria-hidden="true" />
          <div className="rounded-md border border-border bg-card p-3">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span className="inline-flex items-center gap-1 font-medium text-accent-emphasis">
                <Check className="h-3 w-3" aria-hidden="true" />
                {t('proposal.effectiveAt', { date: new Date(v.created_at).toLocaleDateString() })}
              </span>
              <span>{t('proposal.voteResult', { yes: v.yes_votes, no: v.no_votes })}</span>
            </div>
          </div>
        </li>
      ))}
    </ol>
  );
}

function ProposalCreateForm({ ipId, onCreated, onCancel }: { ipId: number; onCreated: () => void; onCancel: () => void }) {
  const t = useTranslations();
  const { toast } = useToast();
  const [description, setDescription] = useState("");
  const [coverUrl, setCoverUrl] = useState("");
  const [tagAdd, setTagAdd] = useState("");
  const [tagRemove, setTagRemove] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit() {
    const payload: Record<string, unknown> = {};
    if (description.trim()) payload.description_change = description.trim();
    if (coverUrl.trim()) payload.cover_url_change = coverUrl.trim();
    if (tagAdd.trim()) payload.tags_add = [tagAdd.trim()];
    if (tagRemove.trim()) payload.tags_remove = [tagRemove.trim()];
    if (Object.keys(payload).length === 0) {
      toast("error", t("proposal.needOneField"));
      return;
    }
    setBusy(true);
    try {
      await api.post(`/api/v1/ips/${ipId}/proposals`, payload);
      toast("success", t("proposal.created"));
      onCreated();
    } catch (e) {
      silentError(e, { component: "ProposalCreateForm", action: "submit" });
      toast("error", t(getUserFacingErrorKey(e, "proposal.createFailed")));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-3 rounded-md border border-border bg-card p-4">
      <h3 className="text-sm font-semibold">{t('proposal.createFormTitle')}</h3>
      <div>
        <label htmlFor="proposal-description" className="text-xs font-medium text-muted-foreground">
          {t('proposal.fieldDescription')}
        </label>
        <textarea
          id="proposal-description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={4}
          className="mt-1 min-h-11 w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          placeholder={t('proposal.descriptionPlaceholder')}
        />
      </div>
      <div>
        <label htmlFor="proposal-cover" className="text-xs font-medium text-muted-foreground">
          {t('proposal.fieldCover')}
        </label>
        <input
          id="proposal-cover"
          type="url"
          value={coverUrl}
          onChange={(e) => setCoverUrl(e.target.value)}
          className="mt-1 min-h-11 w-full rounded-md border border-border bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          placeholder={t('proposal.coverPlaceholder')}
        />
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        <div>
          <label htmlFor="proposal-tag-add" className="text-xs font-medium text-muted-foreground">
            {t('proposal.tagAdd')}
          </label>
          <input
            id="proposal-tag-add"
            value={tagAdd}
            onChange={(e) => setTagAdd(e.target.value)}
            className="mt-1 min-h-11 w-full rounded-md border border-border bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          />
        </div>
        <div>
          <label htmlFor="proposal-tag-remove" className="text-xs font-medium text-muted-foreground">
            {t('proposal.tagRemove')}
          </label>
          <input
            id="proposal-tag-remove"
            value={tagRemove}
            onChange={(e) => setTagRemove(e.target.value)}
            className="mt-1 min-h-11 w-full rounded-md border border-border bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          />
        </div>
      </div>
      <p className="text-xs text-muted-foreground">{t('proposal.createRules')}</p>
      <div className="flex gap-2">
        <Button size="sm" disabled={busy} onClick={() => void submit()}>{t('proposal.submit')}</Button>
        <Button size="sm" variant="outline" onClick={onCancel}>{t('proposal.cancelCreate')}</Button>
      </div>
    </div>
  );
}
