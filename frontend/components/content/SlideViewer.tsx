"use client";

import { lazy, Suspense } from "react";

import { AudioPlayer } from "@/components/content/AudioPlayer";
import type { MediaGalleryItem, MediaSlideType } from "@/components/content/MediaGallery";
import { usePublicConfig } from "@/lib/use-public-config";
import type { PublicConfig } from "@/lib/public-config";

/**
 * 可预览媒体链查看器装配单一深模块（C1 #920）：model3d / document / audio
 * 三查看器族此前在三个宿主（浮层左列轮播位、放大态、独立详情页附件预览）
 * 各自手工装配（懒加载声明 5 份、9-prop 装配逐 token 复制）——本模块收口为
 * 唯一装配点：懒加载声明、公开配置→预算/约束 props 的字段映射、Suspense、
 * 类型→查看器分发全部在模块内。缝切在装配级：包装 div、data-slot 值、附件
 * 预览激活门、轮播位音频封面合成（占位图 + 悬浮底栏）留在宿主。
 *
 * 懒加载统一 React.lazy + Suspense fallback={null}：model3d 与 document 各
 * 维持独立异步 chunk（three.js 与 mammoth/exceljs 不并包，保持按类型分包）；
 * AudioPlayer 本就静态导入。查看器仅在用户可见位/点击入口后挂载，SSR 永不
 * 渲染查看器本体。
 */

/** 本模块管辖的查看器族子集（MediaSlideType 共 9 成员，此处只管三面）。 */
export type ViewerSlideType = Extract<MediaSlideType, "model3d" | "document" | "audio">;

/** slide 视图模型（既有媒体管线通用语 MediaGalleryItem 的类型收窄形态）。 */
export type ViewerSlideItem = Omit<MediaGalleryItem, "type"> & { type: ViewerSlideType };

/** D5 #862 查看器族懒分块：重依赖（three.js / mammoth+exceljs）不进主包，
 * 当前位才挂载（React.lazy + Suspense；宿主均为客户端浮层/点击后挂载，
 * 无 SSR 直出路径）。model3d 与 document 各自独立异步 chunk，不合并。 */
const ModelViewer = lazy(() =>
  import("@/components/content/ModelViewer").then((m) => ({ default: m.ModelViewer })),
);
const DocumentViewer = lazy(() =>
  import("@/components/content/DocumentViewer").then((m) => ({ default: m.DocumentViewer })),
);

/**
 * 宿主全类型分发后的类型收窄转换：宿主保留 MediaSlideType 穷尽 switch
 * （never 守卫）只管布局，对应三查看器族位经本守卫交给本模块。类型收窄
 * 的 as 断言只存在于此（运行时已按 type 判定），宿主不散布断言。
 */
export function asViewerSlide(item: MediaGalleryItem): ViewerSlideItem | null {
  if (item.type === "model3d" || item.type === "document" || item.type === "audio") {
    return item as ViewerSlideItem;
  }
  return null;
}

interface ViewerAssemblyProps {
  item: ViewerSlideItem;
  contentId?: number;
  allowCopy?: boolean;
  /** 可选已解析配置注入（附件预览宿主挂载即取，时序保持现状）；
   * 缺省时模块内 usePublicConfig() 兜底（轮播位/放大态现状即挂载位自取）。 */
  config?: PublicConfig | null;
}

/** model3d 装配：公开配置 → 预览预算/三角面/gcode 行数约束的单一映射。 */
function Model3dAssembly({ item, contentId, allowCopy, config }: ViewerAssemblyProps) {
  const fetchedConfig = usePublicConfig();
  const effectiveConfig = config ?? fetchedConfig;
  return (
    <Suspense fallback={null}>
      <ModelViewer
        url={item.url}
        fileName={item.fileName ?? ""}
        fileSize={item.fileSize}
        maxPreviewMB={effectiveConfig?.upload?.model3d_max_preview_mb}
        maxTriangles={effectiveConfig?.upload?.model3d_max_triangles}
        gcodeMaxLines={effectiveConfig?.upload?.gcode_max_lines}
        contentId={contentId ?? 0}
        attachmentId={item.id}
        allowCopy={Boolean(allowCopy)}
      />
    </Suspense>
  );
}

/** document 装配：公开配置 → 文档预览预算（MB）的单一映射。 */
function DocumentAssembly({ item, contentId, allowCopy, config }: ViewerAssemblyProps) {
  const fetchedConfig = usePublicConfig();
  const effectiveConfig = config ?? fetchedConfig;
  return (
    <Suspense fallback={null}>
      <DocumentViewer
        url={item.url}
        fileName={item.fileName ?? ""}
        fileSize={item.fileSize}
        maxPreviewMB={effectiveConfig?.upload?.document_preview_max_mb}
        contentId={contentId ?? 0}
        attachmentId={item.id}
        allowCopy={Boolean(allowCopy)}
      />
    </Suspense>
  );
}

export interface SlideViewerProps extends ViewerAssemblyProps {
  /** 仅音频面消费（轮播位挂起矩阵）；附件预览侧缺省 = 现状 undefined。 */
  suspended?: boolean;
}

/**
 * 类型→查看器分发（单一入口）：模块内对该子集穷尽 switch——新增
 * ViewerSlideType 成员而漏配装配时，never 赋值在此编译红。
 */
export function SlideViewer({ item, contentId, allowCopy, suspended, config }: SlideViewerProps) {
  switch (item.type) {
    case "model3d":
      return <Model3dAssembly item={item} contentId={contentId} allowCopy={allowCopy} config={config} />;
    case "document":
      return <DocumentAssembly item={item} contentId={contentId} allowCopy={allowCopy} config={config} />;
    case "audio":
      return (
        <AudioPlayer
          url={item.url}
          fileName={item.fileName ?? ""}
          fileSize={item.fileSize}
          contentId={contentId ?? 0}
          attachmentId={item.id}
          allowCopy={Boolean(allowCopy)}
          suspended={suspended}
        />
      );
    default: {
      const _exhaustive: never = item.type;
      return _exhaustive;
    }
  }
}
