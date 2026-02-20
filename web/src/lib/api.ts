const API_BASE = import.meta.env.VITE_API_URL || "";

export interface User {
  id: string;
  kakao_id: number;
  email?: string;
  nickname?: string;
  profile_image?: string;
  role: string;
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
  const res = await fetch(`${API_BASE}/api/v1/auth/me`, {
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
  await fetch(`${API_BASE}/api/v1/auth/logout`, {
    method: "POST",
    credentials: "include",
  });
}

// ── 관심사(구독) ──────────────────────────────────────
export async function getSubscriptions(): Promise<string[]> {
  const res = await fetch(`${API_BASE}/api/v1/subscriptions`, { credentials: "include" });
  if (!res.ok) throw new Error("Failed to fetch subscriptions");
  const body: ApiResponse<string[]> = await res.json();
  return body.data;
}

export async function addSubscription(category: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/subscriptions`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ category }),
  });
  if (!res.ok) throw new Error("Failed to add subscription");
}

export async function deleteSubscription(category: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/subscriptions?category=${encodeURIComponent(category)}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) throw new Error("Failed to delete subscription");
}

// ── 맥락 이력 ─────────────────────────────────────────
export interface HistoryItem {
  id: string;
  category: string;
  rank: number;
  topic: string;
  summary: string;
  detail: string;
}

export interface HistoryEntry {
  date: string;
  delivered_at: string;
  items: HistoryItem[];
}

export async function getContextHistory(): Promise<HistoryEntry[]> {
  const res = await fetch(`${API_BASE}/api/v1/context/history`, { credentials: "include" });
  if (!res.ok) throw new Error("Failed to fetch context history");
  const body: ApiResponse<HistoryEntry[]> = await res.json();
  return body.data;
}

// ── 전달 채널 ─────────────────────────────────────────
export interface ChannelPreference {
  channel: string;
  enabled: boolean;
}

export async function getDeliveryChannels(): Promise<ChannelPreference[]> {
  const res = await fetch(`${API_BASE}/api/v1/user/delivery-channels`, {
    credentials: "include",
  });
  if (!res.ok) throw new Error("Failed to fetch delivery channels");
  const body: { channels: ChannelPreference[] } = await res.json();
  return body.channels;
}

export async function updateDeliveryChannels(
  channels: ChannelPreference[]
): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/user/delivery-channels`, {
    method: "PUT",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ channels }),
  });
  if (!res.ok) throw new Error("Failed to update delivery channels");
}

// ── 전달 상태 ───────────────────────────────────────
export interface ChannelDeliveryStatus {
  channel: string;
  status: "sent" | "failed" | "skipped";
  error_message?: string;
  retry_count: number;
  last_attempt: string;
}

export async function getDeliveryStatus(): Promise<ChannelDeliveryStatus[]> {
  const res = await fetch(`${API_BASE}/api/v1/user/delivery-status`, {
    credentials: "include",
  });
  if (!res.ok) throw new Error("Failed to fetch delivery status");
  const body: ApiResponse<ChannelDeliveryStatus[]> = await res.json();
  return body.data;
}

// ── 즉시 전송 ─────────────────────────────────────────
export interface SendBriefingResult {
  success_count: number;
  failure_count: number;
  skipped_count: number;
  errors: Record<string, string>;
}

export async function sendBriefingNow(): Promise<SendBriefingResult> {
  const res = await fetch(`${API_BASE}/api/v1/delivery/send`, {
    method: "POST",
    credentials: "include",
  });

  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "브리핑 전송에 실패했습니다");
  }

  const body: ApiResponse<SendBriefingResult> = await res.json();
  return body.data;
}

// ── 주제 상세 ─────────────────────────────────────────
export interface TopicDetail {
  id: string;
  topic: string;
  detail: string;
  sources: string[];
  created_at: string;
}

export async function fetchTopicDetail(id: string): Promise<TopicDetail> {
  const res = await fetch(`${API_BASE}/api/v1/context/topic/${id}`);
  if (res.status === 404) throw new Error("not_found");
  if (!res.ok) throw new Error("server_error");
  const body: ApiResponse<TopicDetail> = await res.json();
  return body.data;
}

// ── 이메일 인증 ───────────────────────────────────────
export async function sendVerificationCode(email: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/email-verification/send-code`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email }),
  });

  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "인증 코드 전송에 실패했습니다");
  }
}

export async function verifyEmailCode(code: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/email-verification/verify-code`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),
  });

  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "인증 코드 확인에 실패했습니다");
  }
}

// ── 어드민 ─────────────────────────────────────────────
export interface CollectionResult {
  run_id: string;
  item_count: number;
}

export async function triggerCollection(): Promise<CollectionResult> {
  const res = await fetch(`${API_BASE}/api/v1/admin/collect`, {
    method: "POST",
    credentials: "include",
  });

  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "수집 실행에 실패했습니다");
  }

  const body: ApiResponse<CollectionResult> = await res.json();
  return body.data;
}
