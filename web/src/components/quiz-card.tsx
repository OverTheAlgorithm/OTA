import { useCallback, useEffect, useRef, useState } from "react";
import { submitQuizAnswer, type QuizForUser, type QuizSubmitResult } from "@/lib/api";
import {
  clearPendingAnswer,
  loadPendingAnswer,
  savePendingAnswer,
} from "@/lib/quiz-pending";

/**
 * QuizCard — paced-reveal quiz with hydration and pending-answer flow.
 *
 * State machine (see docs/plans/quiz-ux-redesign-plan.md §3.1):
 *   idle ─click→ selected_waiting_earn ─earn+min→ earn_confirmed
 *     → submitting (min 0.8s + API) → evaluating (0.8s)
 *     → result_correct | result_wrong
 *   submitting ─error→ submit_failed ─retry→ submitting (max 5 retries)
 *
 * Hydration: when `quiz.past_attempt` is non-null, the component skips the entire
 * flow and renders a static result_correct/wrong card with no animations. This
 * lets returning users see their past answer without re-submitting.
 *
 * Non-logged-in users see the quiz and can click options. The selection is
 * persisted via quiz-pending.ts and an `onRequestLogin` callback fires so the
 * page can show the login modal. After OAuth round-trip the selection is
 * restored on mount and the flow continues.
 */

type QuizStage =
  | "idle"
  | "selected_waiting_earn"
  | "earn_confirmed"
  | "submitting"
  | "evaluating"
  | "result_correct"
  | "result_wrong"
  | "submit_failed";

export type { QuizStage };

interface QuizCardProps {
  quiz: QuizForUser | null;
  isLoggedIn: boolean;
  /** True after the topic page has confirmed the earn (point award) succeeded. */
  earnCommitted: boolean;
  contextItemId: string;
  /** Used to scope the localStorage pending-answer key to this topic. */
  topicId: string;
  onCoinsEarned?: (newTotal: number) => void;
  /** Called when a non-logged-in user clicks an option. The page should show the login modal. */
  onRequestLogin?: (selectedIndex: number) => void;
  /** Notifies the page when the quiz enters/leaves a "busy" stage so navigation can be blocked. */
  onStageChange?: (stage: QuizStage) => void;
}

// ── Pacing constants (min display durations) ────────────────────────────────
const MIN_STAGE_A_MS = 600;  // selected_waiting_earn min display before earn-confirmed transition
const STAGE_B_MS = 1000;     // earn_confirmed fixed display
const MIN_STAGE_C_MS = 800;  // submitting min wait (in addition to API round-trip)
const STAGE_D_MS = 800;      // evaluating fixed display
const MAX_RETRIES = 5;       // SUBMIT_FAILED retry budget (4 options + 1 generous)
// Defensive fallback: if Stage A doesn't see earnCommitted within this window,
// the earn flow likely failed (initEarn 5xx, network drop, etc.) and the card
// would otherwise be stuck. 25s = 10s normal countdown + 15s buffer for slow
// mobile networks and the initEarn round-trip.
const STAGE_A_MAX_WAIT_MS = 25000;

