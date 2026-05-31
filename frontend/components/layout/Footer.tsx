"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { Brush } from "lucide-react";

export function Footer() {
  const t = useTranslations();
  const year = new Date().getFullYear();

  return (
    <footer className="mt-auto border-t border-border bg-canvas-subtle">
      <div className="mx-auto flex max-w-7xl flex-col items-center gap-4 px-4 py-6 sm:flex-row sm:justify-between">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Brush className="h-4 w-4" />
          <span>&copy; {year} OmniCraft</span>
        </div>
        <nav className="flex flex-wrap justify-center gap-x-4 gap-y-1 text-sm text-muted-foreground">
          <Link href="/help" className="hover:text-foreground transition-colors">
            {t("footer.help")}
          </Link>
          <Link href="/privacy" className="hover:text-foreground transition-colors">
            {t("footer.privacy")}
          </Link>
          <Link href="/terms" className="hover:text-foreground transition-colors">
            {t("footer.terms")}
          </Link>
          <Link href="/feedback" className="hover:text-foreground transition-colors">
            {t("footer.feedback")}
          </Link>
          <Link href="/client" className="hover:text-foreground transition-colors">
            {t("footer.client")}
          </Link>
        </nav>
      </div>
    </footer>
  );
}
