"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Shield, ShieldAlert, ShieldCheck, Loader2, ChevronDown, ChevronUp } from "lucide-react";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { cn } from "@/lib/utils";

interface ComplianceResult {
  risk_level: "safe" | "warning" | "violation";
  reason?: string;
  suggestions?: string[];
  details?: Record<string, unknown>;
}

interface AgentComplianceCheckBadgeProps {
  contentId?: number;
  title?: string;
  description?: string;
  contentType?: string;
  className?: string;
  onResult?: (result: ComplianceResult) => void;
}

export function AgentComplianceCheckBadge({
  contentId,
  title,
  description,
  contentType,
  className,
  onResult,
}: AgentComplianceCheckBadgeProps) {
  const t = useTranslations();
  const [result, setResult] = useState<ComplianceResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [expanded, setExpanded] = useState(false);

  async function check() {
    setLoading(true);
    setError("");
    try {
      const res = await api.post<ComplianceResult>("/api/v1/agent/compliance-check", {
        title: title || "",
        description: description || "",
        content_type: contentType || "other",
      });
      setResult(res);
      onResult?.(res);
    } catch (e) {
      setError((e as Error).message || "Check failed");
      silentError(e, { component: 'ComplianceCheckBadge', action: 'check' });
    } finally {
      setLoading(false);
    }
  }

  const config = {
    safe: {
      icon: ShieldCheck,
      bg: "bg-emerald-100 dark:bg-emerald-900/30",
      text: "text-emerald-700 dark:text-emerald-400",
      label: t("agent.complianceSafe"),
    },
    warning: {
      icon: Shield,
      bg: "bg-amber-100 dark:bg-amber-900/30",
      text: "text-amber-700 dark:text-amber-400",
      label: t("agent.complianceWarning"),
    },
    violation: {
      icon: ShieldAlert,
      bg: "bg-red-100 dark:bg-red-900/30",
      text: "text-red-700 dark:text-red-400",
      label: t("agent.complianceViolation"),
    },
  };

  return (
    <div className={cn("space-y-2", className)}>
      <div className="flex items-center gap-2">
        {result ? (
          <button
            type="button"
            className={cn(
              "inline-flex items-center gap-1.5 rounded-md border border-border px-2.5 py-1 text-xs font-medium",
              config[result.risk_level].bg,
              config[result.risk_level].text,
            )}
            onClick={() => setExpanded(!expanded)}
          >
            {(() => {
              const Icon = config[result.risk_level].icon;
              return <Icon className="h-3.5 w-3.5" />;
            })()}
            {config[result.risk_level].label}
            {expanded ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
          </button>
        ) : (
          <button
            type="button"
            className="inline-flex items-center gap-1.5 rounded-md border border-border px-2.5 py-1 text-xs font-medium text-muted-foreground hover:bg-muted/30"
            onClick={check}
            disabled={loading}
          >
            {loading ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Shield className="h-3.5 w-3.5" />
            )}
            {t("agent.complianceCheck")}
          </button>
        )}
      </div>

      {error && <p className="text-xs text-destructive">{error}</p>}

      {expanded && result && (
        <div className="rounded-md border border-border bg-card p-3 text-sm space-y-1">
          {result.reason && (
            <p className="text-xs text-muted-foreground">{result.reason}</p>
          )}
          {result.suggestions && result.suggestions.length > 0 && (
            <ul className="list-disc pl-4 text-xs text-muted-foreground">
              {result.suggestions.map((s, i) => (
                <li key={i}>{s}</li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
