export interface CategoryOption {
  key: string;
  label: string;
}

export const ipCategoryOptions: CategoryOption[] = [
  { key: "all", label: "全部" },
  { key: "text", label: "文字" },
  { key: "image", label: "图片" },
  { key: "video", label: "视频" },
  { key: "audio", label: "音频" },
  { key: "mod", label: "Mod" },
  { key: "prompt", label: "AI 提示词" },
  { key: "sheet_music", label: "乐谱" },
  { key: "other", label: "其他" },
];

export function getCategoryLabel(key: string): string {
  return ipCategoryOptions.find((item) => item.key === key)?.label || key;
}
