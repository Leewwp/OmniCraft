"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";

// T33（FIX-37）：settings「信誉明细」区——/reputation-ports/me 自查端点的
// 唯一消费面。视觉按 ui-spec `## Component: ReputationDetail`：1px border
// 扁平、p-4、text-sm/xs、gap-3，无阴影。
// reason 映射进 i18n（处罚原因如 ai_violation 不再裸露英文枚举），未知 key
// 安全回退显示原值。

interface ReputationLogItem {
  id: number;
  delta: number;
  reason: string;
  created_at: string;
}

const PAGE_SIZE = 10;

export default function ReputationDetail() {
  const t = useTranslations();
  const [logs, setLogs] = useState<ReputationLogItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState(false);

  const load = useCallback(async (nextPage: number, append: boolean) => {
    if (append) setLoadingMore(true);
    try {
      const data = await api.get<{ logs?: ReputationLogItem[]; total?: number }>(
        `/api/v1/reputation-logs/me?page=${nextPage}&page_size=${PAGE_SIZE}`
      );
      const incoming = data.logs || [];
      setLogs((current) =>
        append
          ? [...current, ...incoming.filter((item) => !current.some((e) => e.id === item.id))]
          : incoming
      );
      setTotal(data.total ?? 0);
      setPage(nextPage);
      setError(false);
    } catch (e) {
      silentError(e, { component: "ReputationDetail", action: "load" });
      setError(true);
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
  }, []);

  useEffect(() => {
    void load(1, false);
  }, [load]);

  function reasonLabel(reason: string): string {
    const key = `settings.reputation.reasons.${reason}`;
    const label = t(key);
    // next-intl 对缺失键回显 key 本身——此时回退展示原始 reason。
    return label === key ? reason : label;
  }

  function formatTime(value: string): string {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
  }

  const hasMore = logs.length < total;

  return (
    <div className="space-y-3 rounded-md border border-border bg-card p-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">{t("settings.reputation.title")}</h3>
        <span className="text-xs text-muted-foreground">
          {t("settings.reputation.total", { total })}
        </span>
      </div>

      {error ? (
        <p className="text-sm text-destructive">{t("settings.reputation.loadFailed")}</p>
      ) : loading ? (
        <div className="space-y-2" aria-busy="true">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-8 w-full animate-pulse rounded bg-muted" />
          ))}
        </div>
      ) : logs.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("settings.reputation.empty")}</p>
      ) : (
        <>
          <ul className="flex flex-col gap-3">
            {logs.map((log) => (
              <li
                key={log.id}
                className="flex flex-wrap items-center justify-between gap-2 border-b border-border pb-2 text-sm last:border-b-0 last:pb-0"
              >
                <div className="flex flex-col gap-0.5">
                  <span className="text-sm">{reasonLabel(log.reason)}</span>
                  <span className="text-xs text-muted-foreground">
                    {formatTime(log.created_at)}
                  </span>
                </div>
                <span
                  className={`text-sm font-medium ${
                    log.delta >= 0 ? "text-emerald-600" : "text-destructive"
                  }`}
                >
                  {log.delta >= 0 ? `+${log.delta}` : log.delta}
                </span>
              </li>
            ))}
          </ul>
          {hasMore && (
            <Button
              size="sm"
              variant="outline"
              disabled={loadingMore}
              onClick={() => void load(page + 1, true)}
            >
              {loadingMore ? t("settings.reputation.loading") : t("settings.reputation.loadMore")}
            </Button>
          )}
        </>
      )}
    </div>
  );
}
