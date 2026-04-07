// Extracted from: web/src/lib/api.ts — adapted with ApiAdapter pattern
//
// The ApiAdapter interface abstracts platform-specific differences:
// - Web: uses credentials:"include" (cookie-based JWT)
// - Mobile: uses Authorization: Bearer <token> header (SecureStore-backed)

import {
  User,
  ApiResponse,
  ApiError,
  TopicDetail,
  TopicPreview,
  TopicEarnResult,
  InitEarnResult,
  HistoryPage,
  ChannelPreference,
  ChannelDeliveryStatus,
  SendBriefingResult,
  LevelInfo,
  BankAccount,
  WithdrawalDetail,
  WithdrawalInfo,
  WithdrawalListItem,
  Term,
  CoinTransaction,
  EarnStatusItem,
  QuizSubmitResult,
  AdminUserSearchResult,
  AdjustCoinsResult,
  TestEmailResult,
  BrainCategory,
  FilterOptions,
  FilterType,
} from "./types";
import {
  ScheduledPush,
  CreateScheduledPushRequest,
  UpdateScheduledPushRequest,
} from "./push-admin";
import { API_PATHS } from "./constants";

export interface ApiAdapter {
  fetch: (input: string, init?: RequestInit) => Promise<Response>;
  getToken: () => Promise<string | null>;
  setToken: (token: string) => Promise<void>;
  removeToken: () => Promise<void>;
}

