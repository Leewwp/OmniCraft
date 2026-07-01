"use client";

import { useTranslations } from "next-intl";
import dynamic from "next/dynamic";
import "@uiw/react-md-editor/markdown-editor.css";
import "@uiw/react-markdown-preview/markdown.css";

const MDEditor = dynamic(() => import("@uiw/react-md-editor"), { ssr: false });

interface MarkdownEditorProps {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  id?: string;
  ariaDescribedBy?: string;
  ariaInvalid?: boolean;
  ariaLabel?: string;
}

export function MarkdownEditor({
  value,
  onChange,
  disabled,
  id,
  ariaDescribedBy,
  ariaInvalid,
  ariaLabel,
}: MarkdownEditorProps) {
  const t = useTranslations();

  return (
    <div data-color-mode="light" className="rounded-md border border-border bg-card">
      <MDEditor
        value={value}
        onChange={(next) => onChange(next || "")}
        height={320}
        preview="edit"
        visibleDragbar={false}
        textareaProps={{
          id,
          "aria-describedby": ariaDescribedBy,
          "aria-invalid": ariaInvalid,
          "aria-label": ariaLabel,
          placeholder: t('publish.markdownPlaceholder'),
          disabled,
        }}
      />
    </div>
  );
}
