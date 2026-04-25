"use client";

import { useEffect, useMemo, useState } from "react";
import { PRCard, PRCardData } from "@/components/pr/PRCard";
import { DiffViewer } from "@/components/pr/DiffViewer";
import { MergeEditor } from "@/components/pr/MergeEditor";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";

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
  const { user, isLoading } = useAuth();
  const [prs, setPRs] = useState<PRCardData[]>([]);
  const [activePR, setActivePR] = useState<PRDetail | null>(null);
  const [baseText, setBaseText] = useState("");
  const [proposedText, setProposedText] = useState("");
  const [mergeText, setMergeText] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

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
        if (e instanceof ApiRequestError) {
          setError(`${e.code}: ${e.message}`);
        } else {
          setError("加载 PR 列表失败");
        }
      }
    })();
  }, [user]);

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
      if (e instanceof ApiRequestError) {
        setError(`${e.code}: ${e.message}`);
      } else {
        setError("加载 PR 详情失败");
      }
    } finally {
      setBusy(false);
    }
  }

  async function acceptPR(prID: number) {
    const confirmed = window.confirm("确认接受该 PR 吗？接受后将更新版本状态。");
    if (!confirmed) {
      return;
    }

    setBusy(true);
    setError("");
    try {
      await api.post(`/api/v1/pr/${prID}/accept`, {});
      setPRs((prev) => prev.map((item) => (item.id === prID ? { ...item, status: "accepted" } : item)));
      if (activePR?.id === prID) {
        setActivePR({ ...activePR, status: "accepted" });
      }
    } catch (e) {
      if (e instanceof ApiRequestError) {
        setError(`${e.code}: ${e.message}`);
      } else {
        setError("接受 PR 失败");
      }
    } finally {
      setBusy(false);
    }
  }

  async function rejectPR(prID: number) {
    const reason = window.prompt("请输入拒绝理由（必填）");
    if (!reason || !reason.trim()) {
      return;
    }

    const confirmed = window.confirm("确认拒绝该 PR 吗？");
    if (!confirmed) {
      return;
    }

    setBusy(true);
    setError("");
    try {
      await api.post(`/api/v1/pr/${prID}/reject`, { reason: reason.trim() });
      setPRs((prev) => prev.map((item) => (item.id === prID ? { ...item, status: "rejected" } : item)));
      if (activePR?.id === prID) {
        setActivePR({ ...activePR, status: "rejected", reject_reason: reason.trim() });
      }
    } catch (e) {
      if (e instanceof ApiRequestError) {
        setError(`${e.code}: ${e.message}`);
      } else {
        setError("拒绝 PR 失败");
      }
    } finally {
      setBusy(false);
    }
  }

  if (isLoading) {
    return <div className="mx-auto w-full max-w-7xl px-4 py-6 text-sm text-muted-foreground">加载中...</div>;
  }

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6">
      <section className="rounded-md border border-border bg-card p-4 shadow-none">
        <h1 className="text-2xl font-bold tracking-tight">PR 申请管理</h1>
        <p className="mt-1 text-sm text-muted-foreground">待处理 PR：{openCount} 个</p>
        <p className="mt-1 text-xs text-muted-foreground">
          手动合并 API 当前未对外开放，本页已提供 MergeEditor 供你先完成文本合并与复制。
        </p>
      </section>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <section className="grid grid-cols-1 gap-4 xl:grid-cols-[420px_1fr]">
        <div className="space-y-3 rounded-md border border-border bg-card p-3 shadow-none">
          {prs.length === 0 ? (
            <p className="text-sm text-muted-foreground">暂无待处理 PR</p>
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
                  void acceptPR(id);
                }}
                onReject={(id) => {
                  void rejectPR(id);
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
              <div className="rounded-md border border-border bg-card p-3 text-xs text-muted-foreground shadow-none">
                合并结果预览长度：{mergeText.length} 字符
              </div>
            </>
          ) : (
            <div className="rounded-md border border-border bg-card p-4 text-sm text-muted-foreground shadow-none">
              选择左侧 PR 查看 Diff 和合并编辑器。
            </div>
          )}
        </div>
      </section>

      <div className="flex justify-end">
        <Button variant="outline" onClick={() => window.location.reload()}>
          刷新列表
        </Button>
      </div>
    </div>
  );
}
