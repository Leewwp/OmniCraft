"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";

export default function SettingsPage() {
  const t = useTranslations();
  const { user, isLoading } = useAuth();
  const [username, setUsername] = useState("");
  const [bio, setBio] = useState("");
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (user) {
      setUsername(user.username || "");
      setBio(user.bio || "");
    }
  }, [user]);

  async function handleSave() {
    if (!user) return;
    setError("");
    setSuccess("");
    setBusy(true);
    try {
      await api.patch(`/api/v1/users/${user.id}`, { username: username.trim(), bio: bio.trim() });
      setSuccess(t('common.saveSuccess'));
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t('common.saveFailed'));
    } finally {
      setBusy(false);
    }
  }

  if (isLoading) {
    return <div className="mx-auto w-full max-w-lg px-4 py-6 text-sm text-muted-foreground">{t('common.loading')}</div>;
  }

  return (
    <div className="mx-auto w-full max-w-lg space-y-6 px-4 py-6">
      <div className="rounded-md border border-border bg-card p-4 shadow-none">
        <h1 className="text-2xl font-bold tracking-tight">{t('settings.title')}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t('settings.subtitle')}</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}
      {success && <p className="text-sm text-emerald-600">{success}</p>}

      <div className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t('settings.username')}</label>
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
          />
        </div>

        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t('settings.email')}</label>
          <input
            type="email"
            value={user?.email || ""}
            readOnly
            className="w-full rounded-md border border-border bg-muted/20 px-3 py-2 text-sm text-muted-foreground"
          />
          <p className="text-[11px] text-muted-foreground">{t('settings.emailHint')}</p>
        </div>

        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t('settings.bio')}</label>
          <textarea
            value={bio}
            onChange={(e) => setBio(e.target.value)}
            rows={3}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
          />
        </div>

        <Button size="sm" disabled={busy} onClick={() => void handleSave()}>
          {busy ? t('common.saving') : t('settings.saveButton')}
        </Button>
      </div>

      <div className="rounded-md border border-border bg-card p-4 shadow-none">
        <h3 className="text-sm font-semibold">{t('settings.securityTitle')}</h3>
        <p className="mt-1 text-xs text-muted-foreground">{t('settings.securityHint')}</p>
      </div>
    </div>
  );
}
