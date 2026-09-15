"use client";

/* SP-19 G3-1（#520）：Milkdown（Crepe 7.22.1，精确锁版）编辑器封装底座。
 *
 * 架构保证（spec §2）：只替换「输入」侧——输出仍是纯 markdown 字符串，
 * 渲染层 MarkdownRenderer（react-markdown）完全不动。
 *
 * 实现约束（spec §2.2 风险表全部条款）：
 * - replaceAll 纪律：仅草稿恢复 / AI 填充 / 表单重置三场景经 handle.setMarkdown
 *   调用 replaceAll，输入过程绝不整篇替换；markdownUpdated 防抖同步表单。
 * - 单实例：locale 切换/props 变更不重建实例（zh 覆盖表到下次挂载才生效，
 *   接受）；整体换值走 setMarkdown。
 * - 中文化 locale 感知：zh locale 注入 featureConfigs 覆盖表，en 用默认英文。
 * - Latex/AI/TopBar 关闭；图片上传（allowImages）复用 /contents/oss-token
 *   presign 直传，grant 声明字节数与实际上传一致（SP-16 配方教训）。
 * - 草稿（Q9-B）：localStorage 按「用户标识 + 场景 + 表单标识」键防抖保存，
 *   进入时表单为空且有草稿 → 提示恢复（不静默覆盖），提交成功后由调用方
 *   clearDraft()。字数统计显示于自画底栏（rune 计数）。
 */

import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
} from "react";
import { useLocale, useTranslations } from "next-intl";
import { Milkdown, MilkdownProvider, useEditor } from "@milkdown/react";
import { Crepe, CrepeFeature, type CrepeConfig } from "@milkdown/crepe";
import { replaceAll } from "@milkdown/kit/utils";
import { api } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { cn } from "@/lib/utils";

import "@milkdown/crepe/theme/common/style.css";
import "@milkdown/crepe/theme/frame.css";
import "./milkdown-editor.css";

export interface MilkdownEditorHandle {
  /** 当前 markdown（提交时兜底取值，官方表单模式）。 */
  getMarkdown: () => string;
  /** 整体换值——仅草稿恢复 / AI 填充 / 表单重置三场景调用（replaceAll 纪律）。 */
  setMarkdown: (markdown: string) => void;
  /** 提交成功后清除本地草稿。 */
  clearDraft: () => void;
}

export interface MilkdownEditorProps {
  /** 初始值；挂载后整体换值走 handle.setMarkdown（勿依赖本 prop 变更）。 */
  defaultValue?: string;
  /** markdown 防抖同步（约 400ms）。 */
  onChange?: (markdown: string) => void;
  placeholder?: string;
  /** 编辑区最小高度（px），默认 260。 */
  minHeight?: number;
  /** 允许图片块/插图上传（发布正文等）；默认 false（讨论/PR/IP 描述/反馈禁图）。 */
  allowImages?: boolean;
  /** 自定义上传实现（默认复用 /contents/oss-token presign 直传）。 */
  onUpload?: (file: File) => Promise<string>;
  /** 草稿键（场景 + 表单标识）；提供即启用自动草册与恢复提示。 */
  draftKey?: string;
  disabled?: boolean;
  className?: string;
  /** 无障碍：容器为 role="textbox"（编辑区为 contenteditable 组合体），
   * 供页面 <label htmlFor> 或 aria-labelledby 关联。 */
  id?: string;
  "aria-label"?: string;
  "aria-labelledby"?: string;
}

const DRAFT_PREFIX = "milkdown-draft:";
const DRAFT_DEBOUNCE_MS = 800;
const CHANGE_DEBOUNCE_MS = 400;

function draftStorageKey(draftKey: string, userIdentity: string): string {
  return `${DRAFT_PREFIX}${userIdentity}:${draftKey}`;
}

function countRunes(text: string): number {
  return [...text].length;
}

