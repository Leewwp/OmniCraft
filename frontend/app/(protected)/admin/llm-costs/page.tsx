"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Select } from "@/components/ui/select";
import { AdminMetricCard } from "@/components/admin/AdminMetricCard";
import { Coins, ArrowDownToLine, ArrowUpFromLine, AlertTriangle, Wallet } from "lucide-react";

interface LLMCostRateView {
  in_per_m_tokens: number;
  out_per_m_tokens: number;
}

interface LLMCostModelRow {
  model: string;
  tokens_in: number;
  tokens_out: number;
  cost_cny: number;
  estimated: boolean;
}

interface LLMCostDayRow {
  day: string;
  tokens_in: number;
  tokens_out: number;
  cost_cny: number;
}

interface LLMCostConversationRow {
  conversation_id: number;
  turns: number;
  tokens_in: number;
  tokens_out: number;
  cost_cny: number;
  estimated: boolean;
}

interface LLMCostLedger {
  window: { from: string; to: string };
  rates: Record<string, LLMCostRateView>;
  totals: { tokens_in: number; tokens_out: number; cost_cny: number; unpriced_models: number };
  by_model: LLMCostModelRow[];
  by_day: LLMCostDayRow[];
  top_conversations: LLMCostConversationRow[];
}

const TIME_WINDOWS = [
  { key: "24h", hours: 24 },
  { key: "7d", hours: 24 * 7 },
  { key: "30d", hours: 24 * 30 },
  { key: "all", hours: 0 },
] as const;

type WindowKey = (typeof TIME_WINDOWS)[number]["key"];

function formatCNY(cost: number): string {
  if (cost === 0) return "¥0";
  if (cost < 0.01) return `¥${cost.toFixed(4)}`;
  return `¥${cost.toFixed(2)}`;
}

function formatTokens(tokens: number): string {
  return tokens.toLocaleString();
}

