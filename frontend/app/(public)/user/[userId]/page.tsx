import { notFound } from "next/navigation";
import { UserProfileClient } from "./UserProfileClient";

interface UserData {
  id?: number;
  ID?: number;
  username?: string;
  Username?: string;
  email?: string;
  avatar_url?: string;
  AvatarURL?: string;
  bio?: string;
  Bio?: string;
  reputation?: number;
  Reputation?: number;
  role?: string;
  Role?: string;
  preferred_locale?: string;
  created_at?: string;
  CreatedAt?: string;
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
  const { userId } = await params;
  const apiBase = getApiBase();
  const user = await fetchUser(apiBase, userId);

  if (!user) {
    notFound();
  }

  const userIdNum = user.ID ?? user.id ?? 0;
  const displayName = user.Username ?? user.username ?? `用户 #${userId}`;
  const bio = user.Bio ?? user.bio ?? "";
  const avatar = user.AvatarURL ?? user.avatar_url;
  const reputation = user.Reputation ?? user.reputation ?? 0;
  const createdAt = user.CreatedAt ?? user.created_at;

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
              信誉分：{reputation} · 加入于{" "}
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
