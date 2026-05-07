"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";

interface Appeal {
  id: number;
  target_type: string;
  target_id: number;
  reason: string;
  status: string;
  created_at: string;
}

export default function AppealsPage() {
  const t = useTranslations();
  const { user, isLoading } = useAuth();
  const [appeals, setAppeals] = useState<Appeal[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ target_type: "content", target_id: "", reason: "" });
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!user) return;
    void loadAppeals();
  }, [user]);

  async function loadAppeals() {
    setError("");
    setLoading(true);
    try {
      const data = await api.get<{ appeals?: Appeal[] }>("/api/v1/appeals/me");
      setAppeals(data.appeals || []);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t('common.loadFailed'));
    } finally {
      setLoading(false);
    }
  }

  async function submitAppeal() {
    if (!form.target_id || !form.reason) return;
    setSubmitting(true);
    try {
      await api.post("/api/v1/appeals", {
        target_type: form.target_type,
        target_id: parseInt(form.target_id),
        reason: form.reason,
      });
      setShowForm(false);
      setForm({ target_type: "content", target_id: "", reason: "" });
      void loadAppeals();
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t('appeals.submitFailed'));
    } finally {
      setSubmitting(false);
    }
  }

  if (isLoading || loading) {
    return <div className="mx-auto w-full max-w-2xl px-4 py-6 text-sm text-muted-foreground">{t('common.loading')}</div>;
  }

  function getStatusLabel(s: string) {
    switch (s) {
      case "pending": return t('appeals.pending');
      case "approved": return t('appeals.approved');
      case "rejected": return t('appeals.rejected');
      default: return s;
    }
  }

  return (
    <div className="mx-auto w-full max-w-2xl space-y-4 px-4 py-6">
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4 shadow-none">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('appeals.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t('appeals.subtitle')}</p>
        </div>
        <Button size="sm" onClick={() => setShowForm(true)} disabled={showForm}>
          {t('appeals.newAppeal')}
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {showForm && (
        <div className="space-y-3 rounded-md border border-border bg-card p-4 shadow-none">
          <h3 className="text-sm font-semibold">{t('appeals.newAppeal')}</h3>
          <select
            value={form.target_type}
            onChange={(e) => setForm((f) => ({ ...f, target_type: e.target.value }))}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
          >
            <option value="content">{t('appeals.typeContent')}</option>
            <option value="comment">{t('appeals.typeComment')}</option>
          </select>
          <input
            type="number"
            placeholder={t('appeals.targetId')}
            value={form.target_id}
            onChange={(e) => setForm((f) => ({ ...f, target_id: e.target.value }))}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
          />
          <textarea
            placeholder={t('appeals.reason')}
            value={form.reason}
            onChange={(e) => setForm((f) => ({ ...f, reason: e.target.value }))}
            rows={3}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
          />
          <div className="flex gap-2">
            <Button size="sm" disabled={submitting} onClick={() => void submitAppeal()}>
              {submitting ? t('appeals.submitting') : t('appeals.submit')}
            </Button>
            <Button size="sm" variant="outline" onClick={() => setShowForm(false)}>
              {t('appeals.cancel')}
            </Button>
          </div>
        </div>
      )}

      {appeals.length === 0 && !showForm ? (
        <div className="rounded-md border border-border bg-card p-12 text-center shadow-none">
          <p className="text-sm text-muted-foreground">{t('appeals.noAppeals')}</p>
        </div>
      ) : (
        <div className="space-y-2">
          {appeals.map((a) => (
            <div key={a.id} className="rounded-md border border-border bg-card p-4 shadow-none">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium">{a.target_type} #{a.target_id}</span>
                <span className={`rounded px-2 py-0.5 text-xs ${
                  a.status === "approved" ? "bg-emerald-50 text-emerald-700" :
                  a.status === "rejected" ? "bg-red-50 text-red-700" :
                  "bg-amber-50 text-amber-700"
                }`}>{getStatusLabel(a.status)}</span>
              </div>
              <p className="mt-2 text-sm text-muted-foreground">{a.reason}</p>
              <p className="mt-1 text-xs text-muted-foreground">
                {new Date(a.created_at).toLocaleString("zh-CN")}
              </p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
