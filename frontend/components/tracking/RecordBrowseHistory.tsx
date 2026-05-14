"use client";

import { useEffect } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";

interface RecordBrowseHistoryProps {
  contentType: "content" | "ip";
  targetId: number;
}

export function RecordBrowseHistory({ contentType, targetId }: RecordBrowseHistoryProps) {
  const { user } = useAuth();

  useEffect(() => {
    if (!user) return;
    const key = `browse_${contentType}_${targetId}`;
    const lastRecorded = localStorage.getItem(key);
    const now = Date.now();
    if (lastRecorded && now - parseInt(lastRecorded, 10) < 5 * 60 * 1000) return;
    localStorage.setItem(key, String(now));
    api.post("/api/v1/users/me/history", { [contentType === "ip" ? "ip_id" : "content_item_id"]: targetId }).catch(() => {});
  }, [user, contentType, targetId]);

  return null;
}