export function createApiClient(baseUrl: string, adapter: ApiAdapter) {
  // -- Token refresh state -----------------------------------------------
  let isRefreshing = false;
  let refreshPromise: Promise<boolean> | null = null;

  async function tryRefreshToken(): Promise<boolean> {
    if (isRefreshing && refreshPromise) {
      return refreshPromise;
    }
    isRefreshing = true;
    refreshPromise = adapter
      .fetch(`${baseUrl}${API_PATHS.AUTH_REFRESH}`, { method: "POST" })
      .then((res) => {
        if (res.ok) {
          // Mobile: if response contains a new token, store it
          return res.json().then((body) => {
            if (body?.token) {
              return adapter.setToken(body.token).then(() => true);
            }
            return true;
          }).catch(() => true);
        }
        return false;
      })
      .catch(() => false)
      .finally(() => {
        isRefreshing = false;
        refreshPromise = null;
      });
    return refreshPromise;
  }

  // apiFetch wraps fetch with automatic 401 -> refresh -> retry logic.
  async function apiFetch(path: string, init?: RequestInit): Promise<Response> {
    const token = await adapter.getToken();
    const headers: Record<string, string> = {
      ...(init?.headers as Record<string, string>),
    };
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    const res = await adapter.fetch(`${baseUrl}${path}`, { ...init, headers });

    if (res.status === 429) {
      throw new Error("요청이 너무 잦습니다. 잠시 후 다시 시도해주세요.");
    }

    if (res.status !== 401) return res;

    const refreshed = await tryRefreshToken();
    if (!refreshed) return res;

    // After refresh, get potentially new token and retry
    const newToken = await adapter.getToken();
    const retryHeaders: Record<string, string> = {
      ...(init?.headers as Record<string, string>),
    };
    if (newToken) {
      retryHeaders["Authorization"] = `Bearer ${newToken}`;
    }
    return adapter.fetch(`${baseUrl}${path}`, { ...init, headers: retryHeaders });
  }

  // Public (no auth) fetch helper
  async function publicFetch(path: string, init?: RequestInit): Promise<Response> {
    return adapter.fetch(`${baseUrl}${path}`, init);
  }

  // ── Auth ────────────────────────────────────────────────────────────────

  async function fetchMe(): Promise<User> {
    const res = await apiFetch(API_PATHS.AUTH_ME);
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "Failed to fetch user");
    }
    const body: ApiResponse<User> = await res.json();
    return body.data;
  }

  async function logout(): Promise<void> {
    await apiFetch(API_PATHS.AUTH_LOGOUT, { method: "POST" });
  }

  async function completeSignup(signupKey: string, agreedTermIds: string[]): Promise<User> {
    const res = await apiFetch(API_PATHS.AUTH_COMPLETE_SIGNUP, {
      method: "POST",
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

  async function deleteAccount(): Promise<void> {
    const res = await apiFetch(API_PATHS.AUTH_DELETE_ACCOUNT, { method: "DELETE" });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "계정 삭제에 실패했습니다");
    }
  }

  // ── Subscriptions ──────────────────────────────────────────────────────

  async function getSubscriptions(): Promise<string[]> {
    const res = await apiFetch(API_PATHS.SUBSCRIPTIONS);
    if (!res.ok) throw new Error("Failed to fetch subscriptions");
    const body: ApiResponse<string[]> = await res.json();
    return Array.isArray(body.data) ? body.data : [];
  }

  async function addSubscription(category: string): Promise<void> {
    const res = await apiFetch(API_PATHS.SUBSCRIPTIONS, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ category }),
    });
    if (!res.ok) throw new Error("Failed to add subscription");
  }

  async function deleteSubscription(category: string): Promise<void> {
    const res = await apiFetch(
      `${API_PATHS.SUBSCRIPTIONS}?category=${encodeURIComponent(category)}`,
      { method: "DELETE" }
    );
    if (!res.ok) throw new Error("Failed to delete subscription");
  }

  // ── Context / Topics ───────────────────────────────────────────────────

  async function fetchRecentTopics(): Promise<TopicPreview[]> {
    const res = await publicFetch(API_PATHS.CONTEXT_RECENT);
    if (!res.ok) return [];
    const body: ApiResponse<TopicPreview[]> = await res.json();
    return Array.isArray(body.data) ? body.data : [];
  }

  async function fetchLatestRunTopics(): Promise<TopicPreview[]> {
    const res = await publicFetch(API_PATHS.CONTEXT_LATEST);
    if (!res.ok) return [];
    const body: ApiResponse<TopicPreview[]> = await res.json();
    return Array.isArray(body.data) ? body.data : [];
  }

  async function fetchTopicDetail(id: string): Promise<TopicDetail> {
    const res = await publicFetch(API_PATHS.CONTEXT_TOPIC(id));
    if (res.status === 404) throw new Error("not_found");
    if (!res.ok) throw new Error("server_error");
    const body: ApiResponse<TopicDetail> = await res.json();
    return body.data;
  }

  async function fetchAllTopics(
    filterType: FilterType,
    filterValue: string,
    limit: number,
    offset: number
  ): Promise<{ data: TopicPreview[]; has_more: boolean }> {
    const params = new URLSearchParams();
    if (filterType && filterValue) {
      params.set("filter_type", filterType);
      params.set("filter_value", filterValue);
    }
    params.set("limit", String(limit));
    params.set("offset", String(offset));
    const res = await publicFetch(`${API_PATHS.CONTEXT_TOPICS}?${params}`);
    if (!res.ok) return { data: [], has_more: false };
    const body = await res.json();
    return { data: body.data ?? [], has_more: body.has_more ?? false };
  }

  async function fetchFilterOptions(): Promise<FilterOptions> {
    const res = await publicFetch(API_PATHS.CONTEXT_CATEGORIES);
    if (!res.ok) return { categories: [], brain_categories: [] };
    const body: ApiResponse<FilterOptions> = await res.json();
    return {
      categories: Array.isArray(body.data?.categories) ? body.data.categories : [],
      brain_categories: Array.isArray(body.data?.brain_categories)
        ? body.data.brain_categories
        : [],
    };
  }

  async function getContextHistory(limit: number, offset: number): Promise<HistoryPage> {
    const res = await apiFetch(
      `${API_PATHS.CONTEXT_HISTORY}?limit=${limit}&offset=${offset}`
    );
    if (!res.ok) throw new Error("Failed to fetch context history");
    const body = await res.json();
    return { data: body.data ?? [], has_more: body.has_more ?? false };
  }

  // ── Level / Earn ───────────────────────────────────────────────────────

  async function getUserLevel(): Promise<LevelInfo> {
    const res = await apiFetch(API_PATHS.LEVEL);
    if (!res.ok) throw new Error("Failed to fetch level");
    const body: ApiResponse<LevelInfo> = await res.json();
    return body.data;
  }

  async function initEarn(contextItemId: string): Promise<InitEarnResult> {
    const res = await apiFetch(API_PATHS.LEVEL_INIT_EARN, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ context_item_id: contextItemId }),
    });
    if (!res.ok) throw new Error("init_earn_failed");
    const body: ApiResponse<InitEarnResult> = await res.json();
    return body.data;
  }

  async function earnCoin(
    contextItemId: string,
    turnstileToken: string
  ): Promise<TopicEarnResult> {
    const res = await apiFetch(API_PATHS.LEVEL_EARN, {
      method: "POST",
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

  async function batchEarnStatus(ids: string[]): Promise<EarnStatusItem[]> {
    if (ids.length === 0) return [];
    const res = await apiFetch(API_PATHS.LEVEL_BATCH_EARN_STATUS, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ context_item_ids: ids }),
    });
    if (!res.ok) return [];
    const body: ApiResponse<EarnStatusItem[]> = await res.json();
    return Array.isArray(body.data) ? body.data : [];
  }

  // ── Delivery ───────────────────────────────────────────────────────────

  async function sendBriefingNow(): Promise<SendBriefingResult> {
    const res = await apiFetch(API_PATHS.DELIVERY_SEND, { method: "POST" });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "브리핑 전송에 실패했습니다");
    }
    const body: ApiResponse<SendBriefingResult> = await res.json();
    return body.data;
  }

  async function getDeliveryChannels(): Promise<ChannelPreference[]> {
    const res = await apiFetch(API_PATHS.USER_DELIVERY_CHANNELS);
    if (!res.ok) throw new Error("Failed to fetch delivery channels");
    const body: { channels: ChannelPreference[] } = await res.json();
    return body.channels;
  }

  async function updateDeliveryChannels(channels: ChannelPreference[]): Promise<void> {
    const res = await apiFetch(API_PATHS.USER_DELIVERY_CHANNELS, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ channels }),
    });
    if (!res.ok) throw new Error("Failed to update delivery channels");
  }

  async function getDeliveryStatus(): Promise<ChannelDeliveryStatus[]> {
    const res = await apiFetch(API_PATHS.USER_DELIVERY_STATUS);
    if (!res.ok) throw new Error("Failed to fetch delivery status");
    const body: ApiResponse<ChannelDeliveryStatus[]> = await res.json();
    return body.data;
  }

  // ── Email Verification ─────────────────────────────────────────────────

  async function sendVerificationCode(email: string): Promise<void> {
    const res = await apiFetch(API_PATHS.EMAIL_VERIFICATION_SEND, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
    });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "인증 코드 전송에 실패했습니다");
    }
  }

  async function verifyEmailCode(code: string): Promise<void> {
    const res = await apiFetch(API_PATHS.EMAIL_VERIFICATION_VERIFY, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code }),
    });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "인증 코드 확인에 실패했습니다");
    }
  }

  // ── Withdrawal ─────────────────────────────────────────────────────────

  async function getWithdrawalInfo(): Promise<WithdrawalInfo> {
    const res = await apiFetch(API_PATHS.WITHDRAWAL_INFO);
    if (!res.ok) throw new Error("Failed to fetch withdrawal info");
    const body: ApiResponse<WithdrawalInfo> = await res.json();
    return body.data;
  }

  async function getBankAccount(): Promise<BankAccount | null> {
    const res = await apiFetch(API_PATHS.WITHDRAWAL_BANK_ACCOUNT);
    if (!res.ok) throw new Error("Failed to fetch bank account");
    const body: ApiResponse<BankAccount | null> = await res.json();
    return body.data;
  }

  async function saveBankAccount(data: Omit<BankAccount, "user_id">): Promise<void> {
    const res = await apiFetch(API_PATHS.WITHDRAWAL_BANK_ACCOUNT, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "계좌 저장에 실패했습니다");
    }
  }

  async function requestWithdrawal(amount: number): Promise<WithdrawalDetail> {
    const res = await apiFetch(API_PATHS.WITHDRAWAL_REQUEST, {
      method: "POST",
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

  async function getWithdrawalHistory(
    limit: number,
    offset: number
  ): Promise<{ data: WithdrawalDetail[]; has_more: boolean }> {
    const res = await apiFetch(
      `${API_PATHS.WITHDRAWAL_HISTORY}?limit=${limit}&offset=${offset}`
    );
    if (!res.ok) throw new Error("Failed to fetch withdrawal history");
    const body = await res.json();
    return { data: body.data ?? [], has_more: body.has_more ?? false };
  }

  async function cancelWithdrawal(id: string): Promise<void> {
    const res = await apiFetch(API_PATHS.WITHDRAWAL_CANCEL(id), { method: "POST" });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "출금 취소에 실패했습니다");
    }
  }

  // ── Terms ──────────────────────────────────────────────────────────────

  async function getActiveTerms(): Promise<Term[]> {
    const res = await publicFetch(API_PATHS.TERMS_ACTIVE);
    if (!res.ok) throw new Error("약관 목록을 불러올 수 없습니다");
    const body: ApiResponse<Term[]> = await res.json();
    return body.data;
  }

  // ── Brain Categories ───────────────────────────────────────────────────

  async function getBrainCategories(): Promise<BrainCategory[]> {
    const res = await publicFetch(API_PATHS.BRAIN_CATEGORIES);
    if (!res.ok) throw new Error("Failed to fetch brain categories");
    const body: ApiResponse<BrainCategory[]> = await res.json();
    return body.data;
  }

  // ── Mypage ─────────────────────────────────────────────────────────────

  async function getCoinHistory(
    limit: number,
    offset: number
  ): Promise<{ data: CoinTransaction[]; has_more: boolean }> {
    const res = await apiFetch(
      `${API_PATHS.MYPAGE_COIN_HISTORY}?limit=${limit}&offset=${offset}`
    );
    if (!res.ok) throw new Error("포인트 내역을 불러올 수 없습니다");
    const body = await res.json();
    return { data: body.data ?? [], has_more: body.has_more ?? false };
  }

  // ── Quiz ───────────────────────────────────────────────────────────────

  async function submitQuizAnswer(
    contextItemId: string,
    answerIndex: number
  ): Promise<QuizSubmitResult> {
    const res = await apiFetch(API_PATHS.QUIZ_SUBMIT(contextItemId), {
      method: "POST",
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

  // ── Admin ──────────────────────────────────────────────────────────────

  async function triggerCollection(): Promise<void> {
    const res = await apiFetch(API_PATHS.ADMIN_COLLECT, { method: "POST" });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "수집 실행에 실패했습니다");
    }
  }

  async function sendTestEmail(): Promise<TestEmailResult> {
    const res = await apiFetch(API_PATHS.ADMIN_DELIVERY_SEND_TEST, { method: "POST" });
    let body: Record<string, unknown>;
    try {
      body = await res.json();
    } catch {
      throw new Error("서버 응답을 파싱할 수 없습니다");
    }
    if (!res.ok) throw new Error((body.error as string) || "테스트 이메일 전송에 실패했습니다");
    return body as unknown as TestEmailResult;
  }

  async function adminSearchUser(
    type: "id" | "email",
    query: string
  ): Promise<AdminUserSearchResult> {
    const res = await apiFetch(
      `${API_PATHS.ADMIN_COINS_SEARCH}?type=${type}&q=${encodeURIComponent(query)}`
    );
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "유저 검색에 실패했습니다");
    }
    const body: ApiResponse<AdminUserSearchResult> = await res.json();
    return body.data;
  }

  async function adminAdjustCoins(
    userId: string,
    newCoins: number,
    memo: string
  ): Promise<AdjustCoinsResult> {
    const res = await apiFetch(API_PATHS.ADMIN_COINS_ADJUST, {
      method: "POST",
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

  async function getAdminWithdrawals(
    status: string,
    limit: number,
    offset: number
  ): Promise<{ data: WithdrawalListItem[]; total: number }> {
    let path = `${API_PATHS.ADMIN_WITHDRAWALS}?limit=${limit}&offset=${offset}`;
    if (status) path += `&status=${encodeURIComponent(status)}`;
    const res = await apiFetch(path);
    if (!res.ok) throw new Error("Failed to fetch admin withdrawals");
    const body = await res.json();
    return { data: body.data ?? [], total: body.total ?? 0 };
  }

  async function getAdminWithdrawalDetail(id: string): Promise<WithdrawalDetail> {
    const res = await apiFetch(API_PATHS.ADMIN_WITHDRAWAL(id));
    if (!res.ok) throw new Error("Failed to fetch withdrawal detail");
    const body: ApiResponse<WithdrawalDetail> = await res.json();
    return body.data;
  }

  async function approveWithdrawal(id: string, note: string): Promise<void> {
    const res = await apiFetch(API_PATHS.ADMIN_WITHDRAWAL_APPROVE(id), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ note }),
    });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "승인에 실패했습니다");
    }
  }

  async function rejectWithdrawal(id: string, note: string): Promise<void> {
    const res = await apiFetch(API_PATHS.ADMIN_WITHDRAWAL_REJECT(id), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ note }),
    });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "거절에 실패했습니다");
    }
  }

  async function updateTransitionNote(transitionId: string, note: string): Promise<void> {
    const res = await apiFetch(API_PATHS.ADMIN_WITHDRAWAL_TRANSITION_NOTE(transitionId), {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ note }),
    });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "비고 수정에 실패했습니다");
    }
  }

  async function getAdminTerms(): Promise<Term[]> {
    const res = await apiFetch(API_PATHS.ADMIN_TERMS);
    if (!res.ok) throw new Error("약관 목록을 불러올 수 없습니다");
    const body: ApiResponse<Term[]> = await res.json();
    return body.data;
  }

  async function createTerm(term: Omit<Term, "id" | "created_at">): Promise<Term> {
    const res = await apiFetch(API_PATHS.ADMIN_TERMS, {
      method: "POST",
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

  async function updateTermActive(termId: string, active: boolean): Promise<void> {
    const res = await apiFetch(API_PATHS.ADMIN_TERM_ACTIVE(termId), {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ active }),
    });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "활성 상태 변경에 실패했습니다");
    }
  }

  async function updateTerm(
    termId: string,
    payload: { url: string; description: string; required: boolean }
  ): Promise<Term> {
    const res = await apiFetch(API_PATHS.ADMIN_TERM(termId), {
      method: "PATCH",
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

  async function createBrainCategory(
    bc: Omit<BrainCategory, "created_at" | "updated_at">
  ): Promise<void> {
    const res = await apiFetch(API_PATHS.ADMIN_BRAIN_CATEGORIES, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(bc),
    });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "Failed to create brain category");
    }
  }

  async function updateBrainCategory(
    key: string,
    bc: Partial<BrainCategory>
  ): Promise<void> {
    const res = await apiFetch(API_PATHS.ADMIN_BRAIN_CATEGORY(key), {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(bc),
    });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "Failed to update brain category");
    }
  }

  async function deleteBrainCategory(key: string): Promise<void> {
    const res = await apiFetch(API_PATHS.ADMIN_BRAIN_CATEGORY(key), { method: "DELETE" });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "Failed to delete brain category");
    }
  }

  // ── Admin Push ─────────────────────────────────────────────────────────

  async function listScheduledPushes(status?: string): Promise<ScheduledPush[]> {
    const path = status
      ? `${API_PATHS.ADMIN_PUSH}?status=${encodeURIComponent(status)}`
      : API_PATHS.ADMIN_PUSH;
    const res = await apiFetch(path);
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "Failed to list scheduled pushes");
    }
    const body: ApiResponse<ScheduledPush[]> = await res.json();
    return Array.isArray(body.data) ? body.data : [];
  }

  async function createScheduledPush(req: CreateScheduledPushRequest): Promise<ScheduledPush> {
    const res = await apiFetch(API_PATHS.ADMIN_PUSH, {
      method: "POST",
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

  async function updateScheduledPush(id: string, req: UpdateScheduledPushRequest): Promise<void> {
    const res = await apiFetch(API_PATHS.ADMIN_PUSH_ITEM(id), {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(req),
    });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "Failed to update scheduled push");
    }
  }

  async function deleteScheduledPush(id: string): Promise<void> {
    const res = await apiFetch(API_PATHS.ADMIN_PUSH_ITEM(id), { method: "DELETE" });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "Failed to delete scheduled push");
    }
  }

  async function executeScheduledPush(id: string): Promise<void> {
    const res = await apiFetch(API_PATHS.ADMIN_PUSH_SEND(id), { method: "POST" });
    if (!res.ok) {
      const err: ApiError = await res.json();
      throw new Error(err.error || "Failed to execute scheduled push");
    }
  }

  // ── Mobile Push Token ───────────────────────────────────────────────────

  // Register push token without authentication (anonymous).
  // Server stores the token with user_id = NULL.
  async function registerPushTokenPublic(token: string, platform: string): Promise<void> {
    const res = await publicFetch(API_PATHS.MOBILE_PUSH_TOKEN, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token, platform }),
    });
    if (!res.ok) {
      const err: ApiError = await res.json().catch(() => ({ error: "push_register_failed" }));
      throw new Error(err.error || "푸시 토큰 등록에 실패했습니다");
    }
  }

  // Register push token with authentication (links user_id).
  async function registerPushToken(token: string, platform: string): Promise<void> {
    const res = await apiFetch(API_PATHS.MOBILE_PUSH_TOKEN, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token, platform }),
    });
    if (!res.ok) {
      const err: ApiError = await res.json().catch(() => ({ error: "push_register_failed" }));
      throw new Error(err.error || "푸시 토큰 등록에 실패했습니다");
    }
  }

  // Unlink user from push token (token preserved for anonymous push).
  async function unlinkPushToken(token: string): Promise<void> {
    const res = await apiFetch(API_PATHS.MOBILE_PUSH_TOKEN, {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token }),
    });
    if (!res.ok) {
      const err: ApiError = await res.json().catch(() => ({ error: "push_unlink_failed" }));
      throw new Error(err.error || "푸시 토큰 해제에 실패했습니다");
    }
  }

  return {
    // Auth
    fetchMe,
    logout,
    completeSignup,
    deleteAccount,
    // Subscriptions
    getSubscriptions,
    addSubscription,
    deleteSubscription,
    // Context / Topics
    fetchRecentTopics,
    fetchLatestRunTopics,
    fetchTopicDetail,
    fetchAllTopics,
    fetchFilterOptions,
    getContextHistory,
    // Level / Earn
    getUserLevel,
    initEarn,
    earnCoin,
    batchEarnStatus,
    // Delivery
    sendBriefingNow,
    getDeliveryChannels,
    updateDeliveryChannels,
    getDeliveryStatus,
    // Email verification
    sendVerificationCode,
    verifyEmailCode,
    // Withdrawal
    getWithdrawalInfo,
    getBankAccount,
    saveBankAccount,
    requestWithdrawal,
    getWithdrawalHistory,
    cancelWithdrawal,
    // Terms
    getActiveTerms,
    // Brain categories
    getBrainCategories,
    // Mypage
    getCoinHistory,
    // Quiz
    submitQuizAnswer,
    // Admin
    triggerCollection,
    sendTestEmail,
    adminSearchUser,
    adminAdjustCoins,
    getAdminWithdrawals,
    getAdminWithdrawalDetail,
    approveWithdrawal,
    rejectWithdrawal,
    updateTransitionNote,
    getAdminTerms,
    createTerm,
    updateTermActive,
    updateTerm,
    createBrainCategory,
    updateBrainCategory,
    deleteBrainCategory,
    // Admin Push
    listScheduledPushes,
    createScheduledPush,
    updateScheduledPush,
    deleteScheduledPush,
    executeScheduledPush,
    // Mobile Push Token
    registerPushTokenPublic,
    registerPushToken,
    unlinkPushToken,
  };
}

export type ApiClient = ReturnType<typeof createApiClient>;
