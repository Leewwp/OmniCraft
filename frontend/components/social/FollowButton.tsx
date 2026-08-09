"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Plus, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useToast } from "@/components/ui/Toast";
import { useAuth, interactionDenialKey } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";

interface FollowButtonProps {
  targetType: "user" | "ip";
  targetId: number;
  initialFollowing?: boolean;
  className?: string;
}

export function FollowButton({ targetType, targetId, initialFollowing = false, className }: FollowButtonProps) {
  const t = useTranslations();
  const router = useRouter();
  const { user, capabilities } = useAuth();
  const { toast } = useToast();
  const [following, setFollowing] = useState(initialFollowing);
  const isFollowing = !!user && following;
  const [busy, setBusy] = useState(false);

  const interactionBlocked = !!user && !capabilities.can_interact;

  async function toggle() {
    if (!user) {
      router.push("/login");
      return;
    }
    if (interactionBlocked) return;
    setBusy(true);
    const previousState = isFollowing;
    try {
      if (isFollowing) {
        await api.delete(`/api/v1/${targetType}s/${targetId}/follow`);
        setFollowing(false);
      } else {
        await api.post(`/api/v1/${targetType}s/${targetId}/follow`, {});
        setFollowing(true);
      }
    } catch {
      setFollowing(previousState);
      toast("error", t("common.operationFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Button
      size="sm"
      variant={isFollowing ? "outline" : "default"}
      className={cn(
        "group gap-1",
        isFollowing &&
          "hover:border-destructive! hover:text-destructive! focus-visible:border-destructive focus-visible:text-destructive",
        className,
      )}
      onClick={toggle}
      disabled={interactionBlocked || busy}
      title={interactionBlocked ? t(interactionDenialKey(capabilities.interaction_denial_reason)) : undefined}
    >
      {isFollowing ? (
        <>
          <Check className="h-3.5 w-3.5" />
          <span className="group-hover:hidden group-focus-visible:hidden">{t("social.following")}</span>
          <span className="hidden group-hover:inline group-focus-visible:inline">{t("social.unfollow")}</span>
        </>
      ) : (
        <>
          <Plus className="h-3.5 w-3.5" />
          {t("social.follow")}
        </>
      )}
    </Button>
  );
}
