/**
 * Pending quiz answer storage.
 *
 * Used to preserve a non-logged-in user's selected option across the Kakao OAuth
 * redirect. When the user returns and the topic page mounts, QuizCard reads the
 * pending selection and enters the SELECTED_WAITING_EARN stage automatically.
 *
 * Storage is keyed by topic_id so a stale pending from a different topic is ignored.
 * TTL is 10 minutes — generous enough for a normal login flow, short enough to avoid
 * surprising the user days later.
 *
 * All operations swallow errors via try/catch so storage quota issues (Safari private
 * mode, full quota) never crash the page. The worst case is "pending lost" which
 * degrades gracefully to the IDLE state.
 */

const STORAGE_KEY = "wl_quiz_pending_answer_v1";
const TTL_MS = 10 * 60 * 1000; // 10 minutes

interface PendingAnswer {
  topic_id: string;
  selected_index: number;
  saved_at: number;
}

/** Save the pending selection. No-op on storage failure. */
export function savePendingAnswer(topicId: string, selectedIndex: number): void {
  try {
    const payload: PendingAnswer = {
      topic_id: topicId,
      selected_index: selectedIndex,
      saved_at: Date.now(),
    };
    localStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
  } catch {
    // Quota exceeded or storage unavailable — degrade gracefully.
  }
}

/**
 * Load the pending selection for a specific topic.
 * Returns null when:
 *   - no pending exists
 *   - pending is for a different topic
 *   - pending has expired (TTL)
 *   - the stored payload is malformed
 *
 * Stale or invalid entries are auto-cleared as a side effect.
 */
export function loadPendingAnswer(topicId: string): number | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;

    const parsed = JSON.parse(raw) as Partial<PendingAnswer> | null;
    if (
      !parsed ||
      typeof parsed.topic_id !== "string" ||
      typeof parsed.selected_index !== "number" ||
      typeof parsed.saved_at !== "number"
    ) {
      clearPendingAnswer();
      return null;
    }

    if (parsed.topic_id !== topicId) {
      // Different topic — leave it alone (the other topic's tab may still need it
      // if the user navigates there). Just return null for this topic.
      return null;
    }

    if (Date.now() - parsed.saved_at > TTL_MS) {
      clearPendingAnswer();
      return null;
    }

    return parsed.selected_index;
  } catch {
    clearPendingAnswer();
    return null;
  }
}

/** Remove the pending selection from storage. No-op on failure. */
export function clearPendingAnswer(): void {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch {
    // Ignore.
  }
}
