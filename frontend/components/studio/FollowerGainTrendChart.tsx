"use client";

import { useTranslations } from "next-intl";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";

// 工作台概览的趋势图：数据源 = /users/me/followers/stats 的 daily[].count
// （每日新增粉丝数），标题与数据语义一致（SP-25 中-10/D2——不再把涨粉数
// 标注为访问量）。粉丝分析页的双线图见 FollowerTrendChart。
interface FollowerGainTrendChartProps {
  data: Array<{ date: string; count: number }>;
}

export function FollowerGainTrendChart({ data }: FollowerGainTrendChartProps) {
  const t = useTranslations("studio");

  if (data.length === 0) {
    return (
      <div className="flex h-64 items-center justify-center rounded-lg border border-border bg-card text-sm text-muted-foreground">
        {t("chart.noFollowerGainData")}
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-3 text-sm font-medium text-foreground">
        {t("chart.followerGainTrendTitle")}
      </h3>
      <ResponsiveContainer width="100%" height={240}>
        <LineChart data={data} margin={{ top: 4, right: 8, bottom: 4, left: 0 }}>
          <CartesianGrid strokeDasharray="3 3" className="stroke-border/50" />
          <XAxis
            dataKey="date"
            tick={{ fontSize: 11 }}
            className="text-xs text-muted-foreground"
            tickLine={false}
            axisLine={false}
          />
          <YAxis
            tick={{ fontSize: 11 }}
            className="text-xs text-muted-foreground"
            tickLine={false}
            axisLine={false}
            width={40}
            allowDecimals={false}
          />
          <Tooltip
            contentStyle={{
              borderRadius: "6px",
              border: "1px solid var(--border)",
              backgroundColor: "var(--card)",
              fontSize: "12px",
            }}
          />
          <Line
            type="monotone"
            dataKey="count"
            stroke="var(--chart-1)"
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4, fill: "var(--chart-1)" }}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
