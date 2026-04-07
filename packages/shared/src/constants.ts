// Shared API path constants

export const API_PREFIX = "/api/v1";

export const API_PATHS = {
  // Auth
  AUTH_ME: `${API_PREFIX}/auth/me`,
  AUTH_LOGOUT: `${API_PREFIX}/auth/logout`,
  AUTH_REFRESH: `${API_PREFIX}/auth/refresh`,
  AUTH_COMPLETE_SIGNUP: `${API_PREFIX}/auth/complete-signup`,
  AUTH_DELETE_ACCOUNT: `${API_PREFIX}/auth/delete-account`,

  // Subscriptions
  SUBSCRIPTIONS: `${API_PREFIX}/subscriptions`,

  // Context
  CONTEXT_RECENT: `${API_PREFIX}/context/recent`,
  CONTEXT_LATEST: `${API_PREFIX}/context/latest`,
  CONTEXT_TOPICS: `${API_PREFIX}/context/topics`,
  CONTEXT_TOPIC: (id: string) => `${API_PREFIX}/context/topic/${id}`,
  CONTEXT_CATEGORIES: `${API_PREFIX}/context/categories`,
  CONTEXT_HISTORY: `${API_PREFIX}/context/history`,

  // Level / Earn
  LEVEL: `${API_PREFIX}/level`,
  LEVEL_INIT_EARN: `${API_PREFIX}/level/init-earn`,
  LEVEL_EARN: `${API_PREFIX}/level/earn`,
  LEVEL_BATCH_EARN_STATUS: `${API_PREFIX}/level/batch-earn-status`,

  // Delivery
  DELIVERY_SEND: `${API_PREFIX}/delivery/send`,
  USER_DELIVERY_CHANNELS: `${API_PREFIX}/user/delivery-channels`,
  USER_DELIVERY_STATUS: `${API_PREFIX}/user/delivery-status`,

  // Email verification
  EMAIL_VERIFICATION_SEND: `${API_PREFIX}/email-verification/send-code`,
  EMAIL_VERIFICATION_VERIFY: `${API_PREFIX}/email-verification/verify-code`,

  // Withdrawal
  WITHDRAWAL_INFO: `${API_PREFIX}/withdrawal/info`,
  WITHDRAWAL_BANK_ACCOUNT: `${API_PREFIX}/withdrawal/bank-account`,
  WITHDRAWAL_REQUEST: `${API_PREFIX}/withdrawal/request`,
  WITHDRAWAL_HISTORY: `${API_PREFIX}/withdrawal/history`,
  WITHDRAWAL_CANCEL: (id: string) => `${API_PREFIX}/withdrawal/${id}/cancel`,

  // Terms
  TERMS_ACTIVE: `${API_PREFIX}/terms/active`,

  // Brain categories
  BRAIN_CATEGORIES: `${API_PREFIX}/brain-categories`,

  // Mypage
  MYPAGE_COIN_HISTORY: `${API_PREFIX}/mypage/coin-history`,

  // Quiz
  QUIZ_SUBMIT: (contextItemId: string) => `${API_PREFIX}/quiz/${encodeURIComponent(contextItemId)}`,

  // Admin
  ADMIN_COLLECT: `${API_PREFIX}/admin/collect`,
  ADMIN_DELIVERY_SEND_TEST: `${API_PREFIX}/admin/delivery/send-test`,
  ADMIN_BRAIN_CATEGORIES: `${API_PREFIX}/admin/brain-categories`,
  ADMIN_BRAIN_CATEGORY: (key: string) => `${API_PREFIX}/admin/brain-categories/${encodeURIComponent(key)}`,
  ADMIN_COINS_SEARCH: `${API_PREFIX}/admin/coins/search`,
  ADMIN_COINS_ADJUST: `${API_PREFIX}/admin/coins/adjust`,
  ADMIN_WITHDRAWALS: `${API_PREFIX}/admin/withdrawals`,
  ADMIN_WITHDRAWAL: (id: string) => `${API_PREFIX}/admin/withdrawals/${id}`,
  ADMIN_WITHDRAWAL_APPROVE: (id: string) => `${API_PREFIX}/admin/withdrawals/${id}/approve`,
  ADMIN_WITHDRAWAL_REJECT: (id: string) => `${API_PREFIX}/admin/withdrawals/${id}/reject`,
  ADMIN_WITHDRAWAL_TRANSITION_NOTE: (id: string) => `${API_PREFIX}/admin/withdrawals/transitions/${id}/note`,
  ADMIN_TERMS: `${API_PREFIX}/admin/terms`,
  ADMIN_TERM_ACTIVE: (id: string) => `${API_PREFIX}/admin/terms/${id}/active`,
  ADMIN_TERM: (id: string) => `${API_PREFIX}/admin/terms/${id}`,

  // Mobile Push Token
  MOBILE_PUSH_TOKEN: `${API_PREFIX}/mobile/push-token`,

  // Admin Push
  ADMIN_PUSH: `${API_PREFIX}/admin/push`,
  ADMIN_PUSH_ITEM: (id: string) => `${API_PREFIX}/admin/push/${id}`,
  ADMIN_PUSH_SEND: (id: string) => `${API_PREFIX}/admin/push/${id}/send`,
} as const;
