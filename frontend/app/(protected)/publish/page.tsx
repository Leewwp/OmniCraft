"use client";

import { useEffect, useMemo, useState } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ComplianceCheckBadge } from "@/components/content/ComplianceCheckBadge";
import { FileUploader, UploadedAsset } from "@/components/content/FileUploader";
import { MarkdownEditor } from "@/components/content/MarkdownEditor";
import { UploadAssistPanel } from "@/components/content/UploadAssistPanel";
import { api, ApiRequestError } from "@/lib/api";
import { normalizeContentList } from "@/lib/content";

interface IPItem {
  id: number;
  name: string;
}

interface OriginalItem {
  id: number;
  title: string;
}

interface IPSearchResponse {
  ips?: IPItem[];
}

interface ContentSearchResponse {
  contents?: unknown[];
}

interface TagSearchResponse {
  tags?: string[];
}

const SIZE_LIMITS = {
  image: 20,
  video: 300,
  text: 10,
  audio: 50,
  mod: 500,
  sheet_music: 50,
  other: 10,
} as const;

export default function PublishPage() {
  const t = useTranslations();
  const router = useRouter();

  const [title, setTitle] = useState("");
  const [zone, setZone] = useState<"fanwork" | "original">("original");
  const [contentType, setContentType] = useState("text");
  const [ipId, setIPID] = useState("");
  const [sourceOriginalId, setSourceOriginalId] = useState("");
  const [category, setCategory] = useState("");
  const [coverAsset, setCoverAsset] = useState<UploadedAsset | null>(null);
  const [attachments, setAttachments] = useState<UploadedAsset[]>([]);
  const [markdown, setMarkdown] = useState("");
  const [isPublic, setIsPublic] = useState(true);
  const [allowCopy, setAllowCopy] = useState(false);
  const [agentEnabled, setAgentEnabled] = useState(false);
  const [ipKeyword, setIPKeyword] = useState("");
  const [originalKeyword, setOriginalKeyword] = useState("");
  const [tagInput, setTagInput] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [tagSuggestions, setTagSuggestions] = useState<string[]>([]);
  const [ips, setIPs] = useState<IPItem[]>([]);
  const [originals, setOriginals] = useState<OriginalItem[]>([]);
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  const CONTENT_TYPES = useMemo(() => [
    { value: "text", label: t('content.categoryText') },
    { value: "image", label: t('content.categoryImage') },
    { value: "video", label: t('content.categoryVideo') },
    { value: "audio", label: t('judge.audio') },
    { value: "mod", label: "Mod" },
    { value: "sheet_music", label: t('content.categorySheetMusic') },
    { value: "other", label: t('content.categoryOther') },
  ], [t]);

  const uploadFileType = useMemo<"image" | "video" | "text" | "mod" | "sheet_music">(() => {
    switch (contentType) {
      case "image":
      case "video":
      case "mod":
      case "sheet_music":
        return contentType;
      default:
        return "text";
    }
  }, [contentType]);

  const filteredIPs = useMemo(() => {
    const keyword = ipKeyword.trim().toLowerCase();
    return keyword ? ips.filter((ip) => ip.name.toLowerCase().includes(keyword)) : ips;
  }, [ips, ipKeyword]);

  const filteredOriginals = useMemo(() => {
    const keyword = originalKeyword.trim().toLowerCase();
    return keyword ? originals.filter((item) => item.title.toLowerCase().includes(keyword)) : originals;
  }, [originals, originalKeyword]);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const initialZone = params.get("zone");
    const initialSourceOriginalID = params.get("source_original_id");
    if (initialZone === "fanwork" || initialZone === "original") {
      setZone(initialZone);
    }
    if (initialSourceOriginalID) {
      setSourceOriginalId(initialSourceOriginalID);
    }

    void loadIPs();
    void loadOriginals();
  }, []);

  useEffect(() => {
    if (!tagInput.trim()) {
      setTagSuggestions([]);
      return;
    }

    const timer = window.setTimeout(() => {
      void (async () => {
        try {
          const data = await api.get<TagSearchResponse>(`/api/v1/tags/search?q=${encodeURIComponent(tagInput.trim())}`);
          setTagSuggestions((data.tags || []).slice(0, 10));
        } catch {
          setTagSuggestions([]);
        }
      })();
    }, 250);

    return () => window.clearTimeout(timer);
  }, [tagInput]);

  async function loadIPs() {
    try {
      const data = await api.get<IPSearchResponse>("/api/v1/ips?page=1&page_size=100");
      setIPs(data.ips || []);
    } catch {
      setIPs([]);
    }
  }

  async function loadOriginals() {
    try {
      const data = await api.get<ContentSearchResponse>(
        "/api/v1/contents?zone=original&page=1&page_size=100&sort=newest&time_range=all"
      );
      setOriginals(normalizeContentList(data.contents).map((item) => ({ id: item.id, title: item.title })));
    } catch {
      setOriginals([]);
    }
  }

  function addTag(tag: string) {
    const normalized = tag.trim();
    if (!normalized || tags.includes(normalized) || tags.length >= 10) {
      return;
    }
    setTags((prev) => [...prev, normalized]);
    setTagInput("");
    setTagSuggestions([]);
  }

  function removeTag(tag: string) {
    setTags((prev) => prev.filter((item) => item !== tag));
  }

  async function onSubmit() {
    setError("");

    if (!title.trim()) {
      setError(t('publish.errorTitleRequired'));
      return;
    }
    if (zone === "fanwork" && !ipId) {
      setError(t('publish.errorIpRequired'));
      return;
    }
    if (!coverAsset) {
      setError(t('publish.errorCoverRequired'));
      return;
    }
    if (attachments.length === 0 && contentType !== "text" && contentType !== "other") {
      setError(t('publish.errorAttachmentRequired'));
      return;
    }

    setIsSubmitting(true);
    try {
      const content = await api.post<{ content: { id: number } }>("/api/v1/contents", {
        title: title.trim(),
        description: markdown.trim(),
        zone,
        ip_id: zone === "fanwork" ? Number(ipId) : null,
        source_original_id: zone === "fanwork" && sourceOriginalId ? Number(sourceOriginalId) : null,
        category: zone === "original" ? category.trim() : "",
        content_type: contentType,
        cover_image_url: coverAsset.ossKey,
        is_public: isPublic,
        allow_copy: allowCopy,
        agent_enabled: agentEnabled,
        tags,
        attachments: [
          {
            file_type: "image",
            oss_key: coverAsset.ossKey,
            file_size: coverAsset.fileSize,
            mime_type: coverAsset.mimeType,
            is_primary: true,
          },
          ...attachments.map((item) => ({
            file_type: item.fileType,
            oss_key: item.ossKey,
            file_size: item.fileSize,
            mime_type: item.mimeType,
            is_primary: false,
          })),
        ],
      });

      router.push(zone === "original" ? `/original/${content.content.id}` : `/content/${content.content.id}`);
    } catch (e) {
      setError(e instanceof ApiRequestError ? `${e.code}: ${e.message}` : t('publish.errorPublishFailed'));
    } finally {
      setIsSubmitting(false);
    }
  }

  const sourceOptionExists = sourceOriginalId && originals.some((item) => String(item.id) === sourceOriginalId);

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-8 px-4 py-6">
      <section className="rounded-md border border-border bg-card p-4 shadow-none">
        <h1 className="text-2xl font-bold tracking-tight">{t('publish.title')}</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          {t('publish.subtitle')}
        </p>
        <div className="mt-3">
          <ComplianceCheckBadge
            hasTitle={Boolean(title.trim())}
            hasCover={Boolean(coverAsset)}
            hasAttachment={attachments.length > 0 || contentType === "text" || contentType === "other"}
          />
        </div>
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <h2 className="text-base font-semibold">{t('publish.basicInfo')}</h2>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t('publish.contentTitle')}</label>
            <Input value={title} onChange={(e) => setTitle(e.target.value)} maxLength={120} placeholder={t('publish.titlePlaceholder')} />
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">{t('publish.zone')}</label>
            <select
              value={zone}
              onChange={(e) => setZone(e.target.value as "fanwork" | "original")}
              className="h-10 rounded-md border border-border bg-background px-3 text-sm"
            >
              <option value="original">{t('publish.zoneOriginal')}</option>
              <option value="fanwork">{t('publish.zoneFanwork')}</option>
            </select>
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">{t('publish.contentType')}</label>
            <select
              value={contentType}
              onChange={(e) => setContentType(e.target.value)}
              className="h-10 rounded-md border border-border bg-background px-3 text-sm"
            >
              {CONTENT_TYPES.map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">{t('publish.category')}</label>
            <Input value={category} onChange={(e) => setCategory(e.target.value)} disabled={zone !== "original"} placeholder="film_tv / gaming / literature" />
          </div>

          {zone === "fanwork" ? (
            <>
              <div className="space-y-2 md:col-span-2">
                <label className="text-sm font-medium">{t('publish.linkIp')}</label>
                <Input value={ipKeyword} onChange={(e) => setIPKeyword(e.target.value)} placeholder={t('publish.searchIp')} />
                <select
                  value={ipId}
                  onChange={(e) => setIPID(e.target.value)}
                  className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm"
                >
                  <option value="">{t('publish.selectIp')}</option>
                  {filteredIPs.map((ip) => (
                    <option key={ip.id} value={String(ip.id)}>
                      {ip.name}
                    </option>
                  ))}
                </select>
              </div>

              <div className="space-y-2 md:col-span-2">
                <label className="text-sm font-medium">{t('publish.sourceOriginal')}</label>
                <Input value={originalKeyword} onChange={(e) => setOriginalKeyword(e.target.value)} placeholder={t('publish.searchOriginal')} />
                <select
                  value={sourceOriginalId}
                  onChange={(e) => setSourceOriginalId(e.target.value)}
                  className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm"
                >
                  <option value="">{t('publish.noSourceOriginal')}</option>
                  {sourceOriginalId && !sourceOptionExists ? (
                    <option value={sourceOriginalId}>{t('common.userLabel', { id: sourceOriginalId })}</option>
                  ) : null}
                  {filteredOriginals.map((item) => (
                    <option key={item.id} value={String(item.id)}>
                      {item.title}
                    </option>
                  ))}
                </select>
              </div>
            </>
          ) : null}
        </div>
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <h2 className="text-base font-semibold">{t('publish.coverUpload')}</h2>
        <FileUploader
          fileType="image"
          maxMB={SIZE_LIMITS.image}
          accept="image/*"
          onUploaded={(files) => setCoverAsset(files[0] || null)}
          disabled={isSubmitting}
        />
        {coverAsset ? (
          <div className="space-y-2">
            <p className="text-xs text-muted-foreground">{t('publish.coverUploaded', { name: coverAsset.fileName })}</p>
            <div className="relative h-32 w-24">
              <Image src={coverAsset.ossKey} alt="cover preview" fill className="rounded-md border border-border object-cover" sizes="96px" />
            </div>
          </div>
        ) : null}
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <h2 className="text-base font-semibold">{t('publish.bodyEditor')}</h2>
        <MarkdownEditor value={markdown} onChange={setMarkdown} disabled={isSubmitting} />
        <p className="text-xs text-muted-foreground">{t('publish.bodyEditorHint')}</p>
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <h2 className="text-base font-semibold">{t('publish.contentAttachments')}</h2>
        <FileUploader
          fileType={uploadFileType}
          maxMB={SIZE_LIMITS[uploadFileType]}
          accept={
            uploadFileType === "image"
              ? "image/*"
              : uploadFileType === "video"
                ? "video/*"
                : uploadFileType === "mod"
                    ? ".zip,application/zip"
                    : uploadFileType === "sheet_music"
                      ? ".mid,.midi,.xml,.mxl,.mscz,.mscx,.pdf"
                      : "text/*,application/pdf"
          }
          onUploaded={setAttachments}
          disabled={isSubmitting}
        />
        {attachments.length > 0 ? (
          <ul className="space-y-1 text-xs text-muted-foreground">
            {attachments.map((item, index) => (
              <li key={`${item.ossKey}-${index}`}>{item.fileName}</li>
            ))}
          </ul>
        ) : null}
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <h2 className="text-base font-semibold">{t('publish.tagsAndMeta')}</h2>
        <div className="space-y-2">
          <label className="text-sm font-medium">{t('publish.tags')}</label>
          <div className="flex gap-2">
            <Input
              value={tagInput}
              onChange={(e) => setTagInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  addTag(tagInput);
                }
              }}
              placeholder={t('publish.tagPlaceholder')}
            />
            <Button type="button" variant="outline" onClick={() => addTag(tagInput)}>
              {t('publish.add')}
            </Button>
          </div>
          {tagSuggestions.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {tagSuggestions.map((tag) => (
                <Badge key={tag} variant="secondary" className="cursor-pointer" onClick={() => addTag(tag)}>
                  {tag}
                </Badge>
              ))}
            </div>
          ) : null}
          {tags.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {tags.map((tag) => (
                <Badge key={tag} variant="secondary" className="cursor-pointer" onClick={() => removeTag(tag)}>
                  {tag}
                </Badge>
              ))}
            </div>
          ) : null}
        </div>

        <UploadAssistPanel contentType={contentType} />
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <h2 className="text-base font-semibold">{t('publish.permissions')}</h2>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={isPublic} onChange={(e) => setIsPublic(e.target.checked)} />
          {t('publish.publicVisibility')}
        </label>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={allowCopy} onChange={(e) => setAllowCopy(e.target.checked)} />
          {t('publish.allowCopyDownload')}
        </label>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={agentEnabled} onChange={(e) => setAgentEnabled(e.target.checked)} />
          {t('publish.enableAgentDeploy')}
        </label>
      </section>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <div className="flex justify-end gap-3">
        <Button variant="outline" onClick={() => router.back()} disabled={isSubmitting}>
          {t('common.cancel')}
        </Button>
        <Button onClick={() => void onSubmit()} disabled={isSubmitting}>
          {isSubmitting ? t('publish.publishing') : t('publish.publishButton')}
        </Button>
      </div>
    </div>
  );
}
