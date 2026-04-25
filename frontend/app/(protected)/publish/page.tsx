"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { FileUploader, UploadedAsset } from "@/components/content/FileUploader";
import { MarkdownEditor } from "@/components/content/MarkdownEditor";
import { UploadAssistPanel } from "@/components/content/UploadAssistPanel";
import { ComplianceCheckBadge } from "@/components/content/ComplianceCheckBadge";
import { api, ApiRequestError } from "@/lib/api";

interface IPItem {
  id: number;
  name: string;
}

interface IPSearchResponse {
  ips?: IPItem[];
}

interface TagSearchResponse {
  tags?: string[];
}

const SIZE_LIMITS = {
  image: 20,
  video: 300,
  text: 10,
  mod: 500,
  sheet_music: 50,
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
  const [category, setCategory] = useState("");
  const [coverAsset, setCoverAsset] = useState<UploadedAsset | null>(null);
  const [attachments, setAttachments] = useState<UploadedAsset[]>([]);
  const [markdown, setMarkdown] = useState("");
  const [isPublic, setIsPublic] = useState(true);
  const [allowCopy, setAllowCopy] = useState(false);
  const [agentEnabled, setAgentEnabled] = useState(false);
  const [ipKeyword, setIPKeyword] = useState("");
  const [tagInput, setTagInput] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [tagSuggestions, setTagSuggestions] = useState<string[]>([]);
  const [ips, setIPs] = useState<IPItem[]>([]);
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  const uploadFileType = useMemo(() => {
    if (contentType === "image" || contentType === "video" || contentType === "mod" || contentType === "sheet_music") {
      return contentType;
    }
    return "text";
  }, [contentType]);

  const zoneContentTypes = useMemo(() => {
    if (zone === "fanwork") {
      return CONTENT_TYPES.filter((item) => item.value !== "mod");
    }
    return CONTENT_TYPES;
  }, [zone]);

  const filteredIPs = useMemo(() => {
    const keyword = ipKeyword.trim().toLowerCase();
    if (!keyword) {
      return ips;
    }
    return ips.filter((ip) => ip.name.toLowerCase().includes(keyword));
  }, [ips, ipKeyword]);

  useEffect(() => {
    void (async () => {
      try {
        const data = await api.get<IPSearchResponse>("/api/v1/ips?page=1&page_size=100");
        setIPs(data.ips || []);
      } catch {
        setIPs([]);
      }
    })();
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
      setError("请至少上传一个附件");
      return;
    }

    setIsSubmitting(true);

    try {
      const content = await api.post<{ content: { id: number } }>("/api/v1/contents", {
        title: title.trim(),
        zone,
        ip_id: zone === "fanwork" ? Number(ipId) : null,
        category: category.trim(),
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

      router.push(`/original/${content.content.id}`);
    } catch (e) {
      if (e instanceof ApiRequestError) {
        setError(`${e.code}: ${e.message}`);
      } else {
        setError("发布失败，请稍后重试");
      }
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-8 px-4 py-6">
      <section className="rounded-md border border-border bg-card p-4 shadow-none">
        <h1 className="text-2xl font-bold tracking-tight">发布内容</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          文件大小限制来自后端配置：视频 ≤ 300MB，图片 ≤ 20MB，文本 ≤ 10MB，Mod ≤ 500MB，乐谱 ≤ 50MB。
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
              {zoneContentTypes.map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">分类（可选）</label>
            <Input value={category} onChange={(e) => setCategory(e.target.value)} placeholder="例如：影视 / 游戏 / 文学" />
          </div>

          {zone === "fanwork" ? (
            <div className="space-y-2 md:col-span-2">
              <label className="text-sm font-medium">关联 IP</label>
              <Input
                value={ipKeyword}
                onChange={(e) => setIPKeyword(e.target.value)}
                placeholder="搜索 IP 名称"
              />
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
            <img
              src={coverAsset.ossKey}
              alt="cover preview"
              className="h-32 w-24 rounded-md border border-border object-cover"
            />
          </div>
        ) : null}
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <h2 className="text-base font-semibold">正文编辑（Markdown）</h2>
        <MarkdownEditor value={markdown} onChange={setMarkdown} disabled={isSubmitting} />
        <p className="text-xs text-muted-foreground">
          当前后端发布接口尚未接收 markdown 正文字段，已保留编辑能力以便后续 Task 扩展。
        </p>
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
          multiple
          onUploaded={(files) => {
            setAttachments((prev) => [...prev, ...files]);
          }}
          disabled={isSubmitting}
        />

        <UploadAssistPanel contentType={contentType} />

        {attachments.length > 0 ? (
          <ul className="space-y-1 text-xs text-muted-foreground">
            {attachments.map((item, index) => (
              <li key={`${item.ossKey}-${index}`}>{item.fileName}</li>
            ))}
          </ul>
        ) : null}

        {contentType === "image" && attachments.length > 0 ? (
          <div className="space-y-2 rounded-md border border-border bg-muted/20 p-3">
            <p className="text-xs text-muted-foreground">图片类内容可直接从附件选取封面：</p>
            <div className="flex flex-wrap gap-2">
              {attachments.map((item, index) => (
                <Button
                  key={`${item.ossKey}-cover-${index}`}
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setCoverAsset(item)}
                >
                  设为封面：{item.fileName}
                </Button>
              ))}
            </div>
          </div>
        ) : null}
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <h2 className="text-base font-semibold">标签与权限</h2>

        <div className="space-y-2">
          <label className="text-sm font-medium">标签（最多 10 个）</label>
          <div className="flex gap-2">
            <Input
              value={tagInput}
              onChange={(e) => setTagInput(e.target.value)}
              placeholder="输入标签后回车"
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  addTag(tagInput);
                }
              }}
            />
            <Button type="button" variant="outline" onClick={() => addTag(tagInput)}>
              添加
            </Button>
          </div>

          {tagSuggestions.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {tagSuggestions.map((tag) => (
                <button
                  type="button"
                  key={tag}
                  onClick={() => addTag(tag)}
                  className="rounded-md border border-border px-2 py-1 text-xs hover:bg-muted"
                >
                  {tag}
                </button>
              ))}
            </div>
          ) : null}

          <div className="flex flex-wrap gap-2">
            {tags.map((tag) => (
              <Badge key={tag} variant="secondary" className="cursor-pointer" onClick={() => removeTag(tag)}>
                {tag} ×
              </Badge>
            ))}
          </div>
        </div>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={isPublic} onChange={(e) => setIsPublic(e.target.checked)} />
            公开可见
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={allowCopy} onChange={(e) => setAllowCopy(e.target.checked)} />
            允许二创（Allow Copy）
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={agentEnabled} onChange={(e) => setAgentEnabled(e.target.checked)} />
            启用智能助手（Agent）
          </label>
        </div>
      </section>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <div className="flex justify-end gap-3">
        <Button type="button" variant="outline" onClick={() => router.back()} disabled={isSubmitting}>
          取消
        </Button>
        <Button type="button" onClick={() => void onSubmit()} disabled={isSubmitting}>
          {isSubmitting ? "发布中..." : "提交审核"}
        </Button>
      </div>
    </div>
  );
}
