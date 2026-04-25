"use client";

import dynamic from "next/dynamic";
import "@uiw/react-md-editor/markdown-editor.css";
import "@uiw/react-markdown-preview/markdown.css";

const MDEditor = dynamic(() => import("@uiw/react-md-editor"), { ssr: false });

interface MarkdownEditorProps {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}

export function MarkdownEditor({ value, onChange, disabled }: MarkdownEditorProps) {
  return (
    <div data-color-mode="light" className="rounded-md border border-border bg-card">
      <MDEditor
        value={value}
        onChange={(next) => onChange(next || "")}
        height={320}
        preview="edit"
        visibleDragbar={false}
        textareaProps={{
          placeholder: "在这里输入正文（Markdown）...",
          disabled,
        }}
      />
    </div>
  );
}
