"use client";

import { useEffect } from "react";
import { AlertTriangle, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslations } from 'next-intl';

export default function ErrorPage({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const t = useTranslations();

  useEffect(() => {
    console.error("Unhandled page error:", error);
  }, [error]);

  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 px-4">
      <div className="flex h-16 w-16 items-center justify-center rounded-full border border-destructive bg-destructive/10">
        <AlertTriangle className="h-8 w-8 text-destructive" />
      </div>
      <h1 className="text-xl font-semibold text-foreground">{t('error.pageError')}</h1>
      <p className="max-w-md text-center text-sm text-muted-foreground">
        {error.message || t('error.pageErrorDesc')}
      </p>
      <Button variant="outline" onClick={reset} className="gap-2">
        <RefreshCw className="h-4 w-4" />
        {t('common.retry')}
      </Button>
    </div>
  );
}
