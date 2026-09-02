"use client";

import { FormEvent, useEffect, useRef, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { MessageSquare, Eye, ArrowLeft, Lock } from "lucide-react";

interface FeedbackTicket {
  id: number;
  user_id: number | null;
  contact_email: string;
  category: string;
  title: string;
  description: string;
  diagnostic_summary: Record<string, unknown>;
  status: string;
  priority: string;
  assignee_admin_id: number | null;
  replies: FeedbackReply[];
  attachments: FeedbackAttachment[];
  created_at: string;
  updated_at: string;
  resolved_at: string | null;
}

interface FeedbackReply {
  id: number;
  ticket_id: number;
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

const STATUS_COLORS: Record<string, string> = {
  open: "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
  in_progress: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
  closed: "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400",
  reopened: "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400",
};

const PRIORITY_COLORS: Record<string, string> = {
  low: "bg-muted text-muted-foreground",
  normal: "bg-muted text-foreground",
  high: "bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400",
  urgent: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
};

export default function AdminFeedbackPage() {
  const t = useTranslations();
  const [tickets, setTickets] = useState<FeedbackTicket[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [categoryFilter, setCategoryFilter] = useState("");
  const [selectedTicket, setSelectedTicket] = useState<FeedbackTicket | null>(null);
  const [replyBody, setReplyBody] = useState("");
  const [isInternalNote, setIsInternalNote] = useState(false);
  const [replyBusy, setReplyBusy] = useState(false);
  const [replyAttempted, setReplyAttempted] = useState(false);
  const [replyError, setReplyError] = useState("");
  const replyInputRef = useRef<HTMLTextAreaElement>(null);
  const [patchBusy, setPatchBusy] = useState(false);

  const pageSize = 20;

  const loadTickets = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const params = new URLSearchParams();
      params.set("page", String(page));
      params.set("page_size", String(pageSize));
      if (statusFilter) params.set("status", statusFilter);
      if (categoryFilter) params.set("category", categoryFilter);
      const data = await api.get<{ items: FeedbackTicket[]; total: number }>(
        `/api/v1/admin/feedback?${params.toString()}`
      );
      setTickets(data.items || []);
      setTotal(data.total || 0);
    } catch (e) {
      silentError(e, { component: "AdminFeedbackPage", action: "loadTickets" });
      setError(t(getUserFacingErrorKey(e, "admin.feedback.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [page, statusFilter, categoryFilter, t]);

  useEffect(() => {
    void loadTickets();
  }, [loadTickets]);

  async function loadTicketDetail(id: number) {
    try {
      const data = await api.get<FeedbackTicket>(`/api/v1/admin/feedback/${id}`);
      setSelectedTicket(data);
    } catch (e) {
      silentError(e, { component: "AdminFeedbackPage", action: "loadTicketDetail" });
    }
  }

  async function handleReply(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setReplyAttempted(true);
    if (!selectedTicket || !replyBody.trim()) {
      replyInputRef.current?.focus();
      return;
    }
    setReplyBusy(true);
    setReplyError("");
    try {
      await api.post(`/api/v1/admin/feedback/${selectedTicket.id}/replies`, {
        body: replyBody.trim(),
        is_internal_note: isInternalNote,
      });
      setReplyBody("");
      setReplyAttempted(false);
      setIsInternalNote(false);
      await loadTicketDetail(selectedTicket.id);
    } catch (e) {
      silentError(e, { component: "AdminFeedbackPage", action: "handleReply" });
      setReplyError(t("admin.feedback.replyFailed"));
    } finally {
      setReplyBusy(false);
    }
  }

  async function handlePatchStatus(status: string) {
    if (!selectedTicket) return;
    setPatchBusy(true);
    try {
      await api.patch(`/api/v1/admin/feedback/${selectedTicket.id}`, { status });
      await loadTicketDetail(selectedTicket.id);
      await loadTickets();
    } catch (e) {
      silentError(e, { component: "AdminFeedbackPage", action: "handlePatchStatus" });
    } finally {
      setPatchBusy(false);
    }
  }

  async function handlePatchPriority(priority: string) {
    if (!selectedTicket) return;
    setPatchBusy(true);
    try {
      await api.patch(`/api/v1/admin/feedback/${selectedTicket.id}`, { priority });
      await loadTicketDetail(selectedTicket.id);
      await loadTickets();
    } catch (e) {
      silentError(e, { component: "AdminFeedbackPage", action: "handlePatchPriority" });
    } finally {
      setPatchBusy(false);
    }
  }

  const totalPages = Math.ceil(total / pageSize);

  if (loading && tickets.length === 0) {
    return (
      <div className="space-y-4 p-6">
        <div className="space-y-3 rounded-md border border-border bg-card p-6">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-8 w-full animate-pulse rounded bg-muted" />
          ))}
        </div>
      </div>
    );
  }

  if (selectedTicket) {
    return (
      <div className="space-y-4 p-6">
        <button
          type="button"
          className="inline-flex min-h-11 items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
          onClick={() => setSelectedTicket(null)}
        >
          <ArrowLeft className="h-4 w-4" />
          {t("common.back")}
        </button>

        <div className="rounded-md border border-border bg-card p-4">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <h2 className="text-lg font-semibold">{selectedTicket.title}</h2>
              <p className="mt-1 text-xs text-muted-foreground">
                #{selectedTicket.id} · {selectedTicket.category} · {new Date(selectedTicket.created_at).toLocaleString()}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <span className={cn("inline-flex rounded-full px-2 py-0.5 text-xs font-medium", STATUS_COLORS[selectedTicket.status] || STATUS_COLORS.open)}>
                {selectedTicket.status}
              </span>
              <span className={cn("inline-flex rounded-full px-2 py-0.5 text-xs font-medium", PRIORITY_COLORS[selectedTicket.priority] || PRIORITY_COLORS.normal)}>
                {selectedTicket.priority}
              </span>
            </div>
          </div>

          <div className="mt-4 rounded-md border border-border bg-background p-3">
            <p className="whitespace-pre-wrap text-sm">{selectedTicket.description}</p>
          </div>

          {selectedTicket.contact_email && (
            <p className="mt-3 text-xs text-muted-foreground">
              {t("admin.feedback.contactEmail")}: {selectedTicket.contact_email}
            </p>
          )}

          {selectedTicket.diagnostic_summary && Object.keys(selectedTicket.diagnostic_summary).length > 0 && (
            <div className="mt-3">
              <p className="text-xs font-medium text-muted-foreground">{t("admin.feedback.diagnostics")}</p>
              <pre className="mt-1 rounded-md border border-border bg-background p-2 text-xs overflow-x-auto">
                {JSON.stringify(selectedTicket.diagnostic_summary, null, 2)}
              </pre>
            </div>
          )}

          {selectedTicket.attachments && selectedTicket.attachments.length > 0 && (
            <div className="mt-3">
              <p className="text-xs font-medium text-muted-foreground">{t("admin.feedback.screenshots")}</p>
              <div className="mt-1 flex gap-2">
                {selectedTicket.attachments.map((att) => (
                  <div key={att.id} className="rounded-md border border-border p-2 text-xs text-muted-foreground">
                    {att.mime_type} ({(att.size_bytes / 1024).toFixed(1)} KB)
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        <div className="flex flex-wrap gap-2">
          <div className="w-fit min-w-40">
            <Select
              aria-label={t("admin.feedback.status")}
              className="px-3 py-1.5 text-sm"
              value={selectedTicket.status}
              onChange={(e) => handlePatchStatus(e.target.value)}
              disabled={patchBusy}
            >
              <option value="open">open</option>
              <option value="in_progress">in_progress</option>
              <option value="closed">closed</option>
              <option value="reopened">reopened</option>
            </Select>
          </div>
          <div className="w-fit min-w-40">
            <Select
              aria-label={t("admin.feedback.priority")}
              className="px-3 py-1.5 text-sm"
              value={selectedTicket.priority}
              onChange={(e) => handlePatchPriority(e.target.value)}
              disabled={patchBusy}
            >
              <option value="low">low</option>
              <option value="normal">normal</option>
              <option value="high">high</option>
              <option value="urgent">urgent</option>
            </Select>
          </div>
        </div>

        <div className="space-y-3">
          <h3 className="text-sm font-semibold">{t("admin.feedback.replies")}</h3>
          {selectedTicket.replies && selectedTicket.replies.length > 0 ? (
            selectedTicket.replies.map((reply) => (
              <div
                key={reply.id}
                className={cn(
                  "rounded-md border p-3",
                  reply.is_internal_note ? "border-amber-300 bg-amber-50 dark:border-amber-700 dark:bg-amber-900/20" : "border-border bg-card"
                )}
              >
                <div className="flex items-center gap-2">
                  <span className="text-xs font-medium">
                    {reply.author_admin_id ? t("admin.feedback.admin") : t("admin.feedback.user")}
                  </span>
                  {reply.is_internal_note && (
                    <span className="inline-flex items-center gap-1 rounded-full bg-amber-200 px-1.5 py-0.5 text-[10px] font-medium text-amber-800 dark:bg-amber-800 dark:text-amber-200">
                      <Lock className="h-2.5 w-2.5" />
                      {t("admin.feedback.internalNote")}
                    </span>
                  )}
                  <span className="text-xs text-muted-foreground">{new Date(reply.created_at).toLocaleString()}</span>
                </div>
                <p className="mt-1 whitespace-pre-wrap text-sm">{reply.body}</p>
              </div>
            ))
          ) : (
            <p className="text-sm text-muted-foreground">{t("admin.feedback.noReplies")}</p>
          )}
        </div>

        <form className="rounded-md border border-border bg-card p-4" onSubmit={handleReply} noValidate>
          <label htmlFor="feedback-reply" className="sr-only">
            {t("admin.feedback.replyLabel")}
          </label>
          <textarea
            id="feedback-reply"
            ref={replyInputRef}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent aria-invalid:border-destructive aria-invalid:ring-2 aria-invalid:ring-destructive/20"
            rows={3}
            value={replyBody}
            onChange={(e) => {
              setReplyBody(e.target.value);
              if (replyError) setReplyError("");
            }}
            placeholder={t("admin.feedback.replyPlaceholder")}
            aria-invalid={Boolean(replyError || (replyAttempted && !replyBody.trim()))}
            aria-describedby={replyError || (replyAttempted && !replyBody.trim()) ? "feedback-reply-error" : undefined}
          />
          {(replyError || (replyAttempted && !replyBody.trim())) && (
            <p id="feedback-reply-error" role="alert" className="mt-2 text-sm text-destructive">
              {replyError || t("admin.feedback.replyRequired")}
            </p>
          )}
          <div className="mt-2 flex items-center justify-between">
            <label className="inline-flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={isInternalNote}
                onChange={(e) => setIsInternalNote(e.target.checked)}
                className="rounded border-border"
              />
              {t("admin.feedback.markInternalNote")}
            </label>
            <Button type="submit" size="sm" className="[@media(pointer:coarse)]:min-h-11" disabled={replyBusy}>
              {replyBusy ? t("common.saving") : t("admin.feedback.sendReply")}
            </Button>
          </div>
        </form>
      </div>
    );
  }

  return (
    <div className="space-y-4 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight">{t("admin.feedback.title")}</h1>
      </div>

      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}

      <div className="flex items-center gap-3">
        <div className="w-fit min-w-40">
          <Select
            aria-label={t("admin.feedback.allStatuses")}
            className="px-3 py-1.5 text-sm"
            value={statusFilter}
            onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
          >
            <option value="">{t("admin.feedback.allStatuses")}</option>
            <option value="open">open</option>
            <option value="in_progress">in_progress</option>
            <option value="closed">closed</option>
            <option value="reopened">reopened</option>
          </Select>
        </div>
        <div className="w-fit min-w-40">
          <Select
            aria-label={t("admin.feedback.allCategories")}
            className="px-3 py-1.5 text-sm"
            value={categoryFilter}
            onChange={(e) => { setCategoryFilter(e.target.value); setPage(1); }}
          >
            <option value="">{t("admin.feedback.allCategories")}</option>
            <option value="web_bug">web_bug</option>
            <option value="desktop_deploy">desktop_deploy</option>
            <option value="content_or_community">content_or_community</option>
            <option value="account_or_security">account_or_security</option>
            <option value="agent_quality">agent_quality</option>
            <option value="feature_request">feature_request</option>
            <option value="other">other</option>
          </Select>
        </div>
      </div>

      {tickets.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-8 text-center">
          <MessageSquare className="mx-auto h-8 w-8 text-muted-foreground" />
          <p className="mt-2 text-sm text-muted-foreground">{t("admin.feedback.noTickets")}</p>
        </div>
      ) : (
        <div className="space-y-2">
          {tickets.map((ticket) => (
            <button
              key={ticket.id}
              type="button"
              className="min-h-11 w-full rounded-md border border-border bg-card p-3 text-left transition-colors hover:bg-muted/50"
              onClick={() => loadTicketDetail(ticket.id)}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate">{ticket.title}</p>
                  <p className="mt-0.5 text-xs text-muted-foreground">
                    #{ticket.id} · {ticket.category} · {new Date(ticket.created_at).toLocaleDateString()}
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-1.5">
                  <span className={cn("inline-flex rounded-full px-1.5 py-0.5 text-[10px] font-medium", STATUS_COLORS[ticket.status] || STATUS_COLORS.open)}>
                    {ticket.status}
                  </span>
                  <span className={cn("inline-flex rounded-full px-1.5 py-0.5 text-[10px] font-medium", PRIORITY_COLORS[ticket.priority] || PRIORITY_COLORS.normal)}>
                    {ticket.priority}
                  </span>
                  <Eye className="h-3.5 w-3.5 text-muted-foreground" />
                </div>
              </div>
            </button>
          ))}
        </div>
      )}

      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2 pt-2">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
            {t("common.previous")}
          </Button>
          <span className="text-sm text-muted-foreground">{page} / {totalPages}</span>
          <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
            {t("common.next")}
          </Button>
        </div>
      )}
    </div>
  );
}
