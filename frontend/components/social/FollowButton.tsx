"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { useToast } from "@/components/ui/Toast";
import { useAuth, interactionDenialKey } from "@/contexts/AuthContext";
import { useAuthGate } from "@/components/auth/AuthGateProvider";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";

interface FollowButtonProps {
  targetType: "user" | "ip";
  targetId: number;
  initialFollowing?: boolean;
  className?: string;
  /** 关注成功后的回调（提案页一键关注解锁投票权等原地联动）。 */
  onFollowed?: () => void;
}

/* #415 O1b 全站关注按钮唯一规范（2026-09-07 裁决）：
   - 宽度恒定：以最长文案「取消关注」为隐藏占位（grid 同格叠放），任何
     状态/悬停切换宽度不变；
   - 已关注与未关注底色一致（primary 实底），勾号图标移除（无图标）；
   - 悬停已关注 → 文案变「取消关注」+ destructive 红边红字；
   - 未登录点击打开登录浮窗（SP-17/T2，登录成功自动续做）、信誉禁用等
     既有行为保持。 */
export function FollowButton({ targetType, targetId, initialFollowing = false, className, onFollowed }: FollowButtonProps) {
  const t = useTranslations();
  const { user, capabilities } = useAuth();
  const { requireAuth } = useAuthGate();
  const { toast } = useToast();
  const [following, setFollowing] = useState(initialFollowing);
  const isFollowing = !!user && following;
  const [busy, setBusy] = useState(false);

  const interactionBlocked = !!user && !capabilities.can_interact;

  async function performToggle() {
    if (interactionBlocked) return;
    setBusy(true);
    const wasFollowing = isFollowing;
    try {
      if (wasFollowing) {
        await api.delete(`/api/v1/${targetType}s/${targetId}/follow`);
        setFollowing(false);
      } else {
        await api.post(`/api/v1/${targetType}s/${targetId}/follow`, {});
        setFollowing(true);
        onFollowed?.();
      }
    } catch {
      setFollowing(wasFollowing);
      toast("error", t("common.operationFailed"));
    } finally {
      setBusy(false);
    }
  }

  function toggle() {
    // SP-17/T2：门与动作分离——pendingAction 不得依赖触发时的 user state。
    if (!user) {
      requireAuth(() => void performToggle());
      return;
    }
    void performToggle();
  }

  const unfollowLabel = t("social.unfollow");

  return (
    <Button
      size="sm"
      variant="default"
      className={cn(
        "group",
        isFollowing &&
          "hover:border-destructive! hover:text-destructive! focus-visible:border-destructive focus-visible:text-destructive",
        className,
      )}
      onClick={toggle}
      disabled={interactionBlocked || busy}
      title={interactionBlocked ? t(interactionDenialKey(capabilities.interaction_denial_reason)) : undefined}
    >
      {/* 恒宽占位：最长文案「取消关注」常驻隐藏格，宽度任何状态下不变 */}
      <span className="grid justify-items-center">
        <span className="invisible col-start-1 row-start-1" aria-hidden="true">{unfollowLabel}</span>
        {isFollowing ? (
          <>
            <span className="col-start-1 row-start-1 group-hover:hidden group-focus-visible:hidden">{t("social.following")}</span>
            <span className="hidden col-start-1 row-start-1 group-hover:inline group-focus-visible:inline">{unfollowLabel}</span>
          </>
        ) : (
          <span className="col-start-1 row-start-1">{t("social.follow")}</span>
        )}
      </span>
    </Button>
  );
}
