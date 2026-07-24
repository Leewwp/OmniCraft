"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import Link from "next/link";
import { ArrowLeft, MessageSquare, Paperclip } from "lucide-react";

interface FeedbackReply {
  id: number;
  author_user_id: number | null;
  author_admin_id: number | null;
  body: string;
  is_internal_note: boolean;
  created_at: string;
}

interface FeedbackAttachment {
  id: number;
  oss_key: string;
  file_type: string;
  mime_type: string;
  size_bytes: number;
}

interface FeedbackTicket {
  id: number;
  category: string;
  title: string;
  description: string;
  status: string;
  priority: string;
  diagnostic_summary: Record<string, string>;
  replies: FeedbackReply[];
  attachments: FeedbackAttachment[];
  created_at: string;
  updated_at: string;
}

export default function FeedbackDetailPage() {
  const t = useTranslations();
  const params = useParams();
  const router = useRouter();
  const ticketId = params.feedbackId as string;

  const [ticket, setTicket] = useState<FeedbackTicket | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    async function load() {
      setLoading(true);
      setError("");
      try {
        const res = (await api.get(`/api/v1/feedback/${ticketId}`)) as FeedbackTicket;
        setTicket(res);
      } catch (e) {
        silentError(e, { component: "FeedbackDetailPage", action: "load" });
        setError(t(getUserFacingErrorKey(e)));
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [ticketId, t]);

  if (loading) {
    return (
      <div className="mx-auto w-full max-w-2xl px-4 py-8">
        <p className="text-sm text-muted-foreground">{t("common.loading")}</p>
      </div>
    );
  }

  if (error || !ticket) {
    return (
      <div className="mx-auto w-full max-w-2xl px-4 py-8 text-center">
        <p className="text-sm text-destructive">{error || t("feedback.ticketNotFound")}</p>
        <Link href="/feedback/mine" className="mt-2 inline-block text-sm text-primary hover:underline">
          {t("feedback.backToList")}
        </Link>
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-2xl px-4 py-8">
      <div className="mb-4">
        <Button variant="ghost" size="sm" onClick={() => router.push("/feedback/mine")}>
          <ArrowLeft className="mr-1 h-3.5 w-3.5" />
          {t("feedback.backToList")}
        </Button>
      </div>

      <div className="rounded-lg border border-border bg-card p-4 space-y-3">
        <div className="flex items-center gap-2">
          <span
            className={`inline-block rounded px-1.5 py-0.5 text-[10px] font-medium ${
              ticket.status === "open"
                ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                : ticket.status === "in_progress"
                  ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
                  : ticket.status === "resolved"
                    ? "bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400"
                    : "bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-500"
            }`}
          >
            {t(`feedback.status_${ticket.status}`)}
          </span>
          <span className="text-xs text-muted-foreground">
            {t(`feedback.cat_${ticket.category}`)}
          </span>
          <span className="text-xs text-muted-foreground">#{ticket.id}</span>
        </div>

        <h1 className="text-lg font-semibold">{ticket.title}</h1>
        <p className="whitespace-pre-wrap text-sm text-muted-foreground">{ticket.description}</p>

        {ticket.diagnostic_summary && Object.keys(ticket.diagnostic_summary).length > 0 && (
          <div className="rounded border border-border bg-canvas-subtle p-2">
            <p className="text-[10px] font-medium text-muted-foreground uppercase">{t("feedback.diagnostics")}</p>
            <pre className="mt-1 text-[11px] text-muted-foreground overflow-x-auto">
              {JSON.stringify(ticket.diagnostic_summary, null, 2)}
            </pre>
          </div>
        )}

        {ticket.attachments && ticket.attachments.length > 0 && (
          <div className="space-y-1">
            <p className="text-xs font-medium text-muted-foreground">{t("feedback.attachments")}</p>
            {ticket.attachments.map((att) => (
              <div key={att.id} className="flex items-center gap-1 text-xs text-primary">
                <Paperclip className="h-3 w-3" />
                <span>{att.oss_key.split("/").pop()}</span>
                <span className="text-muted-foreground">({(att.size_bytes / 1024).toFixed(1)} KB)</span>
              </div>
            ))}
          </div>
        )}

        <p className="text-[11px] text-muted-foreground">
          {t("feedback.createdAt", { date: new Date(ticket.created_at).toLocaleString() })}
        </p>
      </div>

      {ticket.replies && ticket.replies.length > 0 && (
        <div className="mt-4 space-y-3">
          <h2 className="text-sm font-semibold">{t("feedback.replies")}</h2>
          {ticket.replies.map((reply) => (
            <div
              key={reply.id}
              className="rounded-lg border border-border bg-card p-3"
            >
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                {reply.author_admin_id ? (
                  <span className="font-medium text-primary">{t("feedback.adminReply")}</span>
                ) : (
                  <span>{t("feedback.yourReply")}</span>
                )}
                <span>{new Date(reply.created_at).toLocaleString()}</span>
              </div>
              <p className="mt-2 whitespace-pre-wrap text-sm">{reply.body}</p>
            </div>
          ))}
        </div>
      )}

      {ticket.status !== "closed" && ticket.status !== "resolved" && (
        <div className="mt-4 rounded-lg border border-border bg-canvas-subtle p-3 text-center">
          <MessageSquare className="mx-auto h-5 w-5 text-muted-foreground" />
          <p className="mt-1 text-xs text-muted-foreground">{t("feedback.replyHint")}</p>
        </div>
      )}
    </div>
  );
}
