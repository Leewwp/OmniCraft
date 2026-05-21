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
  Legend,
} from "recharts";

interface FollowerTrendChartProps {
  data: Array<{ date: string; newFollowers: number; netGrowth: number }>;
}

export function FollowerTrendChart({ data }: FollowerTrendChartProps) {
  const t = useTranslations("studio");

  if (data.length === 0) {
    return (
      <div className="flex h-64 items-center justify-center rounded-lg border border-border bg-card text-sm text-muted-foreground">
        {t("followers.noTrendData")}
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-3 text-sm font-medium text-foreground">
        {t("followers.trendTitle")}
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
            width={36}
          />
          <Tooltip
            contentStyle={{
              borderRadius: "6px",
              border: "1px solid var(--border)",
              backgroundColor: "var(--card)",
              fontSize: "12px",
            }}
          />
          <Legend
            wrapperStyle={{ fontSize: "12px" }}
          />
          <Line
            type="monotone"
            dataKey="newFollowers"
            name={t("followers.newFollowers")}
            stroke="var(--chart-1)"
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4, fill: "var(--chart-1)" }}
          />
          <Line
            type="monotone"
            dataKey="netGrowth"
            name={t("followers.netFollowers")}
            stroke="var(--chart-2)"
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4, fill: "var(--chart-2)" }}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
