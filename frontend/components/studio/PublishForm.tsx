"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { ArrowLeft, Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { FileUploader } from "@/components/content/FileUploader";
import { MarkdownEditor } from "@/components/content/MarkdownEditor";
import { TagBadge } from "@/components/ui/TagBadge";
import { ComplianceCheckBadge } from "@/components/content/ComplianceCheckBadge";
import { UploadAssistPanel } from "@/components/content/UploadAssistPanel";
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

interface PublishFormProps {
  zone: "original" | "fanwork";
  contentType: string;
  onBack: () => void;
}

export function PublishForm({ zone, contentType, onBack }: PublishFormProps) {
  const router = useRouter();
  const { toast } = useToast();

  const [title, setTitle] = useState("");
  const [category, setCategory] = useState("");
  const [body, setBody] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");
  const [hasCover, setHasCover] = useState(false);
  const [ipSearch, setIpSearch] = useState("");
  const [isPublic, setIsPublic] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  function addTag() {
    const val = tagInput.trim();
    if (val && !tags.includes(val) && tags.length < 10) {
      setTags([...tags, val]);
    }
    setTagInput("");
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) {
      toast("error", "请输入标题");
      return;
    }
    if (zone === "original" && !category) {
      toast("error", "请选择分类");
      return;
    }

    setSubmitting(true);
    try {
      const payload: Record<string, unknown> = {
        title: title.trim(),
        zone,
        content_type: contentType,
        body,
        tags,
        is_public: isPublic,
        allow_copy: true,
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

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      <button
        type="button"
        onClick={onBack}
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
      >
        <ArrowLeft className="h-4 w-4" />
        返回选择类型
      </button>

      {/* Title */}
      <div>
        <label className="mb-1.5 block text-sm font-medium text-foreground">
          标题 <span className="text-destructive">*</span>
        </label>
        <Input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="输入作品标题..."
          maxLength={200}
        />
      </div>

      {/* Original: category selector */}
      {zone === "original" && (
        <div>
          <label className="mb-1.5 block text-sm font-medium text-foreground">
            分类 <span className="text-destructive">*</span>
          </label>
          <div className="flex flex-wrap gap-2">
            {ORIGINAL_CATEGORIES.map((cat) => (
              <button
                key={cat}
                type="button"
                onClick={() => setCategory(cat)}
                className={cn(
                  "rounded-full border px-3 py-1.5 text-xs font-medium transition-colors",
                  category === cat
                    ? "border-accent-emphasis bg-accent-subtle text-accent-emphasis"
                    : "border-border text-muted-foreground hover:border-border/80 hover:text-foreground"
                )}
              >
                {CATEGORY_LABELS[cat] || cat}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Fanwork: IP selector */}
      {zone === "fanwork" && (
        <div>
          <label className="mb-1.5 block text-sm font-medium text-foreground">
            关联 IP <span className="text-destructive">*</span>
          </label>
          <Input
            value={ipSearch}
            onChange={(e) => setIpSearch(e.target.value)}
            placeholder="搜索并选择 IP..."
          />
          <p className="mt-1 text-xs text-muted-foreground">
            输入 IP 名称进行搜索（功能开发中）
          </p>
        </div>
      )}

      {/* Body */}
      <div>
        <label className="mb-1.5 block text-sm font-medium text-foreground">
          正文
        </label>
        <MarkdownEditor
          value={body}
          onChange={(val) => setBody(val)}
        />
      </div>

      {/* Cover upload */}
      <div>
        <label className="mb-1.5 block text-sm font-medium text-foreground">
          封面图片
        </label>
        <FileUploader
          fileType="image"
          maxMB={20}
          accept="image/*"
          onUploaded={() => setHasCover(true)}
        />
      </div>

      {/* Attachment upload (for mod/sheet_music types) */}
      {["mod", "sheet_music", "audio"].includes(contentType) && (
        <div>
          <label className="mb-1.5 block text-sm font-medium text-foreground">
            附件
          </label>
          <FileUploader
            fileType={fileType}
            maxMB={contentType === "mod" ? 500 : 50}
            accept="*"
            onUploaded={() => {}}
          />
        </div>
      )}

      {/* Tags */}
      <div>
        <label className="mb-1.5 block text-sm font-medium text-foreground">
          标签
        </label>
        <div className="flex gap-2">
          <Input
            value={tagInput}
            onChange={(e) => setTagInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                addTag();
              }
            }}
            placeholder="输入标签后按回车添加..."
            className="flex-1"
          />
          <Button type="button" variant="outline" size="sm" onClick={addTag}>
            添加
          </Button>
        </div>
        {tags.length > 0 && (
          <div className="mt-2 flex flex-wrap gap-1.5">
            {tags.map((tag) => (
              <TagBadge key={tag} color="blue" onRemove={() => setTags(tags.filter((t) => t !== tag))}>
                {tag}
              </TagBadge>
            ))}
          </div>
        )}
      </div>

      {/* Visibility */}
      <label className="flex items-center gap-2 text-sm cursor-pointer">
        <input
          type="checkbox"
          checked={isPublic}
          onChange={(e) => setIsPublic(e.target.checked)}
          className="rounded border-border"
        />
        公开可见
      </label>

      {/* AI assist bar */}
      <div className="flex flex-wrap items-center gap-4 rounded-lg border border-border bg-muted/30 p-4">
        <ComplianceCheckBadge
          hasCover={hasCover}
          hasAttachment={false}
          hasTitle={title.trim().length > 0}
        />
        <UploadAssistPanel contentType={contentType} />
      </div>

      {/* Submit */}
      <div className="flex items-center gap-3 pt-2">
        <Button type="submit" size="lg" disabled={submitting} className="gap-2">
          <Send className="h-4 w-4" />
          {submitting ? "发布中..." : "发布创作"}
        </Button>
        <Button type="button" variant="ghost" onClick={onBack}>
          取消
        </Button>
      </div>
    </form>
  );
}
