"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import useSWR from "swr";
import { api } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { FollowButton } from "@/components/social/FollowButton";
import { MessageComposeButton } from "@/components/social/MessageComposeButton";
import { cn } from "@/lib/utils";

/* SP-17/T3 (#492)：用户悬浮卡——P-01 原型 UserIdentity 的生产版
   （docs/working/2026-07-25-ui-prototype-review.md §6.2 A1.6 裁决）。
   触发：桌面悬停 200ms 开/移开延迟关；键盘聚焦立即；点击头像/昵称始终进
   /user/:id；触屏（hover:none）不弹卡直接导航。
   定位：触发元下方居中（detail-creator）或左对齐（dynamic，评论作者），
   视口/浮层可视区钳制 + 下方不足上翻；scroll/resize 跟随重定位，触发元
   离屏即关。数据：GET /users/:id（SWR 缓存去重，仅浮卡打开时取数）。 */

const HOVER_DELAY_MS = 200;
const CARD_WIDTH = 300;
const CARD_EST_HEIGHT = 230;
const VIEWPORT_MARGIN = 8;

/** 详情浮窗内钳制的滚动可视区（#88 单列 overlay-scroller / 双栏 layer-scroller）。 */
const OVERLAY_SCROLLER_SELECTOR = '[data-slot="overlay-scroller"], [data-slot="layer-scroller"]';

interface HoverCardUser {
  id: number;
  username: string;
  avatar_url: string;
  bio: string;
  followers_count: number;
  is_following: boolean;
  stats: { contents_count: number; likes_received: number };
}

interface UserHoverCardProps {
  userId?: number;
  username: string;
  /** 详情响应的签名头像 URL（触发元展示用；浮卡内用 SWR 新鲜数据）。 */
  avatarUrl?: string;
  /** 触发头像尺寸（px），默认 32。 */
  size?: number;
  /** dynamic=评论作者（左对齐、视口钳制）；detail-creator=创作者区（下方居中、浮层可视区钳制）。 */
  placement?: "dynamic" | "detail-creator";
  className?: string;
}

function useHoverCardUser(userId: number | undefined, open: boolean) {
  const { data } = useSWR<HoverCardUser>(
    open && userId ? `/api/v1/users/${userId}` : null,
    (url: string) => api.get<{ user: HoverCardUser }>(url).then((res) => res.user),
    { revalidateOnFocus: false, dedupingInterval: 60000 },
  );
  return data;
}

