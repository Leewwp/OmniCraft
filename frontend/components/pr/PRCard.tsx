import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export interface PRCardData {
  id: number;
  content_item_id: number;
  submitter_id: number;
  base_version_id: number;
  proposed_version_id?: number | null;
  status: string;
  message?: string;
  created_at?: string;
  contentTitle?: string;
}

interface PRCardProps {
  data: PRCardData;
  active?: boolean;
  disabled?: boolean;
  onSelect?: (id: number) => void;
  onAccept?: (id: number) => void;
  onReject?: (id: number) => void;
}

function getStatusTone(status: string): "default" | "secondary" | "destructive" {
  switch (status) {
    case "accepted":
      return "default";
    case "rejected":
      return "destructive";
    default:
      return "secondary";
  }
}

export function PRCard({ data, active, disabled, onSelect, onAccept, onReject }: PRCardProps) {
  return (
    <div
      className={`rounded-md border p-4 shadow-none transition-colors ${
        active ? "border-primary bg-muted/30" : "border-border bg-card"
      }`}
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <p className="text-sm font-semibold">PR #{data.id}</p>
          <p className="text-xs text-muted-foreground">内容：{data.contentTitle || `#${data.content_item_id}`}</p>
          <p className="text-xs text-muted-foreground">提交者：用户 #{data.submitter_id}</p>
          <p className="text-xs text-muted-foreground">
            基础版本：v{data.base_version_id}
            {data.proposed_version_id ? ` · 提案版本：v${data.proposed_version_id}` : ""}
          </p>
        </div>
        <Badge variant={getStatusTone(data.status)}>{data.status}</Badge>
      </div>

      {data.message ? (
        <p className="mt-3 line-clamp-2 text-xs text-foreground/90">{data.message}</p>
      ) : null}

      <div className="mt-4 flex flex-wrap gap-2">
        <Button size="sm" variant="outline" disabled={disabled} onClick={() => onSelect?.(data.id)}>
          查看 Diff
        </Button>
        <Button size="sm" disabled={disabled || data.status !== "open"} onClick={() => onAccept?.(data.id)}>
          接受
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={disabled || data.status !== "open"}
          onClick={() => onReject?.(data.id)}
        >
          拒绝
        </Button>
      </div>
    </div>
  );
}
