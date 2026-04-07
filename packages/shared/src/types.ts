// Extracted from: web/src/lib/api.ts

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