export function UserHoverCard({
  userId,
  username,
  avatarUrl,
  size = 32,
  placement = "dynamic",
  className,
}: UserHoverCardProps) {
  const t = useTranslations();
  const { user: viewer } = useAuth();
  const [open, setOpen] = useState(false);
  const [hoverCapable, setHoverCapable] = useState(false);
  const [position, setPosition] = useState<{ left: number; top: number } | null>(null);
  const wrapRef = useRef<HTMLSpanElement>(null);
  const triggerRef = useRef<HTMLAnchorElement>(null);
  const openTimerRef = useRef<number | null>(null);
  const closeTimerRef = useRef<number | null>(null);

  const profile = useHoverCardUser(userId, open);
  const isSelf = !!viewer && viewer.id === userId;

  useEffect(() => {
    if (typeof window.matchMedia !== "function") return;
    const media = window.matchMedia("(hover: hover) and (pointer: fine)");
    const sync = () => setHoverCapable(media.matches);
    sync();
    media.addEventListener("change", sync);
    return () => media.removeEventListener("change", sync);
  }, []);

  const clearTimers = useCallback(() => {
    if (openTimerRef.current !== null) window.clearTimeout(openTimerRef.current);
    if (closeTimerRef.current !== null) window.clearTimeout(closeTimerRef.current);
    openTimerRef.current = null;
    closeTimerRef.current = null;
  }, []);

  const computePosition = useCallback(() => {
    const trigger = triggerRef.current;
    if (!trigger) return null;
    const rect = trigger.getBoundingClientRect();
    if (rect.bottom < 0 || rect.top > window.innerHeight || rect.right < 0 || rect.left > window.innerWidth) {
      return null;
    }
    // 详情浮窗内钳制该浮窗可视区（top layer 子树内 fixed 定位按视口坐标）。
    const viewport = trigger.closest<HTMLElement>(OVERLAY_SCROLLER_SELECTOR)?.getBoundingClientRect();
    const visibleLeft = Math.max(VIEWPORT_MARGIN, viewport?.left ?? VIEWPORT_MARGIN);
    const visibleRight = Math.min(
      window.innerWidth - VIEWPORT_MARGIN,
      viewport?.right ?? window.innerWidth - VIEWPORT_MARGIN,
    );
    const visibleTop = Math.max(VIEWPORT_MARGIN, viewport?.top ?? VIEWPORT_MARGIN);
    const visibleBottom = Math.min(
      window.innerHeight - VIEWPORT_MARGIN,
      viewport?.bottom ?? window.innerHeight - VIEWPORT_MARGIN,
    );
    const anchoredLeft =
      placement === "detail-creator"
        ? rect.left + rect.width / 2 - CARD_WIDTH / 2
        : rect.left;
    const left = Math.min(Math.max(visibleLeft, anchoredLeft), Math.max(visibleLeft, visibleRight - CARD_WIDTH));
    const belowTop = rect.bottom + VIEWPORT_MARGIN;
    const top =
      belowTop + CARD_EST_HEIGHT <= visibleBottom
        ? belowTop
        : Math.max(visibleTop, rect.top - CARD_EST_HEIGHT - VIEWPORT_MARGIN);
    return { left, top };
  }, [placement]);

  const showNow = useCallback(() => {
    clearTimers();
    setPosition(computePosition());
    setOpen(true);
  }, [clearTimers, computePosition]);

  const scheduleOpen = useCallback(() => {
    clearTimers();
    openTimerRef.current = window.setTimeout(showNow, HOVER_DELAY_MS);
  }, [clearTimers, showNow]);

  const scheduleClose = useCallback(() => {
    clearTimers();
    closeTimerRef.current = window.setTimeout(() => setOpen(false), HOVER_DELAY_MS);
  }, [clearTimers]);

  const closeNow = useCallback(() => {
    clearTimers();
    setOpen(false);
  }, [clearTimers]);

  /* 滚动/缩放跟随重定位；computePosition 返回 null（触发元离屏）时关闭。 */
  useEffect(() => {
    if (!open) return;
    function onScrollOrResize() {
      setPosition((prev) => {
        const next = computePosition();
        if (!next) {
          setOpen(false);
          return prev;
        }
        return next;
      });
    }
    window.addEventListener("scroll", onScrollOrResize, true);
    window.addEventListener("resize", onScrollOrResize);
    return () => {
      window.removeEventListener("scroll", onScrollOrResize, true);
      window.removeEventListener("resize", onScrollOrResize);
    };
  }, [open, computePosition]);

  /* Esc 关闭并归还焦点；preventDefault 避免连带关闭外层详情浮窗 dialog。 */
  useEffect(() => {
    if (!open) return;
    function onKey(event: KeyboardEvent) {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopPropagation();
      closeNow();
      if (document.activeElement && wrapRef.current?.contains(document.activeElement)) {
        triggerRef.current?.focus();
      }
    }
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [open, closeNow]);

  useEffect(() => () => clearTimers(), [clearTimers]);

  const avatar = (src: string | undefined, px: number) =>
    src ? (
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={src}
        alt=""
        loading="lazy"
        className="shrink-0 rounded-full bg-muted object-cover"
        style={{ width: px, height: px }}
      />
    ) : (
      <span
        aria-hidden="true"
        className="flex shrink-0 items-center justify-center rounded-full bg-accent-subtle font-semibold text-accent-emphasis"
        style={{ width: px, height: px, fontSize: Math.max(11, Math.round(px * 0.4)) }}
      >
        {username.slice(0, 1)}
      </span>
    );

  /* 无 userId（极旧数据/匿名兜底）：退化为纯展示，不挂交互。 */
  if (userId == null) {
    return (
      <span className={cn("inline-flex min-w-0 items-center gap-2 text-sm text-muted-foreground", className)}>
        {avatar(avatarUrl, size)}
        <span className="truncate">{username}</span>
      </span>
    );
  }

  return (
    <span
      ref={wrapRef}
      className={cn("inline-flex min-w-0", className)}
      onPointerEnter={hoverCapable ? scheduleOpen : undefined}
      onPointerLeave={hoverCapable ? scheduleClose : undefined}
      onFocus={(event) => {
        if (event.target === triggerRef.current) showNow();
      }}
      onBlur={(event) => {
        const next = event.relatedTarget as Node | null;
        if (!next || !wrapRef.current?.contains(next)) closeNow();
      }}
    >
      <Link
        ref={triggerRef}
        href={`/user/${userId}`}
        aria-expanded={hoverCapable ? open : undefined}
        className="flex min-w-0 items-center gap-2 rounded-md text-sm font-medium text-foreground transition-colors hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        onClick={(event) => {
          event.stopPropagation();
          closeNow();
        }}
      >
        {avatar(avatarUrl, size)}
        <span className="truncate">{username}</span>
      </Link>
      {open && hoverCapable && position && (
        <span
          role="dialog"
          aria-label={t("user.hoverCardLabel", { name: username })}
          className="fixed z-50 block w-[300px] rounded-lg border border-border bg-card p-4 shadow-md"
          style={{ left: position.left, top: position.top }}
          onPointerEnter={clearTimers}
          onPointerLeave={scheduleClose}
        >
          <span className="flex items-start gap-3">
            {avatar(profile?.avatar_url || avatarUrl, 56)}
            <span className="min-w-0 flex-1">
              <Link
                href={`/user/${userId}`}
                className="block truncate text-sm font-semibold text-foreground hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                onClick={(event) => {
                  event.stopPropagation();
                  closeNow();
                }}
              >
                {profile?.username ?? username}
              </Link>
              {profile?.bio ? (
                <span className="mt-1 line-clamp-3 block text-xs text-muted-foreground">{profile.bio}</span>
              ) : null}
            </span>
          </span>
          <span className="mt-3 flex items-center gap-4 text-xs text-muted-foreground">
            <span>
              <strong className="font-semibold text-foreground">{profile?.stats?.contents_count ?? 0}</strong>{" "}
              {t("user.statContents")}
            </span>
            <span>
              <strong className="font-semibold text-foreground">{profile?.stats?.likes_received ?? 0}</strong>{" "}
              {t("user.statLikes")}
            </span>
            <span>
              <strong className="font-semibold text-foreground">{profile?.followers_count ?? 0}</strong>{" "}
              {t("user.statFollowers")}
            </span>
          </span>
          <span className="mt-3 flex items-center justify-end gap-2">
            {isSelf ? (
              <Link
                href={`/user/${userId}`}
                className="inline-flex h-8 items-center rounded-md border border-border px-3 text-xs font-medium text-foreground transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                onClick={(event) => {
                  event.stopPropagation();
                  closeNow();
                }}
              >
                {t("user.viewProfile")}
              </Link>
            ) : (
              <>
                <MessageComposeButton userId={userId} displayName={profile?.username ?? username} />
                <FollowButton targetType="user" targetId={userId} initialFollowing={profile?.is_following ?? false} />
              </>
            )}
          </span>
        </span>
      )}
    </span>
  );
}
