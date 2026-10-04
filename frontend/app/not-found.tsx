import Link from "next/link";
import { FileQuestion } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { Header } from "@/components/layout/Header";
import { getTranslations } from 'next-intl/server';

export default async function NotFoundPage() {
  const t = await getTranslations();
  return (
    /* SP-26 A-10（#780）：404 页补全站顶栏（与 (headered) 布局同构）——
       此前只有「返回首页」按钮，无导航可达性；root layout 已含
       NextIntlClientProvider + AuthProvider，client Header 可安全复用。 */
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex flex-1 flex-col items-center justify-center gap-4 px-4 py-16">
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
      </main>
    </div>
  );
}
