import { cn } from "@/lib/utils";

interface StatsCardProps {
  label: string;
  value: string | number;
  change?: number;
  icon: React.ReactNode;
}

export function StatsCard({ label, value, change, icon }: StatsCardProps) {
  const isPositive = change !== undefined && change >= 0;

  return (
    <div className="rounded-lg border border-border bg-card p-5 transition-colors hover:border-border/80">
      <div className="mb-3 flex items-center justify-between">
        <span className="text-sm text-muted-foreground">{label}</span>
        <span className="text-muted-foreground">{icon}</span>
      </div>
      <div className="text-2xl font-bold text-foreground">{value}</div>
      {change !== undefined && (
        <div
          className={cn(
            "mt-2 text-xs font-medium",
            isPositive ? "text-emerald-500" : "text-rose-500"
          )}
        >
          {isPositive ? "↑" : "↓"} {Math.abs(change)}% 较上月
        </div>
      )}
    </div>
  );
}
