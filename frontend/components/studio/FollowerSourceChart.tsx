"use client";

import { useTranslations } from "next-intl";
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer, Legend } from "recharts";

interface FollowerSourceChartProps {
  data: Array<{ name: string; value: number }>;
}

const CHART_VARS = [
  "var(--chart-1)",
  "var(--chart-2)",
  "var(--chart-3)",
  "var(--chart-4)",
  "var(--chart-5)",
  "var(--chart-6)",
];

export function FollowerSourceChart({ data }: FollowerSourceChartProps) {
  const t = useTranslations("studio");

  if (data.length === 0) {
    return (
      <div className="flex h-64 items-center justify-center rounded-lg border border-border bg-card text-sm text-muted-foreground">
        {t("followers.noSourceData")}
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-3 text-sm font-medium text-foreground">
        {t("followers.sourceTitle")}
      </h3>
      <ResponsiveContainer width="100%" height={240}>
        <PieChart margin={{ top: 0, right: 0, bottom: 0, left: 0 }}>
          <Pie
            data={data}
            cx="50%"
            cy="50%"
            innerRadius={50}
            outerRadius={80}
            paddingAngle={2}
            dataKey="value"
          >
            {data.map((_, index) => (
              <Cell
                key={`cell-${index}`}
                fill={CHART_VARS[index % CHART_VARS.length]}
              />
            ))}
          </Pie>
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
        </PieChart>
      </ResponsiveContainer>
    </div>
  );
}
