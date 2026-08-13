"use client";

import { useEffect, useState, useRef } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { useToast } from "@/components/ui/Toast";
import { VerificationReminderCard } from "@/components/settings/VerificationReminderCard";
import { User, CheckCircle, AlertCircle, ExternalLink, LoaderCircle } from "lucide-react";

export default function SettingsPage() {
  const t = useTranslations();
  const router = useRouter();
  const { user, refreshUser, logout } = useAuth();
  const { toast } = useToast();
  const [username, setUsername] = useState("");
  const [bio, setBio] = useState("");
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [busy, setBusy] = useState(false);

  const [collabAccept, setCollabAccept] = useState(true);
  const [collabBusy, setCollabBusy] = useState(false);
  const [collabError, setCollabError] = useState("");

  const [avatarUploading, setAvatarUploading] = useState(false);
  const avatarInputRef = useRef<HTMLInputElement>(null);

  const [oldPw, setOldPw] = useState("");
  const [newPw, setNewPw] = useState("");
  const [confirmPw, setConfirmPw] = useState("");
  const [pwError, setPwError] = useState("");
  const [pwSuccess, setPwSuccess] = useState("");
  const [pwBusy, setPwBusy] = useState(false);

  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deletePw, setDeletePw] = useState("");
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [deleteBusy, setDeleteBusy] = useState(false);

  const isVerified = !!user?.email_verified_at;

  useEffect(() => {
    if (user) {
      setUsername(user.username || "");
      setBio(user.bio || "");
      setCollabAccept(user.accept_collab_invites ?? true);
    }
  }, [user]);

  async function handleCollabToggle(next: boolean) {
    if (!user || collabBusy) return;
    const previous = user.accept_collab_invites ?? true;
    setCollabAccept(next);
    setCollabError("");
    setCollabBusy(true);
    try {
      await api.patch(`/api/v1/users/${user.id}`, { accept_collab_invites: next });
      await refreshUser();
      toast("success", t("settings.collaboration.toast.saved"));
    } catch (e) {
      silentError(e, { component: "SettingsPage", action: "handleCollabToggle" });
      setCollabAccept(previous);
      const messageKey = getUserFacingErrorKey(e, "settings.collaboration.error.save");
      setCollabError(t(messageKey));
      toast("error", t("settings.collaboration.toast.failed"));
    } finally {
      setCollabBusy(false);
    }
  }

  async function handleAvatarChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file || !user) return;
    if (file.size > 20 * 1024 * 1024) {
      setError(t("settings.avatarTooLarge"));
      return;
    }
    setAvatarUploading(true);
    setError("");
    try {
      const presignRes = await api.post("/api/v1/contents/oss-token", {
        file_name: file.name,
        file_type: "avatar",
        mime_type: file.type,
        file_size: file.size,
      }) as { upload_url: string; oss_key: string };
      await fetch(presignRes.upload_url, { method: "PUT", body: file, headers: { "Content-Type": file.type } });
      const cfgRes = await fetch("/api/v1/config/public").then(r => r.json()).catch(() => null);
      const cdnBase = cfgRes?.oss_domain || "";
      const avatarUrl = cdnBase ? `${cdnBase}/${presignRes.oss_key}` : presignRes.oss_key;
      await api.patch(`/api/v1/users/${user.id}`, { avatar_url: avatarUrl });
      await refreshUser();
      setSuccess(t("settings.avatarUpdated"));
    } catch (e) {
      silentError(e, { component: 'SettingsPage', action: 'handleAvatarChange' });
      setError(t(getUserFacingErrorKey(e)));
    } finally {
      setAvatarUploading(false);
    }
  }

  async function handleSave() {
    if (!user) return;
    setError("");
    setSuccess("");
    setBusy(true);
    try {
      await api.patch(`/api/v1/users/${user.id}`, { username: username.trim(), bio: bio.trim() });
      setSuccess(t("common.saveSuccess"));
    } catch (e) {
      silentError(e, { component: 'SettingsPage', action: 'handleSave' });
      setError(t(getUserFacingErrorKey(e, "common.saveFailed")));
    } finally {
      setBusy(false);
    }
  }

  async function handleChangePassword() {
    if (newPw !== confirmPw) {
      setPwError(t("settings.passwordMismatch"));
      return;
    }
    if (newPw.length < 8) {
      setPwError(t("settings.passwordTooShort"));
      return;
    }
    setPwBusy(true);
    setPwError("");
    setPwSuccess("");
    try {
      await api.patch("/api/v1/users/me/password", {
        old_password: oldPw,
        new_password: newPw,
      });
      setPwSuccess(t("settings.passwordChanged"));
      setOldPw("");
      setNewPw("");
      setConfirmPw("");
    } catch (e) {
      silentError(e, { component: 'SettingsPage', action: 'handleChangePassword' });
      setPwError(t(getUserFacingErrorKey(e)));
    } finally {
      setPwBusy(false);
    }
  }

  async function handleDeleteAccount() {
    if (!deleteConfirm) return;
    setDeleteBusy(true);
    try {
      await api.delete("/api/v1/users/me");
      logout();
      router.push("/");
    } catch (e) {
      silentError(e, { component: 'SettingsPage', action: 'handleDeleteAccount' });
      setError(t(getUserFacingErrorKey(e)));
    } finally {
      setDeleteBusy(false);
    }
  }

  return (
    <div className="mx-auto w-full max-w-lg space-y-6 px-4 py-6">
      <div className="rounded-md border border-border bg-card p-4">
        <h1 className="text-2xl font-bold tracking-tight">{t("settings.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("settings.subtitle")}</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}
      {success && <p className="text-sm text-emerald-600">{success}</p>}

      <div className="space-y-3 rounded-md border border-border bg-card p-4">
        <h3 className="text-sm font-semibold">{t("settings.avatar")}</h3>
        <div className="flex items-center gap-4">
          {user?.avatar_url ? (
            <img src={user.avatar_url} alt="" className="h-16 w-16 rounded-full object-cover border border-border" />
          ) : (
            <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted border border-border">
              <User className="h-8 w-8 text-muted-foreground" />
            </div>
          )}
          <div>
            <Button size="sm" variant="outline" disabled={avatarUploading} onClick={() => avatarInputRef.current?.click()}>
              {avatarUploading ? t("common.processing") : t("settings.changeAvatar")}
            </Button>
            <p className="mt-1 text-xs text-muted-foreground">{t("settings.avatarHint")}</p>
            <input ref={avatarInputRef} type="file" accept="image/*" className="hidden" onChange={handleAvatarChange} />
          </div>
        </div>
      </div>

      <div className="space-y-4 rounded-md border border-border bg-card p-4 ">
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("settings.username")}</label>
          <Input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("settings.email")}</label>
          <div className="flex items-center gap-2">
            <Input type="email" value={user?.email || ""} readOnly disabled className="flex-1" />
            {isVerified ? (
              <span className="inline-flex items-center gap-1 text-xs text-emerald-600 whitespace-nowrap">
                <CheckCircle className="h-3.5 w-3.5" />
                {t("settings.emailVerified")}
              </span>
            ) : (
              <span className="inline-flex items-center gap-1 text-xs text-amber-600 whitespace-nowrap">
                <AlertCircle className="h-3.5 w-3.5" />
                {t("settings.emailUnverified")}
              </span>
            )}
          </div>
          <p className="text-[11px] text-muted-foreground">{t("settings.emailHint")}</p>
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("settings.bio")}</label>
          <Textarea value={bio} onChange={(e) => setBio(e.target.value)} rows={3} />
        </div>
        <Button size="sm" disabled={busy} onClick={() => void handleSave()}>
          {busy ? t("common.saving") : t("settings.saveButton")}
        </Button>
      </div>

      <div className="space-y-3 rounded-md border border-border bg-card p-4">
        <h3 className="text-sm font-semibold">{t("settings.collaboration.title")}</h3>
        <p className="text-xs text-muted-foreground">{t("settings.collaboration.description")}</p>
        <div className="flex items-center justify-between gap-4">
          <label className="flex items-center gap-3">
            <Switch
              checked={collabAccept}
              onCheckedChange={(checked) => void handleCollabToggle(checked)}
              disabled={collabBusy}
              aria-label={t("settings.collaboration.a11y.acceptInvites")}
            />
            <span className="text-sm font-medium text-foreground">{t("settings.collaboration.acceptInvites.label")}</span>
          </label>
          {collabBusy && (
            <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground" aria-live="polite">
              <LoaderCircle className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
              {t("common.saving")}
            </span>
          )}
        </div>
        <p className="text-[11px] text-muted-foreground">{t("settings.collaboration.acceptInvites.help")}</p>
        {collabError && (
          <p className="text-sm text-destructive" role="alert">
            {collabError}
          </p>
        )}
      </div>

      {!isVerified && (
        <VerificationReminderCard email={user?.email || ""} />
      )}

      <div className="space-y-2 rounded-md border border-border bg-card p-4">
        <h3 className="text-sm font-semibold">{t("settings.legalTitle")}</h3>
        <div className="flex flex-col gap-1">
          <a href="/terms" className="inline-flex items-center gap-1 text-sm text-primary hover:underline">
            {t("auth.termsOfService")} <ExternalLink className="h-3 w-3" />
          </a>
          <a href="/privacy" className="inline-flex items-center gap-1 text-sm text-primary hover:underline">
            {t("auth.privacyPolicy")} <ExternalLink className="h-3 w-3" />
          </a>
          <a href="/feedback" className="inline-flex items-center gap-1 text-sm text-primary hover:underline">
            {t("footer.feedback")} <ExternalLink className="h-3 w-3" />
          </a>
        </div>
      </div>

      <div className="space-y-3 rounded-md border border-border bg-card p-4 ">
        <h3 className="text-sm font-semibold">{t("settings.changePassword")}</h3>
        {pwError && <p className="text-sm text-destructive">{pwError}</p>}
        {pwSuccess && <p className="text-sm text-emerald-600">{pwSuccess}</p>}
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("settings.oldPassword")}</label>
          <Input type="password" value={oldPw} onChange={(e) => setOldPw(e.target.value)} />
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("settings.newPassword")}</label>
          <Input type="password" value={newPw} onChange={(e) => setNewPw(e.target.value)} />
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("settings.confirmPassword")}</label>
          <Input type="password" value={confirmPw} onChange={(e) => setConfirmPw(e.target.value)} />
        </div>
        <Button size="sm" disabled={pwBusy || !oldPw || !newPw} onClick={() => void handleChangePassword()}>
          {pwBusy ? t("common.saving") : t("settings.changePassword")}
        </Button>
      </div>

      <div className="space-y-3 rounded-md border border-red-200 bg-red-50/30 p-4 dark:border-red-900/30 dark:bg-red-950/10">
        <h3 className="text-sm font-semibold text-red-700 dark:text-red-400">{t("settings.deleteAccount")}</h3>
        <p className="text-xs text-red-600/80 dark:text-red-400/70">{t("settings.deleteAccountDesc")}</p>

        {deleteOpen ? (
          <div className="space-y-2">
            <Input type="password" value={deletePw} onChange={(e) => setDeletePw(e.target.value)} placeholder={t("settings.enterPasswordToConfirm")} />
            <label className="flex items-center gap-2 text-xs text-muted-foreground">
              <input type="checkbox" checked={deleteConfirm} onChange={(e) => setDeleteConfirm(e.target.checked)} className="h-3.5 w-3.5" />
              {t("settings.deleteIrreversible")}
            </label>
            <div className="flex gap-2">
              <Button size="sm" variant="destructive" disabled={deleteBusy || !deletePw || !deleteConfirm} onClick={() => void handleDeleteAccount()}>
                {deleteBusy ? t("common.processing") : t("settings.confirmDelete")}
              </Button>
              <Button size="sm" variant="outline" onClick={() => { setDeleteOpen(false); setDeletePw(""); setDeleteConfirm(false); }}>
                {t("common.cancel")}
              </Button>
            </div>
          </div>
        ) : (
          <Button size="sm" variant="destructive" onClick={() => setDeleteOpen(true)}>
            {t("settings.deleteAccount")}
          </Button>
        )}
      </div>
    </div>
  );
}
