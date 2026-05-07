"use client";

import { useTranslations } from "next-intl";
import { ShieldCheck, ShieldAlert } from "lucide-react";

interface ComplianceCheckBadgeProps {
  hasCover: boolean;
  hasAttachment: boolean;
  hasTitle: boolean;
}

export function ComplianceCheckBadge({
  hasCover,
  hasAttachment,
  hasTitle,
}: ComplianceCheckBadgeProps) {
  const t = useTranslations();
  const ready = hasTitle && hasCover && hasAttachment;

  return (
    <div
      className={`inline-flex items-center gap-2 rounded-md border px-2 py-1 text-xs ${
        ready
          ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-700"
          : "border-amber-500/40 bg-amber-500/10 text-amber-700"
      }`}
    >
      {ready ? <ShieldCheck className="h-3.5 w-3.5" /> : <ShieldAlert className="h-3.5 w-3.5" />}
      {ready ? t('content.compliancePassed') : t('content.complianceFailed')}
    </div>
  );
}
