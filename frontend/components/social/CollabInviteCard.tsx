"use client";

import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { LoaderCircle } from "lucide-react";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { TagBadge } from "@/components/ui/TagBadge";
import { useToast } from "@/components/ui/Toast";

export type CollabInviteStatus = "pending" | "accepted" | "declined" | "expired";

export interface CollabInviteCardProps {
  invite: {
    id: number;
    status: CollabInviteStatus;
    contentId: number;
    contentTitle: string;
    inviterUsername: string;
  };
  isCurrentUserInvitee: boolean;
  onAccept: (inviteId: number) => Promise<void>;
  onDecline: (inviteId: number) => Promise<void>;
}

const INVITE_STATUSES: readonly CollabInviteStatus[] = ["pending", "accepted", "declined", "expired"];

const STATUS_COLORS: Record<CollabInviteStatus, "blue" | "green" | "rose" | "sky"> = {
  pending: "blue",
  accepted: "green",
  declined: "rose",
  expired: "sky",
};

const STATUS_LABELS: Record<
  CollabInviteStatus,
  "collabInviteCard.status.pending" | "collabInviteCard.status.accepted" | "collabInviteCard.status.declined" | "collabInviteCard.status.expired"
> = {
  pending: "collabInviteCard.status.pending",
  accepted: "collabInviteCard.status.accepted",
  declined: "collabInviteCard.status.declined",
  expired: "collabInviteCard.status.expired",
};

export function CollabInviteCard({ invite, isCurrentUserInvitee, onAccept, onDecline }: CollabInviteCardProps) {
  const t = useTranslations();
  const { toast } = useToast();
  const [status, setStatus] = useState<CollabInviteStatus>(invite.status);
  const [busy, setBusy] = useState<"accept" | "decline" | null>(null);
  const [error, setError] = useState("");
  const titleId = `collab-invite-${invite.id}-title`;

  const statusLabel =
    status === "pending" && !isCurrentUserInvitee
      ? t("collabInviteCard.status.pendingSender")
      : t(STATUS_LABELS[status]);

  async function respond(action: "accept" | "decline") {
    if (busy) return;
    setBusy(action);
    setError("");
    try {
      const data = await api.post<{ invite?: { status?: string } }>(`/api/v1/collab-invites/${invite.id}/${action}`, {});
      if (data.invite?.status && (INVITE_STATUSES as readonly string[]).includes(data.invite.status)) {
        setStatus(data.invite.status as CollabInviteStatus);
      }
      if (action === "accept") {
        await onAccept(invite.id);
      } else {
        await onDecline(invite.id);
      }
    } catch (e) {
      silentError(e, { component: "CollabInviteCard", action });
      const messageKey = getUserFacingErrorKey(e, "collabInviteCard.errors.failed");
      setError(t(messageKey));
      toast("error", t(messageKey));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div
      role="group"
      aria-labelledby={titleId}
      aria-busy={busy ? "true" : "false"}
      className={cn(
        "max-w-full rounded-lg border border-border-default bg-canvas-default p-4 shadow-none md:max-w-[min(420px,80%)] lg:max-w-[420px]",
        status === "expired" && "bg-canvas-subtle",
      )}
    >
      <h3 id={titleId} className="sr-only">
        {t("collabInviteCard.a11y.title", { title: invite.contentTitle })}
      </h3>
      <p className="text-xs font-medium text-fg-muted">{t("collabInviteCard.type")}</p>
      <p className="mt-1 text-sm text-fg-default">
        {t("collabInviteCard.invitation", { inviter: invite.inviterUsername, title: invite.contentTitle })}
      </p>
      <Link
        href={`/content/${invite.contentId}`}
        className="mt-1 inline-block rounded-sm text-sm text-accent-emphasis underline underline-offset-2 focus:outline-none focus:ring-2 focus:ring-accent-emphasis hover:text-accent-hover"
      >
        {invite.contentTitle}
      </Link>
      <div className="mt-2">
        <TagBadge color={STATUS_COLORS[status]} className={status === "expired" ? "bg-canvas-subtle text-fg-muted" : undefined}>
          {statusLabel}
        </TagBadge>
      </div>
      {error && (
        <p className="mt-2 text-xs text-destructive" role="alert">
          {error}
        </p>
      )}
      {isCurrentUserInvitee && status === "pending" && (
        <div className="mt-3 flex flex-col gap-2 sm:flex-row">
          <Button
            size="sm"
            variant="default"
            className="min-h-11 md:min-h-8"
            disabled={busy !== null}
            onClick={() => void respond("accept")}
            aria-label={t("collabInviteCard.a11y.accept", { title: invite.contentTitle })}
          >
            {busy === "accept" && <LoaderCircle className="h-4 w-4 animate-spin" aria-hidden="true" />}
            {t("collabInviteCard.actions.accept")}
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="min-h-11 md:min-h-8"
            disabled={busy !== null}
            onClick={() => void respond("decline")}
            aria-label={t("collabInviteCard.a11y.decline", { title: invite.contentTitle })}
          >
            {busy === "decline" && <LoaderCircle className="h-4 w-4 animate-spin" aria-hidden="true" />}
            {t("collabInviteCard.actions.decline")}
          </Button>
        </div>
      )}
    </div>
  );
}
