// Extracted from: web/src/lib/api.ts — ScheduledPush, CreateScheduledPushRequest, UpdateScheduledPushRequest
// Shared across web and mobile

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
