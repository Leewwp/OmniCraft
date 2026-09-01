"use client";

import Link from "next/link";
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, List } from "lucide-react";
import { useTranslations } from "next-intl";

import { buttonVariants } from "@/components/ui/button";
import { getSeriesDetail, type SeriesContent } from "@/lib/series";
import type { SeriesMembership } from "@/lib/content";
import { cn } from "@/lib/utils";

interface SeriesNavProps {
  memberships: SeriesMembership[];
  /** 浮层内目录/章节切换压栈（ui-spec:2587）：由浮层上下文注入。
      独立详情页不传：上一章/下一章保持整页 Link，目录指向 /series/:id。 */
  onNavigateInOverlay?: (contentId: number, trigger?: HTMLElement | null) => void;
}

function isValidMembership(value: SeriesMembership): boolean {
  return (
    Number.isInteger(value.series_id) &&
    value.series_id > 0 &&
    Boolean(value.series_title.trim()) &&
    Number.isInteger(value.current_index) &&
    value.current_index > 0 &&
    Number.isInteger(value.total) &&
    value.total >= value.current_index
  );
}

function isValidTarget(value: SeriesMembership["previous"]): value is NonNullable<SeriesMembership["previous"]> {
  return Boolean(value && Number.isInteger(value.id) && value.id > 0 && value.title.trim());
}

function contentHref(id: number, zone?: SeriesMembership["series_zone"]): string {
  return zone === "original" ? `/original/${id}` : `/content/${id}`;
}

/** 目录数据加载状态：章节列表来自公开系列详情合同（/api/v1/series/:id）。 */
type DirectoryLoad =
  | { status: "loading"; items: SeriesContent[] }
  | { status: "ready"; items: SeriesContent[] }
  | { status: "error"; items: [] };

