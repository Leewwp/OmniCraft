interface UploadAssistPanelProps {
  contentType: string;
}

export function UploadAssistPanel({ contentType }: UploadAssistPanelProps) {
  if (contentType === "mod") {
    return (
      <div className="rounded-md border border-border bg-muted/30 p-3 text-xs text-muted-foreground">
        Mod 包上传建议：
        <ul className="ml-4 mt-1 list-disc space-y-1">
          <li>使用 zip 打包，避免 rar/7z。</li>
          <li>压缩包根目录建议包含 README 或 manifest 文件。</li>
          <li>若包含依赖，请在说明文档中标注运行环境。</li>
        </ul>
      </div>
    );
  }

  if (contentType === "sheet_music") {
    return (
      <div className="rounded-md border border-border bg-muted/30 p-3 text-xs text-muted-foreground">
        乐谱上传支持扩展名：mid、midi、xml、mxl、mscz、mscx、pdf。
      </div>
    );
  }

  return (
    <div className="rounded-md border border-border bg-muted/30 p-3 text-xs text-muted-foreground">
      上传后文件将进入内容安全审核流程，请确保内容合规。
    </div>
  );
}
