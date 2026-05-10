"use client";

import { useState } from "react";
import { ContentTypeGrid, type ContentType } from "@/components/studio/ContentTypeGrid";
import { PublishForm } from "@/components/studio/PublishForm";

const CONTENT_TYPES: ContentType[] = [
  { value: "image", icon: "🖼️", label: "图片", description: "同人插画、Cosplay 摄影" },
  { value: "video", icon: "🎬", label: "视频", description: "MAD、AMV、剪辑" },
  { value: "article", icon: "📝", label: "文章", description: "同人文、考据、分析" },
  { value: "audio", icon: "🎵", label: "音频", description: "翻唱、Remix、配音" },
  { value: "sheet_music", icon: "🎼", label: "乐谱", description: "主题曲改编谱" },
  { value: "mod", icon: "🧩", label: "Mod", description: "游戏模组、材质包" },
  { value: "prompt", icon: "🤖", label: "AI 提示词", description: "角色 LoRA 提示词" },
  { value: "other", icon: "📦", label: "其他", description: "其他二创内容" },
];

export default function PublishFanworkPage() {
  const [selectedType, setSelectedType] = useState<string | null>(null);

  if (selectedType) {
    return (
      <div className="max-w-2xl">
        <h1 className="mb-6 text-xl font-bold text-foreground">发布二创</h1>
        <PublishForm
          zone="fanwork"
          contentType={selectedType}
          onBack={() => setSelectedType(null)}
        />
      </div>
    );
  }

  return (
    <div>
      <h1 className="mb-1 text-xl font-bold text-foreground">发布二创</h1>
      <p className="mb-6 text-sm text-muted-foreground">选择内容类型开始创作</p>
      <ContentTypeGrid
        types={CONTENT_TYPES}
        selected={selectedType}
        onSelect={setSelectedType}
      />
    </div>
  );
}
