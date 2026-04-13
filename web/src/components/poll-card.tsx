import { useState } from "react";
import { submitPollVote, type PollForUser } from "@/lib/api";
import { PollEditModal } from "@/components/poll-edit-modal";

interface PollCardProps {
  poll: PollForUser;
  isLoggedIn: boolean;
  isAdmin: boolean;
  /** Invoked when a non-logged-in user clicks an option so the page can open the login modal. */
  onRequestLogin?: () => void;
}

export function PollCard({ poll: initial, isLoggedIn, isAdmin, onRequestLogin }: PollCardProps) {
  const [poll, setPoll] = useState<PollForUser | null>(initial);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);

  if (!poll) return null; // admin deleted in this session

  const hasVoted = poll.user_vote_index !== null;
  const showResults = hasVoted || !isLoggedIn;

  async function handleSelect(idx: number) {
    if (!poll || submitting || hasVoted) return;
    if (!isLoggedIn) {
      onRequestLogin?.();
      return;
    }
    const prev = poll;
    // Optimistic bump of selected option count.
    const optimistic: PollForUser = {
      ...poll,
      user_vote_index: idx,
      total_votes: poll.total_votes + 1,
      tallies: poll.tallies.map((t) =>
        t.option_index === idx ? { ...t, count: t.count + 1 } : t
      ),
    };
    setPoll(optimistic);
    setSubmitting(true);
    setError(null);
    try {
      const refreshed = await submitPollVote(poll.context_item_id, idx);
      setPoll(refreshed);
    } catch (e) {
      setPoll(prev);
      setError("투표 전송에 실패했어요. 다시 시도해주세요.");
      console.warn("poll vote failed", e);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <section className="mb-10 rounded-2xl border border-[#231815]/20 px-6 py-5">
      <div className="mb-2 flex items-center justify-between">
        <p className="text-xs font-bold tracking-wide text-[#43b9d6]">의견 투표</p>
        {isAdmin && (
          <button
            type="button"
            onClick={() => setEditing(true)}
            className="text-xs font-medium text-[#231815]/60 underline hover:text-[#231815]"
          >
            수정
          </button>
        )}
      </div>
      <p className="mb-4 text-base font-bold text-[#231815]">{poll.question}</p>

      {!showResults ? (
        <ul className="flex flex-col gap-2">
          {poll.options.map((opt, idx) => (
            <li key={idx}>
              <button
                type="button"
                disabled={submitting}
                onClick={() => handleSelect(idx)}
                className="w-full rounded-xl border border-[#231815]/20 px-4 py-3 text-left text-sm font-medium text-[#231815] transition-colors hover:border-[#43b9d6] hover:bg-[#43b9d6]/5 disabled:opacity-60"
              >
                {opt}
              </button>
            </li>
          ))}
        </ul>
      ) : (
        <div className="flex flex-col gap-3">
          {poll.options.map((opt, idx) => {
            const count = poll.tallies[idx]?.count ?? 0;
            const pct = poll.total_votes === 0 ? 0 : Math.round((count / poll.total_votes) * 100);
            const isMine = poll.user_vote_index === idx;
            return (
              <div key={idx} className="flex flex-col gap-1">
                <div className="flex items-baseline justify-between text-sm">
                  <span className={`font-medium ${isMine ? "text-[#43b9d6]" : "text-[#231815]"}`}>
                    {isMine ? "● " : ""}
                    {opt}
                  </span>
                  <span className="tabular-nums text-[#231815]/60">
                    {pct}% ({count})
                  </span>
                </div>
                <div className="h-2.5 w-full rounded-full bg-[#231815]/10">
                  <div
                    className={`h-2.5 rounded-full transition-[width] duration-500 ease-out ${
                      isMine ? "bg-[#43b9d6]" : "bg-[#231815]/40"
                    }`}
                    style={{ width: `${pct}%` }}
                  />
                </div>
              </div>
            );
          })}
          <div className="mt-1 flex items-center justify-between text-xs text-[#231815]/60">
            <span>총 {poll.total_votes.toLocaleString()}명 참여</span>
            {!isLoggedIn && <span>로그인하면 투표할 수 있어요</span>}
          </div>
        </div>
      )}

      {error && <p className="mt-3 text-xs text-red-500">{error}</p>}

      {editing && (
        <PollEditModal
          poll={poll}
          onClose={() => setEditing(false)}
          onSaved={({ question, options }) => {
            const sameOpts =
              options.length === poll.options.length &&
              options.every((o, i) => o === poll.options[i]);
            setPoll({
              ...poll,
              question,
              options,
              tallies: sameOpts
                ? poll.tallies
                : options.map((_, i) => ({ option_index: i, count: 0 })),
              total_votes: sameOpts ? poll.total_votes : 0,
              user_vote_index: sameOpts ? poll.user_vote_index : null,
            });
            setEditing(false);
          }}
          onDeleted={() => {
            setPoll(null);
            setEditing(false);
          }}
        />
      )}
    </section>
  );
}
