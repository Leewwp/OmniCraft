"use client";

import { useEffect, useMemo, useState } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
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

const CONTENT_TYPES = [
  { value: "text", label: "文字" },
  { value: "image", label: "图片" },
  { value: "video", label: "视频" },
  { value: "audio", label: "音频" },
  { value: "mod", label: "Mod" },
  { value: "sheet_music", label: "乐谱" },
  { value: "other", label: "其他" },
];

export default function PublishPage() {
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
      setError("请填写标题");
      return;
    }
    if (zone === "fanwork" && !ipId) {
      setError("二创区发布必须选择 IP");
      return;
    }
    if (!coverAsset) {
      setError("请上传封面图");
      return;
    }
    if (attachments.length === 0 && contentType !== "text" && contentType !== "other") {
      setError("请至少上传一个内容附件");
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
      setError(e instanceof ApiRequestError ? `${e.code}: ${e.message}` : "发布失败，请稍后重试");
    } finally {
      setIsSubmitting(false);
    }
  }

  const sourceOptionExists = sourceOriginalId && originals.some((item) => String(item.id) === sourceOriginalId);

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-8 px-4 py-6">
      <section className="rounded-md border border-border bg-card p-4 shadow-none">
        <h1 className="text-2xl font-bold tracking-tight">发布内容</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          上传内容会进入审核流程；二创内容可以选择一个来源原创，方便读者从原创详情跳转到相关二创。
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
        <h2 className="text-base font-semibold">基础信息</h2>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <label className="text-sm font-medium">标题</label>
            <Input value={title} onChange={(e) => setTitle(e.target.value)} maxLength={120} placeholder="输入作品标题" />
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">分区</label>
            <select
              value={zone}
              onChange={(e) => setZone(e.target.value as "fanwork" | "original")}
              className="h-10 rounded-md border border-border bg-background px-3 text-sm"
            >
              <option value="original">原创区</option>
              <option value="fanwork">二创区</option>
            </select>
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">内容类型</label>
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
            <label className="text-sm font-medium">分类（原创区）</label>
            <Input value={category} onChange={(e) => setCategory(e.target.value)} disabled={zone !== "original"} placeholder="例如 film_tv / gaming / literature" />
          </div>

          {zone === "fanwork" ? (
            <>
              <div className="space-y-2 md:col-span-2">
                <label className="text-sm font-medium">关联 IP</label>
                <Input value={ipKeyword} onChange={(e) => setIPKeyword(e.target.value)} placeholder="搜索 IP 名称" />
                <select
                  value={ipId}
                  onChange={(e) => setIPID(e.target.value)}
                  className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm"
                >
                  <option value="">请选择 IP</option>
                  {filteredIPs.map((ip) => (
                    <option key={ip.id} value={String(ip.id)}>
                      {ip.name}
                    </option>
                  ))}
                </select>
              </div>

              <div className="space-y-2 md:col-span-2">
                <label className="text-sm font-medium">来源原创（可选）</label>
                <Input value={originalKeyword} onChange={(e) => setOriginalKeyword(e.target.value)} placeholder="搜索原创标题" />
                <select
                  value={sourceOriginalId}
                  onChange={(e) => setSourceOriginalId(e.target.value)}
                  className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm"
                >
                  <option value="">不关联原创来源</option>
                  {sourceOriginalId && !sourceOptionExists ? (
                    <option value={sourceOriginalId}>当前来源 #{sourceOriginalId}</option>
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
        <h2 className="text-base font-semibold">封面上传</h2>
        <FileUploader
          fileType="image"
          maxMB={SIZE_LIMITS.image}
          accept="image/*"
          onUploaded={(files) => setCoverAsset(files[0] || null)}
          disabled={isSubmitting}
        />
        {coverAsset ? (
          <div className="space-y-2">
            <p className="text-xs text-muted-foreground">已上传封面：{coverAsset.fileName}</p>
            <div className="relative h-32 w-24">
              <Image src={coverAsset.ossKey} alt="cover preview" fill className="rounded-md border border-border object-cover" sizes="96px" />
            </div>
          </div>
        ) : null}
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <h2 className="text-base font-semibold">正文编辑（Markdown）</h2>
        <MarkdownEditor value={markdown} onChange={setMarkdown} disabled={isSubmitting} />
        <p className="text-xs text-muted-foreground">Markdown 正文会随发布内容一起保存，并进入内容审核。</p>
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <h2 className="text-base font-semibold">内容附件</h2>
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
        <h2 className="text-base font-semibold">标签与辅助</h2>
        <div className="space-y-2">
          <label className="text-sm font-medium">标签</label>
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
              placeholder="输入标签后回车"
            />
            <Button type="button" variant="outline" onClick={() => addTag(tagInput)}>
              添加
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
        <h2 className="text-base font-semibold">权限设置</h2>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={isPublic} onChange={(e) => setIsPublic(e.target.checked)} />
          公开展示
        </label>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={allowCopy} onChange={(e) => setAllowCopy(e.target.checked)} />
          允许复制/下载
        </label>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={agentEnabled} onChange={(e) => setAgentEnabled(e.target.checked)} />
          启用 Agent 部署入口
        </label>
      </section>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <div className="flex justify-end gap-3">
        <Button variant="outline" onClick={() => router.back()} disabled={isSubmitting}>
          取消
        </Button>
        <Button onClick={() => void onSubmit()} disabled={isSubmitting}>
          {isSubmitting ? "发布中..." : "发布"}
        </Button>
      </div>
    </div>
  );
}
