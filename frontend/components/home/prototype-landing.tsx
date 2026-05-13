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
      {/* Hero — warm gradient, V2 text layout, V3 components */}
      <section className="relative flex flex-col items-center justify-center gap-8 px-6 py-28 text-center overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-b from-accent-subtle/40 via-background to-background pointer-events-none" />

        <div className="relative z-10 flex flex-col gap-4 max-w-2xl">
          <h1 className="text-5xl font-bold tracking-tight text-fg-default sm:text-6xl lg:text-7xl">
            {t("nav.siteName")}
          </h1>
          <div className="mx-auto h-px w-16 bg-border-default" />
          <p className="text-lg text-fg-muted leading-relaxed max-w-lg mx-auto">
            {t("home.heroTagline")}
          </p>
        </div>

        {/* CTAs — prominent rounded buttons */}
        <div className="relative z-10 flex flex-col items-center gap-4">
          <div className="flex flex-wrap items-center justify-center gap-3">
            <Link
              href="/register"
              className={cn(
                buttonVariants({ size: "lg" }),
                "rounded-2xl px-10 h-12 text-[15px] font-semibold shadow-lg hover:shadow-xl hover:-translate-y-0.5 active:scale-[0.98] transition-all duration-200"
              )}
            >
              注册账号
            </Link>
            <Link
              href="/login"
              className={cn(
                buttonVariants({ variant: "outline", size: "lg" }),
                "rounded-2xl px-10 h-12 text-[15px] font-medium border-2 hover:bg-canvas-subtle transition-all duration-200"
              )}
            >
              登录账号
            </Link>
          </div>
          <div className="flex items-center gap-4 text-sm">
            <Link
              href="/"
              className="text-fg-muted hover:text-fg-default transition-colors underline underline-offset-4"
            >
              浏览二创区
            </Link>
            <span className="text-fg-subtle">·</span>
            <Link
              href="/original"
              className="text-fg-muted hover:text-fg-default transition-colors underline underline-offset-4"
            >
              浏览原创区
            </Link>
          </div>
        </div>
      </section>

      {/* Features — rounded cards */}
      <section className="px-6 py-16">
        <div className="mx-auto grid max-w-3xl gap-4 sm:grid-cols-3">
          {[
            { icon: Layers, title: t("home.featureIpfusion"), desc: t("home.featureIpfusionDesc") },
            { icon: Sparkles, title: t("home.featureAgent"), desc: t("home.featureAgentDesc") },
            { icon: Users, title: t("home.featurePr"), desc: t("home.featurePrDesc") },
          ].map((f, i) => (
            <div
              key={i}
              className="flex flex-col items-center gap-3 rounded-3xl bg-canvas-subtle px-6 py-8 text-center transition-all duration-300 hover:bg-accent-subtle/30 hover:-translate-y-1"
            >
              <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-accent-subtle">
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
        <div className="mx-auto max-w-md rounded-3xl border border-border-muted bg-gradient-to-b from-canvas-subtle to-background p-8">
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
