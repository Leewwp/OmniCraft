"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import {
  Image as ImageIcon,
  Clapperboard,
  FileText,
  Music,
  Music2,
  LayoutTemplate,
  Bot,
  Printer,
  Package,
} from "lucide-react";
import { ContentTypeGrid, applyTypeOrder, type ContentType } from "@/components/studio/ContentTypeGrid";
import { PublishForm } from "@/components/studio/PublishForm";
import { fetchPublicConfig } from "@/lib/public-config";
import { silentError } from "@/lib/error-handler";

/* SP-26 A-9（#780）：emoji 图标换 lucide 线性图标（与 fanwork 发布页同步）。 */
const CONTENT_TYPE_KEYS = [
  { value: "image", icon: ImageIcon },
  { value: "video", icon: Clapperboard },
  { value: "article", icon: FileText },
  { value: "audio", icon: Music },
  { value: "sheet_music", icon: Music2 },
  { value: "template", icon: LayoutTemplate },
  { value: "prompt", icon: Bot },
  { value: "3d_print", icon: Printer },
  { value: "other", icon: Package },
] as const;

export default function PublishOriginalPage() {
  const [selectedType, setSelectedType] = useState<string | null>(null);
  /* T25：类型清单与顺序跟随 /config/public 下发的运营配置 */
  const [typeOrder, setTypeOrder] = useState<string[] | null>(null);
  const t = useTranslations("studio.publish");

  useEffect(() => {
    let active = true;
    fetchPublicConfig()
      .then((config) => {
        if (active) setTypeOrder(config.publish?.type_order_original ?? null);
      })
      .catch((error) => {
        silentError(error, { component: "PublishOriginalPage", action: "fetchTypeOrder" });
      });
    return () => {
      active = false;
    };
  }, []);

  const contentTypes: ContentType[] = applyTypeOrder(CONTENT_TYPE_KEYS, typeOrder).map(({ value, icon }) => ({
    value,
    icon,
    label: t(`typeLabel.${value}`),
    description: t(`typeDescOriginal.${value}`),
  }));

  if (selectedType) {
    return (
      // #548：摘除外层 672px 宽框——编辑列随 StudioLayout 容器弹性（PublishForm
      // 表单自带 880px 上限），双栏网格不再被 672px 锁死。
      <div>
        <h1 className="mb-6 text-xl font-bold text-foreground">{t("originalTitle")}</h1>
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
      <h1 className="mb-1 text-xl font-bold text-foreground">{t("originalTitle")}</h1>
      <p className="mb-6 text-sm text-muted-foreground">{t("selectType")}</p>
      <ContentTypeGrid
        types={contentTypes}
        selected={selectedType}
        onSelect={setSelectedType}
      />
    </div>
  );
}