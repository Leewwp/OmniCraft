"use client";

import { useEffect } from "react";
import { AlertTriangle, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function ErrorPage({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("Unhandled page error:", error);
  }, [error]);

  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 px-4">
      <div className="flex h-16 w-16 items-center justify-center rounded-full border border-destructive bg-destructive/10">
        <AlertTriangle className="h-8 w-8 text-destructive" />
      </div>
      <h1 className="text-xl font-semibold text-foreground">页面出错了</h1>
      <p className="max-w-md text-center text-sm text-muted-foreground">
        {error.message || "发生了意外错误，请稍后重试"}
      </p>
      <Button variant="outline" onClick={reset} className="gap-2">
        <RefreshCw className="h-4 w-4" />
        重试
      </Button>
    </div>
  );
}
