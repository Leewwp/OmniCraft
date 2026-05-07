import Link from "next/link";
import { FileQuestion } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { getTranslations } from 'next-intl/server';

export default async function NotFoundPage() {
  const t = await getTranslations();
  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 px-4">
      <div className="flex h-16 w-16 items-center justify-center rounded-full border border-border bg-muted">
        <FileQuestion className="h-8 w-8 text-muted-foreground" />
      </div>
      <h1 className="text-xl font-semibold text-foreground">{t('error.notFound')}</h1>
      <p className="max-w-md text-center text-sm text-muted-foreground">
        {t('error.notFoundDesc')}
      </p>
      <Link href="/" className={buttonVariants()}>
        {t('error.backHome')}
      </Link>
    </div>
  );
}
