const API_BASE = import.meta.env.VITE_API_URL || "";

export { default as defaultImage } from "@/assets/wizletter_default.png";

// -- Token refresh ------------------------------------------------------------

let isRefreshing = false;
let refreshPromise: Promise<boolean> | null = null;

async function tryRefreshToken(): Promise<boolean> {
  if (isRefreshing && refreshPromise) {
    return refreshPromise;
  }
  isRefreshing = true;
  refreshPromise = fetch(`${API_BASE}/api/v1/auth/refresh`, {
    method: "POST",
    credentials: "include",
  })
    .then((res) => res.ok)
    .catch(() => false)
    .finally(() => {
      isRefreshing = false;
      refreshPromise = null;
    });
  return refreshPromise;
}

// apiFetch wraps fetch with automatic 401 -> refresh -> retry logic.
// If refresh also fails, returns the original 401 response so callers
// decide how to handle the unauthenticated state.
export async function apiFetch(
  input: RequestInfo | URL,
  init?: RequestInit
): Promise<Response> {
  const res = await fetch(input, { credentials: "include", ...init });

  if (res.status === 429) {
    throw new Error("요청이 너무 잦습니다. 잠시 후 다시 시도해주세요.");
  }

  if (res.status !== 401) return res;

  const refreshed = await tryRefreshToken();
  if (!refreshed) return res;

  return fetch(input, { credentials: "include", ...init });
}

// -----------------------------------------------------------------------------


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
  const res = await apiFetch(`${API_BASE}/api/v1/auth/me`);

  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "Failed to fetch user");
  }

  const body: ApiResponse<User> = await res.json();
  return body.data;
}

export async function logout(): Promise<void> {
  await apiFetch(`${API_BASE}/api/v1/auth/logout`, {
    method: "POST",
    credentials: "include",
  });
}

// ── 관심사(구독) ──────────────────────────────────────
export async function getSubscriptions(): Promise<string[]> {
  const res = await apiFetch(`${API_BASE}/api/v1/subscriptions`, { credentials: "include" });
  if (!res.ok) throw new Error("Failed to fetch subscriptions");
  const body: ApiResponse<string[]> = await res.json();
  return Array.isArray(body.data) ? body.data : [];
}

export async function addSubscription(category: string): Promise<void> {
  const res = await apiFetch(`${API_BASE}/api/v1/subscriptions`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ category }),
  });
  if (!res.ok) throw new Error("Failed to add subscription");
}

