"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { api, ApiRequestError } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { Plus, Trash2, Check, Loader2, Power, Wifi } from "lucide-react";
import { cn } from "@/lib/utils";

interface LLMConfig {
  id: number;
  config_name: string;
  provider_type: string;
  api_base: string;
  model: string;
  api_key_enc?: string;
  is_active: boolean;
}

export default function AgentConfigPage() {
  const t = useTranslations();
  const [configs, setConfigs] = useState<LLMConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [testResult, setTestResult] = useState<Record<number, string>>({});

  // Modal state
  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState({ config_name: "", provider_type: "openai_compat", api_base: "", model: "", api_key: "" });
  const [modalBusy, setModalBusy] = useState(false);
  const [modalError, setModalError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.get<{ configs?: LLMConfig[] }>("/api/v1/admin/llm-configs");
      setConfigs(res.configs ?? []);
    } catch (e) {
      silentError(e, { component: 'AdminAgentConfigPage', action: 'loadConfigs' });
      setError(e instanceof ApiRequestError ? e.message : t("common.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { load(); }, [load]);

  function openCreate() {
    setEditingId(null);
    setForm({ config_name: "", provider_type: "openai_compat", api_base: "", model: "", api_key: "" });
    setModalError("");
    setModalOpen(true);
  }

  function openEdit(cfg: LLMConfig) {
    setEditingId(cfg.id);
    setForm({ config_name: cfg.config_name, provider_type: cfg.provider_type, api_base: cfg.api_base, model: cfg.model, api_key: "" });
    setModalError("");
    setModalOpen(true);
  }

  async function handleSave() {
    if (!form.config_name || !form.api_base || !form.model) return;
    setModalBusy(true);
    setModalError("");
    try {
      if (editingId) {
        const body = {
          config_name: form.config_name,
          provider_type: form.provider_type,
          api_base: form.api_base,
          model: form.model,
          ...(form.api_key ? { api_key: form.api_key } : {}),
        };
        await api.patch(`/api/v1/admin/llm-configs/${editingId}`, body);
      } else {
        await api.post("/api/v1/admin/llm-configs", form);
      }
      setModalOpen(false);
      await load();
    } catch (e) {
      silentError(e, { component: 'AdminAgentConfigPage', action: 'handleSave' });
      setModalError(e instanceof ApiRequestError ? e.message : t("common.saveFailed"));
    } finally {
      setModalBusy(false);
    }
  }

  async function handleDelete(id: number) {
    try {
      await api.delete(`/api/v1/admin/llm-configs/${id}`);
      await load();
    } catch (e) {
      silentError(e, { component: 'AdminAgentConfigPage', action: 'handleDelete' });
      setError(e instanceof ApiRequestError ? e.message : t("common.operationFailed"));
    }
  }

  async function handleActivate(id: number) {
    try {
      await api.post(`/api/v1/admin/llm-configs/${id}/activate`, {});
      await load();
    } catch (e) {
      silentError(e, { component: 'AdminAgentConfigPage', action: 'handleActivate' });
      setError(e instanceof ApiRequestError ? e.message : t("common.operationFailed"));
    }
  }

  async function handleTest(id: number) {
    setTestResult((prev) => ({ ...prev, [id]: "testing" }));
    try {
      await api.post(`/api/v1/admin/llm-configs/${id}/test`, {});
      setTestResult((prev) => ({ ...prev, [id]: "ok" }));
    } catch (e) {
      silentError(e, { component: 'AdminAgentConfigPage', action: 'handleTest' });
      setTestResult((prev) => ({ ...prev, [id]: "fail" }));
    }
  }

  const activeConfig = configs.find((c) => c.is_active);

  return (
    <div className="mx-auto w-full max-w-5xl space-y-6 px-4 py-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("agentConfig.title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("agentConfig.subtitle")}</p>
        </div>
        <Button size="sm" onClick={openCreate}>
          <Plus className="mr-1 h-4 w-4" />{t("agentConfig.addConfig")}
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {/* Active config card */}
      <div className="rounded-md border border-border bg-card p-4 ">
        <h3 className="text-sm font-semibold">{t("agentConfig.activeConfig")}</h3>
        {activeConfig ? (
          <div className="mt-2 flex flex-wrap items-center gap-3 text-sm">
            <span className="rounded bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400">
              <Check className="mr-1 inline h-3 w-3" />
              {activeConfig.config_name}
            </span>
            <span className="text-muted-foreground">{activeConfig.provider_type}</span>
            <span className="text-muted-foreground">{activeConfig.model}</span>
          </div>
        ) : (
          <p className="mt-2 text-sm text-muted-foreground">{t("agentConfig.noActiveConfig")}</p>
        )}
      </div>

      {/* Config table */}
      {loading ? (
        <div className="text-sm text-muted-foreground text-center py-8">{t("common.loading")}</div>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border bg-muted/40">
              <tr>
                <th className="px-4 py-2.5 text-xs font-medium">{t("agentConfig.colName")}</th>
                <th className="px-4 py-2.5 text-xs font-medium">{t("agentConfig.colProvider")}</th>
                <th className="px-4 py-2.5 text-xs font-medium">{t("agentConfig.colModel")}</th>
                <th className="px-4 py-2.5 text-xs font-medium">{t("agentConfig.colStatus")}</th>
                <th className="px-4 py-2.5 text-right text-xs font-medium">{t("agentConfig.colActions")}</th>
              </tr>
            </thead>
            <tbody>
              {configs.map((cfg) => (
                <tr key={cfg.id} className="border-b border-border last:border-0">
                  <td className="px-4 py-2.5 font-medium">{cfg.config_name}</td>
                  <td className="px-4 py-2.5 text-muted-foreground">{cfg.provider_type}</td>
                  <td className="px-4 py-2.5 text-muted-foreground">{cfg.model}</td>
                  <td className="px-4 py-2.5">
                    {cfg.is_active ? (
                      <span className="rounded bg-emerald-100 px-1.5 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400">
                        {t("common.enabled")}
                      </span>
                    ) : (
                      <span className="text-xs text-muted-foreground">{t("common.disabled")}</span>
                    )}
                  </td>
                  <td className="px-4 py-2.5 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={() => handleTest(cfg.id)}
                        title={t("agentConfig.testConnection")}>
                        {testResult[cfg.id] === "testing" ? <Loader2 className="h-4 w-4 animate-spin" /> :
                         testResult[cfg.id] === "ok" ? <Check className="h-4 w-4 text-emerald-600" /> :
                         testResult[cfg.id] === "fail" ? <Wifi className="h-4 w-4 text-red-500" /> :
                         <Wifi className="h-4 w-4" />}
                      </Button>
                      {!cfg.is_active && (
                        <Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={() => handleActivate(cfg.id)}
                          title={t("agentConfig.activate")}>
                          <Power className="h-4 w-4" />
                        </Button>
                      )}
                      <Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={() => openEdit(cfg)}>
                        {t("common.edit")}
                      </Button>
                      <Button variant="ghost" size="sm" className="h-8 w-8 p-0 text-destructive" onClick={() => handleDelete(cfg.id)}
                        disabled={cfg.is_active}>
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Modal */}
      {modalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30">
          <div className="w-full max-w-md rounded-md border border-border bg-card p-6 ">
            <h2 className="text-lg font-semibold">
              {editingId ? t("agentConfig.editConfig") : t("agentConfig.addConfig")}
            </h2>
            {modalError && <p className="mt-2 text-sm text-destructive">{modalError}</p>}
            <div className="mt-4 space-y-3">
              <div className="space-y-1">
                <label className="text-xs font-medium">{t("agentConfig.colName")}</label>
                <input type="text" value={form.config_name} onChange={(e) => setForm({ ...form, config_name: e.target.value })}
                  className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent" />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-medium">{t("agentConfig.colProvider")}</label>
                <select value={form.provider_type} onChange={(e) => setForm({ ...form, provider_type: e.target.value })}
                  className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent">
                  <option value="openai_compat">OpenAI Compatible</option>
                  <option value="qwen">Qwen (DashScope)</option>
                  <option value="deepseek">DeepSeek</option>
                </select>
              </div>
              <div className="space-y-1">
                <label className="text-xs font-medium">{t("agentConfig.apiBase")}</label>
                <input type="text" value={form.api_base} onChange={(e) => setForm({ ...form, api_base: e.target.value })}
                  placeholder="https://api.deepseek.com" className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent" />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-medium">{t("agentConfig.colModel")}</label>
                <input type="text" value={form.model} onChange={(e) => setForm({ ...form, model: e.target.value })}
                  placeholder="deepseek-chat" className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent" />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-medium">{t("agentConfig.apiKey")}</label>
                <input type="password" value={form.api_key} onChange={(e) => setForm({ ...form, api_key: e.target.value })}
                  placeholder={editingId ? t("agentConfig.apiKeyHint") : ""} className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent" />
              </div>
            </div>
            <div className="mt-6 flex justify-end gap-2">
              <Button variant="outline" size="sm" onClick={() => setModalOpen(false)} disabled={modalBusy}>
                {t("common.cancel")}
              </Button>
              <Button size="sm" onClick={handleSave} disabled={modalBusy}>
                {modalBusy ? t("common.saving") : t("common.save")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
