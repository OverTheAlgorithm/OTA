// Web-only API client for the community-trend admin surface. Kept separate from
// the shared cross-platform client (@wizletter/shared) because this admin tool
// is web-only — no need to bloat the mobile/shared bundle.
//
// All requests are cookie-authenticated (credentials:"include") and hit the
// /api/v1/admin/community-trend/* endpoints. Responses use the {data} / {error}
// envelope.

const API_BASE = import.meta.env.VITE_API_URL || "";
const PREFIX = `${API_BASE}/api/v1/admin/community-trend`;

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${PREFIX}${path}`, {
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error((body as { error?: string }).error || `요청 실패 (${res.status})`);
  }
  return (body as { data: T }).data;
}

// ── types ───────────────────────────────────────────────────────────────────

export interface CTCommunity {
  id: number;
  key: string;
  name: string;
  home_url: string;
  enabled: boolean;
  meta_tag_ids: number[] | null;
}

export interface CTAxis {
  id: number;
  key: string;
  label: string;
  display_order: number;
  type: "meta" | "topic";
}

export interface CTTag {
  id: number;
  axis_id: number;
  name: string;
  description: string;
  created_by: string;
}

export interface CTWorksheet {
  id: number;
  community_id: number;
  stat_date: string;
  mode: string;
  status: string;
  total_posts: number | null;
}

export interface CTTagSuggestion {
  tag_id: number;
  name: string;
  axis_key: string;
  count: number;
  is_new: boolean;
  new_axis_key: string;
}

export interface CTSuggestion {
  output: {
    tags: CTTagSuggestion[] | null;
    meme_matches: { meme_id: number; name: string; count: number }[] | null;
    meme_candidates: { expression: string; hit_count: number }[] | null;
  };
  total_posts: number;
}

export interface CTTrendPoint {
  stat_date: string;
  post_count: number;
}

export interface CTTagTrend {
  tag_id: number;
  tag_name: string;
  axis_key: string;
  points: CTTrendPoint[];
  latest: number;
  delta_prev_day: number;
  delta_prev_week: number;
}

export interface CTMeme {
  id: number;
  name: string;
  aliases: string[];
  status: string;
  created_via: string;
}

export interface CTMemeCandidate {
  id: number;
  expression: string;
  hit_count: number;
  first_seen: string;
  last_seen: string;
}

export interface CTTagCount {
  tag_id: number;
  count: number;
}

// ── communities / taxonomy ───────────────────────────────────────────────────

export const listCommunities = () => req<CTCommunity[]>("/communities");
export const createCommunity = (key: string, name: string, home_url: string) =>
  req<CTCommunity>("/communities", { method: "POST", body: JSON.stringify({ key, name, home_url }) });
export const updateCommunity = (id: number, name: string, home_url: string, enabled: boolean) =>
  req<CTCommunity>(`/communities/${id}`, { method: "PATCH", body: JSON.stringify({ name, home_url, enabled }) });
export const setMetaTags = (id: number, tag_ids: number[]) =>
  req<{ message: string }>(`/communities/${id}/meta-tags`, { method: "PUT", body: JSON.stringify({ tag_ids }) });

export const listAxes = () => req<CTAxis[]>("/axes");
export const createAxis = (key: string, label: string, display_order: number, type: string) =>
  req<CTAxis>("/axes", { method: "POST", body: JSON.stringify({ key, label, display_order, type }) });
export const listTags = (axisId?: number) =>
  req<CTTag[]>(`/tags${axisId ? `?axis_id=${axisId}` : ""}`);
export const createTag = (axis_id: number, name: string, description: string) =>
  req<CTTag>("/tags", { method: "POST", body: JSON.stringify({ axis_id, name, description }) });
export const updateTag = (id: number, name: string, description: string) =>
  req<CTTag>(`/tags/${id}`, { method: "PATCH", body: JSON.stringify({ name, description }) });
export const deleteTag = (id: number) =>
  req<{ message: string }>(`/tags/${id}`, { method: "DELETE" });

// ── worksheets ────────────────────────────────────────────────────────────────

export const listWorksheets = (date: string) =>
  req<CTWorksheet[]>(`/worksheets?date=${date}`);
export const getSuggestion = (communityId: number, date: string) =>
  req<CTSuggestion | null>(`/worksheets/suggestion?community_id=${communityId}&date=${date}`);
export const confirmWorksheet = (payload: {
  community_id: number;
  stat_date: string;
  mode: string;
  source: string;
  total_posts: number;
  counts: CTTagCount[];
}) => req<{ message: string }>("/worksheets/confirm", { method: "POST", body: JSON.stringify(payload) });

// ── trends ────────────────────────────────────────────────────────────────────

export const communityTrends = (communityId: number, from: string, to: string) =>
  req<CTTagTrend[]>(`/trends/community?community_id=${communityId}&from=${from}&to=${to}`);
export const cohortTrends = (metaTagId: number, from: string, to: string) =>
  req<CTTagTrend[]>(`/trends/cohort?meta_tag_id=${metaTagId}&from=${from}&to=${to}`);

// ── memes ─────────────────────────────────────────────────────────────────────

export const listMemes = (includeRetired = false) =>
  req<CTMeme[]>(`/memes${includeRetired ? "?include_retired=true" : ""}`);
export const createMeme = (name: string, aliases: string[]) =>
  req<CTMeme>("/memes", { method: "POST", body: JSON.stringify({ name, aliases }) });
export const updateMeme = (id: number, name: string, aliases: string[]) =>
  req<CTMeme>(`/memes/${id}`, { method: "PATCH", body: JSON.stringify({ name, aliases }) });
export const retireMeme = (id: number) =>
  req<{ message: string }>(`/memes/${id}`, { method: "DELETE" });
export const listMemeCandidates = () => req<CTMemeCandidate[]>("/meme-candidates");
export const promoteMemeCandidate = (id: number, name: string, aliases: string[]) =>
  req<CTMeme>(`/meme-candidates/${id}/promote`, { method: "POST", body: JSON.stringify({ name, aliases }) });
export const rejectMemeCandidate = (id: number) =>
  req<{ message: string }>(`/meme-candidates/${id}`, { method: "DELETE" });

export interface CTRobotsStatus {
  community_id: number;
  community_key: string;
  community_name: string;
  checked_at: string;
  allowed: boolean;
  snapshot_hash: string;
  note: string;
}

export interface CTRobotsTransition {
  id: number;
  community_id: number;
  community_name: string;
  from_allowed: boolean | null;
  to_allowed: boolean;
  changed_at: string;
}

export interface CTRobotsData {
  status: CTRobotsStatus[];
  transitions: CTRobotsTransition[];
}

export const listRobotsStatus = () => req<CTRobotsData>("/robots-status");

export interface CTCommunityResult {
  key: string;
  mode: string;
  status: string;
  reason: string;
}

export const triggerCollect = (date?: string) =>
  req<CTCommunityResult[]>("/collect", {
    method: "POST",
    body: JSON.stringify({ date }),
  });
