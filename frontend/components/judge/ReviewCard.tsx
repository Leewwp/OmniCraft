"use client";

import { useEffect, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { AlertTriangle, Eye, Flag, SkipForward } from "lucide-react";
import { api, ApiRequestError } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { useToast } from "@/components/ui/Toast";

interface JudgeCaseData {
  id: number;
  target_type: string;
  target_id: number;
  status: string;
  vote_approve: number;
  vote_reject: number;
  min_votes: number;
  created_at: string;
}

interface PreviewAttachment {
  id: number;
  oss_url?: string;
}

interface PreviewData {
  content: {
    id: number;
    title: string;
    description?: string;
    content_type: string;
  };
  attachments: PreviewAttachment[];
}

type PreviewPhase = "idle" | "loading" | "loaded" | "error";

interface ReviewCardProps {
  judgeCase: JudgeCaseData;
  disabled: boolean;
  submitting: boolean;
  onVote: (caseId: number, vote: "approve" | "reject", reason: string) => void;
  onSkip?: () => void;
}

export default function ReviewCard({ judgeCase, disabled, submitting, onVote, onSkip }: ReviewCardProps) {
  const t = useTranslations();
  const locale = useLocale();
  const { toast } = useToast();
  const [showConfirm, setShowConfirm] = useState(false);
  const [pendingVote, setPendingVote] = useState<"approve" | "reject" | null>(null);
  const [reason, setReason] = useState("");

  // T40（FIX-36d）：受控内容预览——点击后才请求内容，媒体再二次确认加载。
  const [preview, setPreview] = useState<PreviewPhase>("idle");
  const [previewData, setPreviewData] = useState<PreviewData | null>(null);
  const [mediaVisible, setMediaVisible] = useState(false);
  const [reportOpen, setReportOpen] = useState(false);
  const [reported, setReported] = useState(false);

  // T40（FIX-36d）：父组件复用本实例切换案件（跳过/投票后前进）时重置预览态，
  // 防止上一案已加载内容/媒体残留到下一案（张冠李戴比盲投更危险）。
  useEffect(() => {
    setPreview("idle");
    setPreviewData(null);
    setMediaVisible(false);
    setReportOpen(false);
    setReported(false);
  }, [judgeCase.id]);

  const totalVotes = judgeCase.vote_approve + judgeCase.vote_reject;
  const approvePct = totalVotes > 0 ? Math.round((judgeCase.vote_approve / totalVotes) * 100) : 0;
  const rejectPct = totalVotes > 0 ? Math.round((judgeCase.vote_reject / totalVotes) * 100) : 0;

  function triggerVote(vote: "approve" | "reject") {
    setPendingVote(vote);
    setShowConfirm(true);
  }

  function confirmVote() {
    if (pendingVote) {
      onVote(judgeCase.id, pendingVote, reason);
      setShowConfirm(false);
      setPendingVote(null);
      setReason("");
    }
  }

  function cancelVote() {
    setShowConfirm(false);
    setPendingVote(null);
  }

  const typeLabelKey: Record<string, string> = {
    article: 'judge.article',
    image: 'judge.image',
    video: 'judge.video',
    audio: 'judge.audio',
    prompt: 'judge.prompt',
    sheet_music: 'judge.sheetMusic',
    template: 'judge.template',
    other: 'judge.other',
    content: 'judge.reviewCard.typeContent',
    comment: 'judge.reviewCard.typeComment',
  };
  const typeLabel = typeLabelKey[judgeCase.target_type] ? t(typeLabelKey[judgeCase.target_type]) : judgeCase.target_type;

  async function loadPreview() {
    setPreview("loading");
    try {
      const data = await api.get<PreviewData>(`/api/v1/contents/${judgeCase.target_id}`);
      setPreviewData(data);
      setMediaVisible(false);
      setPreview("loaded");
    } catch (e) {
      silentError(e, { component: 'ReviewCard', action: 'loadPreview' });
      setPreview("error");
    }
  }

  async function submitReport(reportReason: string) {
    try {
      await api.post(`/api/v1/contents/${judgeCase.target_id}/report`, { reason: reportReason });
      setReported(true);
    } catch (e) {
      if (e instanceof ApiRequestError && e.status === 409) {
        setReported(true);
        return;
      }
      toast("error", t(getUserFacingErrorKey(e, "social.reportFailed")));
      silentError(e, { component: 'ReviewCard', action: 'submitReport' });
      throw e;
    }
  }

  return (
    <div className="space-y-4 rounded-md border border-border bg-card p-4 ">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="rounded bg-accent/10 px-1.5 py-0.5 text-[11px] font-medium text-accent-foreground">
            {typeLabel}
          </span>
          <span className="text-xs text-muted-foreground">#{judgeCase.target_id}</span>
        </div>
        <span className="text-xs text-muted-foreground">
          {new Date(judgeCase.created_at).toLocaleString(locale === "en" ? "en-US" : "zh-CN")}
        </span>
      </div>

      {/* T40（FIX-36d）：受控内容预览——预警横幅 + 点击后加载，媒体不自动加载。 */}
      <div className="space-y-2 rounded-md border border-border bg-muted/30 p-3">
        {preview === "idle" && (
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
            <div className="flex flex-1 items-start gap-2">
              <AlertTriangle className="mt-0.5 size-4 shrink-0 text-amber-600" aria-hidden="true" />
              <p className="text-xs text-amber-700 dark:text-amber-500">
                {t('judge.reviewCard.previewWarning')}
              </p>
            </div>
            <Button size="sm" variant="outline" disabled={submitting} onClick={() => void loadPreview()}>
              <Eye className="mr-1 h-3.5 w-3.5" aria-hidden="true" />
              {t('judge.reviewCard.previewLoadButton')}
            </Button>
          </div>
        )}

        {preview === "loading" && (
          <p className="text-xs text-muted-foreground">{t('common.loading')}</p>
        )}

        {preview === "error" && (
          <div className="flex items-center justify-between gap-2">
            <p className="text-xs text-destructive">{t('judge.reviewCard.previewLoadFailed')}</p>
            <Button size="sm" variant="outline" onClick={() => void loadPreview()}>
              {t('common.retry')}
            </Button>
          </div>
        )}

        {preview === "loaded" && previewData && (
          <div className="space-y-2">
            <h4 className="text-sm font-medium text-card-foreground">{previewData.content.title}</h4>
            {previewData.content.description && (
              <p className="max-h-48 overflow-y-auto whitespace-pre-wrap text-xs text-muted-foreground">
                {previewData.content.description}
              </p>
            )}
            {(previewData.attachments?.length ?? 0) > 0 && !mediaVisible && (
              <Button size="sm" variant="outline" onClick={() => setMediaVisible(true)}>
                <Eye className="mr-1 h-3.5 w-3.5" aria-hidden="true" />
                {t('judge.reviewCard.mediaLoadButton')}
              </Button>
            )}
            {mediaVisible && (
              <div className="space-y-2">
                {previewData.attachments.map((a) =>
                  a.oss_url ? (
                    <img
                      key={a.id}
                      src={a.oss_url}
                      alt={previewData.content.title}
                      loading="lazy"
                      className="max-h-64 rounded-md border border-border"
                    />
                  ) : null
                )}
              </div>
            )}
          </div>
        )}
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>{t('judge.reviewCard.voteProgress')}</span>
          <span>
            {t('judge.reviewCard.votes', { totalVotes, minVotes: judgeCase.min_votes })}
          </span>
        </div>
        <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
          <div className="flex h-full">
            <div
              className="h-full bg-emerald-500 transition-all duration-300"
              style={{ width: `${approvePct}%` }}
            />
            <div
              className="h-full bg-destructive transition-all duration-300"
              style={{ width: `${rejectPct}%` }}
            />
          </div>
        </div>
        <div className="flex justify-between text-xs">
          <span className="text-emerald-600">{t('judge.reviewCard.approve')} {judgeCase.vote_approve}</span>
          <span className="text-destructive">{t('judge.reviewCard.reject')} {judgeCase.vote_reject}</span>
        </div>
      </div>

      {!showConfirm && (
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            className="flex-1 border-emerald-500/30 text-emerald-600 hover:bg-emerald-50"
            disabled={disabled || submitting}
            onClick={() => triggerVote("approve")}
          >
            {t('judge.reviewCard.approve')}
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="flex-1 border-destructive/30 text-destructive hover:bg-destructive/10"
            disabled={disabled || submitting}
            onClick={() => triggerVote("reject")}
          >
            {t('judge.reviewCard.reject')}
          </Button>
        </div>
      )}

      {showConfirm && (
        <div className="space-y-3 rounded-md border border-border bg-muted/20 p-3">
          <p className="text-sm font-medium">
            {t('judge.reviewCard.confirmVote', {
              vote: pendingVote === "approve" ? t('judge.reviewCard.approve') : t('judge.reviewCard.reject'),
            })}
          </p>
          <textarea
            placeholder={t('judge.reviewCard.reasonPlaceholder')}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={2}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
          />
          <div className="flex gap-2">
            <Button size="sm" disabled={submitting} onClick={confirmVote}>
              {submitting ? t('common.submitting') : t('judge.reviewCard.confirm')}
            </Button>
            <Button size="sm" variant="outline" onClick={cancelVote}>
              {t('common.cancel')}
            </Button>
          </div>
        </div>
      )}

      {/* T40：跳过本案 + 举报此内容 */}
      <div className="flex gap-2 border-t border-border pt-3">
        <Button
          variant="ghost"
          size="sm"
          className="flex-1 text-muted-foreground"
          disabled={submitting}
          onClick={() => onSkip?.()}
        >
          <SkipForward className="mr-1 h-3.5 w-3.5" aria-hidden="true" />
          {t('judge.reviewCard.skipCase')}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="flex-1 text-muted-foreground"
          disabled={reported}
          onClick={() => setReportOpen(true)}
        >
          <Flag className="mr-1 h-3.5 w-3.5" aria-hidden="true" />
          {reported ? t('social.reported') : t('judge.reviewCard.reportThisContent')}
        </Button>
      </div>

      <ConfirmModal
        open={reportOpen}
        onOpenChange={setReportOpen}
        title={t('social.reportDialogTitle')}
        description={t('social.reportReason')}
        reasonLabel={t('social.reportReason')}
        confirmLabel={t('social.report')}
        requireReason
        onConfirm={submitReport}
      />
    </div>
  );
}
