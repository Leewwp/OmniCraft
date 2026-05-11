"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { ArrowLeft, Send, ChevronDown, Image, Eye, EyeOff, MessageCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { FileUploader } from "@/components/content/FileUploader";
import { MarkdownEditor } from "@/components/content/MarkdownEditor";
import { TagBadge } from "@/components/ui/TagBadge";
import { cn } from "@/lib/utils";

const ORIGINAL_CATEGORIES = [
  "film_tv", "gaming", "literature", "pet", "food",
  "beauty_fashion", "home", "tech_digital", "travel", "sports", "productivity",
];

const CATEGORY_LABELS: Record<string, string> = {
  film_tv: "影视", gaming: "游戏", literature: "文学", pet: "宠物",
  food: "美食", beauty_fashion: "美妆穿搭", home: "家居",
  tech_digital: "数码科技", travel: "旅行", sports: "运动", productivity: "效率",
};

// Types that use file upload as primary content
const FILE_PRIMARY_TYPES = ["image", "video", "audio", "sheet_music", "mod", "template"];
// Types that use text editor as primary
const TEXT_PRIMARY_TYPES = ["article", "prompt", "other"];

interface PublishFormProps {
  zone: "original" | "fanwork";
  contentType: string;
  onBack: () => void;
}

export function PublishForm({ zone, contentType, onBack }: PublishFormProps) {
  const router = useRouter();
  const { toast } = useToast();
  const isFilePrimary = FILE_PRIMARY_TYPES.includes(contentType);

  // Core fields
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [briefDesc, setBriefDesc] = useState("");

  // Zone-specific
  const [category, setCategory] = useState("");
  const [ipSearch, setIpSearch] = useState("");
  const [sourceSearch, setSourceSearch] = useState("");

  // Settings
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");
  const [hasCustomCover, setHasCustomCover] = useState(false);
  const [allowComments, setAllowComments] = useState(true);
  const [allowCopy, setAllowCopy] = useState(true);
  const [isPublic, setIsPublic] = useState(true);
  const [agentEnabled, setAgentEnabled] = useState(false);

  const [submitting, setSubmitting] = useState(false);

  function addTag() {
    const val = tagInput.trim();
    if (val && !tags.includes(val) && tags.length < 10) {
      setTags([...tags, val]);
    }
    setTagInput("");
  }

  const description = isFilePrimary ? briefDesc : body;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) { toast("error", "请输入标题"); return; }
    if (zone === "original" && !category) { toast("error", "请选择分类"); return; }
    if (zone === "fanwork" && !ipSearch.trim()) { toast("error", "请选择关联 IP"); return; }

    setSubmitting(true);
    try {
      const payload: Record<string, unknown> = {
        title: title.trim(), zone, content_type: contentType,
        body: description, tags, is_public: isPublic,
        allow_copy: allowCopy, agent_enabled: agentEnabled,
        allow_comments: allowComments,
      };
      if (zone === "original") payload.category = category;
      await api.post("/api/v1/contents", payload);
      toast("success", "发布成功！");
      router.push("/studio/contents");
    } catch {
      toast("error", "发布失败，请重试");
    } finally {
      setSubmitting(false);
    }
  }

  const fileType = (
    ["image", "video", "sheet_music", "mod"].includes(contentType)
      ? contentType
      : "text"
  ) as "image" | "video" | "text" | "mod" | "sheet_music";

  const maxMB = contentType === "mod" ? 500 : contentType === "sheet_music" ? 50 : contentType === "video" ? 300 : 20;

  return (
    <form onSubmit={handleSubmit} className="max-w-2xl space-y-6">
      {/* Back */}
      <button type="button" onClick={onBack}
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors">
        <ArrowLeft className="h-4 w-4" /> 返回选择类型
      </button>

      {/* Title — always first */}
      <div>
        <label className="mb-1.5 block text-sm font-medium text-foreground">
          标题 <span className="text-destructive">*</span>
        </label>
        <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="输入作品标题..." maxLength={200} />
      </div>

      {/* Zone-specific: Original category */}
      {zone === "original" && (
        <div>
          <label className="mb-1.5 block text-sm font-medium text-foreground">
            分类 <span className="text-destructive">*</span>
          </label>
          <div className="flex flex-wrap gap-2">
            {ORIGINAL_CATEGORIES.map((cat) => (
              <button key={cat} type="button" onClick={() => setCategory(cat)}
                className={cn(
                  "rounded-full border px-3.5 py-1.5 text-xs font-medium transition-all",
                  category === cat
                    ? "border-[var(--accent-emphasis)] bg-[var(--accent-subtle)] text-[var(--accent-emphasis)]"
                    : "border-border text-muted-foreground hover:border-border/80 hover:text-foreground"
                )}>
                {CATEGORY_LABELS[cat]}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Zone-specific: Fanwork IP + Source */}
      {zone === "fanwork" && (
        <div className="space-y-3">
          <div>
            <label className="mb-1.5 block text-sm font-medium text-foreground">
              关联 IP <span className="text-destructive">*</span>
            </label>
            <Input value={ipSearch} onChange={(e) => setIpSearch(e.target.value)} placeholder="搜索并选择 IP..." />
          </div>
          <div>
            <label className="mb-1.5 block text-sm font-medium text-foreground">来源原创（可选）</label>
            <Input value={sourceSearch} onChange={(e) => setSourceSearch(e.target.value)} placeholder="搜索原创内容标题..." />
          </div>
        </div>
      )}

      {/* Primary content area: Text types → Markdown */}
      {TEXT_PRIMARY_TYPES.includes(contentType) && (
        <div>
          <label className="mb-1.5 block text-sm font-medium text-foreground">正文</label>
          <MarkdownEditor value={body} onChange={(val) => setBody(val)} />
        </div>
      )}

      {/* Primary content area: File types → upload + brief desc */}
      {isFilePrimary && (
        <div className="space-y-4">
          <div>
            <label className="mb-1.5 block text-sm font-medium text-foreground">
              {contentType === "image" ? "图片上传" : contentType === "video" ? "视频上传" : contentType === "audio" ? "音频上传" : contentType === "sheet_music" ? "乐谱文件" : contentType === "mod" ? "Mod 包上传" : "文件上传"}
              <span className="text-destructive"> *</span>
            </label>
            <FileUploader fileType={fileType} maxMB={maxMB} accept="*" onUploaded={() => {}} />
            <p className="mt-1 text-xs text-muted-foreground">
              {contentType === "sheet_music" ? "支持 mid, midi, xml, mxl, mscz, mscx, pdf" :
               contentType === "mod" ? "支持 zip 压缩包，最大 500MB" :
               contentType === "video" ? "最大 300MB，时长 ≤ 180 秒" :
               contentType === "image" ? "最大 20MB" : ""}
            </p>
          </div>
          <div>
            <label className="mb-1.5 block text-sm font-medium text-foreground">简介（可选）</label>
            <textarea
              value={briefDesc}
              onChange={(e) => setBriefDesc(e.target.value)}
              rows={3}
              placeholder="简单描述一下你的作品..."
              className="w-full rounded-lg border border-border bg-card px-3 py-2 text-sm placeholder:text-muted-foreground/60 focus:outline-none focus:ring-2 focus:ring-ring/20 resize-none"
            />
          </div>
        </div>
      )}

      {/* ──── Publishing Settings Panel ──── */}
      <div className="rounded-xl border border-border/60 bg-card">
        <div className="flex items-center gap-2 px-5 py-3 border-b border-border/40">
          <ChevronDown className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm font-semibold text-foreground">发布设置</span>
        </div>
        <div className="p-5 space-y-4">

          {/* Cover toggle */}
          <div className="flex items-center justify-between">
            <div>
              <span className="text-sm font-medium text-foreground">自定义封面</span>
              <p className="text-xs text-muted-foreground">关闭时自动使用正文前 200 字生成文字封面</p>
            </div>
            <button type="button"
              onClick={() => setHasCustomCover(!hasCustomCover)}
              className={cn(
                "relative inline-flex h-6 w-11 items-center rounded-full transition-colors",
                hasCustomCover ? "bg-[var(--accent-emphasis)]" : "bg-muted-foreground/25"
              )}>
              <span className={cn("inline-block h-4 w-4 rounded-full bg-white transition-transform", hasCustomCover ? "translate-x-6" : "translate-x-1")} />
            </button>
          </div>

          {/* Custom cover upload (conditional) */}
          {hasCustomCover && (
            <div className="pl-2 border-l-2 border-[var(--accent-subtle)]">
              <FileUploader fileType="image" maxMB={20} accept="image/*" onUploaded={() => {}} />
            </div>
          )}

          {/* Comment toggle */}
          <div className="flex items-center justify-between border-t border-border/40 pt-4">
            <div className="flex items-center gap-2">
              <MessageCircle className="h-4 w-4 text-muted-foreground" />
              <span className="text-sm font-medium text-foreground">允许评论</span>
            </div>
            <button type="button"
              onClick={() => setAllowComments(!allowComments)}
              className={cn(
                "relative inline-flex h-6 w-11 items-center rounded-full transition-colors",
                allowComments ? "bg-[var(--accent-emphasis)]" : "bg-muted-foreground/25"
              )}>
              <span className={cn("inline-block h-4 w-4 rounded-full bg-white transition-transform", allowComments ? "translate-x-6" : "translate-x-1")} />
            </button>
          </div>

          {/* Tags */}
          <div className="border-t border-border/40 pt-4">
            <label className="mb-1.5 block text-sm font-medium text-foreground">关联标签</label>
            <div className="flex gap-2">
              <Input value={tagInput} onChange={(e) => setTagInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addTag(); } }}
                placeholder="输入标签后按回车添加..." className="flex-1" />
              <Button type="button" variant="outline" size="sm" onClick={addTag}>添加</Button>
            </div>
            {tags.length > 0 && (
              <div className="mt-2 flex flex-wrap gap-1.5">
                {tags.map((tag) => (
                  <TagBadge key={tag} color="blue" onRemove={() => setTags(tags.filter((t) => t !== tag))}>{tag}</TagBadge>
                ))}
              </div>
            )}
          </div>

          {/* Allow copy (fanwork only) */}
          {zone === "fanwork" && (
            <div className="flex items-center justify-between border-t border-border/40 pt-4">
              <div>
                <span className="text-sm font-medium text-foreground">允许复制/PR 协同</span>
                <p className="text-xs text-muted-foreground">允许其他创作者基于此内容提交修改建议</p>
              </div>
              <button type="button"
                onClick={() => setAllowCopy(!allowCopy)}
                className={cn(
                  "relative inline-flex h-6 w-11 items-center rounded-full transition-colors",
                  allowCopy ? "bg-[var(--accent-emphasis)]" : "bg-muted-foreground/25"
                )}>
                <span className={cn("inline-block h-4 w-4 rounded-full bg-white transition-transform", allowCopy ? "translate-x-6" : "translate-x-1")} />
              </button>
            </div>
          )}

          {/* Public toggle */}
          <div className="flex items-center justify-between border-t border-border/40 pt-4">
            <div className="flex items-center gap-2">
              {isPublic ? <Eye className="h-4 w-4 text-muted-foreground" /> : <EyeOff className="h-4 w-4 text-muted-foreground" />}
              <span className="text-sm font-medium text-foreground">公开可见</span>
            </div>
            <button type="button"
              onClick={() => setIsPublic(!isPublic)}
              className={cn(
                "relative inline-flex h-6 w-11 items-center rounded-full transition-colors",
                isPublic ? "bg-[var(--accent-emphasis)]" : "bg-muted-foreground/25"
              )}>
              <span className={cn("inline-block h-4 w-4 rounded-full bg-white transition-transform", isPublic ? "translate-x-6" : "translate-x-1")} />
            </button>
          </div>

          {/* Agent deploy (mod/prompt only) */}
          {(contentType === "mod" || contentType === "prompt") && (
            <div className="flex items-center justify-between border-t border-border/40 pt-4">
              <div>
                <span className="text-sm font-medium text-foreground">Agent 部署入口</span>
                <p className="text-xs text-muted-foreground">允许用户通过客户端一键安装/部署</p>
              </div>
              <button type="button"
                onClick={() => setAgentEnabled(!agentEnabled)}
                className={cn(
                  "relative inline-flex h-6 w-11 items-center rounded-full transition-colors",
                  agentEnabled ? "bg-[var(--accent-emphasis)]" : "bg-muted-foreground/25"
                )}>
                <span className={cn("inline-block h-4 w-4 rounded-full bg-white transition-transform", agentEnabled ? "translate-x-6" : "translate-x-1")} />
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Bottom actions */}
      <div className="flex items-center gap-3 pt-2">
        <Button type="submit" size="lg" disabled={submitting} className="gap-2 rounded-full px-8">
          <Send className="h-4 w-4" />
          {submitting ? "发布中..." : "发布"}
        </Button>
        <Button type="button" variant="ghost" onClick={onBack}>取消</Button>
      </div>
    </form>
  );
}
