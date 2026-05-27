// Thin wrapper over @wizletter/shared — all API logic lives in the shared package.
// Web-specific: webAdapter uses credentials:"include" (cookie-based auth).

import { createApiClient, webAdapter } from "@wizletter/shared";

const API_BASE = import.meta.env.VITE_API_URL || "";
const api = createApiClient(API_BASE, webAdapter);

// ── Re-export types for backward compatibility ──────────────────────────────

export type {
  User,
  UserRole,
  ApiResponse,
  ApiError,
  DetailItem,
  HistoryItem,
  HistoryEntry,
  HistoryPage,
  ChannelPreference,
  ChannelDeliveryStatus,
  SendBriefingResult,
  QuizForUser,
  QuizSubmitResult,
  PollForUser,
  PollTally,
  TopicDetail,
  TopicEarnResult,
  InitEarnResult,
  TopicPreview,
  FilterType,
  FilterCategory,
  FilterBrainCategory,
  FilterOptions,
  EarnStatusItem,
  LevelInfo,
  BankAccount,
  WithdrawalTransition,
  WithdrawalDetail,
  WithdrawalListItem,
  WithdrawalInfo,
  Term,
  CoinTransaction,
  AdminUserSearchResult,
  AdjustCoinsResult,
  TestEmailResult,
  BrainCategory,
  ScheduledPush,
  CreateScheduledPushRequest,
  UpdateScheduledPushRequest,
  EditorPost,
  EditorPostStatus,
  EditorPickCard,
  EditorPickDetail,
  EditorPickPage,
  UploadedImage,
  RoleChangeLog,
  UpdateRoleResult,
} from "@wizletter/shared";

export { hasRoleAtLeast } from "@wizletter/shared";

// ── Re-export API functions (preserves existing import pattern) ──────────────

export const {
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
  searchTopics,
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
  // Polls
  getPoll,
  submitPollVote,
  adminUpdatePoll,
  adminDeletePoll,
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
  // Mobile Push Token (not used in web, but available)
  registerPushTokenPublic,
  registerPushToken,
  unlinkPushToken,
  // Editor (authoring)
  createEditorPost,
  listMyEditorPosts,
  getEditorPost,
  updateEditorPost,
  deleteEditorPost,
  uploadEditorImage,
  updatePenName,
  // Editor Picks (public)
  listEditorPicks,
  getEditorPick,
  // Admin Users (role management)
  adminSearchUserByRole,
  adminUpdateUserRole,
  adminGetRoleHistory,
} = api;

// ── Web-specific exports ────────────────────────────────────────────────────

export { default as defaultImage } from "@/assets/wizletter_default.png";
