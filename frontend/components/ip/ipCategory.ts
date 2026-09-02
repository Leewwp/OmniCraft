export interface CategoryOption {
  key: string;
  label: string;
}

export const ipCategoryOptions: CategoryOption[] = [
  { key: "all", label: "home.all" },
  { key: "text", label: "home.text" },
  { key: "image", label: "home.image" },
  { key: "video", label: "home.video" },
  { key: "audio", label: "home.audio" },
  { key: "mod", label: "home.mod" },
  { key: "prompt", label: "home.aiPrompt" },
  { key: "sheet_music", label: "home.sheetMusic" },
  { key: "other", label: "home.other" },
];

export function getCategoryLabel(key: string): string {
  return ipCategoryOptions.find((item) => item.key === key)?.label || key;
}
