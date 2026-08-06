"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { AlertCircle, FileQuestion, ShieldOff } from "lucide-react";
import Link from "next/link";
import { api, ApiRequestError } from "@/lib/api";
import {
  normalizeContentDetailResponse,
  normalizeContentList,
  type NormalizedContentDetailResponse,
} from "@/lib/content";
import type { ContentCardData } from "@/components/content/ContentCard";
import { ContentDetail } from "@/components/content/ContentDetail";
import { EmptyState } from "@/components/ui/empty-state";
import { SkeletonDetail } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { FollowButton } from "@/components/social/FollowButton";

export type OverlaySource = "recommendation" | "zone-page" | "ip-page" | "agent-citation";

export interface OverlayEntry {
  contentId: number;
  zone: "original" | "fanwork";
  source: OverlaySource;
}

interface ContentDetailOverlayLayerProps {
  entry: OverlayEntry;
  onPush: (entry: OverlayEntry, trigger: HTMLElement | null) => void;
  onTitleChange: (title: string) => void;
}

type LayerStatus = "loading" | "default" | "forbidden" | "not-found" | "error";

const TYPE_LABEL_KEYS: Record<string, string> = {
  article: "home.text",
  image: "home.image",
  video: "home.video",
  audio: "home.audio",
  mod: "home.mod",
  prompt: "home.aiPrompt",
  sheet_music: "home.sheetMusic",
  template: "home.template",
};

/** 浮层内关联内容行：点击压入内部导航栈（下钻不跳页）。 */
function RelatedRow({
  title,
  meta,
  coverUrl,
  onOpen,
}: {
  title: string;
  meta?: string;
  coverUrl?: string;
  onOpen: (trigger: HTMLElement) => void;
}) {
  const t = useTranslations();
  return (
    <button
      type="button"
      onClick={(event) => onOpen(event.currentTarget)}
      aria-label={t("contentDetailOverlay.openRelated", { title })}
      className="flex w-full items-center gap-2.5 rounded-md p-2 text-left transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
    >
      {coverUrl ? (
        <span className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-md border border-border bg-muted">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={coverUrl} alt="" loading="lazy" className="h-full w-full object-cover" />
        </span>
      ) : (
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-accent-subtle text-xs font-semibold text-accent-emphasis">
          {title.slice(0, 1)}
        </span>
      )}
      <span className="min-w-0">
        <strong className="block truncate text-sm font-medium text-foreground">{title}</strong>
        {meta && <small className="block truncate text-xs text-muted-foreground">{meta}</small>}
      </span>
    </button>
  );
}

function AsideCard({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <section className="rounded-lg border border-border/60 bg-card p-5 shadow-[var(--elevation-1)]">
      <p className="text-xs font-bold uppercase tracking-wider text-muted-foreground">{label}</p>
      <div className="mt-3">{children}</div>
    </section>
  );
}

