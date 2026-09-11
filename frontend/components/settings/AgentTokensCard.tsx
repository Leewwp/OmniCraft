"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { Input } from "@/components/ui/input";
import { TagBadge } from "@/components/ui/TagBadge";
import { useToast } from "@/components/ui/Toast";
import { LoaderCircle } from "lucide-react";

// SP-16 #450 (spec D2): PAT management card on the settings page. The token
// plaintext exists only in the one-time panel returned by a successful
// create; everything else (list rows, confirm dialogs) works from the
// 12-char prefix. Revocation requires a confirm step.

interface AgentTokenInfo {
  id: number;
  name: string;
  token_prefix: string;
  scopes: string[];
  last_used_at: string | null;
  created_at: string;
}

interface IssuedToken {
  token: string;
  token_info: AgentTokenInfo;
}

function formatLastUsed(value: string | null, neverLabel: string): string {
  if (!value) return neverLabel;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? neverLabel : date.toLocaleString();
}

export default function AgentTokensCard() {
  const t = useTranslations();
  const { toast } = useToast();

  const [tokens, setTokens] = useState<AgentTokenInfo[]>([]);
  const [loading, setLoading] = useState(true);

  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [scopeDownload, setScopeDownload] = useState(true);
  const [scopeUpload, setScopeUpload] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState("");

  const [issued, setIssued] = useState<IssuedToken | null>(null);
  const [copied, setCopied] = useState(false);

  const [revokeTarget, setRevokeTarget] = useState<AgentTokenInfo | null>(null);
  const [revoking, setRevoking] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const res = await api.get("/api/v1/users/me/agent-tokens") as { tokens: AgentTokenInfo[] };
      setTokens(res.tokens ?? []);
    } catch (e) {
      silentError(e, { component: "AgentTokensCard", action: "refresh" });
      toast("error", t("settings.agentTokens.toast.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t, toast]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  function resetCreateForm() {
    setName("");
    setScopeDownload(true);
    setScopeUpload(false);
    setCreateError("");
    setCreating(false);
  }

  async function handleCreate() {
    const trimmed = name.trim();
    const scopes = [scopeDownload && "download", scopeUpload && "upload"].filter(Boolean) as string[];
    if (!trimmed || scopes.length === 0) {
      setCreateError(t("settings.agentTokens.error.invalid"));
      return;
    }
    setCreating(true);
    setCreateError("");
    try {
      const res = await api.post("/api/v1/users/me/agent-tokens", {
        name: trimmed,
        scopes,
      }) as IssuedToken;
      setIssued(res);
      setCopied(false);
      setCreateOpen(false);
      resetCreateForm();
      await refresh();
    } catch (e) {
      silentError(e, { component: "AgentTokensCard", action: "handleCreate" });
      setCreateError(t(getUserFacingErrorKey(e, "settings.agentTokens.error.create")));
    } finally {
      setCreating(false);
    }
  }

  async function handleCopy() {
    if (!issued) return;
    try {
      await navigator.clipboard.writeText(issued.token);
      setCopied(true);
    } catch {
      toast("error", t("settings.agentTokens.toast.copyFailed"));
    }
  }

  async function handleRevoke() {
    if (!revokeTarget) return;
    setRevoking(true);
    try {
      await api.delete(`/api/v1/users/me/agent-tokens/${revokeTarget.id}`);
      toast("success", t("settings.agentTokens.toast.revoked"));
      await refresh();
    } catch (e) {
      silentError(e, { component: "AgentTokensCard", action: "handleRevoke" });
      toast("error", t("settings.agentTokens.toast.revokeFailed"));
    } finally {
      setRevoking(false);
      setRevokeTarget(null);
    }
  }

  return (
    <div className="space-y-3 rounded-md border border-border bg-card p-4">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h3 className="text-sm font-semibold">{t("settings.agentTokens.title")}</h3>
          <p className="mt-0.5 text-xs text-muted-foreground">{t("settings.agentTokens.description")}</p>
        </div>
        <Button
          size="sm"
          onClick={() => {
            resetCreateForm();
            setCreateOpen((open) => !open);
          }}
          aria-expanded={createOpen}
        >
          {t("settings.agentTokens.create")}
        </Button>
      </div>

      {createOpen && (
        <div className="space-y-3 rounded-md border border-border bg-canvas-subtle p-3">
          <div className="space-y-1">
            <label className="text-xs font-medium text-muted-foreground" htmlFor="agent-token-name">
              {t("settings.agentTokens.nameLabel")}
            </label>
            <Input
              id="agent-token-name"
              type="text"
              maxLength={64}
              value={name}
              onChange={(e) => setName(e.target.value)}
              aria-invalid={!!createError}
            />
          </div>
          <div className="flex flex-wrap items-center gap-4">
            <label className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={scopeDownload}
                onChange={(e) => setScopeDownload(e.target.checked)}
                aria-label={t("settings.agentTokens.scope.download")}
              />
              {t("settings.agentTokens.scope.download")}
            </label>
            <label className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={scopeUpload}
                onChange={(e) => setScopeUpload(e.target.checked)}
                aria-label={t("settings.agentTokens.scope.upload")}
              />
              {t("settings.agentTokens.scope.upload")}
            </label>
          </div>
          <p className="text-[11px] text-muted-foreground">{t("settings.agentTokens.scopeHelp")}</p>
          {createError && (
            <p className="text-xs text-destructive" role="alert">
              {createError}
            </p>
          )}
          <div className="flex items-center gap-2">
            <Button size="sm" disabled={creating} onClick={() => void handleCreate()}>
              {creating ? t("common.processing") : t("settings.agentTokens.createConfirm")}
            </Button>
            <Button size="sm" variant="outline" onClick={() => setCreateOpen(false)}>
              {t("common.cancel")}
            </Button>
          </div>
        </div>
      )}

      {issued && (
        <div className="space-y-2 rounded-md border border-border bg-canvas-subtle p-3">
          <p className="text-xs font-medium text-foreground">{t("settings.agentTokens.issued.title")}</p>
          <div className="flex items-start gap-2">
            <code className="min-w-0 flex-1 break-all rounded bg-background px-2 py-1.5 font-mono text-xs">
              {issued.token}
            </code>
            <Button size="sm" variant="outline" onClick={() => void handleCopy()}>
              {copied ? t("settings.agentTokens.issued.copied") : t("settings.agentTokens.issued.copy")}
            </Button>
          </div>
          <p className="text-xs text-destructive" role="alert">
            {t("settings.agentTokens.issued.onceWarning")}
          </p>
          <Button size="sm" variant="outline" onClick={() => setIssued(null)}>
            {t("settings.agentTokens.issued.done")}
          </Button>
        </div>
      )}

      {loading ? (
        <div className="space-y-2" aria-busy="true">
          <div className="h-9 animate-pulse rounded-md bg-canvas-subtle" />
          <div className="h-9 animate-pulse rounded-md bg-canvas-subtle" />
        </div>
      ) : tokens.length === 0 ? (
        <p className="py-3 text-center text-xs text-muted-foreground">
          {t("settings.agentTokens.empty")}
        </p>
      ) : (
        <ul className="divide-y divide-border">
          {tokens.map((token) => (
            <li key={token.id} className="flex items-center justify-between gap-3 py-2.5">
              <div className="min-w-0 space-y-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="truncate text-sm font-medium">{token.name}</span>
                  <code className="font-mono text-xs text-muted-foreground">{token.token_prefix}</code>
                  {token.scopes.map((scope) => (
                    <TagBadge key={scope} color={scope === "upload" ? "green" : "blue"}>
                      {scope}
                    </TagBadge>
                  ))}
                </div>
                <p className="text-xs text-muted-foreground">
                  {t("settings.agentTokens.lastUsed", {
                    time: formatLastUsed(token.last_used_at, t("settings.agentTokens.never")),
                  })}
                </p>
              </div>
              <Button
                size="sm"
                variant="outline"
                className="text-destructive"
                disabled={revoking}
                onClick={() => setRevokeTarget(token)}
              >
                {revoking && revokeTarget?.id === token.id ? (
                  <LoaderCircle className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
                ) : null}
                {t("settings.agentTokens.revoke")}
              </Button>
            </li>
          ))}
        </ul>
      )}

      <ConfirmModal
        open={!!revokeTarget}
        onOpenChange={(open) => {
          if (!open) setRevokeTarget(null);
        }}
        title={t("settings.agentTokens.revokeConfirm.title")}
        description={t("settings.agentTokens.revokeConfirm.description", {
          name: revokeTarget?.name ?? "",
          prefix: revokeTarget?.token_prefix ?? "",
        })}
        confirmVariant="destructive"
        confirmLabel={t("settings.agentTokens.revoke")}
        onConfirm={() => void handleRevoke()}
      />
    </div>
  );
}