/** 上传令牌与 FileUploader 同形（grant 声明字节数 = file.size，PUT 原文件）。 */
export async function uploadImageViaPresign(file: File): Promise<string> {
  const token = await api.post<{ upload_url: string; oss_key: string; grant_id: string }>(
    "/api/v1/contents/oss-token",
    {
      file_name: file.name,
      file_type: "image",
      mime_type: file.type || "application/octet-stream",
      file_size: file.size,
    },
  );
  const res = await fetch(token.upload_url, {
    method: "PUT",
    body: file,
    headers: { "Content-Type": file.type || "application/octet-stream" },
  });
  if (!res.ok) throw new Error(`image upload failed: ${res.status}`);
  // 裸对象 URL（去签名参数）。读侧 DisplayURLSigner 按域名识别可重签；已知
  // 限制：description 文本内 URL 当前不参与读侧重签（记录于票 #520/#521）。
  return token.upload_url.split("?")[0];
}

/* zh 覆盖表：只覆盖 label/text 字段（featureConfigs 逐字段 ?? 合并，
 * 部分覆盖安全；图标等其余配置走 Crepe 默认）。 */
function buildFeatureConfigs(
  locale: string,
  placeholder: string | undefined,
  upload: ((file: File) => Promise<string>) | undefined,
): CrepeConfig["featureConfigs"] {
  if (locale === "zh") {
    const configs: CrepeConfig["featureConfigs"] = {
      placeholder: { text: placeholder ?? "输入正文…" },
      toolbar: {
        boldLabel: "加粗",
        italicLabel: "斜体",
        strikethroughLabel: "删除线",
        codeLabel: "行内代码",
        linkLabel: "链接",
        latexLabel: "公式",
      },
      "link-tooltip": {
        editButton: "编辑",
        removeButton: "移除",
        confirmButton: "确定",
        inputPlaceholder: "粘贴链接…",
      },
      "block-edit": {
        textGroup: {
          label: "文本",
          text: { label: "正文" },
          h1: { label: "标题 1" },
          h2: { label: "标题 2" },
          h3: { label: "标题 3" },
          h4: { label: "标题 4" },
          h5: { label: "标题 5" },
          h6: { label: "标题 6" },
          quote: { label: "引用" },
          divider: { label: "分隔线" },
        },
      },
      "code-mirror": {
        previewToggleText: (previewOnly: boolean) => (previewOnly ? "编辑代码" : "预览代码"),
      },
    };
    if (upload) {
      configs["image-block"] = {
        onUpload: upload,
        blockOnUpload: upload,
        inlineOnUpload: upload,
        blockUploadButton: "上传图片",
        blockUploadPlaceholderText: "或粘贴图片链接…",
        inlineUploadButton: "上传",
        inlineUploadPlaceholderText: "图片链接…",
        inlineConfirmButton: "确定",
      };
    }
    return configs;
  }
  const configs: CrepeConfig["featureConfigs"] = placeholder ? { placeholder: { text: placeholder } } : {};
  if (upload) {
    configs["image-block"] = { onUpload: upload, blockOnUpload: upload, inlineOnUpload: upload };
  }
  return configs;
}

interface CrepeInstanceProps {
  defaultValue: string;
  features: NonNullable<CrepeConfig["features"]>;
  featureConfigs: CrepeConfig["featureConfigs"];
  disabled: boolean;
  onReady: (crepe: Crepe) => void;
}

function CrepeInstance({ defaultValue, features, featureConfigs, disabled, onReady }: CrepeInstanceProps) {
  const crepeRef = useRef<Crepe | null>(null);
  const onReadyRef = useRef(onReady);
  onReadyRef.current = onReady;

  /* useEditor 工厂只在挂载时执行一次（deps 空数组）：Crepe 构造 + 预注册
   * markdownUpdated 监听（on() 在 create 前调用会自动延迟到 config 阶段）。
   * defaultValue/features/featureConfigs 后续变更不重建实例（单实例纪律）。 */
  const { loading } = useEditor((container) => {
    const crepe = new Crepe({ root: container, defaultValue, features, featureConfigs });
    crepeRef.current = crepe;
    onReadyRef.current(crepe);
    return crepe;
  }, []);

  useEffect(() => {
    crepeRef.current?.setReadonly(disabled);
  }, [disabled]);

  return (
    <div className="milkdown-editor-root w-full flex-1 px-4 py-3" aria-busy={loading || undefined}>
      <Milkdown />
    </div>
  );
}

