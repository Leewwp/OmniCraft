import Link from "next/link";
import { Brush, Sparkles, Users, Layers } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { getTranslations } from 'next-intl/server';

export default async function HomePage() {
  const t = await getTranslations();
  return (
    <div className="flex flex-col">
      {/* Hero */}
      <section className="flex flex-col items-center justify-center gap-6 px-4 py-24 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10">
          <Brush className="h-8 w-8 text-primary" />
        </div>
        <div className="flex flex-col gap-3">
          <h1 className="text-4xl font-bold tracking-tight sm:text-5xl">
            {t('nav.siteName')}
          </h1>
          <p className="max-w-xl text-lg text-muted-foreground leading-relaxed">
            {t('home.heroTagline')}
          </p>
        </div>
        <div className="flex flex-wrap items-center justify-center gap-3">
          <Link href="/register" className={cn(buttonVariants({ size: "lg" }))}>
            {t('home.ctaJoin')}
          </Link>
          <Link href="/login" className={cn(buttonVariants({ variant: "outline", size: "lg" }))}>
            {t('home.ctaLogin')}
          </Link>
        </div>
      </section>

      {/* Features */}
      <section className="border-t border-border bg-muted/30 px-4 py-16">
        <div className="mx-auto grid max-w-4xl gap-8 sm:grid-cols-3">
          <div className="flex flex-col items-center gap-3 text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10">
              <Layers className="h-6 w-6 text-primary" />
            </div>
            <h3 className="font-semibold">{t('home.featureIpfusion')}</h3>
            <p className="text-sm text-muted-foreground leading-relaxed">
              {t('home.featureIpfusionDesc')}
            </p>
          </div>
          <div className="flex flex-col items-center gap-3 text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10">
              <Sparkles className="h-6 w-6 text-primary" />
            </div>
            <h3 className="font-semibold">{t('home.featureAgent')}</h3>
            <p className="text-sm text-muted-foreground leading-relaxed">
              {t('home.featureAgentDesc')}
            </p>
          </div>
          <div className="flex flex-col items-center gap-3 text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10">
              <Users className="h-6 w-6 text-primary" />
            </div>
            <h3 className="font-semibold">{t('home.featurePr')}</h3>
            <p className="text-sm text-muted-foreground leading-relaxed">
              {t('home.featurePrDesc')}
            </p>
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="flex flex-col items-center gap-4 px-4 py-16 text-center">
        <p className="text-muted-foreground text-sm">{t('home.underConstruction')}</p>
        <div className="flex items-center gap-2 text-xs text-muted-foreground/60">
          <span>{t('home.apiStatus')}</span>
          <span className="h-1 w-1 rounded-full bg-green-500"></span>
          <span className="text-green-600 font-medium">{t('home.apiRunning')}</span>
          <span className="mx-2">·</span>
          <Link href="/login" className="hover:text-foreground transition-colors">
            {t('nav.login')}
          </Link>
          <span>·</span>
          <Link href="/register" className="hover:text-foreground transition-colors">
            {t('nav.register')}
          </Link>
        </div>
      </section>
    </div>
  );
}
