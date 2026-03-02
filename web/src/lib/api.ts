const API_BASE = import.meta.env.VITE_API_URL || "";

export interface User {
  id: string;
  kakao_id: number;
  email?: string;
  email_verified: boolean;
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
export interface DetailItem {
  title: string;
  content: string;
}

export interface HistoryItem {
  id: string;
  category: string;
  brain_category: string;
  rank: number;
  topic: string;
  summary: string;
  detail: string;
  details: DetailItem[];
  buzz_score: number;
}

export interface HistoryEntry {
  date: string;
  delivered_at: string;
  items: HistoryItem[];
}

export interface HistoryPage {
  data: HistoryEntry[];
  has_more: boolean;
}

export async function getContextHistory(limit: number, offset: number): Promise<HistoryPage> {
  const res = await fetch(`${API_BASE}/api/v1/context/history?limit=${limit}&offset=${offset}`, { credentials: "include" });
  if (!res.ok) throw new Error("Failed to fetch context history");
  const body = await res.json();
  return { data: body.data ?? [], has_more: body.has_more ?? false };
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
  details: DetailItem[];
  buzz_score: number;
  sources: string[];
  brain_category: string;
  created_at: string;
}

export interface TopicEarnResult {
  attempted: boolean;
  earned: boolean;
  points_earned: number;
  leveled_up: boolean;
  new_level: number;
}

export interface TopicDetailResponse {
  data: TopicDetail;
  earn_result: TopicEarnResult | null;
}

export async function fetchTopicDetail(
  id: string,
  params?: { uid?: string; rid?: string }
): Promise<TopicDetailResponse> {
  const url = new URL(`${API_BASE}/api/v1/context/topic/${id}`, window.location.origin);
  if (params?.uid) url.searchParams.set("uid", params.uid);
  if (params?.rid) url.searchParams.set("rid", params.rid);
  const res = await fetch(url.toString());
  if (res.status === 404) throw new Error("not_found");
  if (!res.ok) throw new Error("server_error");
  return res.json() as Promise<TopicDetailResponse>;
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

// ── Brain Category ────────────────────────────────────
export interface BrainCategory {
  key: string;
  emoji: string;
  label: string;
  accent_color: string;
  display_order: number;
  created_at: string;
  updated_at: string;
}

export async function getBrainCategories(): Promise<BrainCategory[]> {
  const res = await fetch(`${API_BASE}/api/v1/brain-categories`);
  if (!res.ok) throw new Error("Failed to fetch brain categories");
  const body: ApiResponse<BrainCategory[]> = await res.json();
  return body.data;
}

export async function createBrainCategory(bc: Omit<BrainCategory, "created_at" | "updated_at">): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/admin/brain-categories`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(bc),
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "Failed to create brain category");
  }
}

export async function updateBrainCategory(key: string, bc: Partial<BrainCategory>): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/admin/brain-categories/${encodeURIComponent(key)}`, {
    method: "PUT",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(bc),
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "Failed to update brain category");
  }
}

export async function deleteBrainCategory(key: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/admin/brain-categories/${encodeURIComponent(key)}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "Failed to delete brain category");
  }
}

// ── 어드민 ─────────────────────────────────────────────
export async function triggerCollection(): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/admin/collect`, {
    method: "POST",
    credentials: "include",
  });

  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "수집 실행에 실패했습니다");
  }
  // 202 Accepted: collection runs in background, result sent via Slack
}

export interface TestEmailResult {
  success_count: number;
  skipped_count: number;
  failure_count: number;
  errors: Record<string, string>;
}

export async function sendTestEmail(): Promise<TestEmailResult> {
  const res = await fetch(`${API_BASE}/api/v1/admin/delivery/send-test`, {
    method: "POST",
    credentials: "include",
  });
  let body: Record<string, unknown>;
  try {
    body = await res.json();
  } catch {
    throw new Error("서버 응답을 파싱할 수 없습니다");
  }
  if (!res.ok) throw new Error((body.error as string) || "테스트 이메일 전송에 실패했습니다");
  return body as unknown as TestEmailResult;
}

// ── 레벨 시스템 ─────────────────────────────────────
export interface LevelInfo {
  level: number;
  total_points: number;
  current_progress: number;
  points_to_next: number;
  description: string;
}

export interface EarnResult {
  earned: boolean;
  level: number;
  total_points: number;
  current_progress: number;
  points_to_next: number;
  leveled_up: boolean;
}

export async function getUserLevel(): Promise<LevelInfo> {
  const res = await fetch(`${API_BASE}/api/v1/level`, {
    credentials: "include",
  });
  if (!res.ok) throw new Error("Failed to fetch level");
  const body: ApiResponse<LevelInfo> = await res.json();
  return body.data;
}

export async function earnPoint(contextItemId: string): Promise<EarnResult> {
  const res = await fetch(`${API_BASE}/api/v1/level/earn`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ context_item_id: contextItemId }),
  });
  if (!res.ok) throw new Error("Failed to earn point");
  const body: ApiResponse<EarnResult> = await res.json();
  return body.data;
}