export const MilkdownEditor = forwardRef<MilkdownEditorHandle, MilkdownEditorProps>(
  function MilkdownEditor(
    {
      defaultValue = "",
      onChange,
      placeholder,
      minHeight = 260,
      allowImages = false,
      onUpload,
      draftKey,
      disabled = false,
      className,
      id,
      "aria-label": ariaLabel,
      "aria-labelledby": ariaLabelledBy,
    },
    ref,
  ) {
    const t = useTranslations("editor");
    const locale = useLocale();
    const { user } = useAuth();
    const [wordCount, setWordCount] = useState(() => countRunes(defaultValue));
    const [draftFound, setDraftFound] = useState<string | null>(null);
    const [imageUploading, setImageUploading] = useState(false);
    const [imageError, setImageError] = useState(false);

    const crepeRef = useRef<Crepe | null>(null);
    const onChangeRef = useRef(onChange);
    onChangeRef.current = onChange;
    const latestMarkdownRef = useRef(defaultValue);
    const changeTimerRef = useRef<number | null>(null);
    const draftTimerRef = useRef<number | null>(null);
    const userIdentity = user ? String(user.id) : "anon";
    const storageKey = draftKey ? draftStorageKey(draftKey, userIdentity) : null;

    /* 图片上传状态包装（Crepe 的 onUpload 不暴露进度，这里只标记 进行/失败）。 */
    const uploadForEditor = useMemo(() => {
      if (!allowImages) return undefined;
      const impl = onUpload ?? uploadImageViaPresign;
      return async (file: File) => {
        setImageError(false);
        setImageUploading(true);
        try {
          return await impl(file);
        } catch (e) {
          setImageError(true);
          throw e;
        } finally {
          setImageUploading(false);
        }
      };
    }, [allowImages, onUpload]);

    const features = useMemo<NonNullable<CrepeConfig["features"]>>(
      () => ({
        [CrepeFeature.Latex]: false,
        [CrepeFeature.TopBar]: false,
        [CrepeFeature.AI]: false,
        ...(allowImages ? {} : { [CrepeFeature.ImageBlock]: false }),
      }),
      // features 只影响挂载期；allowImages 变更同样到下次挂载生效（与单实例纪律一致）。
      // eslint-disable-next-line react-hooks/exhaustive-deps
      [],
    );
    const featureConfigs = useMemo(
      () => buildFeatureConfigs(locale, placeholder, uploadForEditor),
      // 同上：locale/placeholder/上传实现只在挂载时取值，之后不重建。
      // eslint-disable-next-line react-hooks/exhaustive-deps
      [],
    );

    const flushDraft = useCallback(
      (markdown: string) => {
        if (!storageKey) return;
        if (markdown.trim() === "") {
          window.localStorage.removeItem(storageKey);
          return;
        }
        try {
          window.localStorage.setItem(storageKey, markdown);
        } catch {
          /* 配额满等本地存储异常不阻断编辑 */
        }
      },
      [storageKey],
    );

    const handleReady = useCallback(
      (crepe: Crepe) => {
        crepeRef.current = crepe;
        crepe.on((listener) => {
          listener.markdownUpdated((_, markdown) => {
            latestMarkdownRef.current = markdown;
            setWordCount(countRunes(markdown));
            if (changeTimerRef.current !== null) window.clearTimeout(changeTimerRef.current);
            changeTimerRef.current = window.setTimeout(() => {
              onChangeRef.current?.(markdown);
            }, CHANGE_DEBOUNCE_MS);
            if (draftTimerRef.current !== null) window.clearTimeout(draftTimerRef.current);
            draftTimerRef.current = window.setTimeout(() => flushDraft(markdown), DRAFT_DEBOUNCE_MS);
          });
        });
      },
      [flushDraft],
    );

    /* 进入时草稿检测：表单为空且有草稿 → 提示恢复（编辑器本身仍以空值启动，
     * 恢复动作 = replaceAll，不静默覆盖）。 */
    useEffect(() => {
      if (!storageKey) return;
      try {
        const saved = window.localStorage.getItem(storageKey);
        if (saved && saved.trim() !== "" && defaultValue.trim() === "") {
          setDraftFound(saved);
        }
      } catch {
        /* localStorage 不可用则无草稿功能 */
      }
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    useImperativeHandle(
      ref,
      () => ({
        getMarkdown: () => latestMarkdownRef.current || crepeRef.current?.getMarkdown() || "",
        setMarkdown: (markdown: string) => {
          latestMarkdownRef.current = markdown;
          setWordCount(countRunes(markdown));
          const editor = crepeRef.current?.editor;
          if (editor) {
            editor.action(replaceAll(markdown));
          }
          flushDraft(markdown);
        },
        clearDraft: () => {
          if (storageKey) window.localStorage.removeItem(storageKey);
          setDraftFound(null);
        },
      }),
      [flushDraft, storageKey],
    );

    /* 卸载时清定时器（草稿已在每次防抖点落盘）。 */
    useEffect(() => {
      return () => {
        if (changeTimerRef.current !== null) window.clearTimeout(changeTimerRef.current);
        if (draftTimerRef.current !== null) window.clearTimeout(draftTimerRef.current);
      };
    }, []);

    function restoreDraft() {
      if (draftFound === null) return;
      const editor = crepeRef.current?.editor;
      if (editor) {
        editor.action(replaceAll(draftFound));
        latestMarkdownRef.current = draftFound;
        setWordCount(countRunes(draftFound));
        onChangeRef.current?.(draftFound);
      }
      setDraftFound(null);
    }

    function discardDraft() {
      if (storageKey) window.localStorage.removeItem(storageKey);
      setDraftFound(null);
    }

    return (
      <div
      role="textbox"
      aria-multiline="true"
      id={id}
      aria-label={ariaLabel}
      aria-labelledby={ariaLabelledBy}
      className={cn("flex flex-col rounded-lg border border-border bg-background", className)}
      style={{ minHeight: `${minHeight}px` }}
    >
        {draftFound !== null && (
          <div className="flex items-center gap-2 border-b border-border bg-muted px-3 py-2 text-xs text-muted-foreground">
            <span>{t("draftFound")}</span>
            <button
              type="button"
              onClick={restoreDraft}
              className="rounded-full border border-accent-emphasis bg-accent-subtle px-2.5 py-1 font-medium text-accent-emphasis transition-colors hover:bg-accent-emphasis hover:text-white"
            >
              {t("restoreDraft")}
            </button>
            <button type="button" onClick={discardDraft} className="rounded-md px-2 py-1 underline-offset-2 hover:underline">
              {t("discardDraft")}
            </button>
          </div>
        )}
        <MilkdownProvider>
          <CrepeInstance
            defaultValue={defaultValue}
            features={features}
            featureConfigs={featureConfigs}
            disabled={disabled}
            onReady={handleReady}
          />
        </MilkdownProvider>
        {(imageUploading || imageError) && (
          <div className="border-t border-border px-3 py-1.5 text-xs text-muted-foreground">
            {imageError ? (
              <span role="alert" className="text-destructive">
                {t("imageUploadFailed")}
              </span>
            ) : (
              <span aria-live="polite">{t("imageUploading")}</span>
            )}
          </div>
        )}
        <div className="flex items-center justify-end border-t border-border px-3 py-1.5 text-xs text-muted-foreground" aria-live="polite">
          {t("wordCount", { count: wordCount })}
        </div>
      </div>
    );
  },
);
