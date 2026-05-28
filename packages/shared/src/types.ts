// Extracted from: web/src/lib/api.ts

// Role identifiers. Kept in sync with server/domain/user/model.go.
export type UserRole = "user" | "editor" | "admin";

// roleRank determines the precedence used by hasRoleAtLeast. The server has
// the canonical implementation; this mirror is for client-side gating only.
const ROLE_RANK: Record<string, number> = {
  user: 0,
  editor: 1,
  admin: 2,
};

export function hasRoleAtLeast(role: string | undefined | null, min: UserRole): boolean {
  if (!role) return false;
  return (ROLE_RANK[role] ?? 0) >= ROLE_RANK[min];
}

/** Nickname state machine — see migration 000042 for the source of truth.
 * - default      : nickname from Kakao, user has not seen the warning.
 * - acknowledged : user saw the warning and chose to keep the Kakao name.
 * - custom       : user explicitly set their own nickname; Kakao login
 *                  will no longer refresh it.
 */
export type NicknameState = "default" | "acknowledged" | "custom";

export interface User {
  id: string;
  kakao_id: number;
  email?: string;
  email_verified: boolean;
  nickname?: string;
  profile_image?: string;
  role: UserRole | string;
  pen_name?: string;
  nickname_state: NicknameState;
  created_at: string;
  updated_at: string;
}

// Mirrors the server-side bounds in user.NormaliseNickname.
export const NICKNAME_MIN_LEN = 2;
export const NICKNAME_MAX_LEN = 32;

// Mirrors the server-side bounds in user.NormalisePenName.
export const PEN_NAME_MIN_LEN = 2;
export const PEN_NAME_MAX_LEN = 32;

// ── Editor posts ───────────────────────────────────────────────────────────

export type EditorPostStatus = "draft" | "published";

export interface EditorPost {
  id: string;
  author_id: string;
  title: string;
  content_html: string;
  content_text: string;
  first_image_url?: string | null;
  status: EditorPostStatus;
  published_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface EditorPickCard {
  id: string;
  title: string;
  excerpt: string;
  first_image_url?: string | null;
  author_id: string;
  author_name?: string;
  published_at: string;
}

export interface EditorPickDetail {
  id: string;
  title: string;
  content_html: string;
  first_image_url?: string | null;
  author_id: string;
  author_name?: string;
  published_at: string;
  updated_at: string;
}

export interface EditorPickPage {
  items: EditorPickCard[];
  total: number;
  limit: number;
  offset: number;
}

export interface UploadedImage {
  url: string;
}

// ── Admin users ────────────────────────────────────────────────────────────

export interface RoleChangeLog {
  id: string;
  user_id: string;
  before_role: string;
  after_role: string;
  actor_id?: string | null;
  memo: string;
  created_at: string;
}

export interface UpdateRoleResult {
  user_id: string;
  before_role: string;
  after_role: string;
  unchanged?: boolean;
  log?: RoleChangeLog;
}

export interface ApiResponse<T> {
  data: T;
}

export interface ApiError {
  error: string;
}

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

export interface ChannelPreference {
  channel: string;
  enabled: boolean;
}

export interface ChannelDeliveryStatus {
  channel: string;
  status: "sent" | "failed" | "skipped";
  error_message?: string;
  retry_count: number;
  last_attempt: string;
}

export interface SendBriefingResult {
  success_count: number;
  failure_count: number;
  skipped_count: number;
  errors: Record<string, string>;
}

export interface PastQuizAttempt {
  selected_index: number;
  is_correct: boolean;
  coins_earned: number;
  attempted_at: string;
}

export interface QuizForUser {
  id: string;
  question: string;
  options: string[];
  /**
   * Non-null when the user has already submitted an answer for this quiz.
   * Used to hydrate a static "already completed" card without re-submission.
   * The correct answer index is intentionally omitted — only the user's
   * chosen option and whether it was correct.
   */
  past_attempt?: PastQuizAttempt | null;
}

export interface QuizSubmitResult {
  correct: boolean;
  coins_earned: number;
  total_coins: number;
}

export interface PollTally {
  option_index: number;
  count: number;
}

export interface PollForUser {
  id: string;
  context_item_id: string;
  question: string;
  options: string[];
  /** Length == options.length; zeros are padded server-side. Index by option_index. */
  tallies: PollTally[];
  total_votes: number;
  /** null for non-logged-in viewers and users who have not voted yet. */
  user_vote_index: number | null;
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
  poll: PollForUser | null;
}

export interface TopicEarnResult {
  attempted: boolean;
  earned: boolean;
  reason: string; // "EARNED" | "DUPLICATE" | "EXPIRED" | "DAILY_LIMIT"
  coins_earned: number;
  leveled_up: boolean;
  new_level: number;
}

export interface InitEarnResult {
  status: "PENDING" | "EXPIRED" | "DUPLICATE" | "DAILY_LIMIT";
  required_seconds?: number;
}

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

export interface EarnStatusItem {
  id: string;
  status: "PENDING" | "DUPLICATE" | "EXPIRED" | "DAILY_LIMIT" | "NOT_FOUND";
  coins: number;
  has_quiz: boolean;
  quiz_completed: boolean;
}

export interface LevelInfo {
  level: number;
  total_coins: number;
  daily_limit: number;
  coin_cap: number;
  thresholds: number[];
  description: string;
}

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

export interface CoinTransaction {
  id: string;
  amount: number;
  type: string;
  description: string;
  link_id?: string;
  created_at: string;
}

export interface AdminUserSearchResult {
  user: User;
  level: LevelInfo;
}

export interface AdjustCoinsResult {
  user_id: string;
  delta: number;
  new_coins: number;
  level: LevelInfo;
}

export interface TestEmailResult {
  success_count: number;
  skipped_count: number;
  failure_count: number;
  errors: Record<string, string>;
}

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

// ── Comments ────────────────────────────────────────────────────────────────

export type CommentTargetType = "topic" | "editor_pick";
export type CommentSortOrder = "popular" | "recent";
/** 1 = like, -1 = dislike, 0 = none */
export type CommentReaction = -1 | 0 | 1;

export interface CommentAuthor {
  id: string;
  display_name: string;
}

export interface Comment {
  id: string;
  target_type: CommentTargetType;
  target_id: string;
  group_id: string;
  parent_id: string | null;
  depth: 0 | 1;
  content: string;
  likes_count: number;
  dislikes_count: number;
  my_reaction: CommentReaction;
  reply_count: number;
  is_deleted: boolean;
  created_at: string;
  edited_at?: string | null;
  author: CommentAuthor;
}

export interface CommentPage {
  items: Comment[];
  next_cursor: string;
}

export interface CommentReactResult {
  likes_count: number;
  dislikes_count: number;
  my_reaction: CommentReaction;
}
