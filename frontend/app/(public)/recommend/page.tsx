import { getTranslations } from "next-intl/server";
import type { Metadata } from "next";
import { getBrowserApiBase, getServerApiBase } from "@/lib/server-api";
import { normalizeContentList, type ContentDetailData } from "@/lib/content";
import { RecommendFeedClient } from "@/components/recommend/RecommendFeedClient";

interface ContentResponse {
  contents?: unknown[];
  total?: number;
}

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations();
  return {
    title: `${t("recommend.title")} — ${t("nav.siteName")}`,
    description: t("recommend.subtitle"),
  };
}

async function fetchFirstPage(apiBase: string): Promise<{ items: ContentDetailData[]; total: number | null; error: boolean }> {
  try {
    const res = await fetch(
      `${apiBase}/contents?sort=recommended&page=1&page_size=24`,
      { cache: "no-store" },
    );
    if (!res.ok) return { items: [], total: null, error: true };
    const data = (await res.json()) as ContentResponse;
    return {
      items: normalizeContentList(data.contents),
      total: typeof data.total === "number" ? data.total : null,
      error: false,
    };
  } catch {
    return { items: [], total: null, error: true };
  }
}

export default async function RecommendPage() {
  const t = await getTranslations();
  const apiBase = getServerApiBase();
  const { items, total, error } = await fetchFirstPage(apiBase);

  return (
    <div className="min-h-[calc(100vh-52px)] bg-canvas-subtle">
      <div className="mx-auto w-full max-w-[1280px] px-4 py-6 md:px-6 md:py-8">
        <header className="mb-6">
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">{t("recommend.title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("recommend.subtitle")}</p>
        </header>

        <RecommendFeedClient
          apiBase={getBrowserApiBase()}
          initialItems={items}
          initialTotal={total}
          initialError={error}
        />
      </div>
    </div>
  );
}
