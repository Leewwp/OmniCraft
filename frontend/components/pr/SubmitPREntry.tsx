"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";

interface SubmitPREntryProps {
  contentId: number;
  authorId?: number;
  allowCopy?: boolean;
  zone?: string;
}

export function SubmitPREntry({ contentId, authorId, allowCopy, zone }: SubmitPREntryProps) {
  const t = useTranslations();
  const { user } = useAuth();

  if (zone !== "fanwork" || !allowCopy) {
    return null;
  }

  if (!user || user.id === authorId) {
    return null;
  }

  return (
    <Link
      href={`/dashboard/pr-requests?content_id=${contentId}&create=1`}
      className="inline-flex items-center rounded-md border border-border px-3 py-2 text-xs hover:bg-muted"
    >
      {t('pr.submit')}
    </Link>
  );
}