export function SeriesNav({ memberships, onNavigateInOverlay }: SeriesNavProps) {
  const t = useTranslations("series.nav");
  const validMemberships = useMemo(() => memberships.filter(isValidMembership), [memberships]);
  const visibleTabs = validMemberships.slice(0, 3);
  const overflow = validMemberships.slice(3);
  const [activeSeriesID, setActiveSeriesID] = useState<number | null>(null);
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const [moreOpen, setMoreOpen] = useState(false);
  const moreMenuID = useId();
  const moreWrapperRef = useRef<HTMLDivElement | null>(null);
  const moreTriggerRef = useRef<HTMLButtonElement | null>(null);
  const moreItemRefs = useRef<Array<HTMLAnchorElement | null>>([]);
  const panelID = useId();
  const visibleMembershipKey = visibleTabs.map((membership) => membership.series_id).join(":");

  /* 浮层内目录选择器（#64 决策 14 / ui-spec 目录选择器）：有界高度 + 可滚动 + listbox 语义。 */
  const [directoryOpen, setDirectoryOpen] = useState(false);
  const [directoryBySeries, setDirectoryBySeries] = useState<Record<number, DirectoryLoad>>({});
  const directoryPanelID = useId();
  const directoryWrapperRef = useRef<HTMLDivElement | null>(null);
  const directoryTriggerRef = useRef<HTMLButtonElement | null>(null);
  const directoryOptionRefs = useRef<Array<HTMLButtonElement | null>>([]);

  useEffect(() => {
    setMoreOpen(false);
    setDirectoryOpen(false);
    setActiveSeriesID((current) => {
      if (current !== null && visibleTabs.some((membership) => membership.series_id === current)) {
        return current;
      }
      return visibleTabs[0]?.series_id ?? null;
    });
  }, [visibleMembershipKey]);

  useEffect(() => {
    if (!moreOpen) {
      return;
    }
    function handlePointerDown(event: PointerEvent) {
      if (!moreWrapperRef.current?.contains(event.target as Node)) {
        setMoreOpen(false);
      }
    }
    document.addEventListener("pointerdown", handlePointerDown);
    return () => document.removeEventListener("pointerdown", handlePointerDown);
  }, [moreOpen]);

  useEffect(() => {
    if (!directoryOpen) {
      return;
    }
    function handlePointerDown(event: PointerEvent) {
      if (!directoryWrapperRef.current?.contains(event.target as Node)) {
        setDirectoryOpen(false);
      }
    }
    document.addEventListener("pointerdown", handlePointerDown);
    return () => document.removeEventListener("pointerdown", handlePointerDown);
  }, [directoryOpen]);

  /* 目录打开期间在 document 捕获阶段拦截 Escape：必须先于浮层 dialog 的原生
     cancel 判定（React 合成事件挂在根节点，冒泡阶段 preventDefault 太晚，Esc
     会连带把整个浮层弹掉）。关闭后归还焦点到「目录」trigger。 */
  useEffect(() => {
    if (!directoryOpen) {
      return;
    }
    function handleDocumentKeyDown(event: KeyboardEvent) {
      if (event.key !== "Escape") {
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      setDirectoryOpen(false);
      /* 先卸载选择器再归还焦点：聚焦元素随面板一起移除会夺走焦点。 */
      window.requestAnimationFrame(() => directoryTriggerRef.current?.focus());
    }
    document.addEventListener("keydown", handleDocumentKeyDown, true);
    return () => document.removeEventListener("keydown", handleDocumentKeyDown, true);
  }, [directoryOpen]);

  if (validMemberships.length === 0) {
    return null;
  }

  const matchedActiveIndex = visibleTabs.findIndex((membership) => membership.series_id === activeSeriesID);
  const safeActiveIndex = matchedActiveIndex >= 0 ? matchedActiveIndex : 0;
  const active = visibleTabs[safeActiveIndex] ?? validMemberships[0];
  const previous = active.current_index > 1 && isValidTarget(active.previous) ? active.previous : undefined;
  const next = active.current_index < active.total && isValidTarget(active.next) ? active.next : undefined;

  const directoryLoad = directoryBySeries[active.series_id];
  const directoryItems = directoryLoad?.status === "ready" ? directoryLoad.items : [];
  const directoryCurrentChapterID = directoryItems[active.current_index - 1]?.id;
  const directoryActiveOptionIndex = directoryCurrentChapterID !== undefined ? active.current_index - 1 : 0;

  function selectTab(index: number, focus = false) {
    setActiveSeriesID(visibleTabs[index]?.series_id ?? null);
    setDirectoryOpen(false);
    if (focus) {
      window.requestAnimationFrame(() => tabRefs.current[index]?.focus());
    }
  }

  function handleTabKeyDown(event: React.KeyboardEvent<HTMLButtonElement>, index: number) {
    if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") {
      return;
    }
    event.preventDefault();
    const direction = event.key === "ArrowRight" ? 1 : -1;
    selectTab((index + direction + visibleTabs.length) % visibleTabs.length, true);
  }

  function openMoreMenu() {
    setDirectoryOpen(false);
    setMoreOpen(true);
    window.requestAnimationFrame(() => moreItemRefs.current[0]?.focus());
  }

  function handleMoreItemKeyDown(event: React.KeyboardEvent<HTMLAnchorElement>, index: number) {
    if (event.key === "Escape") {
      event.preventDefault();
      setMoreOpen(false);
      moreTriggerRef.current?.focus();
      return;
    }
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") {
      return;
    }
    event.preventDefault();
    const direction = event.key === "ArrowDown" ? 1 : -1;
    const nextIndex = (index + direction + overflow.length) % overflow.length;
    moreItemRefs.current[nextIndex]?.focus();
  }

  function loadDirectory(seriesId: number) {
    if (directoryBySeries[seriesId]?.status === "loading") {
      return;
    }
    setDirectoryBySeries((prev) => ({ ...prev, [seriesId]: { status: "loading", items: [] } }));
    getSeriesDetail(seriesId)
      .then((detail) => {
        setDirectoryBySeries((prev) => {
          if (prev[seriesId]?.status !== "loading") return prev;
          const items = detail.items
            .map((item) => item.content)
            .filter((content) => Number.isInteger(content.id) && content.id > 0 && content.title.trim());
          return { ...prev, [seriesId]: { status: "ready", items } };
        });
      })
      .catch(() => {
        setDirectoryBySeries((prev) =>
          prev[seriesId]?.status === "loading"
            ? { ...prev, [seriesId]: { status: "error", items: [] } }
            : prev,
        );
      });
  }

  function openDirectory() {
    setMoreOpen(false);
    setDirectoryOpen(true);
    if (!directoryLoad || directoryLoad.status === "error") {
      loadDirectory(active.series_id);
    }
  }

  function isCurrentChapter(item: SeriesContent, index: number): boolean {
    return directoryCurrentChapterID !== undefined && item.id === directoryCurrentChapterID;
  }

  function activateChapter(item: SeriesContent, index: number) {
    const optionElement = directoryOptionRefs.current[index];
    setDirectoryOpen(false);
    if (isCurrentChapter(item, index)) {
      window.requestAnimationFrame(() => directoryTriggerRef.current?.focus());
      return;
    }
    onNavigateInOverlay?.(item.id, optionElement ?? directoryTriggerRef.current);
  }

  function handleOptionKeyDown(event: React.KeyboardEvent<HTMLButtonElement>, index: number) {
    if (event.key === "Home" || event.key === "End") {
      event.preventDefault();
      directoryOptionRefs.current[event.key === "Home" ? 0 : directoryItems.length - 1]?.focus();
      return;
    }
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const direction = event.key === "ArrowDown" ? 1 : -1;
      const nextIndex = (index + direction + directoryItems.length) % directoryItems.length;
      directoryOptionRefs.current[nextIndex]?.focus();
      return;
    }
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      activateChapter(directoryItems[index], index);
    }
  }

  /* 目录打开后焦点进入选择器（数据就绪时聚焦当前章节项）。 */
  useEffect(() => {
    if (!directoryOpen || directoryLoad?.status !== "ready") {
      return;
    }
    const focusIndex = Math.min(Math.max(active.current_index - 1, 0), directoryItems.length - 1);
    directoryOptionRefs.current[focusIndex]?.focus();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [directoryOpen, directoryLoad?.status, active.series_id]);

  const catalogTriggerClasses = cn(
    buttonVariants({ variant: "ghost" }),
    "min-h-11 min-w-0 max-w-full cursor-pointer px-3 min-[1101px]:max-w-44",
  );

  return (
    <nav
      aria-label={t("tabsLabel")}
      className="rounded-lg border border-border-default bg-card p-4 shadow-none"
    >
      {(visibleTabs.length > 1 || overflow.length > 0) && (
        <div className="mb-3 flex items-start gap-2">
          <div className="min-w-0 flex-1 overflow-x-auto pb-1">
            <div role="tablist" aria-label={t("tabsLabel")} className="flex min-w-max items-center gap-1">
              {visibleTabs.map((membership, index) => (
              <button
                key={membership.series_id}
                id={`${panelID}-tab-${membership.series_id}`}
                ref={(node) => { tabRefs.current[index] = node; }}
                type="button"
                role="tab"
                aria-selected={safeActiveIndex === index}
                aria-controls={panelID}
                tabIndex={safeActiveIndex === index ? 0 : -1}
                onClick={() => selectTab(index)}
                onKeyDown={(event) => handleTabKeyDown(event, index)}
                className={cn(
                  "inline-flex min-h-11 max-w-48 cursor-pointer items-center rounded-md px-3 text-sm font-medium transition-colors duration-200",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
                  safeActiveIndex === index
                    ? "bg-muted text-foreground"
                    : "text-muted-foreground hover:bg-muted/60 hover:text-foreground",
                )}
              >
                <span className="truncate">{membership.series_title}</span>
                </button>
              ))}
            </div>
          </div>

          {overflow.length > 0 && (
            <div ref={moreWrapperRef} className="relative shrink-0">
              <button
                ref={moreTriggerRef}
                type="button"
                aria-label={t("more", { count: overflow.length })}
                aria-haspopup="menu"
                aria-expanded={moreOpen}
                aria-controls={moreMenuID}
                onClick={() => (moreOpen ? setMoreOpen(false) : openMoreMenu())}
                onKeyDown={(event) => {
                  if (event.key === "ArrowDown") {
                    event.preventDefault();
                    openMoreMenu();
                  }
                }}
                className={cn(
                  buttonVariants({ variant: "ghost" }),
                  "min-h-11 shrink-0 cursor-pointer px-3 focus-visible:ring-2 focus-visible:ring-ring",
                )}
              >
                {t("more", { count: overflow.length })}
              </button>
              {moreOpen && (
                <div
                  id={moreMenuID}
                  role="menu"
                  className="absolute right-0 z-50 mt-1 min-w-48 rounded-lg border border-border bg-popover p-1 text-popover-foreground shadow-none"
                >
                  {overflow.map((membership, index) => (
                    <Link
                      key={membership.series_id}
                      ref={(node) => { moreItemRefs.current[index] = node; }}
                      role="menuitem"
                      href={`/series/${membership.series_id}`}
                      onClick={() => setMoreOpen(false)}
                      onKeyDown={(event) => handleMoreItemKeyDown(event, index)}
                      className="flex min-h-11 cursor-pointer items-center rounded-md px-3 text-sm outline-none transition-colors duration-200 hover:bg-muted focus-visible:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
                    >
                      {membership.series_title}
                    </Link>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      )}

      <div
        id={panelID}
        role={visibleTabs.length > 1 ? "tabpanel" : undefined}
        aria-labelledby={visibleTabs.length > 1 ? `${panelID}-tab-${active.series_id}` : undefined}
        className="flex flex-col gap-3 min-[1101px]:flex-row min-[1101px]:items-center min-[1101px]:justify-between"
      >
        <div
          className="min-w-0 min-[1101px]:flex-1"
        >
          <p className="truncate text-sm font-medium text-foreground">{active.series_title}</p>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {t("position", { current: active.current_index, total: active.total })}
          </p>
        </div>

        <div className="grid min-w-0 grid-cols-2 gap-2 sm:grid-cols-3 min-[1101px]:flex min-[1101px]:max-w-[72%]">
          {previous ? (
            onNavigateInOverlay ? (
              <button
                type="button"
                onClick={(event) => onNavigateInOverlay(previous.id, event.currentTarget)}
                aria-label={t("previousA11y", { title: previous.title })}
                className={cn(
                  buttonVariants({ variant: "outline" }),
                  "min-h-11 min-w-0 max-w-full cursor-pointer justify-start px-3 min-[1101px]:max-w-56",
                )}
              >
                <ChevronLeft aria-hidden="true" />
                <span className="truncate">{t("previous", { title: previous.title })}</span>
              </button>
            ) : (
              <Link
                href={contentHref(previous.id, active.series_zone)}
                aria-label={t("previousA11y", { title: previous.title })}
                className={cn(buttonVariants({ variant: "outline" }), "min-h-11 min-w-0 max-w-full cursor-pointer justify-start px-3 min-[1101px]:max-w-56")}
              >
                <ChevronLeft aria-hidden="true" />
                <span className="truncate">{t("previous", { title: previous.title })}</span>
              </Link>
            )
          ) : (
            <button
              type="button"
              disabled
              aria-disabled="true"
              aria-label={t("firstA11y")}
              className={cn(buttonVariants({ variant: "outline" }), "min-h-11 min-w-0 max-w-full justify-start px-3 text-fg-muted disabled:opacity-100 min-[1101px]:max-w-56")}
            >
              <ChevronLeft aria-hidden="true" />
              <span className="truncate">{t("first")}</span>
            </button>
          )}

          {onNavigateInOverlay ? (
            <div ref={directoryWrapperRef} className="relative min-w-0">
              <button
                ref={directoryTriggerRef}
                type="button"
                aria-label={t("catalogA11y", { title: active.series_title })}
                aria-haspopup="listbox"
                aria-expanded={directoryOpen}
                aria-controls={directoryPanelID}
                onClick={() => (directoryOpen ? setDirectoryOpen(false) : openDirectory())}
                onKeyDown={(event) => {
                  if (event.key === "ArrowDown") {
                    event.preventDefault();
                    openDirectory();
                  }
                }}
                className={cn(catalogTriggerClasses, "w-full")}
              >
                <List aria-hidden="true" />
                <span className="truncate">{t("catalog")}</span>
              </button>

              {directoryOpen && (
                <div
                  id={directoryPanelID}
                  role="listbox"
                  aria-label={t("directory.listLabel", { title: active.series_title })}
                  aria-busy={directoryLoad?.status === "loading" ? "true" : undefined}
                  className="absolute right-0 z-50 mt-1 max-h-72 w-64 max-w-[calc(100vw-2rem)] overflow-y-auto rounded-lg border border-border bg-popover p-1 text-popover-foreground shadow-none sm:left-0 sm:right-auto"
                >
                  <div className="border-b border-border px-3 py-2">
                    <p className="text-xs font-medium text-muted-foreground">{t("directory.title")}</p>
                    <p className="mt-0.5 truncate text-sm font-medium text-foreground">{active.series_title}</p>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      {t("directory.position", { current: active.current_index, total: active.total })}
                    </p>
                  </div>

                  {directoryLoad?.status === "loading" && (
                    <div className="px-3 py-3 text-sm text-muted-foreground">{t("directory.loading")}</div>
                  )}

                  {directoryLoad?.status === "error" && (
                    <div className="px-3 py-3">
                      <p className="text-sm text-muted-foreground">{t("directory.loadFailed")}</p>
                      <button
                        type="button"
                        onClick={() => loadDirectory(active.series_id)}
                        className={cn(
                          buttonVariants({ variant: "outline", size: "sm" }),
                          "mt-2 min-h-11 cursor-pointer focus-visible:ring-2 focus-visible:ring-ring",
                        )}
                      >
                        {t("directory.retry")}
                      </button>
                    </div>
                  )}

                  {directoryLoad?.status === "ready" && directoryItems.length === 0 && (
                    <div className="px-3 py-3 text-sm text-muted-foreground">{t("directory.empty")}</div>
                  )}

                  {directoryLoad?.status === "ready" &&
                    directoryItems.map((item, index) => {
                      const current = isCurrentChapter(item, index);
                      return (
                        <button
                          key={item.id}
                          ref={(node) => { directoryOptionRefs.current[index] = node; }}
                          type="button"
                          role="option"
                          aria-selected={current}
                          aria-label={
                            current
                              ? t("directory.currentOption", { index: index + 1, title: item.title })
                              : t("directory.option", { index: index + 1, title: item.title })
                          }
                          tabIndex={index === directoryActiveOptionIndex ? 0 : -1}
                          onClick={() => activateChapter(item, index)}
                          onKeyDown={(event) => handleOptionKeyDown(event, index)}
                          className={cn(
                            "flex min-h-11 w-full cursor-pointer items-center rounded-md px-3 text-sm outline-none transition-colors duration-200 hover:bg-muted focus-visible:bg-muted focus-visible:ring-2 focus-visible:ring-ring",
                            current && "bg-muted/60 font-medium",
                          )}
                        >
                          <span className="truncate">
                            {t("directory.option", { index: index + 1, title: item.title })}
                          </span>
                        </button>
                      );
                    })}
                </div>
              )}
            </div>
          ) : (
            <Link
              href={`/series/${active.series_id}`}
              aria-label={t("catalogA11y", { title: active.series_title })}
              className={catalogTriggerClasses}
            >
              <List aria-hidden="true" />
              <span className="truncate">{t("catalog")}</span>
            </Link>
          )}

          {next ? (
            onNavigateInOverlay ? (
              <button
                type="button"
                onClick={(event) => onNavigateInOverlay(next.id, event.currentTarget)}
                aria-label={t("nextA11y", { title: next.title })}
                className={cn(
                  buttonVariants({ variant: "outline" }),
                  "col-span-2 min-h-11 min-w-0 max-w-full cursor-pointer justify-end px-3 sm:col-span-1 min-[1101px]:max-w-56",
                )}
              >
                <span className="truncate">{t("next", { title: next.title })}</span>
                <ChevronRight aria-hidden="true" />
              </button>
            ) : (
              <Link
                href={contentHref(next.id, active.series_zone)}
                aria-label={t("nextA11y", { title: next.title })}
                className={cn(buttonVariants({ variant: "outline" }), "col-span-2 min-h-11 min-w-0 max-w-full cursor-pointer justify-end px-3 sm:col-span-1 min-[1101px]:max-w-56")}
              >
                <span className="truncate">{t("next", { title: next.title })}</span>
                <ChevronRight aria-hidden="true" />
              </Link>
            )
          ) : (
            <button
              type="button"
              disabled
              aria-disabled="true"
              aria-label={t("lastA11y")}
              className={cn(buttonVariants({ variant: "outline" }), "col-span-2 min-h-11 min-w-0 max-w-full justify-end px-3 text-fg-muted disabled:opacity-100 sm:col-span-1 min-[1101px]:max-w-56")}
            >
              <span className="truncate">{t("last")}</span>
              <ChevronRight aria-hidden="true" />
            </button>
          )}
        </div>
      </div>
    </nav>
  );
}
