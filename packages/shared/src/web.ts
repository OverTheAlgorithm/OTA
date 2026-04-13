// Web convenience entry point — re-exports individual API functions
// so web can migrate without changing every import site at once.
//
// Before (web/src/lib/api.ts):
//   import { fetchMe, logout, getSubscriptions } from "@/lib/api";
//
// After (drop-in replacement):
//   import { fetchMe, logout, getSubscriptions } from "@wizletter/shared/web";
//
// This file creates a singleton ApiClient with the webAdapter and
// re-exports every function as a named export for backward compatibility.

import { createApiClient } from "./api";
import { webAdapter } from "./web-adapter";

// API_BASE is injected at build time by the web bundler (Vite).
// Falls back to empty string for same-origin requests.
const API_BASE = typeof import.meta !== "undefined"
  ? (import.meta as Record<string, Record<string, string>>).env?.VITE_API_URL ?? ""
  : "";

const api = createApiClient(API_BASE, webAdapter);

// Re-export all types so consumers get types + functions from one import
export * from "./types";
export * from "./push-admin";
export * from "./constants";
export { type ApiAdapter, type ApiClient } from "./api";

// Re-export every API function as a named export (matches web's current import pattern)
export const {
  fetchMe,
  logout,
  completeSignup,
  deleteAccount,
  getSubscriptions,
  addSubscription,
  deleteSubscription,
  fetchRecentTopics,
  fetchLatestRunTopics,
  fetchTopicDetail,
  fetchAllTopics,
  fetchFilterOptions,
  getContextHistory,
  getUserLevel,
  initEarn,
  earnCoin,
  batchEarnStatus,
  sendBriefingNow,
  getDeliveryChannels,
  updateDeliveryChannels,
  getDeliveryStatus,
  sendVerificationCode,
  verifyEmailCode,
  getWithdrawalInfo,
  getBankAccount,
  saveBankAccount,
  requestWithdrawal,
  getWithdrawalHistory,
  cancelWithdrawal,
  getActiveTerms,
  getBrainCategories,
  getCoinHistory,
  submitQuizAnswer,
  getPoll,
  submitPollVote,
  adminUpdatePoll,
  adminDeletePoll,
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
  listScheduledPushes,
  createScheduledPush,
  updateScheduledPush,
  deleteScheduledPush,
  executeScheduledPush,
  registerPushTokenPublic,
  registerPushToken,
  unlinkPushToken,
} = api;

// Also export the client instance for cases where object access is preferred
export { api };

// Web-specific: default image is a bundled asset, not a server URL.
// Each platform handles this differently — web re-exports from its own assets.
// This is intentionally NOT included here.
