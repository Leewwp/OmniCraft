"use client";

// PROTOTYPE — refined landing page. Keep for non-logged-in visitors.
// Logged-in users are redirected to / (fanwork zone).

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Sparkles, Users, Layers } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";

export default function PrototypeLanding() {
  const t = useTranslations();
  const { user, isLoading: authLoading } = useAuth();
  const router = useRouter();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (authLoading) return;
    if (user) {
      router.replace("/");
    } else {
      setReady(true);
    }
  }, [user, authLoading, router]);

  // Redirecting — show nothing while auth resolves
  if (!ready) return null;

  return (
    <div className="flex flex-col">
      {/* Hero — flat accent wash, V2 text layout, V3 components */}
      <section className="relative flex flex-col items-center justify-center gap-8 px-6 py-28 text-center overflow-hidden">
        <div className="absolute inset-0 bg-accent-subtle/20 pointer-events-none" />

        <div className="relative z-10 flex flex-col gap-4 max-w-2xl">
          <h1 className="text-5xl font-bold tracking-tight text-fg-default sm:text-6xl lg:text-7xl">
            {t("nav.siteName")}
          </h1>
          <div className="mx-auto h-px w-16 bg-border-default" />
          <p className="text-lg text-fg-muted leading-relaxed max-w-lg mx-auto">
            {t("home.heroTagline")}
          </p>
        </div>

        {/* CTAs — prominent flat buttons */}
        <div className="relative z-10 flex flex-col items-center gap-4">
          <div className="flex flex-wrap items-center justify-center gap-3">
            <Link
              href="/register"
              className={cn(
                buttonVariants({ size: "lg" }),
                "rounded-lg px-10 h-12 text-[15px] font-semibold hover:bg-primary/90 active:bg-primary/80 transition-colors duration-200"
              )}
            >
              {t("auth.registerNow")}
            </Link>
            <Link
              href="/login"
              className={cn(
                buttonVariants({ variant: "outline", size: "lg" }),
                "rounded-lg px-10 h-12 text-[15px] font-medium border-2 hover:bg-canvas-subtle active:bg-muted transition-colors duration-200"
              )}
            >
              {t("auth.loginNow")}
            </Link>
          </div>
          <div className="flex items-center gap-4 text-sm">
            <Link
              href="/"
              className="text-fg-muted hover:text-fg-default transition-colors underline underline-offset-4"
            >
              {t("nav.fanworkZone")}
            </Link>
            <span className="text-fg-subtle">·</span>
            <Link
              href="/original"
              className="text-fg-muted hover:text-fg-default transition-colors underline underline-offset-4"
            >
              {t("nav.originalZone")}
            </Link>
          </div>
        </div>
      </section>

      {/* Features — flat cards */}
      <section className="px-6 py-16">
        <div className="mx-auto grid max-w-3xl gap-4 sm:grid-cols-3">
          {[
            { icon: Layers, title: t("home.featureIpfusion"), desc: t("home.featureIpfusionDesc") },
            { icon: Sparkles, title: t("home.featureAgent"), desc: t("home.featureAgentDesc") },
            { icon: Users, title: t("home.featurePr"), desc: t("home.featurePrDesc") },
          ].map((f, i) => (
            <div
              key={i}
              className="flex flex-col items-center gap-3 rounded-lg bg-canvas-subtle px-6 py-8 text-center transition-colors duration-200 hover:bg-accent-subtle/30"
            >
              <div className="flex h-11 w-11 items-center justify-center rounded-lg bg-accent-subtle">
                <f.icon className="h-5 w-5 text-accent-emphasis" />
              </div>
              <h3 className="font-semibold text-fg-default">{f.title}</h3>
              <p className="text-[13px] text-fg-muted leading-relaxed">{f.desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Status */}
      <section className="px-6 py-16 text-center">
        <div className="mx-auto max-w-md rounded-lg border border-border-muted bg-canvas-subtle p-8">
          <p className="text-sm text-fg-muted mb-4">
            {t("home.underConstruction")}
          </p>
          <div className="flex items-center justify-center gap-2 text-xs text-fg-subtle">
            <span className="h-1.5 w-1.5 rounded-full bg-green-500" />
            <span>{t("home.apiRunning")}</span>
          </div>
        </div>
      </section>
    </div>
  );
}
