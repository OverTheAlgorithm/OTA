export interface User {
  id: string;
  kakao_id: number;
  email?: string;
  nickname?: string;
  profile_image?: string;
  created_at: string;
  updated_at: string;
}

interface ApiResponse<T> {
  data: T;
}

interface ApiError {
  error: string;
}

export async function fetchMe(): Promise<User> {
  const res = await fetch("/api/v1/auth/me", {
    credentials: "include",
  });

  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "Failed to fetch user");
  }

  const body: ApiResponse<User> = await res.json();
  return body.data;
}

export async function logout(): Promise<void> {
  await fetch("/api/v1/auth/logout", {
    method: "POST",
    credentials: "include",
  });
}

// ── 관심사(구독) ──────────────────────────────────────
export async function getSubscriptions(): Promise<string[]> {
  const res = await fetch("/api/v1/subscriptions", { credentials: "include" });
  if (!res.ok) throw new Error("Failed to fetch subscriptions");
  const body: ApiResponse<string[]> = await res.json();
  return body.data;
}

export async function addSubscription(category: string): Promise<void> {
  const res = await fetch("/api/v1/subscriptions", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ category }),
  });
  if (!res.ok) throw new Error("Failed to add subscription");
}

export async function deleteSubscription(category: string): Promise<void> {
  const res = await fetch(`/api/v1/subscriptions/${encodeURIComponent(category)}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) throw new Error("Failed to delete subscription");
}

// ── 맥락 이력 ─────────────────────────────────────────
export interface HistoryItem {
  category: string;
  rank: number;
  topic: string;
  summary: string;
}

export interface HistoryEntry {
  date: string;
  delivered_at: string;
  items: HistoryItem[];
}

export async function getContextHistory(): Promise<HistoryEntry[]> {
  const res = await fetch("/api/v1/context/history", { credentials: "include" });
  if (!res.ok) throw new Error("Failed to fetch context history");
  const body: ApiResponse<HistoryEntry[]> = await res.json();
  return body.data;
}
