"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { Send } from "lucide-react";
import { cn } from "@/lib/utils";

interface Reply {
  id: number;
  author?: { id?: number; username?: string };
  body: string;
  parent_id?: number | null;
  created_at?: string;
}

interface ReplyListProps {
  discussionId: number;
  replies: Reply[];
  onRefresh?: () => void;
  className?: string;
}

export function ReplyList({ discussionId, replies, onRefresh, className }: ReplyListProps) {
  const t = useTranslations();
  const { user } = useAuth();
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit() {
    if (!body.trim()) return;
    setBusy(true);
    setError("");
    try {
      await api.post(`/api/v1/discussions/${discussionId}/comments`, { content: body.trim() });
      setBody("");
      onRefresh?.();
    } catch (e) {
      setError(t(getUserFacingErrorKey(e)));
      silentError(e, { component: 'ReplyList', action: 'handleSubmit' });
    } finally {
      setBusy(false);
    }
  }

  function renderReplies(parentId: number | null, depth: number): React.ReactNode {
    return replies
      .filter((r) => (r.parent_id ?? null) === parentId)
      .map((r) => (
        <div key={r.id} className={cn(depth > 0 && "ml-6 border-l-2 border-border pl-4")}>
          <div className="rounded-md bg-muted/20 p-3">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <span className="font-medium text-foreground">{r.author?.username ?? `#${r.author?.id}`}</span>
              {r.created_at && <span>{new Date(r.created_at).toLocaleDateString()}</span>}
            </div>
            <p className="mt-1 text-sm">{r.body}</p>
          </div>
          {depth < 2 && renderReplies(r.id, depth + 1)}
        </div>
      ));
  }

  return (
    <div className={cn("space-y-3", className)}>
      {replies.length === 0 ? (
        <p className="text-sm text-muted-foreground text-center py-6">{t("discussion.noReplies")}</p>
      ) : (
        <div className="space-y-2">{renderReplies(null, 0)}</div>
      )}

      {user && (
        <div className="flex gap-2">
          <input
            type="text"
            value={body}
            onChange={(e) => setBody(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") handleSubmit(); }}
            placeholder={t("discussion.replyPlaceholder")}
            className="flex-1 rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
          />
          <Button size="sm" onClick={handleSubmit} disabled={busy || !body.trim()}>
            <Send className="h-4 w-4" />
          </Button>
        </div>
      )}

      {error && <p className="text-xs text-destructive">{error}</p>}
    </div>
  );
}
