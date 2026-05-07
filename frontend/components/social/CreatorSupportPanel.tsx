"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Heart, ExternalLink, Upload } from "lucide-react";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface SupportInfo {
  donation_image_url?: string;
  external_links?: string[];
}

interface CreatorSupportPanelProps {
  supportInfo?: SupportInfo;
  isOwner?: boolean;
  className?: string;
}

export function CreatorSupportPanel({ supportInfo, isOwner, className }: CreatorSupportPanelProps) {
  const t = useTranslations();
  const [imageUrl, setImageUrl] = useState(supportInfo?.donation_image_url || "");
  const [links, setLinks] = useState<string[]>(supportInfo?.external_links || []);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  async function handleSave() {
    setBusy(true);
    setError("");
    setSuccess("");
    try {
      await api.patch("/api/v1/users/me/support-info", {
        donation_image_url: imageUrl || null,
        external_links: links.filter(Boolean),
      });
      setSuccess(t("common.saveSuccess"));
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t("common.saveFailed"));
    } finally {
      setBusy(false);
    }
  }

  function addLink() {
    if (links.length >= 3) return;
    setLinks([...links, ""]);
  }

  function updateLink(idx: number, val: string) {
    setLinks(links.map((l, i) => (i === idx ? val : l)));
  }

  function removeLink(idx: number) {
    setLinks(links.filter((_, i) => i !== idx));
  }

  // Read-only view
  if (!isOwner) {
    const hasInfo = supportInfo?.donation_image_url || (supportInfo?.external_links?.length ?? 0) > 0;
    if (!hasInfo) return null;

    return (
      <div className={cn("rounded-md border border-border bg-card p-4 shadow-none space-y-3", className)}>
        <div className="flex items-center gap-2">
          <Heart className="h-4 w-4 text-red-500" />
          <span className="text-sm font-medium">{t("support.title")}</span>
        </div>
        {supportInfo?.donation_image_url && (
          <img
            src={supportInfo.donation_image_url}
            alt={t("support.donationImage")}
            className="max-h-48 rounded-md border border-border object-contain"
          />
        )}
        {supportInfo?.external_links?.map((url) => (
          <a
            key={url}
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1 text-sm text-accent hover:underline"
          >
            <ExternalLink className="h-3.5 w-3.5" />
            {url}
          </a>
        ))}
      </div>
    );
  }

  // Owner edit view
  return (
    <div className={cn("rounded-md border border-border bg-card p-4 shadow-none space-y-3", className)}>
      <div className="flex items-center gap-2">
        <Heart className="h-4 w-4 text-red-500" />
        <span className="text-sm font-medium">{t("support.title")}</span>
      </div>

      {error && <p className="text-xs text-destructive">{error}</p>}
      {success && <p className="text-xs text-emerald-600">{success}</p>}

      <div className="space-y-1">
        <label className="text-xs font-medium text-muted-foreground">{t("support.donationImageUrl")}</label>
        <input
          type="text"
          value={imageUrl}
          onChange={(e) => setImageUrl(e.target.value)}
          placeholder="https://..."
          className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
        />
      </div>

      <div className="space-y-2">
        <label className="text-xs font-medium text-muted-foreground">
          {t("support.externalLinks")} ({links.length}/3)
        </label>
        {links.map((url, idx) => (
          <div key={idx} className="flex gap-1">
            <input
              type="text"
              value={url}
              onChange={(e) => updateLink(idx, e.target.value)}
              placeholder="https://..."
              className="flex-1 rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
            />
            <Button size="sm" variant="ghost" className="text-destructive h-9 w-9 p-0" onClick={() => removeLink(idx)}>×</Button>
          </div>
        ))}
        {links.length < 3 && (
          <Button size="sm" variant="outline" onClick={addLink}>
            <Upload className="mr-1 h-3.5 w-3.5" />
            {t("support.addLink")}
          </Button>
        )}
      </div>

      <Button size="sm" onClick={handleSave} disabled={busy}>
        {busy ? t("common.saving") : t("common.save")}
      </Button>
    </div>
  );
}
