"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";

export default function SettingsPage() {
  const t = useTranslations();
  const router = useRouter();
  const { user, isLoading, logout } = useAuth();
  const [username, setUsername] = useState("");
  const [bio, setBio] = useState("");
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [busy, setBusy] = useState(false);

  // Password change
  const [oldPw, setOldPw] = useState("");
  const [newPw, setNewPw] = useState("");
  const [confirmPw, setConfirmPw] = useState("");
  const [pwError, setPwError] = useState("");
  const [pwSuccess, setPwSuccess] = useState("");
  const [pwBusy, setPwBusy] = useState(false);

  // Account deletion
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deletePw, setDeletePw] = useState("");
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [deleteBusy, setDeleteBusy] = useState(false);

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
      setSuccess(t("common.saveSuccess"));
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t("common.saveFailed"));
    } finally {
      setBusy(false);
    }
  }

  async function handleChangePassword() {
    if (newPw !== confirmPw) {
      setPwError(t("settings.passwordMismatch"));
      return;
    }
    if (newPw.length < 6) {
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
      setPwError(e instanceof ApiRequestError ? e.message : t("common.operationFailed"));
    } finally {
      setPwBusy(false);
    }
  }

  async function handleDeleteAccount() {
    if (!deleteConfirm) return;
    setDeleteBusy(true);
    try {
      await api.delete("/api/v1/users/me");
      logout?.();
      router.push("/");
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t("common.operationFailed"));
    } finally {
      setDeleteBusy(false);
    }
  }

  if (isLoading) {
    return <div className="mx-auto w-full max-w-lg px-4 py-6 text-sm text-muted-foreground">{t("common.loading")}</div>;
  }

  return (
    <div className="mx-auto w-full max-w-lg space-y-6 px-4 py-6">
      <div className="rounded-md border border-border bg-card p-4 shadow-none">
        <h1 className="text-2xl font-bold tracking-tight">{t("settings.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("settings.subtitle")}</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}
      {success && <p className="text-sm text-emerald-600">{success}</p>}

      {/* Profile */}
      <div className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("settings.username")}</label>
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
          />
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("settings.email")}</label>
          <input type="email" value={user?.email || ""} readOnly className="w-full rounded-md border border-border bg-muted/20 px-3 py-2 text-sm text-muted-foreground" />
          <p className="text-[11px] text-muted-foreground">{t("settings.emailHint")}</p>
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("settings.bio")}</label>
          <textarea value={bio} onChange={(e) => setBio(e.target.value)} rows={3} className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent" />
        </div>
        <Button size="sm" disabled={busy} onClick={() => void handleSave()}>
          {busy ? t("common.saving") : t("settings.saveButton")}
        </Button>
      </div>

      {/* Password change */}
      <div className="space-y-3 rounded-md border border-border bg-card p-4 shadow-none">
        <h3 className="text-sm font-semibold">{t("settings.changePassword")}</h3>
        {pwError && <p className="text-sm text-destructive">{pwError}</p>}
        {pwSuccess && <p className="text-sm text-emerald-600">{pwSuccess}</p>}
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("settings.oldPassword")}</label>
          <input type="password" value={oldPw} onChange={(e) => setOldPw(e.target.value)} className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent" />
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("settings.newPassword")}</label>
          <input type="password" value={newPw} onChange={(e) => setNewPw(e.target.value)} className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent" />
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">{t("settings.confirmPassword")}</label>
          <input type="password" value={confirmPw} onChange={(e) => setConfirmPw(e.target.value)} className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent" />
        </div>
        <Button size="sm" disabled={pwBusy || !oldPw || !newPw} onClick={() => void handleChangePassword()}>
          {pwBusy ? t("common.saving") : t("settings.changePassword")}
        </Button>
      </div>

      {/* Account deletion (danger zone) */}
      <div className="space-y-3 rounded-md border border-red-200 bg-red-50/30 p-4 shadow-none dark:border-red-900/30 dark:bg-red-950/10">
        <h3 className="text-sm font-semibold text-red-700 dark:text-red-400">{t("settings.deleteAccount")}</h3>
        <p className="text-xs text-red-600/80 dark:text-red-400/70">{t("settings.deleteAccountDesc")}</p>

        {deleteOpen ? (
          <div className="space-y-2">
            <input type="password" value={deletePw} onChange={(e) => setDeletePw(e.target.value)} placeholder={t("settings.enterPasswordToConfirm")} className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent" />
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
