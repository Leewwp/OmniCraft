"use client";

import { useTranslations } from "next-intl";

interface UploadAssistPanelProps {
  contentType: string;
}

export function UploadAssistPanel({ contentType }: UploadAssistPanelProps) {
  const t = useTranslations();

  if (contentType === "mod") {
    return (
      <div className="rounded-md border border-border bg-muted/30 p-3 text-xs text-muted-foreground">
        {t('content.modUploadTips')}
        <ul className="ml-4 mt-1 list-disc space-y-1">
          <li>{t('content.modUploadTip1')}</li>
          <li>{t('content.modUploadTip2')}</li>
          <li>{t('content.modUploadTip3')}</li>
        </ul>
      </div>
    );
  }

  if (contentType === "sheet_music") {
    return (
      <div className="rounded-md border border-border bg-muted/30 p-3 text-xs text-muted-foreground">
        {t('content.sheetMusicUploadHint')}
      </div>
    );
  }

  return (
    <div className="rounded-md border border-border bg-muted/30 p-3 text-xs text-muted-foreground">
      {t('content.uploadComplianceHint')}
    </div>
  );
}
