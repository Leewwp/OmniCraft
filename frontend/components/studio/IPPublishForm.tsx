"use client";

import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { ArrowLeft, CheckCircle2, Send } from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { TagBadge } from "@/components/ui/TagBadge";
import { FileUploader, type UploadedAsset } from "@/components/content/FileUploader";
import { ipCategoryOptions } from "@/components/ip/ipCategory";
import { api } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { fetchPublicConfig } from "@/lib/public-config";
import { silentError } from "@/lib/error-handler";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { cn } from "@/lib/utils";

// Matches CreateIPInput (backend): name 1-255; single tag column is
// VARCHAR(50), backend normalizeIPTags enforces the same cap.
const NAME_MAX_LENGTH = 255;
const MAX_TAGS = 10;

const CATEGORY_OPTIONS = ipCategoryOptions.filter((option) => option.key !== "all");

interface IPPublishFormProps {
  onBack: () => void;
}

interface CreatedIP {
  id: number;
  name: string;
}

export function IPPublishForm({ onBack }: IPPublishFormProps) {
  const t = useTranslations("studio.publishIP");
  // Category option labels live in the root namespace (home.*), same as the
  // other ip category consumers.
  const tRoot = useTranslations();
  const { toast } = useToast();

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");
  const [cover, setCover] = useState<UploadedAsset | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [nameError, setNameError] = useState("");
  const [created, setCreated] = useState<CreatedIP | null>(null);

  const nameTrimmed = name.trim();
  const nameInvalid = nameTrimmed.length < 1 || name.length > NAME_MAX_LENGTH;

  function addTag() {
    const value = tagInput.trim();
    if (value && !tags.includes(value) && tags.length < MAX_TAGS) {
      setTags([...tags, value]);
    }
    setTagInput("");
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (nameTrimmed.length < 1) {
      setNameError(t("nameRequired"));
      return;
    }
    if (name.length > NAME_MAX_LENGTH) {
      setNameError(t("nameTooLong", { max: NAME_MAX_LENGTH }));
      return;
    }
    setNameError("");

    setSubmitting(true);
    try {
      let coverUrl = "";
      if (cover) {
        const config = await fetchPublicConfig();
        coverUrl = config.oss_domain
          ? `${config.oss_domain}/${cover.ossKey}`
          : cover.ossKey;
      }
      const data = await api.post<{ ip?: CreatedIP }>("/api/v1/ips", {
        name: nameTrimmed,
        description: description.trim(),
        cover_url: coverUrl,
        category,
        tags,
      });
      const ip = data.ip;
      if (!ip || typeof ip.id !== "number") {
        throw new Error("invalid create response");
      }
      setCreated(ip);
      toast("success", t("successTitle"));
    } catch (error) {
      silentError(error, { component: "IPPublishForm", action: "handleSubmit" });
      toast("error", t(getUserFacingErrorKey(error, "failed")));
    } finally {
      setSubmitting(false);
    }
  }

  function handleCreateAnother() {
    setCreated(null);
    setName("");
    setDescription("");
    setCategory("");
    setTags([]);
    setTagInput("");
    setCover(null);
    setNameError("");
  }

  if (created) {
    return (
      <div className="max-w-2xl space-y-6">
        <button
          type="button"
          onClick={onBack}
          className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          <ArrowLeft className="h-4 w-4" />
          {t("backToStudio")}
        </button>
        <div
          role="status"
          aria-live="polite"
          className="space-y-4 rounded-xl border border-border bg-card p-6"
        >
          <div className="flex items-center gap-2">
            <CheckCircle2 className="h-5 w-5 text-accent-emphasis" aria-hidden="true" />
            <h2 className="text-base font-semibold text-foreground">{t("successTitle")}</h2>
          </div>
          <p className="text-sm text-muted-foreground">{t("successPending", { name: created.name })}</p>
          <div className="flex flex-wrap gap-3">
            <Link
              href={`/ip/${created.id}`}
              className={buttonVariants({ size: "lg", className: "px-8" })}
            >
              {t("successView")}
            </Link>
            <Button type="button" variant="outline" size="lg" onClick={handleCreateAnother}>
              {t("successCreateAnother")}
            </Button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="max-w-2xl space-y-6">
      <button
        type="button"
        onClick={onBack}
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
      >
        <ArrowLeft className="h-4 w-4" />
        {t("backToStudio")}
      </button>

      <div>
        <label htmlFor="ip-name" className="mb-1.5 block text-sm font-medium text-foreground">
          {t("nameLabel")} <span className="text-destructive">*</span>
        </label>
        <Input
          id="ip-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t("namePlaceholder")}
          maxLength={NAME_MAX_LENGTH + 1}
          aria-invalid={Boolean(nameError)}
          aria-describedby={nameError ? "ip-name-error" : undefined}
        />
        {nameError && (
          <p id="ip-name-error" className="mt-1 text-xs text-destructive" role="alert">
            {nameError}
          </p>
        )}
      </div>

      <div>
        <label htmlFor="ip-description" className="mb-1.5 block text-sm font-medium text-foreground">
          {t("descriptionLabel")}
        </label>
        <textarea
          id="ip-description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={3}
          placeholder={t("descriptionPlaceholder")}
          className="min-h-11 w-full rounded-lg border border-border bg-card px-3 py-2 text-sm placeholder:text-muted-foreground/60 focus:outline-none focus:ring-2 focus:ring-ring/20 resize-none"
        />
      </div>

      <div>
        <label className="mb-1.5 block text-sm font-medium text-foreground">{t("coverLabel")}</label>
        <FileUploader
          fileType="image"
          maxMB={20}
          accept="image/*"
          disabled={submitting}
          onUploaded={(files) => setCover(files[0] ?? null)}
        />
        <p className="mt-1 text-xs text-muted-foreground">{t("coverHint")}</p>
      </div>

      <div>
        <label className="mb-1.5 block text-sm font-medium text-foreground">{t("categoryLabel")}</label>
        <div className="flex flex-wrap gap-2">
          {CATEGORY_OPTIONS.map((option) => {
            const active = category === option.key;
            return (
              <button
                key={option.key}
                type="button"
                onClick={() => setCategory(active ? "" : option.key)}
                aria-pressed={active}
                className={cn(
                  "min-h-11 rounded-full border px-3.5 text-xs font-medium transition-colors duration-150",
                  active
                    ? "border-accent-emphasis bg-accent-subtle text-accent-emphasis font-semibold"
                    : "border-border text-muted-foreground hover:border-border/80 hover:text-foreground",
                )}
              >
                {tRoot(option.label)}
              </button>
            );
          })}
        </div>
      </div>

      <div>
        <label htmlFor="ip-tag-input" className="mb-1.5 block text-sm font-medium text-foreground">
          {t("tagLabel")}
        </label>
        <div className="flex gap-2">
          <Input
            id="ip-tag-input"
            value={tagInput}
            onChange={(e) => setTagInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                addTag();
              }
            }}
            placeholder={t("tagPlaceholder")}
            className="flex-1"
          />
          <Button type="button" variant="outline" size="sm" onClick={addTag}>
            {t("addTag")}
          </Button>
        </div>
        {tags.length > 0 && (
          <div className="mt-2 flex flex-wrap gap-1.5">
            {tags.map((tag) => (
              <TagBadge
                key={tag}
                color="blue"
                onRemove={() => setTags(tags.filter((item) => item !== tag))}
              >
                {tag}
              </TagBadge>
            ))}
          </div>
        )}
      </div>

      <div className="flex items-center gap-3 pt-2">
        <Button
          type="submit"
          size="lg"
          disabled={submitting || nameInvalid}
          className="gap-2 px-8"
        >
          <Send className="h-4 w-4" />
          {submitting ? t("submitting") : t("submit")}
        </Button>
        <Button type="button" variant="ghost" onClick={onBack}>
          {t("cancel")}
        </Button>
      </div>
    </form>
  );
}