const OPTION_MARKERS = ["①", "②", "③", "④"] as const;

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export function QuizCard({
  quiz,
  isLoggedIn,
  earnCommitted,
  contextItemId,
  topicId,
  onCoinsEarned,
  onRequestLogin,
  onStageChange,
}: QuizCardProps) {
  const [stage, setStage] = useState<QuizStage>("idle");
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null);
  const [submitResult, setSubmitResult] = useState<QuizSubmitResult | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [isHydrated, setIsHydrated] = useState(false);
  // True when Stage A timed out before earnCommitted ever became true.
  // Renders the SUBMIT_FAILED branch with a refresh-only message (no retry button).
  const [earnStuck, setEarnStuck] = useState(false);
  // Reactive retry counter so the "다시 시도 (n/5)" label and exhausted gate
  // re-render correctly. (A ref would be technically reset by transitionTo's
  // setStage but couples brittleness to incidental re-renders.)
  const [retryCount, setRetryCount] = useState(0);

  // Refs — survive re-renders. Per-stage timers are owned by their useEffects
  // and cleared via the effect's own cleanup (no central registry needed).
  const stageEnteredAtRef = useRef<number>(0);
  const submitCalledRef = useRef(false);
  const mountedRef = useRef(true);

  // ── Helpers ────────────────────────────────────────────────────────────────

  const transitionTo = useCallback((next: QuizStage) => {
    if (!mountedRef.current) return;
    stageEnteredAtRef.current = Date.now();
    setStage(next);
  }, []);

  // ── Mount: hydration > pending restore > idle ──────────────────────────────

  useEffect(() => {
    mountedRef.current = true;

    // Priority 1: Hydration from past attempt (already submitted).
    if (quiz?.past_attempt) {
      const past = quiz.past_attempt;
      setSelectedIndex(past.selected_index);
      setSubmitResult({
        correct: past.is_correct,
        coins_earned: past.coins_earned,
        total_coins: 0, // not used in hydrated render
      });
      setStage(past.is_correct ? "result_correct" : "result_wrong");
      setIsHydrated(true);
      // Clean up any stale pending so it doesn't interfere with future flows.
      clearPendingAnswer();
      return () => {
        mountedRef.current = false;
      };
    }

    // Priority 2: Pending restore (after OAuth round-trip).
    // Only restore for logged-in users — otherwise the user would enter
    // SELECTED_WAITING_EARN with no countdown coming, hit the 25s timeout, and
    // see "포인트 획득 실패". The pending stays in localStorage (TTL handles
    // expiry) and will be picked up on the next mount that has a logged-in user.
    if (quiz && isLoggedIn) {
      const pending = loadPendingAnswer(topicId);
      if (pending !== null && pending >= 0 && pending < quiz.options.length) {
        setSelectedIndex(pending);
        stageEnteredAtRef.current = Date.now();
        setStage("selected_waiting_earn");
      }
    }

    return () => {
      mountedRef.current = false;
    };
    // Re-run on quiz identity change OR auth state change. The latter is needed
    // so that a non-logged-in user who dismissed the quiz_submit modal and then
    // logs in via another path picks up their pending answer on the next render.
    // Other props (callbacks, contextItemId) are intentionally excluded.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [quiz?.id, isLoggedIn]);

  // ── Notify parent of stage transitions ─────────────────────────────────────

  useEffect(() => {
    onStageChange?.(stage);
  }, [stage, onStageChange]);

  // ── Stage A → B: earn confirmed + min display elapsed ─────────────────────

  useEffect(() => {
    if (isHydrated) return;
    if (stage !== "selected_waiting_earn") return;
    if (!earnCommitted) return;
    if (selectedIndex === null) return;

    const elapsed = Date.now() - stageEnteredAtRef.current;
    const wait = Math.max(0, MIN_STAGE_A_MS - elapsed);
    const t = setTimeout(() => {
      if (!mountedRef.current) return;
      transitionTo("earn_confirmed");
    }, wait);
    return () => clearTimeout(t);
  }, [stage, earnCommitted, selectedIndex, isHydrated, transitionTo]);

  // ── Stage A defensive timeout: bail out if earn never confirms ─────────────
  // Pre-mortem #1 mitigation: if initEarn fails or the countdown never starts,
  // the user would otherwise be permanently stuck in selected_waiting_earn.
  // After STAGE_A_MAX_WAIT_MS we transition to submit_failed with a refresh-only
  // message (no retry button — the underlying earn flow is the issue, not submit).

  useEffect(() => {
    if (isHydrated) return;
    if (stage !== "selected_waiting_earn") return;
    if (earnCommitted) return; // already on the path to earn_confirmed

    const elapsed = Date.now() - stageEnteredAtRef.current;
    const wait = Math.max(0, STAGE_A_MAX_WAIT_MS - elapsed);
    const t = setTimeout(() => {
      if (!mountedRef.current) return;
      setEarnStuck(true);
      transitionTo("submit_failed");
    }, wait);
    return () => clearTimeout(t);
  }, [stage, earnCommitted, isHydrated, transitionTo]);

  // ── Stage B → C: fixed 1.0s display ───────────────────────────────────────

  useEffect(() => {
    if (isHydrated) return;
    if (stage !== "earn_confirmed") return;

    const t = setTimeout(() => {
      if (!mountedRef.current) return;
      transitionTo("submitting");
    }, STAGE_B_MS);
    return () => clearTimeout(t);
  }, [stage, isHydrated, transitionTo]);

  // ── Stage C: submit API call (min 0.8s OR API round-trip, whichever longer) ─

  useEffect(() => {
    if (isHydrated) return;
    if (stage !== "submitting") return;
    if (selectedIndex === null) return;
    if (submitCalledRef.current) return;
    submitCalledRef.current = true;

    setSubmitError(null);

    const run = async () => {
      try {
        const [result] = await Promise.all([
          submitQuizAnswer(contextItemId, selectedIndex),
          delay(MIN_STAGE_C_MS),
        ]);
        if (!mountedRef.current) return;
        setSubmitResult(result);
        clearPendingAnswer();
        if (onCoinsEarned) onCoinsEarned(result.total_coins);
        transitionTo("evaluating");
      } catch {
        if (!mountedRef.current) return;
        // Allow another retry attempt.
        submitCalledRef.current = false;
        setSubmitError("제출에 실패했어요. 다시 시도해주세요.");
        transitionTo("submit_failed");
      }
    };

    void run();
  }, [stage, isHydrated, selectedIndex, contextItemId, onCoinsEarned, transitionTo]);

  // ── Stage D → E: fixed 0.8s evaluating, then result ────────────────────────

  useEffect(() => {
    if (isHydrated) return;
    if (stage !== "evaluating") return;
    if (!submitResult) return;

    const t = setTimeout(() => {
      if (!mountedRef.current) return;
      transitionTo(submitResult.correct ? "result_correct" : "result_wrong");
    }, STAGE_D_MS);
    return () => clearTimeout(t);
  }, [stage, submitResult, isHydrated, transitionTo]);

  // ── Click handlers ─────────────────────────────────────────────────────────

  const handleOptionClick = (index: number) => {
    if (isHydrated) return;
    if (!quiz) return;

    // Non-logged-in: persist selection and ask the page to show the login modal.
    if (!isLoggedIn) {
      savePendingAnswer(topicId, index);
      setSelectedIndex(index);
      onRequestLogin?.(index);
      return;
    }

    // Logged-in: only IDLE and SELECTED_WAITING_EARN allow selection changes.
    if (stage === "idle") {
      setSelectedIndex(index);
      stageEnteredAtRef.current = Date.now();
      // If the earn already happened (user read the article first, then clicked
      // the quiz), skip the "포인트 획득 대기중 / 완료" stages — those messages
      // would be misleading after the fact. Go straight to submitting; the pulse
      // animation on the selected option still gives clear "I picked this" feedback.
      if (earnCommitted) {
        setStage("submitting");
      } else {
        setStage("selected_waiting_earn");
      }
      return;
    }

    if (stage === "selected_waiting_earn") {
      setSelectedIndex(index);
      // Note: don't reset stageEnteredAt — the min display window keeps running
      // from the original click so re-selection doesn't extend the wait.
      return;
    }
    // Other stages: locked, ignore clicks.
  };

  const handleRetry = () => {
    if (retryCount >= MAX_RETRIES) return;
    setRetryCount((c) => c + 1);
    submitCalledRef.current = false;
    setSubmitError(null);
    transitionTo("submitting");
  };

  // ── Early return: no quiz at all ───────────────────────────────────────────

  if (!quiz) return null;

  // ── Render helpers ─────────────────────────────────────────────────────────

  const renderTopMessage = (text: string, anim: "pulse-dot" | "static" = "static") => (
    <p
      className="flex items-center gap-2 text-sm font-semibold text-[#43b9d6] mb-3"
      aria-live="polite"
    >
      {anim === "pulse-dot" && (
        <span
          className="inline-block w-1.5 h-1.5 rounded-full animate-pulse"
          style={{ backgroundColor: "#43b9d6" }}
        />
      )}
      {text}
    </p>
  );

  const renderOptions = (opts: {
    selectedHighlight: "none" | "orange" | "green" | "red" | "wobble" | "pulse";
    interactive: boolean;
    flatNonSelected: boolean;
  }) => (
    <div className="flex flex-col gap-2">
      {quiz.options.map((option, i) => {
        const isSelected = i === selectedIndex;
        const baseClasses =
          "w-full text-left px-4 py-3.5 rounded-xl border-[2px] text-sm font-medium min-h-[48px] transition-all duration-150";

        let classes = baseClasses;
        let extraAnim = "";

        if (isSelected) {
          switch (opts.selectedHighlight) {
            case "orange":
              classes += " border-[#f5a623] bg-[#fffbf2] text-[#231815]";
              break;
            case "green":
              classes += " border-green-500 bg-green-50 text-green-700";
              break;
            case "red":
              classes += " border-red-500 bg-red-50 text-red-600";
              break;
            case "wobble":
              classes += " border-[#f5a623] bg-[#fffbf2] text-[#231815]";
              extraAnim = " wl-anim-wobble";
              break;
            case "pulse":
              classes += " border-[#f5a623] bg-[#fffbf2] text-[#231815]";
              extraAnim = " wl-anim-pulse-soft";
              break;
            case "none":
            default:
              classes += " border-[#f5a623]/40 bg-white text-[#231815]";
              break;
          }
        } else if (opts.flatNonSelected) {
          classes += " border-[#231815]/15 bg-[#231815]/[0.02] text-[#231815]/45";
        } else {
          classes +=
            " border-[#f5a623]/40 bg-white text-[#231815] hover:border-[#f5a623] hover:bg-[#f5a623]/5";
        }

        const markerColor = isSelected && opts.selectedHighlight === "green"
          ? "text-green-600"
          : isSelected && (opts.selectedHighlight === "red")
            ? "text-red-500"
            : "text-[#f5a623]";

        return (
          <button
            key={i}
            type="button"
            onClick={() => opts.interactive && handleOptionClick(i)}
            disabled={!opts.interactive}
            aria-pressed={isSelected}
            aria-disabled={!opts.interactive}
            className={
              classes +
              extraAnim +
              (opts.interactive ? "" : " cursor-default")
            }
          >
            <span className={`font-bold mr-2 ${markerColor}`}>{OPTION_MARKERS[i]}</span>
            {option}
          </button>
        );
      })}
    </div>
  );

  // ── Stage-specific render branches ─────────────────────────────────────────

  // 1) Result stages (handles both hydration and fresh submissions)
  if (stage === "result_correct" || stage === "result_wrong") {
    const isCorrect = stage === "result_correct";
    const accentBg = isCorrect ? "bg-green-50" : "bg-red-50";
    const accentBorder = isCorrect ? "border-green-300" : "border-red-300";
    const badgeClasses = isCorrect
      ? "bg-green-100 text-green-700"
      : "bg-red-100 text-red-600";
    const badgeLabel = isCorrect ? "정답" : "오답";

    const headerLabel = isHydrated ? "이미 완료했어요" : "보너스 퀴즈";
    const headerColor = isHydrated ? "text-[#231815]/60" : "text-[#231815]";
    // Shake the whole card once for fresh wrong answers — skip on hydration
    // so the static "이미 완료했어요" view stays calm.
    const animateShake = stage === "result_wrong" && !isHydrated;

    return (
      <div
        className={`rounded-2xl border-[2px] ${accentBorder} ${accentBg} px-6 py-5 mb-8 transition-all duration-300${
          animateShake ? " wl-anim-shake" : ""
        }`}
      >
        <div className="flex items-center gap-2 mb-3 flex-wrap">
          <span className={`text-base font-bold ${headerColor}`}>{headerLabel}</span>
          <span
            className={`px-2 py-0.5 rounded-full text-[10px] font-bold whitespace-nowrap flex-shrink-0 ${badgeClasses}`}
          >
            {badgeLabel}
          </span>
        </div>

        {/* Question for context */}
        <p className="text-base font-semibold text-[#231815] leading-snug mb-4">
          {quiz.question}
        </p>

        {/* Coin reward — only for fresh correct submission */}
        {isCorrect && !isHydrated && submitResult && submitResult.coins_earned > 0 && (
          <div className="mb-4 flex items-center gap-2 px-4 py-2.5 rounded-xl bg-green-100/60 border border-green-200">
            <span className="text-green-700 font-bold text-sm wl-anim-coin-pop inline-block">
              +{submitResult.coins_earned} 보너스!
            </span>
            <span className="text-xs text-green-700/80">포인트가 적립되었습니다</span>
          </div>
        )}

        {/* Wrong message — only for fresh wrong submission */}
        {!isCorrect && !isHydrated && (
          <div className="mb-4 px-4 py-2.5 rounded-xl bg-red-100/60 border border-red-200">
            <p className="text-sm text-red-700">아쉽지만 틀렸어요.</p>
          </div>
        )}

        {renderOptions({
          selectedHighlight: isCorrect ? "green" : "red",
          interactive: false,
          flatNonSelected: true,
        })}
      </div>
    );
  }

  // 2) submit_failed (covers both submit network failure and Stage A earn timeout)
  if (stage === "submit_failed") {
    const exhausted = retryCount >= MAX_RETRIES;
    const showRetry = !earnStuck && !exhausted;
    const badgeLabel = earnStuck ? "포인트 획득 실패" : "제출 실패";
    let message: string;
    if (earnStuck) {
      message = "포인트 획득에 실패했어요. 페이지를 새로고침해주세요 🙏";
    } else if (exhausted) {
      message = "여러 번 시도했지만 제출이 안 돼요. 잠시 후 다시 방문해주세요 🙏";
    } else {
      message = submitError ?? "제출에 실패했어요. 다시 시도해주세요.";
    }
    return (
      <div className="rounded-2xl border-[2px] border-red-300 bg-red-50 px-6 py-5 mb-8 transition-all duration-300">
        <div className="flex items-center gap-2 mb-3">
          <span className="text-base font-bold text-[#231815]">보너스 퀴즈</span>
          <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-red-100 text-red-600 whitespace-nowrap">
            {badgeLabel}
          </span>
        </div>
        <p className="text-base font-semibold text-[#231815] leading-snug mb-4">
          {quiz.question}
        </p>
        <p className="text-sm text-red-700 mb-4" aria-live="polite">
          {message}
        </p>
        {showRetry && (
          <button
            type="button"
            onClick={handleRetry}
            className="w-full px-4 py-3 rounded-xl border-[2px] border-red-400 bg-white text-sm font-semibold text-red-600 hover:bg-red-50 transition-colors min-h-[48px]"
          >
            다시 시도 ({retryCount}/{MAX_RETRIES})
          </button>
        )}
        {renderOptions({
          selectedHighlight: "orange",
          interactive: false,
          flatNonSelected: true,
        })}
      </div>
    );
  }

  // 3) submitting
  if (stage === "submitting") {
    return (
      <div className="rounded-2xl border-[2px] border-[#f5a623] bg-[#fffbf2] px-6 py-5 mb-8 transition-all duration-300">
        <div className="flex items-center gap-2 mb-3">
          <span className="text-base font-bold text-[#f5a623]">보너스 퀴즈</span>
        </div>
        {renderTopMessage("퀴즈 정답 제출 중...", "pulse-dot")}
        <p className="text-base font-semibold text-[#231815] leading-snug mb-4">
          {quiz.question}
        </p>
        {renderOptions({
          selectedHighlight: "pulse",
          interactive: false,
          flatNonSelected: true,
        })}
      </div>
    );
  }

  // 4) evaluating
  if (stage === "evaluating") {
    return (
      <div className="rounded-2xl border-[2px] border-[#f5a623] bg-[#fffbf2] px-6 py-5 mb-8 transition-all duration-300">
        <div className="flex items-center gap-2 mb-3">
          <span className="text-base font-bold text-[#f5a623]">보너스 퀴즈</span>
        </div>
        {renderTopMessage("정답 검사 중...", "pulse-dot")}
        <p className="text-base font-semibold text-[#231815] leading-snug mb-4">
          {quiz.question}
        </p>
        {renderOptions({
          selectedHighlight: "wobble",
          interactive: false,
          flatNonSelected: true,
        })}
      </div>
    );
  }

  // 5) earn_confirmed
  if (stage === "earn_confirmed") {
    return (
      <div className="rounded-2xl border-[2px] border-[#f5a623] bg-[#fffbf2] px-6 py-5 mb-8 transition-all duration-300">
        <div className="flex items-center gap-2 mb-3">
          <span className="text-base font-bold text-[#f5a623]">보너스 퀴즈</span>
        </div>
        <p
          className="flex items-center gap-2 text-sm font-bold text-[#43b9d6] mb-3 wl-anim-coin-pop"
          aria-live="polite"
        >
          <span aria-hidden="true">🪙</span>
          포인트 획득 완료!
        </p>
        <p className="text-base font-semibold text-[#231815] leading-snug mb-4">
          {quiz.question}
        </p>
        {renderOptions({
          selectedHighlight: "orange",
          interactive: false,
          flatNonSelected: true,
        })}
      </div>
    );
  }

  // 6) selected_waiting_earn
  if (stage === "selected_waiting_earn") {
    return (
      <div className="rounded-2xl border-[2px] border-[#f5a623] bg-[#fffbf2] px-6 py-5 mb-8 transition-all duration-300">
        <div className="flex items-center gap-2 mb-3">
          <span className="text-base font-bold text-[#f5a623]">보너스 퀴즈</span>
        </div>
        {renderTopMessage("포인트 획득 대기중...", "pulse-dot")}
        <p className="text-base font-semibold text-[#231815] leading-snug mb-4">
          {quiz.question}
        </p>
        {renderOptions({
          selectedHighlight: "orange",
          interactive: true, // user can still change selection while waiting
          flatNonSelected: false,
        })}
      </div>
    );
  }

  // 7) idle (default)
  const idleBadgeText = isLoggedIn ? "정답 맞히면 보너스 포인트!" : "로그인하고 도전해보세요";

  return (
    <div className="rounded-2xl border-[2px] border-[#f5a623] bg-[#fffbf2] px-6 py-5 mb-8 transition-all duration-300">
      <div className="flex items-center gap-2 mb-3 flex-wrap">
        <span className="text-base font-bold text-[#f5a623]">보너스 퀴즈</span>
        <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-[#f5a623]/15 text-[#f5a623] whitespace-nowrap flex-shrink-0">
          {idleBadgeText}
        </span>
      </div>
      <p className="text-base font-semibold text-[#231815] leading-snug mb-4">
        {quiz.question}
      </p>
      {renderOptions({
        selectedHighlight: "none",
        interactive: true,
        flatNonSelected: false,
      })}
    </div>
  );
}
