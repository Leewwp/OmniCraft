"use client";

import Link from "next/link";
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, List } from "lucide-react";
import { useTranslations } from "next-intl";

import { buttonVariants } from "@/components/ui/button";
import type { SeriesMembership } from "@/lib/content";
import { cn } from "@/lib/utils";

interface SeriesNavProps {
  memberships: SeriesMembership[];
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

export function SeriesNav({ memberships }: SeriesNavProps) {
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

  useEffect(() => {
    setMoreOpen(false);
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

  if (validMemberships.length === 0) {
    return null;
  }

  const matchedActiveIndex = visibleTabs.findIndex((membership) => membership.series_id === activeSeriesID);
  const safeActiveIndex = matchedActiveIndex >= 0 ? matchedActiveIndex : 0;
  const active = visibleTabs[safeActiveIndex] ?? validMemberships[0];
  const previous = active.current_index > 1 && isValidTarget(active.previous) ? active.previous : undefined;
  const next = active.current_index < active.total && isValidTarget(active.next) ? active.next : undefined;

  function selectTab(index: number, focus = false) {
    setActiveSeriesID(visibleTabs[index]?.series_id ?? null);
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

  return (
    <nav
      aria-label={t("tabsLabel")}
      className="rounded-lg border border-border-default bg-canvas-default p-4 shadow-none"
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
            <Link
              href={contentHref(previous.id, active.series_zone)}
              aria-label={t("previousA11y", { title: previous.title })}
              className={cn(buttonVariants({ variant: "outline" }), "min-h-11 min-w-0 max-w-full cursor-pointer justify-start px-3 min-[1101px]:max-w-56")}
            >
              <ChevronLeft aria-hidden="true" />
              <span className="truncate">{t("previous", { title: previous.title })}</span>
            </Link>
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

          <Link
            href={`/series/${active.series_id}`}
            aria-label={t("catalogA11y", { title: active.series_title })}
            className={cn(buttonVariants({ variant: "ghost" }), "min-h-11 min-w-0 max-w-full cursor-pointer px-3 min-[1101px]:max-w-44")}
          >
            <List aria-hidden="true" />
            <span className="truncate">{t("catalog")}</span>
          </Link>

          {next ? (
            <Link
              href={contentHref(next.id, active.series_zone)}
              aria-label={t("nextA11y", { title: next.title })}
              className={cn(buttonVariants({ variant: "outline" }), "col-span-2 min-h-11 min-w-0 max-w-full cursor-pointer justify-end px-3 sm:col-span-1 min-[1101px]:max-w-56")}
            >
              <span className="truncate">{t("next", { title: next.title })}</span>
              <ChevronRight aria-hidden="true" />
            </Link>
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