export function ContentDetailOverlayLayer({ entry, onPush, onTitleChange }: ContentDetailOverlayLayerProps) {
  const t = useTranslations();
  const [status, setStatus] = useState<LayerStatus>("loading");
  const [detail, setDetail] = useState<NormalizedContentDetailResponse | null>(null);
  const [related, setRelated] = useState<ContentCardData[]>([]);
  const [relatedTotal, setRelatedTotal] = useState(0);
  const [attempt, setAttempt] = useState(0);

  const onTitleChangeRef = useRef(onTitleChange);
  useEffect(() => {
    onTitleChangeRef.current = onTitleChange;
  }, [onTitleChange]);

  useEffect(() => {
    let cancelled = false;
    setStatus("loading");
    setDetail(null);
    setRelated([]);
    setRelatedTotal(0);

    api
      .get(`/api/v1/contents/${entry.contentId}`)
      .then((raw) => {
        if (cancelled) return;
        const normalized = normalizeContentDetailResponse(raw);
        if (!normalized.content) {
          setStatus("not-found");
          return;
        }
        if (normalized.content.status === "banned") {
          setStatus("forbidden");
          return;
        }
        setDetail(normalized);
        onTitleChangeRef.current(normalized.content.title);
        setStatus("default");
      })
      .catch((error) => {
        if (cancelled) return;
        if (error instanceof ApiRequestError && error.status === 404) {
          setStatus("not-found");
        } else if (error instanceof ApiRequestError && error.status === 403) {
          setStatus("forbidden");
        } else {
          setStatus("error");
        }
      });

    api
      .get<{ contents?: unknown[]; total?: number }>(
        `/api/v1/contents/${entry.contentId}/related-fanworks?page=1&page_size=8`,
      )
      .then((raw) => {
        if (cancelled) return;
        setRelated(normalizeContentList(raw.contents));
        setRelatedTotal(raw.total ?? 0);
      })
      .catch(() => {
        if (!cancelled) setRelated([]);
      });

    return () => {
      cancelled = true;
    };
  }, [entry.contentId, attempt]);

  if (status === "loading") {
    return (
      <div aria-busy="true" aria-label={t("contentDetailOverlay.title")}>
        <SkeletonDetail />
      </div>
    );
  }

  if (status === "not-found") {
    return (
      <EmptyState
        icon={FileQuestion}
        title={t("contentDetailOverlay.notFoundTitle")}
        description={t("contentDetailOverlay.notFoundDescription")}
      />
    );
  }

  if (status === "forbidden") {
    return (
      <EmptyState
        icon={ShieldOff}
        title={t("contentDetailOverlay.forbiddenTitle")}
        description={t("contentDetailOverlay.forbiddenDescription")}
      />
    );
  }

  if (status === "error") {
    return (
      <EmptyState
        icon={AlertCircle}
        title={t("contentDetailOverlay.loadFailedTitle")}
        description={t("contentDetailOverlay.loadFailedDescription")}
        action={
          <Button variant="outline" size="sm" onClick={() => setAttempt((value) => value + 1)}>
            {t("common.retry")}
          </Button>
        }
      />
    );
  }

  if (!detail?.content) return null;

  const content = detail.content;
  const isFanwork = content.zone === "fanwork";
  const sourceOriginal = detail.sourceOriginal;
  const relatedLabelKey = isFanwork ? "contentDetailOverlay.derivatives" : "content.relatedFanworks";

  return (
    <div className="mx-auto flex w-full max-w-[1280px] gap-6">
      <div className="min-w-0 flex-1">
        <ContentDetail data={{ ...content, attachments: detail.attachments, tags: detail.tags }} />
      </div>

      <aside className="hidden w-[280px] flex-shrink-0 lg:block" aria-label={t("contentDetailOverlay.title")}>
        <div className="sticky top-4 space-y-4">
          {content.author?.id && (
            <AsideCard label={t("contentDetailOverlay.creator")}>
              <div className="flex items-center gap-3">
                <div className="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-full bg-accent-subtle text-sm font-bold text-accent-emphasis">
                  {(content.author.username || "?").slice(0, 2).toUpperCase()}
                </div>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-semibold text-foreground">
                    {content.author.username || t("common.userLabel", { id: content.author.id })}
                  </div>
                </div>
                <FollowButton targetType="user" targetId={content.author.id} />
              </div>
            </AsideCard>
          )}

          {isFanwork && content.ip?.name && content.ip.id && (
            <AsideCard label={t("contentDetailOverlay.ip")}>
              <Link
                href={`/ip/${content.ip.id}`}
                className="inline-flex max-w-full items-center gap-2 text-sm text-foreground transition-colors hover:text-accent-emphasis focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                <span className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md bg-muted text-xs font-bold text-muted-foreground">
                  {content.ip.name.slice(0, 2)}
                </span>
                <span className="truncate font-medium">{content.ip.name}</span>
              </Link>
            </AsideCard>
          )}

          {isFanwork && sourceOriginal && (
            <AsideCard label={t("contentDetailOverlay.sourceOriginal")}>
              <RelatedRow
                title={sourceOriginal.title}
                onOpen={(trigger) =>
                  onPush({ contentId: sourceOriginal.id, zone: "original", source: "zone-page" }, trigger)
                }
              />
            </AsideCard>
          )}

          {related.length > 0 && (
            <AsideCard label={t(relatedLabelKey)}>
              <div className="space-y-1.5">
                {related.map((item) => {
                  const typeLabelKey =
                    TYPE_LABEL_KEYS[item.content_type ?? "other"] ?? "home.other";
                  const meta = `${t(typeLabelKey)} · @${item.author?.username ?? t("common.userLabel", { id: item.author_id ?? "-" })}`;
                  return (
                    <RelatedRow
                      key={item.id}
                      title={item.title}
                      meta={meta}
                      coverUrl={item.cover_image_url}
                      onOpen={(trigger) =>
                        onPush(
                          {
                            contentId: item.id,
                            zone: item.zone === "original" ? "original" : "fanwork",
                            source: "zone-page",
                          },
                          trigger,
                        )
                      }
                    />
                  );
                })}
              </div>
              {!isFanwork && relatedTotal > 8 && (
                <Link
                  href={`/original/${content.id}/fanworks`}
                  className="mt-2 inline-flex items-center gap-1 text-xs font-medium text-accent-emphasis transition-colors hover:text-accent-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                >
                  {t("contentDetailOverlay.viewAll")}
                </Link>
              )}
            </AsideCard>
          )}
        </div>
      </aside>
    </div>
  );
}
