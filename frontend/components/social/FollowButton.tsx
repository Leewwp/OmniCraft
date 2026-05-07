"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Plus, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/AuthContext";
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
  const { user } = useAuth();
  const [following, setFollowing] = useState(initialFollowing);
  const [busy, setBusy] = useState(false);

  async function toggle() {
    if (!user) {
      router.push("/login");
      return;
    }
    setBusy(true);
    try {
      if (following) {
        await api.delete(`/api/v1/${targetType}s/${targetId}/follow`);
        setFollowing(false);
      } else {
        await api.post(`/api/v1/${targetType}s/${targetId}/follow`, {});
        setFollowing(true);
      }
    } catch {
      // ignore
    } finally {
      setBusy(false);
    }
  }

  return (
    <Button
      size="sm"
      variant={following ? "default" : "outline"}
      className={cn("gap-1", className)}
      onClick={toggle}
      disabled={busy}
    >
      {following ? (
        <>
          <Check className="h-3.5 w-3.5" />
          {t("social.following")}
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
