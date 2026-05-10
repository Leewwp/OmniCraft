import Link from "next/link";
import { Eye } from "lucide-react";

interface RankItem {
  id: number;
  title: string;
  viewCount: number;
  zone?: string;
}

interface ContentRankListProps {
  items: RankItem[];
}

export function ContentRankList({ items }: ContentRankListProps) {
  return (
    <div className="rounded-lg border border-border bg-card p-5">
      <h3 className="mb-4 text-sm font-semibold text-foreground">热门内容 Top 5</h3>
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">暂无数据</p>
      ) : (
        <ul className="space-y-3">
          {items.map((item, i) => (
            <li key={item.id}>
              <Link
                href={item.zone === "original" ? `/original/${item.id}` : `/content/${item.id}`}
                className="flex items-center gap-3 rounded-md p-1.5 -mx-1.5 transition-colors hover:bg-muted"
              >
                <span className="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full bg-muted text-xs font-bold text-muted-foreground">
                  {i + 1}
                </span>
                <span className="flex-1 truncate text-sm text-foreground">
                  {item.title}
                </span>
                <span className="flex items-center gap-1 text-xs text-muted-foreground flex-shrink-0">
                  <Eye className="h-3 w-3" />
                  {item.viewCount.toLocaleString()}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
