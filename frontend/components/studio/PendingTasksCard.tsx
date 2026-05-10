import Link from "next/link";
import { GitPullRequest, Tags } from "lucide-react";

interface PendingItem {
  type: "pr" | "tag";
  id: number;
  title: string;
}

interface PendingTasksCardProps {
  items: PendingItem[];
}

export function PendingTasksCard({ items }: PendingTasksCardProps) {
  return (
    <div className="rounded-lg border border-border bg-card p-5">
      <h3 className="mb-4 text-sm font-semibold text-foreground">待处理事项</h3>
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">暂无待处理事项 🎉</p>
      ) : (
        <ul className="space-y-2">
          {items.map((item) => (
            <li key={`${item.type}-${item.id}`}>
              <Link
                href={item.type === "pr" ? `/studio/pr-requests` : `/studio/tag-suggestions`}
                className="flex items-center gap-2 rounded-md p-1.5 -mx-1.5 text-sm transition-colors hover:bg-muted"
              >
                {item.type === "pr" ? (
                  <GitPullRequest className="h-4 w-4 text-emerald-500 flex-shrink-0" />
                ) : (
                  <Tags className="h-4 w-4 text-amber-500 flex-shrink-0" />
                )}
                <span className="truncate text-foreground">{item.title}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
