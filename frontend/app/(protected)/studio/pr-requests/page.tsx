"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { PRCard, PRCardData } from "@/components/pr/PRCard";
import { DiffViewer } from "@/components/pr/DiffViewer";
import { MergeEditor } from "@/components/pr/MergeEditor";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";

interface ContentItem {
  id: number;
  title: string;
}

interface VersionContentResponse {
  content?: string;
}

interface PRDetail extends PRCardData {
  reject_reason?: string;
}

export default function PRRequestsPage() {
  const t = useTranslations();
  const { user } = useAuth();
  const [prs, setPRs] = useState<PRCardData[]>([]);
  const [activePR, setActivePR] = useState<PRDetail | null>(null);
  const [baseText, setBaseText] = useState("");
  const [proposedText, setProposedText] = useState("");
  const [mergeText, setMergeText] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [pendingAction, setPendingAction] = useState<{ type: "accept" | "reject"; id: number } | null>(null);

  const openCount = useMemo(() => prs.filter((item) => item.status === "open").length, [prs]);

  useEffect(() => {
    if (!user) {
      return;
    }

    void (async () => {
      setError("");
      try {
        const contentData = await api.get<{ contents?: ContentItem[] }>(
          `/api/v1/contents?author_id=${user.id}&page=1&page_size=50&sort=newest&time_range=all`
        );

        const contents = contentData.contents || [];
        const allPRs = await Promise.all(
          contents.map(async (content) => {
            const data = await api.get<{ prs?: PRCardData[] }>(`/api/v1/contents/${content.id}/prs?status=open`);
            return (data.prs || []).map((pr) => ({ ...pr, contentTitle: content.title }));
          })
        );

        setPRs(allPRs.flat().sort((a, b) => b.id - a.id));
      } catch (e) {
        silentError(e, { component: 'PRRequestsPage', action: 'loadPRs' });
        setError(t(getUserFacingErrorKey(e, "dashboard.pr.loadFailed")));
      }
    })();
  }, [user, t]);

  async function loadPRDetail(prID: number) {
    setError("");
    setBusy(true);
    try {
      const detail = await api.get<{ pr: PRDetail }>(`/api/v1/pr/${prID}`);
      setActivePR(detail.pr);

      const [base, proposed] = await Promise.all([
        api.get<VersionContentResponse>(`/api/v1/versions/${detail.pr.base_version_id}`),
        detail.pr.proposed_version_id
          ? api.get<VersionContentResponse>(`/api/v1/versions/${detail.pr.proposed_version_id}`)
          : Promise.resolve({ content: "" } as VersionContentResponse),
      ]);

      const left = base.content || "";
      const right = proposed.content || "";
      setBaseText(left);
      setProposedText(right);
      setMergeText(right || left);
    } catch (e) {
      silentError(e, { component: 'PRRequestsPage', action: 'loadPRDetail' });
      setError(t(getUserFacingErrorKey(e, "dashboard.pr.loadDetailFailed")));
    } finally {
      setBusy(false);
    }
  }

  async function acceptPR(prID: number) {
    setBusy(true);
    setError("");
    try {
      await api.post(`/api/v1/pr/${prID}/accept`, {});
      setPRs((prev) => prev.map((item) => (item.id === prID ? { ...item, status: "accepted" } : item)));
      if (activePR?.id === prID) {
        setActivePR({ ...activePR, status: "accepted" });
      }
    } catch (e) {
      silentError(e, { component: 'PRRequestsPage', action: 'acceptPR' });
      setError(t(getUserFacingErrorKey(e, "dashboard.pr.acceptFailed")));
      throw e;
    } finally {
      setBusy(false);
    }
  }

  async function rejectPR(prID: number, reason: string) {
    setBusy(true);
    setError("");
    try {
      await api.post(`/api/v1/pr/${prID}/reject`, { reason: reason.trim() });
      setPRs((prev) => prev.map((item) => (item.id === prID ? { ...item, status: "rejected" } : item)));
      if (activePR?.id === prID) {
        setActivePR({ ...activePR, status: "rejected", reject_reason: reason.trim() });
      }
    } catch (e) {
      silentError(e, { component: 'PRRequestsPage', action: 'rejectPR' });
      setError(t(getUserFacingErrorKey(e, "dashboard.pr.rejectFailed")));
      throw e;
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6">
      <section className="rounded-md border border-border bg-card p-4 ">
        <h1 className="text-2xl font-bold tracking-tight">{t('dashboard.pr.title')}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t('dashboard.pr.pendingCount', { count: openCount })}</p>
        <p className="mt-1 text-xs text-muted-foreground">
          {t('dashboard.pr.apiNotice')}
        </p>
      </section>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <section className="grid grid-cols-1 gap-4 xl:grid-cols-[420px_1fr]">
        <div className="space-y-3 rounded-md border border-border bg-card p-3 ">
          {prs.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('dashboard.pr.noPrs')}</p>
          ) : (
            prs.map((item) => (
              <PRCard
                key={item.id}
                data={item}
                active={activePR?.id === item.id}
                disabled={busy}
                onSelect={(id) => {
                  void loadPRDetail(id);
                }}
                onAccept={(id) => {
                  setPendingAction({ type: "accept", id });
                }}
                onReject={(id) => {
                  setPendingAction({ type: "reject", id });
                }}
              />
            ))
          )}
        </div>

        <div className="space-y-4">
          {activePR ? (
            <>
              <DiffViewer baseText={baseText} proposedText={proposedText} />
              <MergeEditor
                baseText={baseText}
                proposedText={proposedText}
                onChange={setMergeText}
              />
              <div className="rounded-md border border-border bg-card p-3 text-xs text-muted-foreground ">
                {t('dashboard.pr.mergePreview', { length: mergeText.length })}
              </div>
            </>
          ) : (
            <div className="rounded-md border border-border bg-card p-4 text-sm text-muted-foreground ">
              {t('dashboard.pr.selectHint')}
            </div>
          )}
        </div>
      </section>

      <div className="flex justify-end">
        <Button variant="outline" onClick={() => window.location.reload()}>
          {t('dashboard.pr.refresh')}
        </Button>
      </div>
      <ConfirmModal
        open={pendingAction !== null}
        onOpenChange={(open) => { if (!open) setPendingAction(null); }}
        title={pendingAction?.type === "reject" ? t('pr.reject') : t('pr.accept')}
        description={pendingAction?.type === "reject" ? t('dashboard.pr.rejectConfirm') : t('dashboard.pr.acceptConfirm')}
        confirmLabel={pendingAction?.type === "reject" ? t('pr.reject') : t('pr.accept')}
        requireReason={pendingAction?.type === "reject"}
        reasonLabel={t('dashboard.pr.rejectReasonRequired')}
        onConfirm={(reason) => {
          if (!pendingAction) return Promise.resolve();
          return pendingAction.type === "reject"
            ? rejectPR(pendingAction.id, reason)
            : acceptPR(pendingAction.id);
        }}
      />
    </div>
  );
}
