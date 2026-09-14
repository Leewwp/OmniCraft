import { getServerApiBase } from "@/lib/server-api";
import { notFound } from "next/navigation";
import { getTranslations, getLocale } from 'next-intl/server';
import { ProfileSummaryCard } from "./ProfileSummaryCard";
import { UserProfileClient } from "./UserProfileClient";
import { normalizeProfileTab } from "./profile-tab";

interface UserData {
  id?: number;
  username?: string;
  avatar_url?: string;
  bio?: string;
  reputation?: number;
  created_at?: string;
  followers_count?: number;
  stats?: { contents_count?: number; likes_received?: number };
}


async function fetchUser(apiBase: string, userId: string): Promise<UserData | null> {
  try {
    const res = await fetch(`${apiBase}/users/${userId}`, { next: { revalidate: 30 } });
    if (!res.ok) return null;
    const data = await res.json();
    return data.user || data;
  } catch {
    return null;
  }
}

export default async function UserProfilePage({
  params,
  searchParams,
}: {
  params: Promise<{ userId: string }>;
  searchParams: Promise<{ tab?: string }>;
}) {
  const t = await getTranslations();
  const locale = await getLocale();
  const { userId } = await params;
  const { tab } = await searchParams;
  const apiBase = getServerApiBase();
  const user = await fetchUser(apiBase, userId);

  if (!user) {
    notFound();
  }

  const userIdNum = user.id ?? 0;
  const displayName = user.username ?? t('common.userLabel', { id: userId });
  const stats = {
    contents: user.stats?.contents_count ?? 0,
    likes: user.stats?.likes_received ?? 0,
    followers: user.followers_count ?? 0,
  };

  return (
    <div className="mx-auto w-full max-w-4xl space-y-6 px-4 py-6">
      <ProfileSummaryCard
        displayName={displayName}
        avatarUrl={user.avatar_url}
        bio={user.bio ?? ""}
        meta={
          <span>
            {t('user.reputation', { reputation: user.reputation ?? 0 })}{" "}
            {user.created_at
              ? new Date(user.created_at).toLocaleDateString(locale === "en" ? "en-US" : "zh-CN")
              : "-"}
          </span>
        }
        stats={stats}
      />

      <UserProfileClient userId={userIdNum} displayName={displayName} initialTab={normalizeProfileTab(tab)} />
    </div>
  );
}