export default function AdminLLMCostsPage() {
  const t = useTranslations();
  const [ledger, setLedger] = useState<LLMCostLedger | null>(null);
  const [windowKey, setWindowKey] = useState<WindowKey>("7d");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const params = new URLSearchParams();
      const win = TIME_WINDOWS.find((w) => w.key === windowKey);
      if (win && win.hours > 0) {
        params.set("from", new Date(Date.now() - win.hours * 3600 * 1000).toISOString());
      }
      setLedger(await api.get<LLMCostLedger>(`/api/v1/admin/llm-costs?${params.toString()}`));
    } catch (e) {
      silentError(e, { component: "AdminLLMCostsPage", action: "load" });
      setError(t(getUserFacingErrorKey(e, "admin.llmCosts.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [windowKey, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const byDay = ledger?.by_day ?? [];
  const byModel = ledger?.by_model ?? [];
  const topConversations = ledger?.top_conversations ?? [];
  const totals = ledger?.totals;

  // The trend bar uses spend when anything is priced, falling back to raw
  // token volume when the whole window is unpriced.
  const anyPriced = (totals?.cost_cny ?? 0) > 0;
  const barValues = byDay.map((d) => (anyPriced ? d.cost_cny : d.tokens_in + d.tokens_out));
  const barMax = Math.max(0, ...barValues);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold">{t("admin.llmCosts.title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("admin.llmCosts.subtitle")}</p>
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-xs text-muted-foreground" htmlFor="llm-cost-window">
            {t("admin.llmCosts.filterWindow")}
          </label>
          <Select
            id="llm-cost-window"
            aria-label={t("admin.llmCosts.filterWindow")}
            className="h-8 px-2 py-0 text-sm"
            value={windowKey}
            onChange={(e) => setWindowKey(e.target.value as WindowKey)}
          >
            {TIME_WINDOWS.map((w) => (
              <option key={w.key} value={w.key}>
                {t(`admin.llmCosts.window_${w.key}`)}
              </option>
            ))}
          </Select>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <AdminMetricCard
          label={t("admin.llmCosts.metricCost")}
          value={totals ? formatCNY(totals.cost_cny) : "—"}
          icon={Coins}
          loading={loading && !ledger}
        />
        <AdminMetricCard
          label={t("admin.llmCosts.metricTokensIn")}
          value={totals ? formatTokens(totals.tokens_in) : "—"}
          icon={ArrowDownToLine}
          loading={loading && !ledger}
        />
        <AdminMetricCard
          label={t("admin.llmCosts.metricTokensOut")}
          value={totals ? formatTokens(totals.tokens_out) : "—"}
          icon={ArrowUpFromLine}
          loading={loading && !ledger}
        />
        <AdminMetricCard
          label={t("admin.llmCosts.metricUnpriced")}
          value={totals ? String(totals.unpriced_models) : "—"}
          icon={AlertTriangle}
          variant={totals && totals.unpriced_models > 0 ? "warning" : "default"}
          loading={loading && !ledger}
        />
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}
      {totals && totals.unpriced_models > 0 && (
        <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-400">
          {t("admin.llmCosts.unpricedHint")}
        </p>
      )}

      {loading && !ledger ? (
        <div className="rounded-md border border-border bg-card p-8 text-center">
          <Wallet className="mx-auto h-8 w-8 animate-pulse text-muted-foreground" />
          <p className="mt-2 text-sm text-muted-foreground">{t("admin.llmCosts.loading")}</p>
        </div>
      ) : byModel.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-8 text-center">
          <Wallet className="mx-auto h-8 w-8 text-muted-foreground" />
          <p className="mt-2 text-sm text-muted-foreground">{t("admin.llmCosts.noData")}</p>
        </div>
      ) : (
        <>
          <div className="rounded-md border border-border bg-card p-4">
            <h2 className="mb-3 text-sm font-semibold">{t("admin.llmCosts.byDayTitle")}</h2>
            {byDay.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">{t("admin.llmCosts.noData")}</p>
            ) : (
              <div>
                <div className="flex h-32 items-end gap-1">
                  {byDay.map((d, i) => {
                    const pct = barMax > 0 ? Math.max(2, Math.round((barValues[i] / barMax) * 100)) : 2;
                    return (
                      <div
                        key={d.day}
                        className="relative flex h-full flex-1 items-end"
                        title={`${d.day} · ${formatTokens(d.tokens_in)} / ${formatTokens(d.tokens_out)} · ${formatCNY(d.cost_cny)}`}
                      >
                        <div className="w-full rounded-sm bg-primary/70" style={{ height: `${pct}%` }} />
                      </div>
                    );
                  })}
                </div>
                <div className="mt-1 flex justify-between text-[10px] text-muted-foreground">
                  <span>{byDay[0]?.day}</span>
                  <span>{byDay[byDay.length - 1]?.day}</span>
                </div>
              </div>
            )}
          </div>

          <div className="overflow-x-auto rounded-md border border-border bg-card">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs text-muted-foreground">
                  <th className="px-3 py-2 font-medium">{t("admin.llmCosts.colModel")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.llmCosts.colTokensIn")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.llmCosts.colTokensOut")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.llmCosts.colCost")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.llmCosts.colEstimated")}</th>
                </tr>
              </thead>
              <tbody>
                {byModel.map((row) => (
                  <tr key={row.model} className="border-b border-border last:border-0 hover:bg-muted/50">
                    <td className="whitespace-nowrap px-3 py-2 font-mono text-xs">{row.model || "—"}</td>
                    <td className="whitespace-nowrap px-3 py-2 tabular-nums">{formatTokens(row.tokens_in)}</td>
                    <td className="whitespace-nowrap px-3 py-2 tabular-nums">{formatTokens(row.tokens_out)}</td>
                    <td className="whitespace-nowrap px-3 py-2 tabular-nums">{row.estimated ? formatCNY(row.cost_cny) : "—"}</td>
                    <td className="whitespace-nowrap px-3 py-2">
                      {row.estimated ? (
                        <span className="rounded-full bg-emerald-100 px-2 py-0.5 text-xs text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400">
                          {t("admin.llmCosts.estimatedYes")}
                        </span>
                      ) : (
                        <span className="rounded-full bg-amber-100 px-2 py-0.5 text-xs text-amber-700 dark:bg-amber-900/30 dark:text-amber-400">
                          {t("admin.llmCosts.estimatedNo")}
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="overflow-x-auto rounded-md border border-border bg-card">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs text-muted-foreground">
                  <th className="px-3 py-2 font-medium">{t("admin.llmCosts.colConversation")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.llmCosts.colTurns")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.llmCosts.colTokensIn")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.llmCosts.colTokensOut")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.llmCosts.colCost")}</th>
                  <th className="px-3 py-2 font-medium">{t("admin.llmCosts.colEstimated")}</th>
                </tr>
              </thead>
              <tbody>
                {topConversations.map((row) => (
                  <tr key={row.conversation_id} className="border-b border-border last:border-0 hover:bg-muted/50">
                    <td className="whitespace-nowrap px-3 py-2">
                      <Link
                        href={`/admin/traces?conversation_id=${row.conversation_id}`}
                        className="font-mono text-xs text-primary underline-offset-2 hover:underline"
                      >
                        #{row.conversation_id}
                      </Link>
                    </td>
                    <td className="whitespace-nowrap px-3 py-2 tabular-nums">{row.turns}</td>
                    <td className="whitespace-nowrap px-3 py-2 tabular-nums">{formatTokens(row.tokens_in)}</td>
                    <td className="whitespace-nowrap px-3 py-2 tabular-nums">{formatTokens(row.tokens_out)}</td>
                    <td className="whitespace-nowrap px-3 py-2 tabular-nums">{formatCNY(row.cost_cny)}</td>
                    <td className="whitespace-nowrap px-3 py-2 text-xs text-muted-foreground">
                      {row.estimated ? t("admin.llmCosts.estimatedYes") : t("admin.llmCosts.estimatedNo")}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}
