"use client";

import { useState } from "react";
import { ContentTypeGrid, type ContentType } from "@/components/studio/ContentTypeGrid";
import { PublishForm } from "@/components/studio/PublishForm";

const CONTENT_TYPES: ContentType[] = [
  { value: "image", icon: "🖼️", label: "图片", description: "摄影、插画、设计作品" },
  { value: "video", icon: "🎬", label: "视频", description: "短片、Vlog、教程" },
  { value: "article", icon: "📝", label: "文章", description: "随笔、教程、故事" },
  { value: "audio", icon: "🎵", label: "音频", description: "音乐、播客、翻唱" },
  { value: "sheet_music", icon: "🎼", label: "乐谱", description: "原创曲谱、改编" },
  { value: "template", icon: "📋", label: "效率模板", description: "Notion、Excel 等模板" },
  { value: "prompt", icon: "🤖", label: "AI 提示词", description: "Prompt 模板分享" },
  { value: "other", icon: "📦", label: "其他", description: "其他原创内容" },
];

export default function PublishOriginalPage() {
  const [selectedType, setSelectedType] = useState<string | null>(null);

  if (selectedType) {
    return (
      <div className="max-w-2xl">
        <h1 className="mb-6 text-xl font-bold text-foreground">发布原创</h1>
        <PublishForm
          zone="original"
          contentType={selectedType}
          onBack={() => setSelectedType(null)}
        />
      </div>
    );
  }

  return (
    <div>
      <h1 className="mb-1 text-xl font-bold text-foreground">发布原创</h1>
      <p className="mb-6 text-sm text-muted-foreground">选择内容类型开始创作</p>
      <ContentTypeGrid
        types={CONTENT_TYPES}
        selected={selectedType}
        onSelect={setSelectedType}
      />
    </div>
  );
}
