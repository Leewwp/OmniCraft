"use client";

import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { buttonVariants } from "@/components/ui/button";
import { Plus, RefreshCcw } from "lucide-react";

// T52 (FIX-23b): the creator-facing "my IPs" listing. Every status is shown
// with a badge; rejected rows carry the latest review reason (T16) and a
// resubmit shortcut that reuses the existing create flow (state machine has
// no reverse transition, so resubmission = new submission).

interface MyIPItem {
  id: number;
  name: string;
  slug: string;
  category: string;
  status: "pending" | "approved" | "rejected" | string;
  review_reason?: string;
  created_at: string;
}

const STATUS_BADGE: Record<string, string> = {
  pending: "bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300",
  approved: "bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300",
  rejected: "bg-red-50 text-red-600 dark:bg-red-950/40 dark:text-red-300",
};

export default function StudioIPsPage() {
  const t = useTranslations();
  const { user } = useAuth();
  const [ips, setIps] = useState<MyIPItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const loadIPs = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await api.get<{ ips: MyIPItem[] }>(
        "/api/v1/users/me/ips?page=1&page_size=50"
      );
      setIps(data.ips || []);
    } catch (e) {
      silentError(e, { component: "StudioIPsPage", action: "loadIPs" });
      setError(t(getUserFacingErrorKey(e, "studio.myIPs.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    if (!user) return;
    loadIPs();
  }, [user, loadIPs]);

  function statusLabel(status: string) {
    if (status === "approved") return t("studio.myIPs.statusApproved");
    if (status === "rejected") return t("studio.myIPs.statusRejected");
    return t("studio.myIPs.statusPending");
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <div className="space-y-3 rounded-md border border-border bg-card p-6">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-8 w-full animate-pulse rounded bg-muted" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("studio.myIPs.title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("studio.myIPs.subtitle")}</p>
        </div>
        <Link
          href="/studio/publish/ip"
          className={buttonVariants({
            size: "sm",
            className: "[@media(pointer:coarse)]:min-h-11",
          })}
        >
          <Plus className="h-4 w-4" aria-hidden />
          {t("studio.myIPs.create")}
        </Link>
      </div>

      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}

      {ips.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center">
          <p className="text-sm text-muted-foreground">{t("studio.myIPs.empty")}</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border bg-muted/40">
              <tr>
                <th className="px-4 py-2.5 text-xs font-medium text-muted-foreground">
                  {t("studio.myIPs.colName")}
                </th>
                <th className="px-4 py-2.5 text-xs font-medium text-muted-foreground">
                  {t("studio.myIPs.colCategory")}
                </th>
                <th className="px-4 py-2.5 text-xs font-medium text-muted-foreground">
                  {t("studio.myIPs.colStatus")}
                </th>
                <th className="px-4 py-2.5 text-xs font-medium text-muted-foreground">
                  {t("studio.myIPs.colReason")}
                </th>
                <th className="px-4 py-2.5 text-xs font-medium text-muted-foreground">
                  {t("studio.myIPs.colDate")}
                </th>
                <th className="px-4 py-2.5 text-right text-xs font-medium text-muted-foreground">
                  {t("studio.myIPs.colActions")}
                </th>
              </tr>
            </thead>
            <tbody>
              {ips.map((ip) => (
                <tr key={ip.id} className="border-b border-border last:border-0">
                  <td className="px-4 py-2.5 font-medium">{ip.name}</td>
                  <td className="px-4 py-2.5 text-xs text-muted-foreground">{ip.category || "-"}</td>
                  <td className="px-4 py-2.5">
                    <span
                      className={`rounded-full px-2 py-0.5 text-xs ${
                        STATUS_BADGE[ip.status] ?? STATUS_BADGE.pending
                      }`}
                    >
                      {statusLabel(ip.status)}
                    </span>
                  </td>
                  <td className="max-w-64 px-4 py-2.5 text-xs text-muted-foreground">
                    {ip.status === "rejected" && ip.review_reason ? (
                      ip.review_reason
                    ) : (
                      t("studio.myIPs.noReason")
                    )}
                  </td>
                  <td className="px-4 py-2.5 text-xs text-muted-foreground">
                    {new Date(ip.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-2.5 text-right">
                    {ip.status === "rejected" ? (
                      <Link
                        href="/studio/publish/ip"
                        className={buttonVariants({
                          size: "sm",
                          variant: "outline",
                          className: "[@media(pointer:coarse)]:min-h-11",
                        })}
                      >
                        <RefreshCcw className="h-3.5 w-3.5" aria-hidden />
                        {t("studio.myIPs.resubmit")}
                      </Link>
                    ) : ip.status === "approved" ? (
                      <Link
                        href={`/ip/${ip.id}`}
                        className={buttonVariants({
                          size: "sm",
                          variant: "ghost",
                          className: "[@media(pointer:coarse)]:min-h-11",
                        })}
                      >
                        {t("studio.myIPs.view")}
                      </Link>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