export async function deleteSubscription(category: string): Promise<void> {
  const res = await apiFetch(`${API_BASE}/api/v1/subscriptions?category=${encodeURIComponent(category)}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) throw new Error("Failed to delete subscription");
}

// ── 소식 이력 ─────────────────────────────────────────
export interface DetailItem {
  title: string;
  content: string;
}

export interface HistoryItem {
  id: string;
  category: string;
  priority: string;
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
  const res = await apiFetch(`${API_BASE}/api/v1/context/history?limit=${limit}&offset=${offset}`, { credentials: "include" });
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
  const res = await apiFetch(`${API_BASE}/api/v1/user/delivery-channels`, {
    credentials: "include",
  });
  if (!res.ok) throw new Error("Failed to fetch delivery channels");
  const body: { channels: ChannelPreference[] } = await res.json();
  return body.channels;
}

export async function updateDeliveryChannels(
  channels: ChannelPreference[]
): Promise<void> {
  const res = await apiFetch(`${API_BASE}/api/v1/user/delivery-channels`, {
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
  const res = await apiFetch(`${API_BASE}/api/v1/user/delivery-status`, {
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
  const res = await apiFetch(`${API_BASE}/api/v1/delivery/send`, {
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
export interface QuizForUser {
  id: string;
  question: string;
  options: string[];
}

export interface QuizSubmitResult {
  correct: boolean;
  coins_earned: number;
  total_coins: number;
}

export interface TopicDetail {
  id: string;
  topic: string;
  category: string;
  detail: string;
  details: DetailItem[];
  buzz_score: number;
  sources: string[];
  brain_category: string;
  created_at: string;
  image_url: string | null;
  has_quiz: boolean;
  quiz: QuizForUser | null;
}

export interface TopicEarnResult {
  attempted: boolean;
  earned: boolean;
  reason: string; // "EARNED" | "DUPLICATE" | "EXPIRED" | "DAILY_LIMIT"
  coins_earned: number;
  leveled_up: boolean;
  new_level: number;
}

export async function fetchTopicDetail(id: string): Promise<TopicDetail> {
  const res = await apiFetch(`${API_BASE}/api/v1/context/topic/${id}`);
  if (res.status === 404) throw new Error("not_found");
  if (!res.ok) throw new Error("server_error");
  const body: ApiResponse<TopicDetail> = await res.json();
  return body.data;
}

export async function earnCoin(
  contextItemId: string,
  turnstileToken: string
): Promise<TopicEarnResult> {
  const res = await apiFetch(`${API_BASE}/api/v1/level/earn`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      context_item_id: contextItemId,
      turnstile_token: turnstileToken,
    }),
  });
  if (!res.ok) {
    const err: ApiError = await res.json().catch(() => ({ error: "earn_failed" }));
    throw new Error(err.error || "earn_failed");
  }
  const body: ApiResponse<TopicEarnResult> = await res.json();
  return body.data;
}

export interface InitEarnResult {
  status: "PENDING" | "EXPIRED" | "DUPLICATE" | "DAILY_LIMIT";
  required_seconds?: number;
}

export async function initEarn(
  contextItemId: string
): Promise<InitEarnResult> {
  const res = await apiFetch(`${API_BASE}/api/v1/level/init-earn`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ context_item_id: contextItemId }),
  });
  if (!res.ok) throw new Error("init_earn_failed");
  const body: ApiResponse<InitEarnResult> = await res.json();
  return body.data;
}

// ── 최신 뉴스 (랜딩 페이지) ──────────────────────────
export interface TopicPreview {
  id: string;
  topic: string;
  summary: string;
  image_url: string | null;
  run_id?: string;
  category?: string;
  brain_category?: string;
  priority?: string;
  created_at?: string;
  has_quiz?: boolean;
}

export async function fetchRecentTopics(): Promise<TopicPreview[]> {
  const res = await fetch(`${API_BASE}/api/v1/context/recent`);
  if (!res.ok) return [];
  const body: ApiResponse<TopicPreview[]> = await res.json();
  return Array.isArray(body.data) ? body.data : [];
}

export async function fetchLatestRunTopics(): Promise<TopicPreview[]> {
  const res = await fetch(`${API_BASE}/api/v1/context/latest`);
  if (!res.ok) return [];
  const body: ApiResponse<TopicPreview[]> = await res.json();
  return Array.isArray(body.data) ? body.data : [];
}

// ── 전체 뉴스 (allnews 페이지) ──────────────────────────
export type FilterType = "category" | "brain_category" | "";

export interface FilterCategory {
  key: string;
  label: string;
  display_order: number;
}

export interface FilterBrainCategory {
  key: string;
  emoji: string;
  label: string;
  display_order: number;
}

export interface FilterOptions {
  categories: FilterCategory[];
  brain_categories: FilterBrainCategory[];
}

export async function fetchAllTopics(
  filterType: FilterType,
  filterValue: string,
  limit: number,
  offset: number,
): Promise<{ data: TopicPreview[]; has_more: boolean }> {
  const params = new URLSearchParams();
  if (filterType && filterValue) {
    params.set("filter_type", filterType);
    params.set("filter_value", filterValue);
  }
  params.set("limit", String(limit));
  params.set("offset", String(offset));
  const res = await fetch(`${API_BASE}/api/v1/context/topics?${params}`);
  if (!res.ok) return { data: [], has_more: false };
  const body = await res.json();
  return { data: body.data ?? [], has_more: body.has_more ?? false };
}

export async function fetchFilterOptions(): Promise<FilterOptions> {
  const res = await fetch(`${API_BASE}/api/v1/context/categories`);
  if (!res.ok) return { categories: [], brain_categories: [] };
  const body: ApiResponse<FilterOptions> = await res.json();
  return {
    categories: Array.isArray(body.data?.categories) ? body.data.categories : [],
    brain_categories: Array.isArray(body.data?.brain_categories) ? body.data.brain_categories : [],
  };
}

export interface EarnStatusItem {
  id: string;
  status: "PENDING" | "DUPLICATE" | "EXPIRED" | "DAILY_LIMIT" | "NOT_FOUND";
  coins: number;
  has_quiz: boolean;
  quiz_completed: boolean;
}

export async function batchEarnStatus(ids: string[]): Promise<EarnStatusItem[]> {
  if (ids.length === 0) return [];
  const res = await apiFetch(`${API_BASE}/api/v1/level/batch-earn-status`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ context_item_ids: ids }),
  });
  if (!res.ok) return [];
  const body: ApiResponse<EarnStatusItem[]> = await res.json();
  return Array.isArray(body.data) ? body.data : [];
}

// ── 이메일 인증 ───────────────────────────────────────
export async function sendVerificationCode(email: string): Promise<void> {
  const res = await apiFetch(`${API_BASE}/api/v1/email-verification/send-code`, {
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
  const res = await apiFetch(`${API_BASE}/api/v1/email-verification/verify-code`, {
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
  instruction: string | null;
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
  const res = await apiFetch(`${API_BASE}/api/v1/admin/brain-categories`, {
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
  const res = await apiFetch(`${API_BASE}/api/v1/admin/brain-categories/${encodeURIComponent(key)}`, {
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
  const res = await apiFetch(`${API_BASE}/api/v1/admin/brain-categories/${encodeURIComponent(key)}`, {
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
  const res = await apiFetch(`${API_BASE}/api/v1/admin/collect`, {
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
  const res = await apiFetch(`${API_BASE}/api/v1/admin/delivery/send-test`, {
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
  total_coins: number;
  daily_limit: number;
  coin_cap: number;
  thresholds: number[];
  description: string;
}

export async function getUserLevel(): Promise<LevelInfo> {
  const res = await apiFetch(`${API_BASE}/api/v1/level`, {
    credentials: "include",
  });
  if (!res.ok) throw new Error("Failed to fetch level");
  const body: ApiResponse<LevelInfo> = await res.json();
  return body.data;
}

// ── 출금 ─────────────────────────────────────────────
export interface BankAccount {
  user_id: string;
  bank_name: string;
  account_number: string;
  account_holder: string;
}

export interface WithdrawalTransition {
  id: string;
  withdrawal_id: string;
  status: "pending" | "approved" | "rejected" | "cancelled";
  note: string;
  actor_id: string;
  actor_name?: string;
  created_at: string;
  updated_at: string;
}

export interface WithdrawalDetail {
  id: string;
  user_id: string;
  amount: number;
  bank_name: string;
  account_number: string;
  account_holder: string;
  created_at: string;
  current_status: "pending" | "approved" | "rejected" | "cancelled";
  transitions: WithdrawalTransition[];
  adblock_detected_at: string | null;
  adblock_not_detected_at: string | null;
}

export interface WithdrawalListItem {
  id: string;
  user_id: string;
  amount: number;
  bank_name: string;
  account_number: string;
  account_holder: string;
  created_at: string;
  current_status: "pending" | "approved" | "rejected" | "cancelled";
  user_nickname: string;
  user_email: string;
  adblock_detected_at: string | null;
  adblock_not_detected_at: string | null;
}

export interface WithdrawalInfo {
  min_withdrawal_amount: number;
  withdrawal_unit_amount: number;
  current_balance: number;
  has_bank_account: boolean;
}

export async function getWithdrawalInfo(): Promise<WithdrawalInfo> {
  const res = await apiFetch(`${API_BASE}/api/v1/withdrawal/info`, { credentials: "include" });
  if (!res.ok) throw new Error("Failed to fetch withdrawal info");
  const body: ApiResponse<WithdrawalInfo> = await res.json();
  return body.data;
}

export async function getBankAccount(): Promise<BankAccount | null> {
  const res = await apiFetch(`${API_BASE}/api/v1/withdrawal/bank-account`, { credentials: "include" });
  if (!res.ok) throw new Error("Failed to fetch bank account");
  const body: ApiResponse<BankAccount | null> = await res.json();
  return body.data;
}

export async function saveBankAccount(data: Omit<BankAccount, "user_id">): Promise<void> {
  const res = await apiFetch(`${API_BASE}/api/v1/withdrawal/bank-account`, {
    method: "PUT",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "계좌 저장에 실패했습니다");
  }
}

export async function requestWithdrawal(amount: number): Promise<WithdrawalDetail> {
  const res = await apiFetch(`${API_BASE}/api/v1/withdrawal/request`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ amount }),
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "출금 신청에 실패했습니다");
  }
  const body: ApiResponse<WithdrawalDetail> = await res.json();
  return body.data;
}

export async function getWithdrawalHistory(
  limit: number,
  offset: number
): Promise<{ data: WithdrawalDetail[]; has_more: boolean }> {
  const res = await apiFetch(
    `${API_BASE}/api/v1/withdrawal/history?limit=${limit}&offset=${offset}`,
    { credentials: "include" }
  );
  if (!res.ok) throw new Error("Failed to fetch withdrawal history");
  const body = await res.json();
  return { data: body.data ?? [], has_more: body.has_more ?? false };
}

export async function cancelWithdrawal(id: string): Promise<void> {
  const res = await apiFetch(`${API_BASE}/api/v1/withdrawal/${id}/cancel`, {
    method: "POST",
    credentials: "include",
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "출금 취소에 실패했습니다");
  }
}

// ── 이용 약관 ─────────────────────────────────────────
export interface Term {
  id: string;
  title: string;
  description: string;
  url: string;
  active: boolean;
  required: boolean;
  version: string;
  created_at: string;
}

export async function getActiveTerms(): Promise<Term[]> {
  const res = await fetch(`${API_BASE}/api/v1/terms/active`);
  if (!res.ok) throw new Error("약관 목록을 불러올 수 없습니다");
  const body: ApiResponse<Term[]> = await res.json();
  return body.data;
}

export async function completeSignup(
  signupKey: string,
  agreedTermIds: string[]
): Promise<User> {
  const res = await apiFetch(`${API_BASE}/api/v1/auth/complete-signup`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ signup_key: signupKey, agreed_term_ids: agreedTermIds }),
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "회원가입에 실패했습니다");
  }
  const body: ApiResponse<User> = await res.json();
  return body.data;
}

// ── 이용 약관 관리자 ──────────────────────────────────
export async function getAdminTerms(): Promise<Term[]> {
  const res = await apiFetch(`${API_BASE}/api/v1/admin/terms`, { credentials: "include" });
  if (!res.ok) throw new Error("약관 목록을 불러올 수 없습니다");
  const body: ApiResponse<Term[]> = await res.json();
  return body.data;
}

export async function createTerm(
  term: Omit<Term, "id" | "created_at">
): Promise<Term> {
  const res = await apiFetch(`${API_BASE}/api/v1/admin/terms`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(term),
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "약관 생성에 실패했습니다");
  }
  const body: ApiResponse<Term> = await res.json();
  return body.data;
}

export async function updateTermActive(termId: string, active: boolean): Promise<void> {
  const res = await apiFetch(`${API_BASE}/api/v1/admin/terms/${termId}/active`, {
    method: "PATCH",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ active }),
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "활성 상태 변경에 실패했습니다");
  }
}

export async function updateTerm(
  termId: string,
  payload: { url: string; description: string; required: boolean }
): Promise<Term> {
  const res = await apiFetch(`${API_BASE}/api/v1/admin/terms/${termId}`, {
    method: "PATCH",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "약관 수정에 실패했습니다");
  }
  const body: ApiResponse<Term> = await res.json();
  return body.data;
}

// ── 마이페이지 ──────────────────────────────────────
export interface CoinTransaction {
  id: string;
  amount: number;
  type: string;
  description: string;
  link_id?: string;
  created_at: string;
}

export async function getCoinHistory(
  limit: number,
  offset: number
): Promise<{ data: CoinTransaction[]; has_more: boolean }> {
  const res = await apiFetch(
    `${API_BASE}/api/v1/mypage/coin-history?limit=${limit}&offset=${offset}`,
    { credentials: "include" }
  );
  if (!res.ok) throw new Error("포인트 내역을 불러올 수 없습니다");
  const body = await res.json();
  return { data: body.data ?? [], has_more: body.has_more ?? false };
}

// ── 출금 관리자 ──────────────────────────────────────
export async function getAdminWithdrawals(
  status: string,
  limit: number,
  offset: number
): Promise<{ data: WithdrawalListItem[]; total: number }> {
  let url = `${API_BASE}/api/v1/admin/withdrawals?limit=${limit}&offset=${offset}`;
  if (status) url += `&status=${encodeURIComponent(status)}`;
  const res = await apiFetch(url, { credentials: "include" });
  if (!res.ok) throw new Error("Failed to fetch admin withdrawals");
  const body = await res.json();
  return { data: body.data ?? [], total: body.total ?? 0 };
}

export async function getAdminWithdrawalDetail(id: string): Promise<WithdrawalDetail> {
  const res = await apiFetch(`${API_BASE}/api/v1/admin/withdrawals/${id}`, { credentials: "include" });
  if (!res.ok) throw new Error("Failed to fetch withdrawal detail");
  const body: ApiResponse<WithdrawalDetail> = await res.json();
  return body.data;
}

export async function approveWithdrawal(id: string, note: string): Promise<void> {
  const res = await apiFetch(`${API_BASE}/api/v1/admin/withdrawals/${id}/approve`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ note }),
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "승인에 실패했습니다");
  }
}

export async function rejectWithdrawal(id: string, note: string): Promise<void> {
  const res = await apiFetch(`${API_BASE}/api/v1/admin/withdrawals/${id}/reject`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ note }),
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "거절에 실패했습니다");
  }
}

export async function updateTransitionNote(transitionId: string, note: string): Promise<void> {
  const res = await apiFetch(`${API_BASE}/api/v1/admin/withdrawals/transitions/${transitionId}/note`, {
    method: "PUT",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ note }),
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "비고 수정에 실패했습니다");
  }
}

// ── 회원 탈퇴 ───────────────────────────────────────
export async function deleteAccount(): Promise<void> {
  const res = await apiFetch(`${API_BASE}/api/v1/auth/delete-account`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "계정 삭제에 실패했습니다");
  }
}

// ── 관리자 포인트 조정 ─────────────────────────────────
export interface AdminUserSearchResult {
  user: User;
  level: LevelInfo;
}

export async function adminSearchUser(
  type: "id" | "email",
  query: string
): Promise<AdminUserSearchResult> {
  const res = await apiFetch(
    `${API_BASE}/api/v1/admin/coins/search?type=${type}&q=${encodeURIComponent(query)}`,
    { credentials: "include" }
  );
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "유저 검색에 실패했습니다");
  }
  const body: ApiResponse<AdminUserSearchResult> = await res.json();
  return body.data;
}

export interface AdjustCoinsResult {
  user_id: string;
  delta: number;
  new_coins: number;
  level: LevelInfo;
}

export async function adminAdjustCoins(
  userId: string,
  newCoins: number,
  memo: string
): Promise<AdjustCoinsResult> {
  const res = await apiFetch(`${API_BASE}/api/v1/admin/coins/adjust`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ user_id: userId, new_coins: newCoins, memo }),
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "포인트 수정에 실패했습니다");
  }
  const body: ApiResponse<AdjustCoinsResult> = await res.json();
  return body.data;
}

// ── 푸시 알림 관리 (어드민) ──────────────────────────
export interface ScheduledPush {
  id: string;
  title: string;
  body: string;
  link: string;
  data?: Record<string, unknown>;
  status: 'pending' | 'sent' | 'failed' | 'cancelled';
  scheduled_at: string | null;
  sent_at: string | null;
  error_message?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CreateScheduledPushRequest {
  title: string;
  body: string;
  link?: string;
  data?: Record<string, unknown>;
  scheduled_at?: string;
}

export interface UpdateScheduledPushRequest {
  title: string;
  body: string;
  link?: string;
  data?: Record<string, unknown>;
  scheduled_at?: string;
}

export async function listScheduledPushes(status?: string): Promise<ScheduledPush[]> {
  const url = status
    ? `${API_BASE}/api/v1/admin/push?status=${encodeURIComponent(status)}`
    : `${API_BASE}/api/v1/admin/push`;
  const res = await apiFetch(url, { credentials: "include" });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "Failed to list scheduled pushes");
  }
  const body: ApiResponse<ScheduledPush[]> = await res.json();
  return Array.isArray(body.data) ? body.data : [];
}

export async function createScheduledPush(req: CreateScheduledPushRequest): Promise<ScheduledPush> {
  const res = await apiFetch(`${API_BASE}/api/v1/admin/push`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "Failed to create scheduled push");
  }
  const body: ApiResponse<ScheduledPush> = await res.json();
  return body.data;
}

export async function updateScheduledPush(id: string, req: UpdateScheduledPushRequest): Promise<void> {
  const res = await apiFetch(`${API_BASE}/api/v1/admin/push/${id}`, {
    method: "PUT",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "Failed to update scheduled push");
  }
}

export async function deleteScheduledPush(id: string): Promise<void> {
  const res = await apiFetch(`${API_BASE}/api/v1/admin/push/${id}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "Failed to delete scheduled push");
  }
}

export async function executeScheduledPush(id: string): Promise<void> {
  const res = await apiFetch(`${API_BASE}/api/v1/admin/push/${id}/send`, {
    method: "POST",
    credentials: "include",
  });
  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "Failed to execute scheduled push");
  }
}

// ── 퀴즈 ─────────────────────────────────────────────
export async function submitQuizAnswer(
  contextItemId: string,
  answerIndex: number
): Promise<QuizSubmitResult> {
  const res = await apiFetch(`${API_BASE}/api/v1/quiz/${encodeURIComponent(contextItemId)}`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ answer_index: answerIndex }),
  });
  if (!res.ok) {
    const err: ApiError = await res.json().catch(() => ({ error: "quiz_failed" }));
    throw new Error(err.error || "quiz_failed");
  }
  const body: ApiResponse<QuizSubmitResult> = await res.json();
  return body.data;
}
