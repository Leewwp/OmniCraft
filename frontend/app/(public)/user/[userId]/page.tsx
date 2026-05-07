import { notFound } from "next/navigation";
import { getTranslations } from 'next-intl/server';
import { UserProfileClient } from "./UserProfileClient";

interface UserData {
  id?: number;
  username?: string;
  avatar_url?: string;
  bio?: string;
  reputation?: number;
  created_at?: string;
}

function getApiBase() {
  const raw = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  return `${raw.replace(/\/$/, "")}/api/v1`;
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
}: {
  params: Promise<{ userId: string }>;
}) {
  const t = await getTranslations();
  const { userId } = await params;
  const apiBase = getApiBase();
  const user = await fetchUser(apiBase, userId);

  if (!user) {
    notFound();
  }

  const userIdNum = user.id ?? 0;
  const displayName = user.username ?? t('common.userLabel', { id: userId });
  const bio = user.bio ?? "";
  const reputation = user.reputation ?? 0;
  const createdAt = user.created_at;

  return (
    <div className="mx-auto w-full max-w-4xl space-y-6 px-4 py-6">
      <div className="rounded-md border border-border bg-card p-6 shadow-none">
        <div className="flex items-start gap-4">
          <div className="flex h-16 w-16 shrink-0 items-center justify-center rounded-full bg-muted text-xl font-bold text-muted-foreground">
            {displayName.slice(0, 1)}
          </div>
          <div className="space-y-1">
            <h1 className="text-2xl font-bold tracking-tight">{displayName}</h1>
            <p className="text-sm text-muted-foreground">
              {t('user.reputation', { reputation })}{" "}
              {createdAt
                ? new Date(createdAt).toLocaleDateString("zh-CN")
                : "-"}
            </p>
            {bio && <p className="text-sm text-foreground/80">{bio}</p>}
          </div>
        </div>
      </div>

      <UserProfileClient userId={userIdNum} displayName={displayName} />
    </div>
  );
}
